package player

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/mp3"
)

// Engine handles actual audio I/O for the player.
//
// It uses github.com/gopxl/beep for MP3 decoding and volume/speed control.
// The speaker backend requires CGO (ALSA on Linux), so it is conditionally
// compiled via engine_speaker.go (CGO) and engine_speaker_stub.go (no CGO).
// The Engine itself is always available; it degrades gracefully when no audio
// hardware is present or CGO is disabled.
type Engine struct {
	mu       sync.Mutex
	playing  bool
	filePath string

	// beep pipeline components (nil when no file is open)
	file     *os.File
	streamer beep.StreamSeekCloser
	format   beep.Format

	// volume wraps streamer and controls gain
	volume *effects.Volume

	// resampled wraps volume and adjusts playback speed
	resampled *beep.Resampler

	// speed is the requested playback rate (1.0 = normal)
	speed float64

	// volumeLevel is the requested gain (0.0 – 1.0, mapped to beep's [-5, 0] dB range)
	volumeLevel float64

	// samplesPlayed tracks how many samples have been sent to the speaker
	samplesPlayed int64

	// sampleRate is the format's sample rate (used for position calculation)
	sampleRate beep.SampleRate
}

// NewEngine creates an audio engine with sensible defaults.
func NewEngine() *Engine {
	return &Engine{
		speed:       1.0,
		volumeLevel: 0.8,
	}
}

// PlayFile opens the MP3 at filePath, seeks to positionMS, and begins streaming.
// If audio hardware is unavailable the file is still opened for metadata.
func (e *Engine) PlayFile(filePath string, positionMS int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Verify the file exists before attempting anything else.
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("audio file not found: %w", err)
	}

	// Close any previously open stream.
	e.closeLocked()

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open audio file: %w", err)
	}

	streamer, format, err := mp3.Decode(f)
	if err != nil {
		f.Close()
		return fmt.Errorf("decode mp3: %w", err)
	}

	// Seek to the requested position.
	if positionMS > 0 {
		targetSample := format.SampleRate.N(time.Duration(positionMS) * time.Millisecond)
		if seekErr := streamer.Seek(targetSample); seekErr != nil {
			// Non-fatal: just start from the beginning.
			_ = seekErr
		}
	}

	// Build the beep pipeline: streamer → volume → resampler.
	vol := &effects.Volume{
		Streamer: streamer,
		Base:     2,
		Volume:   volumeGain(e.volumeLevel),
		Silent:   false,
	}

	ratio := e.speed
	if ratio <= 0 {
		ratio = 1.0
	}
	resampled := beep.ResampleRatio(4, ratio, vol)

	e.file = f
	e.streamer = streamer
	e.format = format
	e.volume = vol
	e.resampled = resampled
	e.sampleRate = format.SampleRate
	e.filePath = filePath
	e.playing = true

	// Delegate actual speaker output to the platform-specific implementation.
	// On CGO builds this initialises the ALSA/CoreAudio speaker; on no-CGO
	// builds this is a no-op that returns nil.
	if err := speakerPlay(format, resampled); err != nil {
		// Degrade gracefully: state is still tracked, no audio output.
		e.playing = false
		return fmt.Errorf("speaker init: %w", err)
	}

	return nil
}

// PauseResume toggles the pause state of the speaker.
func (e *Engine) PauseResume() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.playing = !e.playing
	speakerPauseResume()
}

// Stop stops playback and releases all resources.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.playing = false
	speakerClear()
	e.closeLocked()
}

// IsPlaying reports whether the engine believes audio is playing.
func (e *Engine) IsPlaying() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.playing
}

// Position returns the estimated current playback position based on samples played.
func (e *Engine) Position() time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.sampleRate == 0 {
		return 0
	}
	return e.sampleRate.D(int(e.samplesPlayed))
}

// SetSpeed adjusts the playback speed via the resampler.
// The change takes effect on the next PlayFile call if the resampler is not yet active.
func (e *Engine) SetSpeed(speed float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if speed <= 0 {
		speed = 1.0
	}
	e.speed = speed

	if e.resampled != nil {
		speakerLock()
		e.resampled.SetRatio(speed)
		speakerUnlock()
	}
}

// SetVolume adjusts the volume gain (0.0 = silent, 1.0 = full).
func (e *Engine) SetVolume(vol float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if vol < 0 {
		vol = 0
	} else if vol > 1 {
		vol = 1
	}
	e.volumeLevel = vol

	if e.volume != nil {
		speakerLock()
		e.volume.Volume = volumeGain(vol)
		e.volume.Silent = vol == 0
		speakerUnlock()
	}
}

// Close releases all audio resources.
func (e *Engine) Close() {
	e.Stop()
}

// closeLocked tears down the open stream without acquiring the lock.
// Caller must hold e.mu.
func (e *Engine) closeLocked() {
	if e.streamer != nil {
		e.streamer.Close()
		e.streamer = nil
	}
	if e.file != nil {
		e.file.Close()
		e.file = nil
	}
	e.volume = nil
	e.resampled = nil
	e.samplesPlayed = 0
	e.sampleRate = 0
}

// volumeGain maps a linear 0.0–1.0 volume level to the dB gain expected by
// effects.Volume (which uses base-2 exponentiation: gain = base^Volume).
// At vol=1.0 → 0 dB (unity); at vol=0.5 → -1 dB; at vol=0.0 → -5 dB (near silence).
func volumeGain(vol float64) float64 {
	// Map [0, 1] → [-5, 0]: full silence at 0, unity at 1.
	return (vol - 1.0) * 5.0
}
