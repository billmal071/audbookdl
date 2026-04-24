package db

import (
	"testing"
)

func TestCreateAndListBookmarks(t *testing.T) {
	db := setupTestDB(t)

	bm := &Bookmark{
		AudiobookID: "ab-001",
		Title:       "The Hobbit",
		Author:      "J.R.R. Tolkien",
		Narrator:    "Rob Inglis",
		Source:      "librivox",
		PageURL:     "https://librivox.org/the-hobbit",
		Note:        "Great narration",
	}

	id, err := CreateBookmark(db, bm)
	if err != nil {
		t.Fatalf("CreateBookmark() error: %v", err)
	}
	if id == 0 {
		t.Fatal("CreateBookmark() returned id=0, want non-zero")
	}

	bookmarks, err := ListBookmarks(db)
	if err != nil {
		t.Fatalf("ListBookmarks() error: %v", err)
	}
	if len(bookmarks) != 1 {
		t.Fatalf("ListBookmarks() count = %d, want 1", len(bookmarks))
	}

	got := bookmarks[0]
	if got.ID != id {
		t.Errorf("ID = %d, want %d", got.ID, id)
	}
	if got.AudiobookID != bm.AudiobookID {
		t.Errorf("AudiobookID = %q, want %q", got.AudiobookID, bm.AudiobookID)
	}
	if got.Title != bm.Title {
		t.Errorf("Title = %q, want %q", got.Title, bm.Title)
	}
	if got.Author != bm.Author {
		t.Errorf("Author = %q, want %q", got.Author, bm.Author)
	}
	if got.Narrator != bm.Narrator {
		t.Errorf("Narrator = %q, want %q", got.Narrator, bm.Narrator)
	}
	if got.Source != bm.Source {
		t.Errorf("Source = %q, want %q", got.Source, bm.Source)
	}
	if got.PageURL != bm.PageURL {
		t.Errorf("PageURL = %q, want %q", got.PageURL, bm.PageURL)
	}
	if got.Note != bm.Note {
		t.Errorf("Note = %q, want %q", got.Note, bm.Note)
	}
}

func TestDeleteBookmark(t *testing.T) {
	db := setupTestDB(t)

	bm := &Bookmark{
		AudiobookID: "ab-002",
		Title:       "Dune",
		Author:      "Frank Herbert",
	}
	id, err := CreateBookmark(db, bm)
	if err != nil {
		t.Fatalf("CreateBookmark() error: %v", err)
	}

	if err := DeleteBookmark(db, id); err != nil {
		t.Fatalf("DeleteBookmark() error: %v", err)
	}

	bookmarks, err := ListBookmarks(db)
	if err != nil {
		t.Fatalf("ListBookmarks() error: %v", err)
	}
	if len(bookmarks) != 0 {
		t.Fatalf("ListBookmarks() count = %d, want 0 after delete", len(bookmarks))
	}
}
