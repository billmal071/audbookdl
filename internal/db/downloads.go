package db

import (
	"database/sql"
	"time"
)

// CreateDownload inserts a new AudiobookDownload row and returns the new ID.
func CreateDownload(db *sql.DB, d *AudiobookDownload) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO audiobook_downloads
			(audiobook_id, title, author, narrator, source, status, base_path, total_size, downloaded_size)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.AudiobookID, d.Title, d.Author, d.Narrator, d.Source,
		string(d.Status), d.BasePath, d.TotalSize, d.DownloadedSize,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetDownload retrieves a single AudiobookDownload by its primary key.
func GetDownload(db *sql.DB, id int64) (*AudiobookDownload, error) {
	row := db.QueryRow(`
		SELECT id, audiobook_id, title, author, narrator, source, status,
		       base_path, total_size, downloaded_size, created_at, updated_at, completed_at
		FROM audiobook_downloads
		WHERE id = ?`, id)

	d := &AudiobookDownload{}
	var completedAt sql.NullTime
	err := row.Scan(
		&d.ID, &d.AudiobookID, &d.Title, &d.Author, &d.Narrator, &d.Source,
		&d.Status, &d.BasePath, &d.TotalSize, &d.DownloadedSize,
		&d.CreatedAt, &d.UpdatedAt, &completedAt,
	)
	if err != nil {
		return nil, err
	}
	if completedAt.Valid {
		d.CompletedAt = &completedAt.Time
	}
	return d, nil
}

// ListDownloads returns all AudiobookDownload rows ordered newest-first.
func ListDownloads(db *sql.DB) ([]*AudiobookDownload, error) {
	rows, err := db.Query(`
		SELECT id, audiobook_id, title, author, narrator, source, status,
		       base_path, total_size, downloaded_size, created_at, updated_at, completed_at
		FROM audiobook_downloads
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var downloads []*AudiobookDownload
	for rows.Next() {
		d := &AudiobookDownload{}
		var completedAt sql.NullTime
		if err := rows.Scan(
			&d.ID, &d.AudiobookID, &d.Title, &d.Author, &d.Narrator, &d.Source,
			&d.Status, &d.BasePath, &d.TotalSize, &d.DownloadedSize,
			&d.CreatedAt, &d.UpdatedAt, &completedAt,
		); err != nil {
			return nil, err
		}
		if completedAt.Valid {
			d.CompletedAt = &completedAt.Time
		}
		downloads = append(downloads, d)
	}
	return downloads, rows.Err()
}

// UpdateDownloadStatus sets the status (and updated_at) for the given download.
// When status is StatusCompleted, completed_at is also set to now.
func UpdateDownloadStatus(db *sql.DB, id int64, status DownloadStatus) error {
	now := time.Now().UTC()
	if status == StatusCompleted {
		_, err := db.Exec(`
			UPDATE audiobook_downloads
			SET status = ?, updated_at = ?, completed_at = ?
			WHERE id = ?`,
			string(status), now, now, id,
		)
		return err
	}
	_, err := db.Exec(`
		UPDATE audiobook_downloads
		SET status = ?, updated_at = ?
		WHERE id = ?`,
		string(status), now, id,
	)
	return err
}

// UpdateDownloadProgress sets downloaded_size (and updated_at) for the given download.
func UpdateDownloadProgress(db *sql.DB, id int64, downloadedSize int64) error {
	_, err := db.Exec(`
		UPDATE audiobook_downloads
		SET downloaded_size = ?, updated_at = ?
		WHERE id = ?`,
		downloadedSize, time.Now().UTC(), id,
	)
	return err
}

// CreateChapterDownload inserts a new ChapterDownload row and returns the new ID.
func CreateChapterDownload(db *sql.DB, c *ChapterDownload) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO chapter_downloads
			(download_id, chapter_index, title, file_path, file_size, downloaded, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.DownloadID, c.ChapterIndex, c.Title, c.FilePath,
		c.FileSize, c.Downloaded, string(c.Status),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListChapterDownloads returns all ChapterDownload rows for a given download,
// ordered by chapter_index ascending.
func ListChapterDownloads(db *sql.DB, downloadID int64) ([]*ChapterDownload, error) {
	rows, err := db.Query(`
		SELECT id, download_id, chapter_index, title, file_path, file_size, downloaded, status
		FROM chapter_downloads
		WHERE download_id = ?
		ORDER BY chapter_index ASC`, downloadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chapters []*ChapterDownload
	for rows.Next() {
		c := &ChapterDownload{}
		if err := rows.Scan(
			&c.ID, &c.DownloadID, &c.ChapterIndex, &c.Title,
			&c.FilePath, &c.FileSize, &c.Downloaded, &c.Status,
		); err != nil {
			return nil, err
		}
		chapters = append(chapters, c)
	}
	return chapters, rows.Err()
}

// UpdateChapterStatus sets the status for the given chapter.
func UpdateChapterStatus(db *sql.DB, id int64, status DownloadStatus) error {
	_, err := db.Exec(`
		UPDATE chapter_downloads SET status = ? WHERE id = ?`,
		string(status), id,
	)
	return err
}

// UpdateChapterProgress sets the downloaded bytes for the given chapter.
func UpdateChapterProgress(db *sql.DB, id int64, downloaded int64) error {
	_, err := db.Exec(`
		UPDATE chapter_downloads SET downloaded = ? WHERE id = ?`,
		downloaded, id,
	)
	return err
}
