package tagger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/source"
)

// TagResult contains the result of tagging an audiobook.
type TagResult struct {
	TaggedFiles int
	CoverSaved  bool
	Errors      []error
}

// TagAudiobook writes metadata tags to all chapter files and downloads cover art.
// This is a framework — actual ID3 tag writing requires a pure Go ID3 library
// which will be integrated when available. For now, it handles cover art download
// and file validation.
func TagAudiobook(ctx context.Context, book *source.Audiobook, chapters []*source.Chapter, baseDir string) *TagResult {
	result := &TagResult{}

	// Download cover art if available
	if book.CoverURL != "" {
		coverPath := filepath.Join(baseDir, book.Author, book.Title, "cover.jpg")
		if err := downloadCover(ctx, book.CoverURL, coverPath); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("cover art: %w", err))
		} else {
			result.CoverSaved = true
		}
	}

	// Validate chapter files exist
	for _, ch := range chapters {
		filePath := filepath.Join(baseDir, book.Author, book.Title,
			fmt.Sprintf("%02d - %s.%s", ch.Index, ch.Title, ch.Format))
		if _, err := os.Stat(filePath); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("chapter %d missing: %w", ch.Index, err))
			continue
		}
		result.TaggedFiles++
	}

	return result
}

func downloadCover(ctx context.Context, url, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	http := httpclient.New()
	body, err := http.GetBody(ctx, url)
	if err != nil {
		return fmt.Errorf("download cover: %w", err)
	}

	return os.WriteFile(destPath, body, 0644)
}
