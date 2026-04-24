package db

import (
	"testing"
)

func TestSaveAndGetPlaybackState(t *testing.T) {
	db := setupTestDB(t)

	state := &PlaybackState{
		AudiobookID:   "ab-100",
		ChapterIndex:  5,
		PositionMS:    123456,
		PlaybackSpeed: 1.5,
	}

	if err := SavePlaybackState(db, state); err != nil {
		t.Fatalf("SavePlaybackState() error: %v", err)
	}

	got, err := GetPlaybackState(db, "ab-100")
	if err != nil {
		t.Fatalf("GetPlaybackState() error: %v", err)
	}

	if got.AudiobookID != state.AudiobookID {
		t.Errorf("AudiobookID = %q, want %q", got.AudiobookID, state.AudiobookID)
	}
	if got.ChapterIndex != state.ChapterIndex {
		t.Errorf("ChapterIndex = %d, want %d", got.ChapterIndex, state.ChapterIndex)
	}
	if got.PositionMS != state.PositionMS {
		t.Errorf("PositionMS = %d, want %d", got.PositionMS, state.PositionMS)
	}
	if got.PlaybackSpeed != state.PlaybackSpeed {
		t.Errorf("PlaybackSpeed = %f, want %f", got.PlaybackSpeed, state.PlaybackSpeed)
	}
}

func TestSavePlaybackState_Upsert(t *testing.T) {
	db := setupTestDB(t)

	first := &PlaybackState{
		AudiobookID:   "ab-200",
		ChapterIndex:  1,
		PositionMS:    1000,
		PlaybackSpeed: 1.0,
	}
	if err := SavePlaybackState(db, first); err != nil {
		t.Fatalf("SavePlaybackState() first save error: %v", err)
	}

	second := &PlaybackState{
		AudiobookID:   "ab-200",
		ChapterIndex:  3,
		PositionMS:    99999,
		PlaybackSpeed: 2.0,
	}
	if err := SavePlaybackState(db, second); err != nil {
		t.Fatalf("SavePlaybackState() second save error: %v", err)
	}

	// Verify only one row exists and second values win
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM playback_state WHERE audiobook_id = 'ab-200'`).Scan(&count); err != nil {
		t.Fatalf("count query error: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want 1 after upsert", count)
	}

	got, err := GetPlaybackState(db, "ab-200")
	if err != nil {
		t.Fatalf("GetPlaybackState() error: %v", err)
	}
	if got.ChapterIndex != second.ChapterIndex {
		t.Errorf("ChapterIndex = %d, want %d", got.ChapterIndex, second.ChapterIndex)
	}
	if got.PositionMS != second.PositionMS {
		t.Errorf("PositionMS = %d, want %d", got.PositionMS, second.PositionMS)
	}
	if got.PlaybackSpeed != second.PlaybackSpeed {
		t.Errorf("PlaybackSpeed = %f, want %f", got.PlaybackSpeed, second.PlaybackSpeed)
	}
}

func TestGetPlaybackState_NotFound(t *testing.T) {
	db := setupTestDB(t)

	_, err := GetPlaybackState(db, "nonexistent-audiobook")
	if err == nil {
		t.Fatal("GetPlaybackState() expected error for nonexistent audiobook, got nil")
	}
}
