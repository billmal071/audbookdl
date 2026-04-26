package player

import (
	"os/exec"
	"testing"
	"time"
)

func TestProbeAudioDuration_NoFfprobe(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		_, probeErr := ProbeAudioDuration("/nonexistent.mp3")
		if probeErr == nil {
			t.Error("expected error when ffprobe not available")
		}
		return
	}
	t.Skip("ffprobe is installed — this test checks the no-ffprobe path")
}

func TestProbeAudioDuration_FileNotFound(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	_, err := ProbeAudioDuration("/nonexistent/file.mp3")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestProbeChapterDurations_Empty(t *testing.T) {
	result := ProbeChapterDurations(nil)
	if len(result) != 0 {
		t.Errorf("expected nil, got %d chapters", len(result))
	}
}

func TestProbeChapterDurations_NonexistentFiles(t *testing.T) {
	chapters := []ChapterInfo{
		{Index: 0, Title: "Ch1", FilePath: "/nonexistent/ch1.mp3"},
		{Index: 1, Title: "Ch2", FilePath: "/nonexistent/ch2.mp3"},
	}
	result := ProbeChapterDurations(chapters)
	for _, ch := range result {
		if ch.Duration != 0 {
			t.Errorf("chapter %d: expected Duration 0, got %v", ch.Index, ch.Duration)
		}
	}
}

func TestProbeChapterDurations_PreservesDuration(t *testing.T) {
	chapters := []ChapterInfo{
		{Index: 0, Title: "Ch1", FilePath: "/nonexistent.mp3", Duration: 5 * time.Minute},
	}
	result := ProbeChapterDurations(chapters)
	// ffprobe will fail on nonexistent file, so Duration should be preserved (only overwritten on success)
	if result[0].Duration != 5*time.Minute {
		t.Errorf("expected Duration preserved at 5m, got %v", result[0].Duration)
	}
}
