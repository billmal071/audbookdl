// internal/converter/manager.go
package converter

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/billmal071/audbookdl/internal/db"
	"github.com/billmal071/audbookdl/internal/extractor"
	"github.com/billmal071/audbookdl/internal/tts"
)

// Options configures a conversion run.
type Options struct {
	OutputDir   string
	Voice       string
	Rate        string
	Volume      string
	SkipConfirm bool
}

// Manager orchestrates the conversion pipeline.
type Manager struct {
	engine   tts.Engine
	database *sql.DB
}

// NewManager creates a conversion manager.
func NewManager(engine tts.Engine, database *sql.DB) *Manager {
	return &Manager{engine: engine, database: database}
}

// Convert runs the full pipeline: synthesize each chapter, save MP3, create DB records.
func (m *Manager) Convert(ctx context.Context, book *extractor.Book, opts Options) error {
	if len(book.Chapters) == 0 {
		return fmt.Errorf("no chapters to convert")
	}

	// Create output directory: OutputDir/Author/Title/
	outDir := filepath.Join(opts.OutputDir, book.Author, book.Title)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	// Create DB record for the audiobook.
	var downloadID int64
	if m.database != nil {
		audiobookID := fmt.Sprintf("converted-%s-%s", book.Author, book.Title)
		id, err := db.CreateDownload(m.database, &db.AudiobookDownload{
			AudiobookID: audiobookID,
			Title:       book.Title,
			Author:      book.Author,
			Narrator:    fmt.Sprintf("%s (TTS)", opts.Voice),
			Source:      "converted",
			Status:      db.StatusDownloading,
			BasePath:    outDir,
		})
		if err != nil {
			return fmt.Errorf("create download record: %w", err)
		}
		downloadID = id
	}

	synthOpts := tts.SynthOptions{
		Voice:  opts.Voice,
		Rate:   opts.Rate,
		Volume: opts.Volume,
		Format: "audio-24khz-48kbitrate-mono-mp3",
	}

	total := len(book.Chapters)
	succeeded := 0
	var failures []string
	startTime := time.Now()

	for i, ch := range book.Chapters {
		chStart := time.Now()
		fmt.Printf("[%d/%d] Converting %q...", i+1, total, ch.Title)

		fileName := fmt.Sprintf("%02d - %s.mp3", ch.Index, sanitizeFilename(ch.Title))
		filePath := filepath.Join(outDir, fileName)

		// Create chapter DB record.
		var chapterID int64
		if m.database != nil {
			cid, err := db.CreateChapterDownload(m.database, &db.ChapterDownload{
				DownloadID:   downloadID,
				ChapterIndex: ch.Index,
				Title:        ch.Title,
				FilePath:     filePath,
				Status:       db.StatusDownloading,
			})
			if err == nil {
				chapterID = cid
			}
		}

		// Synthesize with one retry.
		audio, err := m.synthesizeWithRetry(ctx, ch.Text, synthOpts)
		if err != nil {
			fmt.Printf(" FAILED: %v\n", err)
			failures = append(failures, fmt.Sprintf("Chapter %d (%s): %v", ch.Index, ch.Title, err))
			if m.database != nil && chapterID > 0 {
				db.UpdateChapterStatus(m.database, chapterID, db.StatusFailed)
			}
			continue
		}

		// Write MP3 file.
		if err := os.WriteFile(filePath, audio, 0644); err != nil {
			fmt.Printf(" FAILED: %v\n", err)
			failures = append(failures, fmt.Sprintf("Chapter %d (%s): write: %v", ch.Index, ch.Title, err))
			continue
		}

		elapsed := time.Since(chStart).Round(time.Second)
		fmt.Printf(" done (%s)\n", elapsed)
		succeeded++

		if m.database != nil && chapterID > 0 {
			db.UpdateChapterProgress(m.database, chapterID, int64(len(audio)))
			db.UpdateChapterStatus(m.database, chapterID, db.StatusCompleted)
		}
	}

	// Update download status.
	if m.database != nil {
		if succeeded == total {
			db.UpdateDownloadStatus(m.database, downloadID, db.StatusCompleted)
		} else if succeeded == 0 {
			db.UpdateDownloadStatus(m.database, downloadID, db.StatusFailed)
		} else {
			db.UpdateDownloadStatus(m.database, downloadID, db.StatusCompleted)
		}
	}

	// Summary.
	totalElapsed := time.Since(startTime).Round(time.Second)
	fmt.Printf("\nConversion complete: %d/%d chapters in %s\n", succeeded, total, totalElapsed)
	if len(failures) > 0 {
		fmt.Println("Failed chapters:")
		for _, f := range failures {
			fmt.Printf("  - %s\n", f)
		}
	}
	fmt.Printf("Output: %s\n", outDir)

	return nil
}

func (m *Manager) synthesizeWithRetry(ctx context.Context, text string, opts tts.SynthOptions) ([]byte, error) {
	audio, err := m.engine.Synthesize(ctx, text, opts)
	if err != nil {
		// Retry once after 2 seconds.
		time.Sleep(2 * time.Second)
		audio, err = m.engine.Synthesize(ctx, text, opts)
		if err != nil {
			return nil, err
		}
	}
	return audio, nil
}

// sanitizeFilename removes characters that are invalid in filenames.
func sanitizeFilename(name string) string {
	replacer := []string{
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	}
	r := name
	for i := 0; i < len(replacer); i += 2 {
		r = filepath.Clean(r)
		for j := 0; j < len(r); j++ {
			if string(r[j]) == replacer[i] {
				r = r[:j] + replacer[i+1] + r[j+1:]
			}
		}
	}
	if len(r) > 100 {
		r = r[:100]
	}
	return r
}
