package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
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

// GetCachedSearch retrieves cached search results if not expired.
func GetCachedSearch(database *sql.DB, query, source string) ([]byte, error) {
	key := cacheKey(query, source)
	var results []byte
	err := database.QueryRow(
		"SELECT results FROM search_cache WHERE cache_key = ? AND expires_at > ?",
		key, time.Now(),
	).Scan(&results)
	if err != nil {
		return nil, err
	}
	return results, nil
}

// SetCachedSearch stores search results in the cache.
func SetCachedSearch(database *sql.DB, query, source string, results []byte, ttl time.Duration) error {
	key := cacheKey(query, source)
	_, err := database.Exec(`
		INSERT INTO search_cache (cache_key, results, expires_at)
		VALUES (?, ?, ?)
		ON CONFLICT(cache_key) DO UPDATE SET
			results = excluded.results,
			expires_at = excluded.expires_at,
			created_at = CURRENT_TIMESTAMP
	`, key, results, time.Now().Add(ttl))
	return err
}

// CleanExpiredCache removes expired cache entries.
func CleanExpiredCache(database *sql.DB) error {
	_, err := database.Exec("DELETE FROM search_cache WHERE expires_at < ?", time.Now())
	return err
}

// cacheKey returns a deterministic hex key for (query, source) pair.
func cacheKey(query, source string) string {
	h := sha256.Sum256([]byte(query + "|" + source))
	return hex.EncodeToString(h[:])
}
