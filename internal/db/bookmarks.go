package db

import (
	"database/sql"
	"time"
)

// CreateBookmark inserts a new bookmark and returns its ID.
func CreateBookmark(db *sql.DB, bm *Bookmark) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO bookmarks (audiobook_id, title, author, narrator, source, page_url, note)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		bm.AudiobookID, bm.Title, bm.Author, bm.Narrator, bm.Source, bm.PageURL, bm.Note,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListBookmarks returns all bookmarks ordered by creation time descending.
func ListBookmarks(db *sql.DB) ([]*Bookmark, error) {
	rows, err := db.Query(
		`SELECT id, audiobook_id, title, author, narrator, source, page_url, note, created_at
		 FROM bookmarks ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookmarks []*Bookmark
	for rows.Next() {
		bm := &Bookmark{}
		var createdAt string
		if err := rows.Scan(
			&bm.ID, &bm.AudiobookID, &bm.Title, &bm.Author,
			&bm.Narrator, &bm.Source, &bm.PageURL, &bm.Note, &createdAt,
		); err != nil {
			return nil, err
		}
		bm.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		bookmarks = append(bookmarks, bm)
	}
	return bookmarks, rows.Err()
}

// DeleteBookmark removes the bookmark with the given ID.
func DeleteBookmark(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM bookmarks WHERE id = ?`, id)
	return err
}
