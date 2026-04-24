//go:build !cgo

package player

import "github.com/gopxl/beep/v2"

// Stub implementations used when CGO is disabled (no ALSA/CoreAudio available).
// The player state is fully tracked; only actual audio output is absent.

func speakerPlay(_ beep.Format, _ beep.Streamer) error { return nil }
func speakerPauseResume()                              {}
func speakerClear()                                    {}
func speakerLock()                                     {}
func speakerUnlock()                                   {}
