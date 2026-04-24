package db

import (
	"database/sql"
	"time"
)

// AddSearchHistory records a new search history entry.
func AddSearchHistory(db *sql.DB, query, source string, resultCount int) error {
	_, err := db.Exec(
		`INSERT INTO search_history (query, source, result_count) VALUES (?, ?, ?)`,
		query, source, resultCount,
	)
	return err
}

// ListSearchHistory returns up to limit entries ordered by creation time descending.
func ListSearchHistory(db *sql.DB, limit int) ([]*SearchHistoryEntry, error) {
	rows, err := db.Query(
		`SELECT id, query, source, result_count, created_at
		 FROM search_history ORDER BY created_at DESC, id DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*SearchHistoryEntry
	for rows.Next() {
		e := &SearchHistoryEntry{}
		var createdAt string
		if err := rows.Scan(&e.ID, &e.Query, &e.Source, &e.ResultCount, &createdAt); err != nil {
			return nil, err
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ClearSearchHistory deletes all search history entries.
func ClearSearchHistory(db *sql.DB) error {
	_, err := db.Exec(`DELETE FROM search_history`)
	return err
}
