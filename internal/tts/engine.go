// internal/tts/engine.go
package tts

import "context"

// Engine synthesizes text into audio.
type Engine interface {
	Name() string
	Synthesize(ctx context.Context, text string, opts SynthOptions) ([]byte, error)
	ListVoices(ctx context.Context) ([]Voice, error)
}

// SynthOptions configures speech synthesis.
type SynthOptions struct {
	Voice  string // e.g., "en-US-AriaNeural"
	Rate   string // e.g., "+20%", "-10%"
	Volume string // e.g., "+0%"
	Format string // "audio-24khz-48kbitrate-mono-mp3"
}

// Voice describes an available TTS voice.
type Voice struct {
	ID       string // "en-US-AriaNeural"
	Name     string // "Aria"
	Language string // "en-US"
	Gender   string // "Female"
}

// DefaultSynthOptions returns sensible defaults for Edge TTS.
func DefaultSynthOptions() SynthOptions {
	return SynthOptions{
		Voice:  "en-US-AriaNeural",
		Rate:   "+0%",
		Volume: "+0%",
		Format: "audio-24khz-48kbitrate-mono-mp3",
	}
}
