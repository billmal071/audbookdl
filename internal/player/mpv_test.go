package player

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"
)

func hasMpv() bool {
	_, err := exec.LookPath("mpv")
	return err == nil
}

func TestNewMpvController_NoMpv(t *testing.T) {
	// This test verifies the constructor doesn't panic.
	// If mpv is installed, it should return a non-nil controller with a socket path.
	// If mpv is not installed, it should return nil.
	ctrl := NewMpvController()
	if ctrl != nil {
		if ctrl.socketPath == "" {
			t.Error("expected socketPath to be set")
		}
	}
}

func TestMpvController_SendCommand_NotConnected(t *testing.T) {
	ctrl := &MpvController{
		responses: make(map[int64]chan json.RawMessage),
	}
	_, err := ctrl.sendCommand("get_property", "time-pos")
	if err == nil {
		t.Error("expected error when not connected, got nil")
	}
}

func TestMpvController_StopWhenNotRunning(t *testing.T) {
	ctrl := &MpvController{
		responses: make(map[int64]chan json.RawMessage),
	}
	// Should not panic
	ctrl.Stop()
}

func TestMpvController_IsRunning_Default(t *testing.T) {
	ctrl := &MpvController{
		responses: make(map[int64]chan json.RawMessage),
	}
	if ctrl.IsRunning() {
		t.Error("expected IsRunning to be false by default")
	}
}

func TestMpvController_Integration(t *testing.T) {
	if !hasMpv() {
		t.Skip("mpv not installed, skipping integration test")
	}

	// Look for audio files to test with
	patterns := []string{
		"/tmp/*.mp3",
		"/tmp/*.m4a",
		"/tmp/*.ogg",
		"/tmp/*.wav",
		"/tmp/*.flac",
	}

	var audioFile string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			audioFile = matches[0]
			break
		}
	}

	if audioFile == "" {
		t.Skip("no audio files found for integration test")
	}

	ctrl := NewMpvController()
	if ctrl == nil {
		t.Fatal("expected non-nil controller")
	}

	if err := ctrl.Start(audioFile, 0); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer ctrl.Stop()

	if !ctrl.IsRunning() {
		t.Error("expected IsRunning to be true after Start")
	}

	// Test GetDuration
	dur, err := ctrl.GetDuration()
	if err != nil {
		t.Errorf("GetDuration failed: %v", err)
	} else if dur <= 0 {
		t.Errorf("expected positive duration, got %d", dur)
	}

	// Test GetPosition
	pos, err := ctrl.GetPosition()
	if err != nil {
		t.Errorf("GetPosition failed: %v", err)
	} else if pos < 0 {
		t.Errorf("expected non-negative position, got %d", pos)
	}

	// Test Pause/Resume
	if err := ctrl.Pause(); err != nil {
		t.Errorf("Pause failed: %v", err)
	}
	if err := ctrl.Resume(); err != nil {
		t.Errorf("Resume failed: %v", err)
	}

	// Test SetSpeed
	if err := ctrl.SetSpeed(1.5); err != nil {
		t.Errorf("SetSpeed failed: %v", err)
	}

	// Test SetVolume
	if err := ctrl.SetVolume(0.5); err != nil {
		t.Errorf("SetVolume failed: %v", err)
	}

	// Test Seek
	if err := ctrl.Seek(1000); err != nil {
		t.Errorf("Seek failed: %v", err)
	}

	ctrl.Stop()

	if ctrl.IsRunning() {
		t.Error("expected IsRunning to be false after Stop")
	}
}
