package downloader

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

const testContent = "hello world this is test content for download"

func TestDownloadFile_Simple(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testContent))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "test.mp3")
	err := DownloadFile(context.Background(), srv.URL, dest, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(got) != testContent {
		t.Errorf("content mismatch: got %q, want %q", string(got), testContent)
	}
}

func TestDownloadFile_CreatesParentDirs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testContent))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "sub", "dir", "test.mp3")
	err := DownloadFile(context.Background(), srv.URL, dest, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if _, statErr := os.Stat(dest); os.IsNotExist(statErr) {
		t.Errorf("expected file to exist at %s", dest)
	}
}

func TestDownloadFile_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "test.mp3")
	err := DownloadFile(context.Background(), srv.URL, dest, nil)
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
}

func TestDownloadFile_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testContent))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dest := filepath.Join(t.TempDir(), "test.mp3")
	err := DownloadFile(ctx, srv.URL, dest, nil)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestDownloadFile_ProgressCallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testContent))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "test.mp3")
	var lastReported int64
	progressFn := func(downloaded int64) {
		lastReported = downloaded
	}

	err := DownloadFile(context.Background(), srv.URL, dest, progressFn)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expected := int64(len(testContent))
	if lastReported != expected {
		t.Errorf("progress callback final count: got %d, want %d", lastReported, expected)
	}
}
