package source_test

import (
	"testing"
	"time"

	"github.com/billmal071/audbookdl/internal/source"
)

func TestAudiobook_DurationFormatted(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "zero duration",
			duration: 0,
			want:     "0m",
		},
		{
			name:     "45 minutes",
			duration: 45 * time.Minute,
			want:     "45m",
		},
		{
			name:     "2h30m",
			duration: 2*time.Hour + 30*time.Minute,
			want:     "2h 30m",
		},
		{
			name:     "5 hours exactly",
			duration: 5 * time.Hour,
			want:     "5h 0m",
		},
		{
			name:     "11h32m",
			duration: 11*time.Hour + 32*time.Minute,
			want:     "11h 32m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &source.Audiobook{Duration: tt.duration}
			got := a.DurationFormatted()
			if got != tt.want {
				t.Errorf("DurationFormatted() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAudiobook_SizeFormatted(t *testing.T) {
	tests := []struct {
		name      string
		totalSize int64
		want      string
	}{
		{
			name:      "zero bytes",
			totalSize: 0,
			want:      "0 B",
		},
		{
			name:      "500 bytes",
			totalSize: 500,
			want:      "500 B",
		},
		{
			name:      "1.5 KB",
			totalSize: 1536,
			want:      "1.5 KB",
		},
		{
			name:      "5.0 MB",
			totalSize: 5 * 1024 * 1024,
			want:      "5.0 MB",
		},
		{
			name:      "1.0 GB",
			totalSize: 1024 * 1024 * 1024,
			want:      "1.0 GB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &source.Audiobook{TotalSize: tt.totalSize}
			got := a.SizeFormatted()
			if got != tt.want {
				t.Errorf("SizeFormatted() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestChapter_FilenamePrefix(t *testing.T) {
	tests := []struct {
		name  string
		index int
		title string
		want  string
	}{
		{
			name:  "single digit index",
			index: 1,
			title: "Title",
			want:  "01 - Title",
		},
		{
			name:  "double digit index",
			index: 12,
			title: "Title",
			want:  "12 - Title",
		},
		{
			name:  "zero index",
			index: 0,
			title: "Title",
			want:  "00 - Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &source.Chapter{Index: tt.index, Title: tt.title}
			got := c.FilenamePrefix()
			if got != tt.want {
				t.Errorf("FilenamePrefix() = %q, want %q", got, tt.want)
			}
		})
	}
}
