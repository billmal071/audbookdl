package tagger

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/billmal071/audbookdl/internal/source"
)

func TestTagAudiobook_ValidatesFiles(t *testing.T) {
	dir := t.TempDir()

	// Create fake chapter files
	bookDir := filepath.Join(dir, "Author", "Title")
	os.MkdirAll(bookDir, 0755)
	os.WriteFile(filepath.Join(bookDir, "01 - Chapter One.mp3"), []byte("fake"), 0644)
	os.WriteFile(filepath.Join(bookDir, "02 - Chapter Two.mp3"), []byte("fake"), 0644)

	book := &source.Audiobook{Title: "Title", Author: "Author"}
	chapters := []*source.Chapter{
		{Index: 1, Title: "Chapter One", Format: "mp3"},
		{Index: 2, Title: "Chapter Two", Format: "mp3"},
	}

	result := TagAudiobook(context.Background(), book, chapters, dir)
	if result.TaggedFiles != 2 {
		t.Errorf("TaggedFiles = %d, want 2", result.TaggedFiles)
	}
	if len(result.Errors) != 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
}

func TestTagAudiobook_MissingFile(t *testing.T) {
	dir := t.TempDir()
	bookDir := filepath.Join(dir, "Author", "Title")
	os.MkdirAll(bookDir, 0755)
	os.WriteFile(filepath.Join(bookDir, "01 - Chapter One.mp3"), []byte("fake"), 0644)
	// Chapter 2 is missing

	book := &source.Audiobook{Title: "Title", Author: "Author"}
	chapters := []*source.Chapter{
		{Index: 1, Title: "Chapter One", Format: "mp3"},
		{Index: 2, Title: "Chapter Two", Format: "mp3"},
	}

	result := TagAudiobook(context.Background(), book, chapters, dir)
	if result.TaggedFiles != 1 {
		t.Errorf("TaggedFiles = %d, want 1", result.TaggedFiles)
	}
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(result.Errors))
	}
}

func TestTagAudiobook_DownloadsCover(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fake jpg data"))
	}))
	defer server.Close()

	dir := t.TempDir()
	bookDir := filepath.Join(dir, "Author", "Title")
	os.MkdirAll(bookDir, 0755)

	book := &source.Audiobook{Title: "Title", Author: "Author", CoverURL: server.URL + "/cover.jpg"}

	result := TagAudiobook(context.Background(), book, nil, dir)
	if !result.CoverSaved {
		t.Error("CoverSaved should be true")
	}

	coverPath := filepath.Join(bookDir, "cover.jpg")
	data, err := os.ReadFile(coverPath)
	if err != nil {
		t.Fatalf("cover file not found: %v", err)
	}
	if string(data) != "fake jpg data" {
		t.Errorf("cover content = %q", string(data))
	}
}
