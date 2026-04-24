package db

import "time"

// DownloadStatus represents the state of a download or chapter.
type DownloadStatus string

const (
	StatusPending     DownloadStatus = "pending"
	StatusDownloading DownloadStatus = "downloading"
	StatusCompleted   DownloadStatus = "completed"
	StatusFailed      DownloadStatus = "failed"
	StatusPaused      DownloadStatus = "paused"
)

// AudiobookDownload maps to the audiobook_downloads table.
type AudiobookDownload struct {
	ID             int64
	AudiobookID    string
	Title          string
	Author         string
	Narrator       string
	Source         string
	Status         DownloadStatus
	BasePath       string
	TotalSize      int64
	DownloadedSize int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
}

// ChapterDownload maps to the chapter_downloads table.
type ChapterDownload struct {
	ID           int64
	DownloadID   int64
	ChapterIndex int
	Title        string
	FilePath     string
	FileSize     int64
	Downloaded   int64
	Status       DownloadStatus
}

// Chunk maps to the chunks table.
type Chunk struct {
	ID                int64
	ChapterDownloadID int64
	ChunkIndex        int
	StartByte         int64
	EndByte           int64
	Downloaded        int64
	Status            DownloadStatus
}

// Bookmark maps to the bookmarks table.
type Bookmark struct {
	ID          int64
	AudiobookID string
	Title       string
	Author      string
	Narrator    string
	Source      string
	PageURL     string
	Note        string
	CreatedAt   time.Time
}

// PlaybackState maps to the playback_state table.
type PlaybackState struct {
	ID            int64
	AudiobookID   string
	ChapterIndex  int
	PositionMS    int64
	PlaybackSpeed float64
	UpdatedAt     time.Time
}

// SearchHistoryEntry maps to the search_history table.
type SearchHistoryEntry struct {
	ID          int64
	Query       string
	Source      string
	ResultCount int
	CreatedAt   time.Time
}
