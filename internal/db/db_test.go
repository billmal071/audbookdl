package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := InitWithPath(dbPath)
	if err != nil {
		t.Fatalf("InitWithPath() error: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestInitWithPath_CreatesTablesAndIndexes(t *testing.T) {
	db := setupTestDB(t)

	expectedTables := []string{
		"audiobook_downloads",
		"chapter_downloads",
		"chunks",
		"bookmarks",
		"playback_state",
		"search_history",
		"search_cache",
	}

	for _, table := range expectedTables {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		} else if name != table {
			t.Errorf("expected table %q, got %q", table, name)
		}
	}

	expectedIndexes := []string{
		"idx_audiobook_downloads_status",
		"idx_audiobook_downloads_audiobook_id",
		"idx_chapter_downloads_download",
		"idx_chunks_chapter",
		"idx_bookmarks_audiobook_id",
		"idx_search_history_created",
		"idx_search_cache_key",
		"idx_search_cache_expires",
	}

	for _, idx := range expectedIndexes {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='index' AND name=?",
			idx,
		).Scan(&name)
		if err != nil {
			t.Errorf("index %q not found: %v", idx, err)
		} else if name != idx {
			t.Errorf("expected index %q, got %q", idx, name)
		}
	}
}

func TestInitWithPath_WALMode(t *testing.T) {
	db := setupTestDB(t)

	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode error: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want \"wal\"", mode)
	}
}

func TestInitWithPath_ForeignKeysEnabled(t *testing.T) {
	db := setupTestDB(t)

	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("PRAGMA foreign_keys error: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
}

func TestInitWithPath_CascadeDelete(t *testing.T) {
	db := setupTestDB(t)

	// Insert audiobook_downloads row
	res, err := db.Exec(`
		INSERT INTO audiobook_downloads (audiobook_id, title, author, source, base_path)
		VALUES ('ab1', 'Test Book', 'Author A', 'librivox', '/tmp/testbook')
	`)
	if err != nil {
		t.Fatalf("insert audiobook_downloads: %v", err)
	}
	downloadID, _ := res.LastInsertId()

	// Insert chapter_downloads row
	res, err = db.Exec(`
		INSERT INTO chapter_downloads (download_id, chapter_index, title, file_path, file_size, downloaded, status)
		VALUES (?, 0, 'Chapter 1', '/tmp/testbook/ch1.mp3', 1024, 0, 'pending')
	`, downloadID)
	if err != nil {
		t.Fatalf("insert chapter_downloads: %v", err)
	}
	chapterID, _ := res.LastInsertId()

	// Insert chunks row
	_, err = db.Exec(`
		INSERT INTO chunks (chapter_download_id, chunk_index, start_byte, end_byte, downloaded, status)
		VALUES (?, 0, 0, 512, 0, 'pending')
	`, chapterID)
	if err != nil {
		t.Fatalf("insert chunks: %v", err)
	}

	// Delete the audiobook download — cascade should remove chapters and chunks
	_, err = db.Exec("DELETE FROM audiobook_downloads WHERE id = ?", downloadID)
	if err != nil {
		t.Fatalf("delete audiobook_downloads: %v", err)
	}

	// Verify chapter is gone
	var chapterCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM chapter_downloads WHERE download_id = ?", downloadID).Scan(&chapterCount); err != nil {
		t.Fatalf("count chapter_downloads: %v", err)
	}
	if chapterCount != 0 {
		t.Errorf("expected 0 chapters after cascade delete, got %d", chapterCount)
	}

	// Verify chunk is gone
	var chunkCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM chunks WHERE chapter_download_id = ?", chapterID).Scan(&chunkCount); err != nil {
		t.Fatalf("count chunks: %v", err)
	}
	if chunkCount != 0 {
		t.Errorf("expected 0 chunks after cascade delete, got %d", chunkCount)
	}
}

func TestInitWithPath_FileCreated(t *testing.T) {
	dir := t.TempDir()
	// Use a nested subdirectory that doesn't exist yet
	dbPath := filepath.Join(dir, "nested", "subdir", "audbookdl.db")

	db, err := InitWithPath(dbPath)
	if err != nil {
		t.Fatalf("InitWithPath() error: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("database file not created at %q", dbPath)
	}
}
