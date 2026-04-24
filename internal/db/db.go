package db

import (
	"database/sql"
	"os"
	"path/filepath"

	"github.com/billmal071/audbookdl/internal/config"
	_ "modernc.org/sqlite"
)

var database *sql.DB

const schema = `
CREATE TABLE IF NOT EXISTS audiobook_downloads (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    audiobook_id     TEXT NOT NULL,
    title            TEXT NOT NULL,
    author           TEXT NOT NULL,
    narrator         TEXT NOT NULL DEFAULT '',
    source           TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'pending',
    base_path        TEXT NOT NULL,
    total_size       INTEGER DEFAULT 0,
    downloaded_size  INTEGER DEFAULT 0,
    created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at     DATETIME
);

CREATE INDEX IF NOT EXISTS idx_audiobook_downloads_status      ON audiobook_downloads(status);
CREATE INDEX IF NOT EXISTS idx_audiobook_downloads_audiobook_id ON audiobook_downloads(audiobook_id);

CREATE TABLE IF NOT EXISTS chapter_downloads (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    download_id         INTEGER NOT NULL,
    chapter_index       INTEGER NOT NULL,
    title               TEXT NOT NULL DEFAULT '',
    file_path           TEXT NOT NULL DEFAULT '',
    file_size           INTEGER DEFAULT 0,
    downloaded          INTEGER DEFAULT 0,
    status              TEXT NOT NULL DEFAULT 'pending',
    FOREIGN KEY (download_id) REFERENCES audiobook_downloads(id) ON DELETE CASCADE,
    UNIQUE(download_id, chapter_index)
);

CREATE INDEX IF NOT EXISTS idx_chapter_downloads_download ON chapter_downloads(download_id);

CREATE TABLE IF NOT EXISTS chunks (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    chapter_download_id INTEGER NOT NULL,
    chunk_index         INTEGER NOT NULL,
    start_byte          INTEGER NOT NULL,
    end_byte            INTEGER NOT NULL,
    downloaded          INTEGER DEFAULT 0,
    status              TEXT NOT NULL DEFAULT 'pending',
    FOREIGN KEY (chapter_download_id) REFERENCES chapter_downloads(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_chunks_chapter ON chunks(chapter_download_id);

CREATE TABLE IF NOT EXISTS bookmarks (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    audiobook_id TEXT NOT NULL,
    title        TEXT NOT NULL,
    author       TEXT NOT NULL,
    narrator     TEXT NOT NULL DEFAULT '',
    source       TEXT NOT NULL DEFAULT '',
    page_url     TEXT NOT NULL DEFAULT '',
    note         TEXT NOT NULL DEFAULT '',
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_bookmarks_audiobook_id ON bookmarks(audiobook_id);

CREATE TABLE IF NOT EXISTS playback_state (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    audiobook_id   TEXT NOT NULL UNIQUE,
    chapter_index  INTEGER NOT NULL DEFAULT 0,
    position_ms    INTEGER NOT NULL DEFAULT 0,
    playback_speed REAL NOT NULL DEFAULT 1.0,
    updated_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS search_history (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    query        TEXT NOT NULL,
    source       TEXT NOT NULL DEFAULT '',
    result_count INTEGER DEFAULT 0,
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_search_history_created ON search_history(created_at DESC);

CREATE TABLE IF NOT EXISTS search_cache (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    cache_key  TEXT NOT NULL UNIQUE,
    results    BLOB NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_search_cache_key     ON search_cache(cache_key);
CREATE INDEX IF NOT EXISTS idx_search_cache_expires ON search_cache(expires_at);
`

// InitWithPath opens (or creates) a SQLite database at dbPath,
// applies PRAGMAs, and creates the schema. Returns the *sql.DB.
func InitWithPath(dbPath string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// SQLite works best with a single connection (one write lock)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Enable foreign key enforcement
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, err
	}

	// Wait up to 10 s before returning SQLITE_BUSY
	if _, err := db.Exec("PRAGMA busy_timeout = 10000"); err != nil {
		db.Close()
		return nil, err
	}

	// WAL mode — better concurrent read/write performance
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		db.Close()
		return nil, err
	}

	// NORMAL sync — safe in WAL mode, faster than FULL
	if _, err := db.Exec("PRAGMA synchronous = NORMAL"); err != nil {
		db.Close()
		return nil, err
	}

	// Create all tables and indexes
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// Init initializes the package-level database using the path from config.
func Init() error {
	db, err := InitWithPath(config.GetDBPath())
	if err != nil {
		return err
	}
	database = db
	return nil
}

// DB returns the package-level *sql.DB.
func DB() *sql.DB {
	return database
}

// Close closes the package-level database connection.
func Close() error {
	if database != nil {
		return database.Close()
	}
	return nil
}
