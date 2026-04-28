package downloader

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/billmal071/audbookdl/internal/db"
	"github.com/billmal071/audbookdl/internal/source"
)

// DownloadProgressFunc is called periodically as each chapter downloads.
// chapterIndex is the chapter's index, totalChapters is the count of all chapters,
// and chapterBytes is how many bytes have been downloaded for this chapter so far.
type DownloadProgressFunc func(chapterIndex, totalChapters int, chapterBytes int64)

// Manager coordinates downloading all chapters of an audiobook as a single unit.
type Manager struct {
	db            *sql.DB
	baseDir       string
	maxConcurrent int
}

// NewManager creates a new Manager. maxConcurrent controls how many chapters
// are downloaded in parallel; if <= 0, defaults to 3.
func NewManager(database *sql.DB, baseDir string, maxConcurrent int) *Manager {
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	return &Manager{db: database, baseDir: baseDir, maxConcurrent: maxConcurrent}
}

// DownloadAudiobook downloads all chapters of book concurrently, tracking
// state in SQLite. It returns the first error encountered, if any.
func (m *Manager) DownloadAudiobook(ctx context.Context, book *source.Audiobook, chapters []*source.Chapter, progressFn DownloadProgressFunc) error {
	var totalSize int64
	for _, ch := range chapters {
		totalSize += ch.FileSize
	}

	basePath := filepath.Join(m.baseDir, book.Author, book.Title)
	dlID, err := db.CreateDownload(m.db, &db.AudiobookDownload{
		AudiobookID: book.ID,
		Title:       book.Title,
		Author:      book.Author,
		Narrator:    book.Narrator,
		Source:      book.Source,
		BasePath:    basePath,
		TotalSize:   totalSize,
	})
	if err != nil {
		return fmt.Errorf("create download record: %w", err)
	}

	if err := db.UpdateDownloadStatus(m.db, dlID, db.StatusDownloading); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	for _, ch := range chapters {
		filePath := m.buildChapterPath(book.Author, book.Title, ch)
		_, err := db.CreateChapterDownload(m.db, &db.ChapterDownload{
			DownloadID:   dlID,
			ChapterIndex: ch.Index,
			Title:        ch.Title,
			FilePath:     filePath,
			FileSize:     ch.FileSize,
		})
		if err != nil {
			return fmt.Errorf("create chapter record: %w", err)
		}
	}

	sem := make(chan struct{}, m.maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var downloadErr error

	// Build a map from chapter index to chapter DB ID for progress updates.
	chapterDBIDs := make(map[int]int64)
	chapterList, _ := db.ListChapterDownloads(m.db, dlID)
	for _, c := range chapterList {
		chapterDBIDs[c.ChapterIndex] = c.ID
	}

	// Track per-chapter downloaded bytes; aggregate for the parent record.
	chapterBytes := make(map[int]int64) // chapter index → latest downloaded bytes
	var progressMu sync.Mutex

	for _, ch := range chapters {
		select {
		case <-ctx.Done():
			db.UpdateDownloadStatus(m.db, dlID, db.StatusFailed) //nolint:errcheck
			return ctx.Err()
		default:
		}

		wg.Add(1)
		go func(chapter *source.Chapter) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			filePath := m.buildChapterPath(book.Author, book.Title, chapter)
			chDBID := chapterDBIDs[chapter.Index]

			retryCfg := DefaultRetryConfig()
			retryCfg.MaxAttempts = 3
			retryCfg.BaseDelay = 100 * time.Millisecond

			err := RetryOperation(ctx, retryCfg, func() error {
				return DownloadFile(ctx, chapter.DownloadURL, filePath, func(downloaded int64) {
					// Update chapter progress in DB.
					if chDBID > 0 {
						db.UpdateChapterProgress(m.db, chDBID, downloaded) //nolint:errcheck
					}
					// Update aggregate progress (downloaded is cumulative per chapter).
					progressMu.Lock()
					chapterBytes[chapter.Index] = downloaded
					var total int64
					for _, b := range chapterBytes {
						total += b
					}
					db.UpdateDownloadProgress(m.db, dlID, total) //nolint:errcheck
					progressMu.Unlock()

					if progressFn != nil {
						progressFn(chapter.Index, len(chapters), downloaded)
					}
				})
			})
			if err != nil {
				if chDBID > 0 {
					db.UpdateChapterStatus(m.db, chDBID, db.StatusFailed) //nolint:errcheck
				}
				mu.Lock()
				if downloadErr == nil {
					downloadErr = fmt.Errorf("chapter %d (%s): %w", chapter.Index, chapter.Title, err)
				}
				mu.Unlock()
			} else {
				// Update chapter file size from actual file on disk.
				if chDBID > 0 {
					db.UpdateChapterStatus(m.db, chDBID, db.StatusCompleted) //nolint:errcheck
				}
			}
		}(ch)
	}

	wg.Wait()

	// After all downloads, compute actual total size from files on disk
	// and update the record (handles sources that don't report FileSize).
	if totalSize == 0 {
		var actualSize int64
		for _, ch := range chapters {
			fp := m.buildChapterPath(book.Author, book.Title, ch)
			if info, err := os.Stat(fp); err == nil {
				actualSize += info.Size()
			}
		}
		if actualSize > 0 {
			m.db.Exec("UPDATE audiobook_downloads SET total_size = ?, downloaded_size = ? WHERE id = ?",
				actualSize, actualSize, dlID) //nolint:errcheck
		}
	}

	if downloadErr != nil {
		db.UpdateDownloadStatus(m.db, dlID, db.StatusFailed) //nolint:errcheck
		return downloadErr
	}

	if err := db.UpdateDownloadStatus(m.db, dlID, db.StatusCompleted); err != nil {
		return fmt.Errorf("mark completed: %w", err)
	}
	return nil
}

// ResumeDownload retries a failed or paused download. It loads the existing
// record from the database, skips chapters already completed, and downloads
// only the remaining ones using fresh chapter URLs from the source.
func (m *Manager) ResumeDownload(ctx context.Context, dlID int64, chapters []*source.Chapter, progressFn DownloadProgressFunc) error {
	dl, err := db.GetDownload(m.db, dlID)
	if err != nil {
		return fmt.Errorf("get download %d: %w", dlID, err)
	}
	if dl.Status != db.StatusPaused && dl.Status != db.StatusFailed {
		return fmt.Errorf("download %d has status %q; only paused or failed downloads can be resumed", dlID, dl.Status)
	}

	if err := db.UpdateDownloadStatus(m.db, dlID, db.StatusDownloading); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	// Load existing chapter records and index by chapter number.
	existingChapters, err := db.ListChapterDownloads(m.db, dlID)
	if err != nil {
		return fmt.Errorf("list chapters: %w", err)
	}
	chapterDBByIndex := make(map[int]*db.ChapterDownload)
	for _, c := range existingChapters {
		chapterDBByIndex[c.ChapterIndex] = c
	}

	// Determine which chapters still need downloading.
	var pending []*source.Chapter
	for _, ch := range chapters {
		if rec, ok := chapterDBByIndex[ch.Index]; ok && rec.Status == db.StatusCompleted {
			continue // already done
		}
		pending = append(pending, ch)
	}

	if len(pending) == 0 {
		if err := db.UpdateDownloadStatus(m.db, dlID, db.StatusCompleted); err != nil {
			return fmt.Errorf("mark completed: %w", err)
		}
		return nil
	}

	// Create chapter records for any new chapters not yet tracked.
	for _, ch := range pending {
		if _, ok := chapterDBByIndex[ch.Index]; !ok {
			filePath := m.buildChapterPath(dl.Author, dl.Title, ch)
			id, createErr := db.CreateChapterDownload(m.db, &db.ChapterDownload{
				DownloadID:   dlID,
				ChapterIndex: ch.Index,
				Title:        ch.Title,
				FilePath:     filePath,
				FileSize:     ch.FileSize,
			})
			if createErr == nil {
				chapterDBByIndex[ch.Index] = &db.ChapterDownload{ID: id, ChapterIndex: ch.Index}
			}
		} else {
			// Reset failed chapter status so progress is tracked fresh.
			rec := chapterDBByIndex[ch.Index]
			db.UpdateChapterStatus(m.db, rec.ID, db.StatusDownloading) //nolint:errcheck
		}
	}

	sem := make(chan struct{}, m.maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var downloadErr error

	// Seed per-chapter bytes from already-completed chapters.
	chapterBytes := make(map[int]int64)
	var progressMu sync.Mutex
	for _, rec := range existingChapters {
		if rec.Status == db.StatusCompleted {
			chapterBytes[rec.ChapterIndex] = rec.Downloaded
		}
	}

	for _, ch := range pending {
		select {
		case <-ctx.Done():
			db.UpdateDownloadStatus(m.db, dlID, db.StatusFailed) //nolint:errcheck
			return ctx.Err()
		default:
		}

		wg.Add(1)
		go func(chapter *source.Chapter) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			filePath := m.buildChapterPath(dl.Author, dl.Title, chapter)
			chDBID := chapterDBByIndex[chapter.Index].ID

			retryCfg := DefaultRetryConfig()
			retryCfg.MaxAttempts = 3
			retryCfg.BaseDelay = 100 * time.Millisecond

			err := RetryOperation(ctx, retryCfg, func() error {
				return DownloadFile(ctx, chapter.DownloadURL, filePath, func(downloaded int64) {
					if chDBID > 0 {
						db.UpdateChapterProgress(m.db, chDBID, downloaded) //nolint:errcheck
					}
					progressMu.Lock()
					chapterBytes[chapter.Index] = downloaded
					var total int64
					for _, b := range chapterBytes {
						total += b
					}
					db.UpdateDownloadProgress(m.db, dlID, total) //nolint:errcheck
					progressMu.Unlock()

					if progressFn != nil {
						progressFn(chapter.Index, len(chapters), downloaded)
					}
				})
			})
			if err != nil {
				if chDBID > 0 {
					db.UpdateChapterStatus(m.db, chDBID, db.StatusFailed) //nolint:errcheck
				}
				mu.Lock()
				if downloadErr == nil {
					downloadErr = fmt.Errorf("chapter %d (%s): %w", chapter.Index, chapter.Title, err)
				}
				mu.Unlock()
			} else {
				if chDBID > 0 {
					db.UpdateChapterStatus(m.db, chDBID, db.StatusCompleted) //nolint:errcheck
				}
			}
		}(ch)
	}

	wg.Wait()

	if downloadErr != nil {
		db.UpdateDownloadStatus(m.db, dlID, db.StatusFailed) //nolint:errcheck
		return downloadErr
	}

	if err := db.UpdateDownloadStatus(m.db, dlID, db.StatusCompleted); err != nil {
		return fmt.Errorf("mark completed: %w", err)
	}
	return nil
}

// buildChapterPath returns the full file path for a chapter file.
func (m *Manager) buildChapterPath(author, title string, ch *source.Chapter) string {
	filename := fmt.Sprintf("%02d - %s.%s", ch.Index, ch.Title, ch.Format)
	return filepath.Join(m.baseDir, author, title, filename)
}
