package downloader

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/billmal071/audbookdl/internal/db"
	"github.com/billmal071/audbookdl/internal/source"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	conn, err := db.InitWithPath(dbPath)
	if err != nil {
		t.Fatalf("InitWithPath() error: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// fakeMP3 is a tiny but plausible-looking byte slice used as chapter content.
var fakeMP3 = []byte("ID3\x03\x00\x00\x00\x00\x00\x00" + strings.Repeat("audio", 20))

func TestManager_DownloadAudiobook(t *testing.T) {
	// Serve a fake MP3 for every request.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		w.Write(fakeMP3) //nolint:errcheck
	}))
	defer srv.Close()

	conn := setupTestDB(t)
	baseDir := t.TempDir()
	mgr := NewManager(conn, baseDir, 2)

	book := &source.Audiobook{
		ID:     "book-001",
		Title:  "My Book",
		Author: "Jane Doe",
		Source: "test",
	}

	chapters := []*source.Chapter{
		{Index: 1, Title: "Chapter One", Format: "mp3", DownloadURL: srv.URL + "/ch01.mp3"},
		{Index: 2, Title: "Chapter Two", Format: "mp3", DownloadURL: srv.URL + "/ch02.mp3"},
	}

	var progressCalls int
	err := mgr.DownloadAudiobook(context.Background(), book, chapters, func(chIdx, total int, bytes int64) {
		progressCalls++
	})
	if err != nil {
		t.Fatalf("DownloadAudiobook() error: %v", err)
	}

	// Verify files exist on disk.
	for _, ch := range chapters {
		expected := filepath.Join(baseDir, book.Author, book.Title,
			filepath.Base(mgr.buildChapterPath(book.Author, book.Title, ch)))
		if _, statErr := os.Stat(expected); os.IsNotExist(statErr) {
			t.Errorf("expected file %s to exist, but it does not", expected)
		}
	}

	// Verify DB record is completed.
	downloads, err := db.ListDownloads(conn)
	if err != nil {
		t.Fatalf("ListDownloads() error: %v", err)
	}
	if len(downloads) != 1 {
		t.Fatalf("expected 1 download record, got %d", len(downloads))
	}
	if downloads[0].Status != db.StatusCompleted {
		t.Errorf("expected status %q, got %q", db.StatusCompleted, downloads[0].Status)
	}

	// Verify chapter records.
	chapterRecords, err := db.ListChapterDownloads(conn, downloads[0].ID)
	if err != nil {
		t.Fatalf("ListChapterDownloads() error: %v", err)
	}
	if len(chapterRecords) != len(chapters) {
		t.Errorf("expected %d chapter records, got %d", len(chapters), len(chapterRecords))
	}

	if progressCalls == 0 {
		t.Error("expected at least one progress callback call")
	}
}

func TestManager_DownloadAudiobook_PartialFailure(t *testing.T) {
	// Serve ch01.mp3 successfully but return 500 for ch02.mp3.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "ch02") {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		w.Write(fakeMP3) //nolint:errcheck
	}))
	defer srv.Close()

	conn := setupTestDB(t)
	baseDir := t.TempDir()
	// Use maxConcurrent=1 so we have deterministic ordering, but the test
	// only cares that the overall download is marked failed.
	mgr := NewManager(conn, baseDir, 1)

	book := &source.Audiobook{
		ID:     "book-002",
		Title:  "Failing Book",
		Author: "John Smith",
		Source: "test",
	}

	chapters := []*source.Chapter{
		{Index: 1, Title: "Chapter One", Format: "mp3", DownloadURL: srv.URL + "/ch01.mp3"},
		{Index: 2, Title: "Chapter Two", Format: "mp3", DownloadURL: srv.URL + "/ch02.mp3"},
	}

	err := mgr.DownloadAudiobook(context.Background(), book, chapters, nil)
	if err == nil {
		t.Fatal("expected an error for the failing chapter, got nil")
	}

	// DB record must be marked failed.
	downloads, listErr := db.ListDownloads(conn)
	if listErr != nil {
		t.Fatalf("ListDownloads() error: %v", listErr)
	}
	if len(downloads) != 1 {
		t.Fatalf("expected 1 download record, got %d", len(downloads))
	}
	if downloads[0].Status != db.StatusFailed {
		t.Errorf("expected status %q, got %q", db.StatusFailed, downloads[0].Status)
	}
}

func TestManager_BuildFilePath(t *testing.T) {
	conn := setupTestDB(t)
	mgr := NewManager(conn, "/downloads", 2)

	cases := []struct {
		author  string
		title   string
		chapter *source.Chapter
		want    string
	}{
		{
			author: "Jane Doe",
			title:  "Great Book",
			chapter: &source.Chapter{Index: 1, Title: "Introduction", Format: "mp3"},
			want:   "/downloads/Jane Doe/Great Book/01 - Introduction.mp3",
		},
		{
			author: "Author",
			title:  "Series Part 2",
			chapter: &source.Chapter{Index: 10, Title: "Finale", Format: "m4b"},
			want:   "/downloads/Author/Series Part 2/10 - Finale.m4b",
		},
		{
			author: "A",
			title:  "T",
			chapter: &source.Chapter{Index: 3, Title: "Ch", Format: "ogg"},
			want:   "/downloads/A/T/03 - Ch.ogg",
		},
	}

	for _, tc := range cases {
		got := mgr.buildChapterPath(tc.author, tc.title, tc.chapter)
		if got != tc.want {
			t.Errorf("buildChapterPath(%q, %q, %+v) = %q, want %q",
				tc.author, tc.title, tc.chapter, got, tc.want)
		}
	}
}
