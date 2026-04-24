package player

import (
	"database/sql"
	"time"

	"github.com/billmal071/audbookdl/internal/db"
)

// State holds the persisted playback state for a single audiobook.
type State struct {
	AudiobookID  string
	ChapterIndex int
	PositionMS   int64
	Speed        float64
}

// SaveState persists the given playback state to the database.
func SaveState(database *sql.DB, s *State) error {
	return db.SavePlaybackState(database, &db.PlaybackState{
		AudiobookID:   s.AudiobookID,
		ChapterIndex:  s.ChapterIndex,
		PositionMS:    s.PositionMS,
		PlaybackSpeed: s.Speed,
	})
}

// LoadState retrieves the saved playback state for an audiobook.
// Returns nil if no state exists or on error.
func LoadState(database *sql.DB, audiobookID string) *State {
	ps, err := db.GetPlaybackState(database, audiobookID)
	if err != nil {
		return nil
	}
	return &State{
		AudiobookID:  ps.AudiobookID,
		ChapterIndex: ps.ChapterIndex,
		PositionMS:   ps.PositionMS,
		Speed:        ps.PlaybackSpeed,
	}
}

// Playlist holds the metadata and chapter list for an audiobook to be played.
type Playlist struct {
	AudiobookID string
	Title       string
	Author      string
	Narrator    string
	Chapters    []ChapterInfo
}

// ChapterInfo describes a single chapter in a playlist.
type ChapterInfo struct {
	Index    int
	Title    string
	FilePath string
	Duration time.Duration
}
