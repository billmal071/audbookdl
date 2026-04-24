package db

import (
	"database/sql"
	"fmt"
	"time"
)

// SavePlaybackState upserts the playback state for an audiobook.
func SavePlaybackState(db *sql.DB, state *PlaybackState) error {
	_, err := db.Exec(
		`INSERT INTO playback_state (audiobook_id, chapter_index, position_ms, playback_speed, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(audiobook_id) DO UPDATE SET
		     chapter_index  = excluded.chapter_index,
		     position_ms    = excluded.position_ms,
		     playback_speed = excluded.playback_speed,
		     updated_at     = CURRENT_TIMESTAMP`,
		state.AudiobookID, state.ChapterIndex, state.PositionMS, state.PlaybackSpeed,
	)
	return err
}

// GetPlaybackState retrieves the playback state for the given audiobook ID.
// Returns an error if no state is found.
func GetPlaybackState(db *sql.DB, audiobookID string) (*PlaybackState, error) {
	state := &PlaybackState{}
	var updatedAt string
	err := db.QueryRow(
		`SELECT id, audiobook_id, chapter_index, position_ms, playback_speed, updated_at
		 FROM playback_state WHERE audiobook_id = ?`,
		audiobookID,
	).Scan(
		&state.ID, &state.AudiobookID, &state.ChapterIndex,
		&state.PositionMS, &state.PlaybackSpeed, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no playback state for audiobook_id %q", audiobookID)
	}
	if err != nil {
		return nil, err
	}
	state.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return state, nil
}
