package source

import (
	"fmt"
	"time"
)

// Audiobook represents a single audiobook with metadata.
type Audiobook struct {
	ID           string
	Title        string
	Author       string
	Narrator     string
	Description  string
	Language     string
	Year         string
	Duration     time.Duration
	CoverURL     string
	PageURL      string
	Format       string
	Source       string
	TotalSize    int64
	ChapterCount int
}

// DurationFormatted returns the duration as "Xh Ym" or "Ym" when hours == 0.
func (a *Audiobook) DurationFormatted() string {
	total := int(a.Duration.Minutes())
	hours := total / 60
	minutes := total % 60

	if hours == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

// SizeFormatted returns a human-readable file size string.
func (a *Audiobook) SizeFormatted() string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	size := a.TotalSize
	switch {
	case size == 0:
		return "0 B"
	case size < KB:
		return fmt.Sprintf("%d B", size)
	case size < MB:
		return fmt.Sprintf("%.1f KB", float64(size)/KB)
	case size < GB:
		return fmt.Sprintf("%.1f MB", float64(size)/MB)
	default:
		return fmt.Sprintf("%.1f GB", float64(size)/GB)
	}
}

// Chapter represents a single chapter within an audiobook.
type Chapter struct {
	Index       int
	Title       string
	Duration    time.Duration
	DownloadURL string
	Format      string
	FileSize    int64
}

// FilenamePrefix returns a zero-padded filename prefix in the form "02 - Title".
func (c *Chapter) FilenamePrefix() string {
	return fmt.Sprintf("%02d - %s", c.Index, c.Title)
}
