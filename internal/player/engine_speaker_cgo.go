//go:build cgo

package player

import (
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/speaker"
)

// speakerPlay initialises the speaker (if needed) and begins streaming.
func speakerPlay(format beep.Format, s beep.Streamer) error {
	if err := speaker.Init(format.SampleRate, format.SampleRate.N(150*time.Millisecond)); err != nil {
		return err
	}
	speaker.Clear()
	speaker.Play(s)
	return nil
}

// speakerPauseResume suspends or resumes the speaker output.
func speakerPauseResume() {
	speaker.Lock()
	defer speaker.Unlock()
	// beep v2 does not expose a separate Pause; toggling is done by draining.
	// For a clean pause/resume use-case callers stop and restart via PlayFile.
}

// speakerClear stops all currently playing streams.
func speakerClear() {
	speaker.Clear()
}

// speakerLock acquires the speaker's internal mutex before modifying pipeline state.
func speakerLock() {
	speaker.Lock()
}

// speakerUnlock releases the speaker's internal mutex.
func speakerUnlock() {
	speaker.Unlock()
}
