package db

import (
	"testing"
)

func TestAddAndListSearchHistory(t *testing.T) {
	db := setupTestDB(t)

	if err := AddSearchHistory(db, "tolkien audiobooks", "librivox", 5); err != nil {
		t.Fatalf("AddSearchHistory() first entry error: %v", err)
	}
	if err := AddSearchHistory(db, "dune frank herbert", "librivox", 3); err != nil {
		t.Fatalf("AddSearchHistory() second entry error: %v", err)
	}

	entries, err := ListSearchHistory(db, 10)
	if err != nil {
		t.Fatalf("ListSearchHistory() error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ListSearchHistory() count = %d, want 2", len(entries))
	}

	// Most recent entry should be first (dune was added second)
	if entries[0].Query != "dune frank herbert" {
		t.Errorf("entries[0].Query = %q, want %q", entries[0].Query, "dune frank herbert")
	}
	if entries[1].Query != "tolkien audiobooks" {
		t.Errorf("entries[1].Query = %q, want %q", entries[1].Query, "tolkien audiobooks")
	}

	// Verify fields on first entry
	if entries[0].Source != "librivox" {
		t.Errorf("Source = %q, want %q", entries[0].Source, "librivox")
	}
	if entries[0].ResultCount != 3 {
		t.Errorf("ResultCount = %d, want 3", entries[0].ResultCount)
	}
}

func TestClearSearchHistory(t *testing.T) {
	db := setupTestDB(t)

	if err := AddSearchHistory(db, "query one", "source-a", 10); err != nil {
		t.Fatalf("AddSearchHistory() error: %v", err)
	}
	if err := AddSearchHistory(db, "query two", "source-b", 20); err != nil {
		t.Fatalf("AddSearchHistory() error: %v", err)
	}

	if err := ClearSearchHistory(db); err != nil {
		t.Fatalf("ClearSearchHistory() error: %v", err)
	}

	entries, err := ListSearchHistory(db, 10)
	if err != nil {
		t.Fatalf("ListSearchHistory() after clear error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("ListSearchHistory() count = %d, want 0 after clear", len(entries))
	}
}
