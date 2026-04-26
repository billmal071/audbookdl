package player

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ProbeAudioDuration uses ffprobe to get the duration of an audio file.
func ProbeAudioDuration(filePath string) (time.Duration, error) {
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0, fmt.Errorf("ffprobe not found: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffprobe,
		"-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		filePath,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe %s: %w", filePath, err)
	}
	secs, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, fmt.Errorf("parse duration: %w", err)
	}
	return time.Duration(secs * float64(time.Second)), nil
}

// ProbeChapterDurations probes all chapters in parallel and populates their Duration fields.
// Errors are non-fatal: chapters with probe failures keep their existing Duration.
func ProbeChapterDurations(chapters []ChapterInfo) []ChapterInfo {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i := range chapters {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			d, err := ProbeAudioDuration(chapters[idx].FilePath)
			if err == nil {
				chapters[idx].Duration = d
			}
		}(i)
	}
	wg.Wait()
	return chapters
}
