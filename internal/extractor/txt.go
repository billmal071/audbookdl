// internal/extractor/txt.go
package extractor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func extractTXT(filePath string) (*Book, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read txt: %w", err)
	}

	text := string(data)
	title := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	chapters := DetectChapters(text)
	if len(chapters) == 0 {
		// Entire file as one chapter.
		chapters = []Chapter{{Index: 1, Title: title, Text: strings.TrimSpace(text)}}
	}

	return &Book{
		Title:    title,
		Author:   "Unknown",
		Chapters: chapters,
	}, nil
}
