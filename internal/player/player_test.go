package player

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/billmal071/audbookdl/internal/db"
)

// makePlaylist creates a test playlist with the given number of chapters.
func makePlaylist(n int) *Playlist {
	chapters := make([]ChapterInfo, n)
	for i := 0; i < n; i++ {
		chapters[i] = ChapterInfo{
			Index:    i,
			Title:    "Chapter " + string(rune('A'+i)),
			FilePath: "/audio/ch" + string(rune('0'+i)) + ".mp3",
			Duration: 30 * time.Minute,
		}
	}
	return &Playlist{
		AudiobookID: "test-book",
		Title:       "Test Audiobook",
		Author:      "Test Author",
		Narrator:    "Test Narrator",
		Chapters:    chapters,
	}
}

func TestNewPlayer(t *testing.T) {
	p := NewPlayer(nil)

	if p.status != StatusStopped {
		t.Errorf("status: got %v, want StatusStopped", p.status)
	}
	if p.speed != 1.0 {
		t.Errorf("speed: got %f, want 1.0", p.speed)
	}
	if p.volume != 0.8 {
		t.Errorf("volume: got %f, want 0.8", p.volume)
	}
}

func TestPlayer_PlayPauseStop(t *testing.T) {
	p := NewPlayer(nil)
	p.Load(makePlaylist(2))

	p.Play()
	if p.status != StatusPlaying {
		t.Errorf("after Play: got %v, want StatusPlaying", p.status)
	}

	p.Pause()
	if p.status != StatusPaused {
		t.Errorf("after Pause: got %v, want StatusPaused", p.status)
	}

	p.Stop()
	if p.status != StatusStopped {
		t.Errorf("after Stop: got %v, want StatusStopped", p.status)
	}
}

func TestPlayer_ChapterNavigation(t *testing.T) {
	p := NewPlayer(nil)
	pl := makePlaylist(3)
	p.Load(pl)

	// Starts at chapter 0.
	if p.chapterIndex != 0 {
		t.Fatalf("initial chapterIndex: got %d, want 0", p.chapterIndex)
	}

	p.NextChapter()
	if p.chapterIndex != 1 {
		t.Errorf("after 1st next: got %d, want 1", p.chapterIndex)
	}

	p.NextChapter()
	if p.chapterIndex != 2 {
		t.Errorf("after 2nd next: got %d, want 2", p.chapterIndex)
	}

	// Capped at last chapter.
	p.NextChapter()
	if p.chapterIndex != 2 {
		t.Errorf("after 3rd next (cap): got %d, want 2", p.chapterIndex)
	}

	// Go back from chapter 2 (positionMS == 0, so go to previous).
	p.PrevChapter()
	if p.chapterIndex != 1 {
		t.Errorf("after prev: got %d, want 1", p.chapterIndex)
	}
}

func TestPlayer_PrevChapter_RestartsIfDeepInChapter(t *testing.T) {
	p := NewPlayer(nil)
	p.Load(makePlaylist(3))
	p.NextChapter() // chapter 1
	p.positionMS = 5000

	p.PrevChapter()

	// Should restart current chapter, not go to previous.
	if p.chapterIndex != 1 {
		t.Errorf("chapterIndex: got %d, want 1 (should not have gone back)", p.chapterIndex)
	}
	if p.positionMS != 0 {
		t.Errorf("positionMS: got %d, want 0 (should have restarted)", p.positionMS)
	}
}

func TestPlayer_SkipForwardBackward(t *testing.T) {
	p := NewPlayer(nil)
	p.Load(makePlaylist(1))

	p.SkipForward(15 * time.Second)
	if p.positionMS != 15000 {
		t.Errorf("after +15s: got %d, want 15000", p.positionMS)
	}

	p.SkipBackward(5 * time.Second)
	if p.positionMS != 10000 {
		t.Errorf("after -5s: got %d, want 10000", p.positionMS)
	}

	// Clamp to 0 when skipping back more than current position.
	p.SkipBackward(20 * time.Second)
	if p.positionMS != 0 {
		t.Errorf("after -20s (clamp): got %d, want 0", p.positionMS)
	}
}

func TestPlayer_SpeedClamping(t *testing.T) {
	p := NewPlayer(nil)

	p.SetSpeed(0.1)
	if p.speed != 0.5 {
		t.Errorf("speed 0.1 → %f, want 0.5", p.speed)
	}

	p.SetSpeed(5.0)
	if p.speed != 3.0 {
		t.Errorf("speed 5.0 → %f, want 3.0", p.speed)
	}

	p.SetSpeed(1.5)
	if p.speed != 1.5 {
		t.Errorf("speed 1.5 → %f, want 1.5", p.speed)
	}
}

func TestPlayer_VolumeClamping(t *testing.T) {
	p := NewPlayer(nil)

	p.SetVolume(-0.5)
	if p.volume != 0.0 {
		t.Errorf("volume -0.5 → %f, want 0.0", p.volume)
	}

	p.SetVolume(1.5)
	if p.volume != 1.0 {
		t.Errorf("volume 1.5 → %f, want 1.0", p.volume)
	}

	p.SetVolume(0.6)
	if p.volume != 0.6 {
		t.Errorf("volume 0.6 → %f, want 0.6", p.volume)
	}
}

func TestPlayer_GetStatus(t *testing.T) {
	p := NewPlayer(nil)
	pl := makePlaylist(3)
	p.Load(pl)
	p.Play()

	s := p.GetStatus()

	if s.Status != StatusPlaying {
		t.Errorf("Status: got %v, want StatusPlaying", s.Status)
	}
	if s.AudiobookTitle != pl.Title {
		t.Errorf("AudiobookTitle: got %q, want %q", s.AudiobookTitle, pl.Title)
	}
	if s.Author != pl.Author {
		t.Errorf("Author: got %q, want %q", s.Author, pl.Author)
	}
	if s.Narrator != pl.Narrator {
		t.Errorf("Narrator: got %q, want %q", s.Narrator, pl.Narrator)
	}
	if s.TotalChapters != 3 {
		t.Errorf("TotalChapters: got %d, want 3", s.TotalChapters)
	}
	if s.ChapterIndex != 0 {
		t.Errorf("ChapterIndex: got %d, want 0", s.ChapterIndex)
	}
	if s.ChapterTitle != pl.Chapters[0].Title {
		t.Errorf("ChapterTitle: got %q, want %q", s.ChapterTitle, pl.Chapters[0].Title)
	}
	if s.ChapterDurationMS != pl.Chapters[0].Duration.Milliseconds() {
		t.Errorf("ChapterDurationMS: got %d, want %d", s.ChapterDurationMS, pl.Chapters[0].Duration.Milliseconds())
	}
	if s.Speed != 1.0 {
		t.Errorf("Speed: got %f, want 1.0", s.Speed)
	}
	if s.Volume != 0.8 {
		t.Errorf("Volume: got %f, want 0.8", s.Volume)
	}

	// Cleanup goroutine.
	p.Stop()
}

func TestPlayer_JumpToChapter(t *testing.T) {
	p := NewPlayer(nil)
	p.Load(makePlaylist(5))
	p.Play()

	err := p.JumpToChapter(3)
	if err != nil {
		t.Fatalf("JumpToChapter: %v", err)
	}
	if p.chapterIndex != 3 {
		t.Errorf("chapterIndex: got %d, want 3", p.chapterIndex)
	}
	if p.positionMS != 0 {
		t.Errorf("positionMS: got %d, want 0", p.positionMS)
	}
	err = p.JumpToChapter(10)
	if err == nil {
		t.Error("expected error for out-of-range index")
	}
	err = p.JumpToChapter(-1)
	if err == nil {
		t.Error("expected error for negative index")
	}
	p.Stop()
}

func TestPlayer_LoadRestoresState(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := db.InitWithPath(dbPath)
	if err != nil {
		t.Fatalf("InitWithPath: %v", err)
	}
	defer database.Close()

	// Pre-save a state.
	saved := &State{
		AudiobookID:  "test-book",
		ChapterIndex: 2,
		PositionMS:   75000,
		Speed:        1.75,
	}
	if err := SaveState(database, saved); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	// Create a new player with the same database and load the same playlist.
	p := NewPlayer(database)
	p.Load(makePlaylist(3))

	if p.chapterIndex != saved.ChapterIndex {
		t.Errorf("chapterIndex: got %d, want %d", p.chapterIndex, saved.ChapterIndex)
	}
	if p.positionMS != saved.PositionMS {
		t.Errorf("positionMS: got %d, want %d", p.positionMS, saved.PositionMS)
	}
	if p.speed != saved.Speed {
		t.Errorf("speed: got %f, want %f", p.speed, saved.Speed)
	}
}
