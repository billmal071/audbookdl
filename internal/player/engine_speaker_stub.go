//go:build !cgo

package player

import (
	"os/exec"
	"sync"

	"github.com/gopxl/beep/v2"
)

// Stub speaker that delegates to mpv or ffplay when CGO is disabled.

var (
	extPlayer   *exec.Cmd
	extPlayerMu sync.Mutex
	extPaused   bool
)

func speakerPlay(format beep.Format, streamer beep.Streamer) error {
	// Actual playback is handled by playExternal in player.go
	// This just signals success so the engine state is tracked.
	return nil
}

func speakerPauseResume() {
	extPlayerMu.Lock()
	defer extPlayerMu.Unlock()
	extPaused = !extPaused
	// Can't pause mpv via this interface — handled at player level
}

func speakerClear() {
	extPlayerMu.Lock()
	defer extPlayerMu.Unlock()
	if extPlayer != nil && extPlayer.Process != nil {
		extPlayer.Process.Kill()
		extPlayer.Wait()
		extPlayer = nil
	}
}

func speakerLock()   {}
func speakerUnlock() {}

// playExternal launches mpv or ffplay for the given file.
// Called from Player.Play() on non-CGO builds.
func playExternal(filePath string) error {
	extPlayerMu.Lock()
	defer extPlayerMu.Unlock()

	// Kill any existing player
	if extPlayer != nil && extPlayer.Process != nil {
		extPlayer.Process.Kill()
		extPlayer.Wait()
		extPlayer = nil
	}

	// Try mpv first
	if mpv, err := exec.LookPath("mpv"); err == nil {
		extPlayer = exec.Command(mpv, "--no-video", "--really-quiet", filePath)
		return extPlayer.Start()
	}

	// Try ffplay
	if ffplay, err := exec.LookPath("ffplay"); err == nil {
		extPlayer = exec.Command(ffplay, "-nodisp", "-autoexit", filePath)
		return extPlayer.Start()
	}

	return nil // No player available, degrade silently
}

// stopExternal kills the external player process.
func stopExternal() {
	speakerClear()
}
