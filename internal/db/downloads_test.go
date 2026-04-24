package db

import (
	"testing"
)

func makeDownload(audiobookID, title string) *AudiobookDownload {
	return &AudiobookDownload{
		AudiobookID: audiobookID,
		Title:       title,
		Author:      "Test Author",
		Narrator:    "Test Narrator",
		Source:      "librivox",
		Status:      StatusPending,
		BasePath:    "/tmp/" + audiobookID,
		TotalSize:   10 * 1024 * 1024,
	}
}

func TestCreateAndGetDownload(t *testing.T) {
	db := setupTestDB(t)

	d := makeDownload("ab-001", "The Great Book")
	id, err := CreateDownload(db, d)
	if err != nil {
		t.Fatalf("CreateDownload() error: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero ID from CreateDownload")
	}

	got, err := GetDownload(db, id)
	if err != nil {
		t.Fatalf("GetDownload() error: %v", err)
	}

	if got.ID != id {
		t.Errorf("ID: got %d, want %d", got.ID, id)
	}
	if got.AudiobookID != d.AudiobookID {
		t.Errorf("AudiobookID: got %q, want %q", got.AudiobookID, d.AudiobookID)
	}
	if got.Title != d.Title {
		t.Errorf("Title: got %q, want %q", got.Title, d.Title)
	}
	if got.Author != d.Author {
		t.Errorf("Author: got %q, want %q", got.Author, d.Author)
	}
	if got.Narrator != d.Narrator {
		t.Errorf("Narrator: got %q, want %q", got.Narrator, d.Narrator)
	}
	if got.Source != d.Source {
		t.Errorf("Source: got %q, want %q", got.Source, d.Source)
	}
	if got.Status != StatusPending {
		t.Errorf("Status: got %q, want %q", got.Status, StatusPending)
	}
	if got.BasePath != d.BasePath {
		t.Errorf("BasePath: got %q, want %q", got.BasePath, d.BasePath)
	}
	if got.TotalSize != d.TotalSize {
		t.Errorf("TotalSize: got %d, want %d", got.TotalSize, d.TotalSize)
	}
	if got.CompletedAt != nil {
		t.Errorf("CompletedAt: expected nil, got %v", got.CompletedAt)
	}
}

func TestUpdateDownloadStatus(t *testing.T) {
	db := setupTestDB(t)

	id, err := CreateDownload(db, makeDownload("ab-002", "Another Book"))
	if err != nil {
		t.Fatalf("CreateDownload() error: %v", err)
	}

	if err := UpdateDownloadStatus(db, id, StatusDownloading); err != nil {
		t.Fatalf("UpdateDownloadStatus() error: %v", err)
	}

	got, err := GetDownload(db, id)
	if err != nil {
		t.Fatalf("GetDownload() error: %v", err)
	}
	if got.Status != StatusDownloading {
		t.Errorf("Status: got %q, want %q", got.Status, StatusDownloading)
	}
	if got.CompletedAt != nil {
		t.Errorf("CompletedAt: expected nil for downloading status, got %v", got.CompletedAt)
	}
}

func TestUpdateDownloadStatus_Completed(t *testing.T) {
	db := setupTestDB(t)

	id, err := CreateDownload(db, makeDownload("ab-003", "Finished Book"))
	if err != nil {
		t.Fatalf("CreateDownload() error: %v", err)
	}

	if err := UpdateDownloadStatus(db, id, StatusCompleted); err != nil {
		t.Fatalf("UpdateDownloadStatus() error: %v", err)
	}

	got, err := GetDownload(db, id)
	if err != nil {
		t.Fatalf("GetDownload() error: %v", err)
	}
	if got.Status != StatusCompleted {
		t.Errorf("Status: got %q, want %q", got.Status, StatusCompleted)
	}
	if got.CompletedAt == nil {
		t.Error("CompletedAt: expected non-nil when status is completed")
	}
}

func TestListDownloads(t *testing.T) {
	db := setupTestDB(t)

	for i, title := range []string{"Book A", "Book B", "Book C"} {
		audiobookID := "list-ab-" + string(rune('1'+i))
		if _, err := CreateDownload(db, makeDownload(audiobookID, title)); err != nil {
			t.Fatalf("CreateDownload(%q) error: %v", title, err)
		}
	}

	list, err := ListDownloads(db)
	if err != nil {
		t.Fatalf("ListDownloads() error: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("ListDownloads() count: got %d, want 3", len(list))
	}
}

func TestCreateAndListChapters(t *testing.T) {
	db := setupTestDB(t)

	dlID, err := CreateDownload(db, makeDownload("ab-ch-001", "Chaptered Book"))
	if err != nil {
		t.Fatalf("CreateDownload() error: %v", err)
	}

	ch1 := &ChapterDownload{
		DownloadID:   dlID,
		ChapterIndex: 0,
		Title:        "Chapter 1",
		FilePath:     "/tmp/ab-ch-001/ch0.mp3",
		FileSize:     1024 * 1024,
		Status:       StatusPending,
	}
	ch2 := &ChapterDownload{
		DownloadID:   dlID,
		ChapterIndex: 1,
		Title:        "Chapter 2",
		FilePath:     "/tmp/ab-ch-001/ch1.mp3",
		FileSize:     2 * 1024 * 1024,
		Status:       StatusPending,
	}

	if _, err := CreateChapterDownload(db, ch1); err != nil {
		t.Fatalf("CreateChapterDownload(ch1) error: %v", err)
	}
	if _, err := CreateChapterDownload(db, ch2); err != nil {
		t.Fatalf("CreateChapterDownload(ch2) error: %v", err)
	}

	chapters, err := ListChapterDownloads(db, dlID)
	if err != nil {
		t.Fatalf("ListChapterDownloads() error: %v", err)
	}
	if len(chapters) != 2 {
		t.Fatalf("ListChapterDownloads() count: got %d, want 2", len(chapters))
	}

	// Verify ordering by chapter_index
	if chapters[0].ChapterIndex != 0 {
		t.Errorf("chapters[0].ChapterIndex: got %d, want 0", chapters[0].ChapterIndex)
	}
	if chapters[1].ChapterIndex != 1 {
		t.Errorf("chapters[1].ChapterIndex: got %d, want 1", chapters[1].ChapterIndex)
	}
	if chapters[0].Title != "Chapter 1" {
		t.Errorf("chapters[0].Title: got %q, want %q", chapters[0].Title, "Chapter 1")
	}
	if chapters[1].Title != "Chapter 2" {
		t.Errorf("chapters[1].Title: got %q, want %q", chapters[1].Title, "Chapter 2")
	}
}

func TestUpdateDownloadProgress(t *testing.T) {
	db := setupTestDB(t)

	id, err := CreateDownload(db, makeDownload("ab-progress", "Progress Book"))
	if err != nil {
		t.Fatalf("CreateDownload() error: %v", err)
	}

	const fiveMB = 5 * 1024 * 1024
	if err := UpdateDownloadProgress(db, id, fiveMB); err != nil {
		t.Fatalf("UpdateDownloadProgress() error: %v", err)
	}

	got, err := GetDownload(db, id)
	if err != nil {
		t.Fatalf("GetDownload() error: %v", err)
	}
	if got.DownloadedSize != fiveMB {
		t.Errorf("DownloadedSize: got %d, want %d", got.DownloadedSize, fiveMB)
	}
}
