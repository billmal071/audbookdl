package player

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/billmal071/audbookdl/internal/db"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := db.InitWithPath(dbPath)
	if err != nil {
		t.Fatalf("InitWithPath: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestSaveAndLoadState(t *testing.T) {
	database := setupTestDB(t)

	s := &State{
		AudiobookID:  "book-001",
		ChapterIndex: 3,
		PositionMS:   12345,
		Speed:        1.5,
	}
	if err := SaveState(database, s); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	got := LoadState(database, "book-001")
	if got == nil {
		t.Fatal("LoadState returned nil, expected a state")
	}
	if got.AudiobookID != s.AudiobookID {
		t.Errorf("AudiobookID: got %q, want %q", got.AudiobookID, s.AudiobookID)
	}
	if got.ChapterIndex != s.ChapterIndex {
		t.Errorf("ChapterIndex: got %d, want %d", got.ChapterIndex, s.ChapterIndex)
	}
	if got.PositionMS != s.PositionMS {
		t.Errorf("PositionMS: got %d, want %d", got.PositionMS, s.PositionMS)
	}
	if got.Speed != s.Speed {
		t.Errorf("Speed: got %f, want %f", got.Speed, s.Speed)
	}
}

func TestLoadState_NotFound(t *testing.T) {
	database := setupTestDB(t)

	got := LoadState(database, "nonexistent-book")
	if got != nil {
		t.Errorf("expected nil for missing audiobook, got %+v", got)
	}
}

func TestSaveState_Upsert(t *testing.T) {
	database := setupTestDB(t)

	first := &State{
		AudiobookID:  "book-002",
		ChapterIndex: 1,
		PositionMS:   1000,
		Speed:        1.0,
	}
	if err := SaveState(database, first); err != nil {
		t.Fatalf("SaveState first: %v", err)
	}

	second := &State{
		AudiobookID:  "book-002",
		ChapterIndex: 5,
		PositionMS:   99999,
		Speed:        2.0,
	}
	if err := SaveState(database, second); err != nil {
		t.Fatalf("SaveState second: %v", err)
	}

	got := LoadState(database, "book-002")
	if got == nil {
		t.Fatal("LoadState returned nil after upsert")
	}
	if got.ChapterIndex != second.ChapterIndex {
		t.Errorf("ChapterIndex: got %d, want %d", got.ChapterIndex, second.ChapterIndex)
	}
	if got.PositionMS != second.PositionMS {
		t.Errorf("PositionMS: got %d, want %d", got.PositionMS, second.PositionMS)
	}
	if got.Speed != second.Speed {
		t.Errorf("Speed: got %f, want %f", got.Speed, second.Speed)
	}
}
