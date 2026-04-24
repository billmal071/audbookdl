package player

import (
	"testing"
	"time"
)

func TestEngine_NewEngine(t *testing.T) {
	e := NewEngine()
	if e.IsPlaying() {
		t.Error("new engine should not be playing")
	}
}

func TestEngine_PlayFile_NotFound(t *testing.T) {
	e := NewEngine()
	err := e.PlayFile("/nonexistent/file.mp3", 0)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestEngine_PauseResume(t *testing.T) {
	e := NewEngine()
	e.playing = true
	e.PauseResume()
	if e.playing {
		t.Error("should be paused after PauseResume")
	}
	e.PauseResume()
	if !e.playing {
		t.Error("should be playing after second PauseResume")
	}
}

func TestEngine_Stop(t *testing.T) {
	e := NewEngine()
	e.playing = true
	e.Stop()
	if e.IsPlaying() {
		t.Error("engine should not be playing after Stop")
	}
}

func TestEngine_SetSpeed(t *testing.T) {
	e := NewEngine()
	e.SetSpeed(1.5)
	if e.speed != 1.5 {
		t.Errorf("speed: got %f, want 1.5", e.speed)
	}

	// Zero/negative should default to 1.0.
	e.SetSpeed(0)
	if e.speed != 1.0 {
		t.Errorf("speed after 0: got %f, want 1.0", e.speed)
	}
}

func TestEngine_SetVolume(t *testing.T) {
	e := NewEngine()

	e.SetVolume(0.5)
	if e.volumeLevel != 0.5 {
		t.Errorf("volume: got %f, want 0.5", e.volumeLevel)
	}

	// Clamp below 0.
	e.SetVolume(-1.0)
	if e.volumeLevel != 0.0 {
		t.Errorf("volume clamp low: got %f, want 0.0", e.volumeLevel)
	}

	// Clamp above 1.
	e.SetVolume(2.0)
	if e.volumeLevel != 1.0 {
		t.Errorf("volume clamp high: got %f, want 1.0", e.volumeLevel)
	}
}

func TestEngine_Position_NoFile(t *testing.T) {
	e := NewEngine()
	if e.Position() != 0 {
		t.Error("position with no file should be 0")
	}
}

func TestEngine_Close(t *testing.T) {
	e := NewEngine()
	e.playing = true
	e.Close()
	if e.IsPlaying() {
		t.Error("engine should not be playing after Close")
	}
}

func TestEngine_VolumeGain(t *testing.T) {
	tests := []struct {
		vol  float64
		want float64
	}{
		{1.0, 0.0},
		{0.0, -5.0},
		{0.5, -2.5},
	}
	for _, tt := range tests {
		got := volumeGain(tt.vol)
		if got != tt.want {
			t.Errorf("volumeGain(%v) = %v, want %v", tt.vol, got, tt.want)
		}
	}
}

func TestEngine_IsPlaying_Default(t *testing.T) {
	e := NewEngine()
	if e.IsPlaying() {
		t.Error("default engine should not be playing")
	}
	// Verify position is zero duration.
	if e.Position() != time.Duration(0) {
		t.Error("position should be zero for idle engine")
	}
}
