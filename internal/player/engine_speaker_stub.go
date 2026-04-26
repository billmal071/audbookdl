//go:build !cgo

package player

import (
	"github.com/gopxl/beep/v2"
)

func speakerPlay(format beep.Format, streamer beep.Streamer) error { return nil }
func speakerPauseResume() {}
func speakerClear()       {}
func speakerLock()        {}
func speakerUnlock()      {}

// playExternal is a no-op. Audio is handled by MpvController.
func playExternal(filePath string) error { return nil }

// stopExternal is a no-op. Audio is handled by MpvController.
func stopExternal() {}
