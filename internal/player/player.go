package player

import (
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// Status represents the current playback state of the player.
type Status int

const (
	StatusStopped Status = iota
	StatusPlaying
	StatusPaused
)

// PlayerStatus is a thread-safe snapshot of the player's current state.
type PlayerStatus struct {
	Status            Status
	AudiobookTitle    string
	Author            string
	Narrator          string
	ChapterTitle      string
	ChapterIndex      int
	TotalChapters     int
	PositionMS        int64
	ChapterDurationMS int64
	Speed             float64
	Volume            float64
	SleepRemainMS     int64
}

// Player manages playback state for an audiobook playlist.
// Audio output is delegated to Engine, which handles MP3 decoding and speaker
// integration. The engine field is optional: if NewEngine returns nil or audio
// hardware is unavailable, the player operates as a state-only controller.
type Player struct {
	mu             sync.RWMutex
	status         Status
	playlist       *Playlist
	chapterIndex   int
	positionMS     int64
	speed          float64
	volume         float64
	sleepTimer     *time.Timer
	sleepRemainMS  int64
	playStartedAt  time.Time // when current playback started (for elapsed tracking)
	pausedPosition int64     // positionMS when paused
	database       *sql.DB
	saveTicker     *time.Ticker
	stopChan       chan struct{}
	engine         *Engine
	mpv            *MpvController
	chapterGen     uint64 // incremented on each chapter start to invalidate stale callbacks
}

// NewPlayer creates a new Player with sensible defaults.
// Pass a non-nil *sql.DB to enable state persistence.
func NewPlayer(database *sql.DB) *Player {
	return &Player{
		status:   StatusStopped,
		speed:    1.0,
		volume:   0.8,
		database: database,
		engine:   NewEngine(),
		mpv:      NewMpvController(),
	}
}

// Load sets the playlist for playback and restores saved state if available.
func (p *Player) Load(playlist *Playlist) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.playlist = playlist
	p.chapterIndex = 0
	p.positionMS = 0

	// Restore persisted state when a database is configured.
	if p.database != nil && playlist != nil {
		if s := LoadState(p.database, playlist.AudiobookID); s != nil {
			p.chapterIndex = s.ChapterIndex
			p.positionMS = s.PositionMS
			if s.Speed >= 0.5 && s.Speed <= 3.0 {
				p.speed = s.Speed
			}
		}
	}
}

// Play transitions the player to StatusPlaying, starts the periodic save goroutine,
// and delegates audio output to the engine or mpv.
// If the player is resuming from pause (mpv is still running but paused), it sends
// a resume command instead of restarting the chapter.
func (p *Player) Play() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Resume from pause: mpv is still running, just unpause it.
	if p.status == StatusPaused && p.mpv != nil && p.mpv.IsRunning() {
		_ = p.mpv.Resume()
		p.status = StatusPlaying
		p.playStartedAt = time.Now()
		p.startSaveLoop()
		return
	}

	// Resume from pause via engine.
	if p.status == StatusPaused && p.engine != nil && p.engine.IsPlaying() {
		p.engine.PauseResume()
		p.status = StatusPlaying
		p.playStartedAt = time.Now()
		p.startSaveLoop()
		return
	}

	p.status = StatusPlaying
	p.playStartedAt = time.Now()
	p.pausedPosition = p.positionMS
	p.startSaveLoop()

	if p.playlist != nil {
		idx := p.chapterIndex
		if idx >= 0 && idx < len(p.playlist.Chapters) {
			p.startChapterLocked()
		}
	}
}

// Pause transitions the player to StatusPaused and pauses audio output.
func (p *Player) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.mpv != nil && p.mpv.IsRunning() {
		if pos, err := p.mpv.GetPosition(); err == nil {
			p.positionMS = pos
		}
		_ = p.mpv.Pause()
	} else {
		// Snapshot current position based on elapsed time
		if p.status == StatusPlaying && !p.playStartedAt.IsZero() {
			elapsed := time.Since(p.playStartedAt).Milliseconds()
			p.positionMS = p.pausedPosition + int64(float64(elapsed)*p.speed)
		}
		stopExternal()
		if p.engine != nil && p.engine.IsPlaying() {
			p.engine.PauseResume()
		}
	}
	p.pausedPosition = p.positionMS
	p.status = StatusPaused
	p.saveState()
}

// Stop halts playback, stops the save ticker, persists the final state, and
// releases engine resources.
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.mpv != nil && p.mpv.IsRunning() {
		if pos, err := p.mpv.GetPosition(); err == nil {
			p.positionMS = pos
		}
		p.mpv.Stop()
	} else {
		if p.engine != nil {
			p.engine.Stop()
		}
		stopExternal()
	}
	p.status = StatusStopped
	p.stopSaveLoop()
	p.saveState()
}

// NextChapter advances to the next chapter, capped at the last chapter.
// Position is reset to 0.
func (p *Player) NextChapter() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.playlist == nil {
		return
	}
	last := len(p.playlist.Chapters) - 1
	if p.chapterIndex < last {
		p.chapterIndex++
		p.positionMS = 0
		p.pausedPosition = 0
		p.playStartedAt = time.Now()
		if p.status == StatusPlaying {
			p.startChapterLocked()
		}
	}
}

// PrevChapter goes to the previous chapter unless we're more than 3 seconds in,
// in which case it restarts the current chapter.
func (p *Player) PrevChapter() {
	p.mu.Lock()
	defer p.mu.Unlock()

	currentPos := p.livePosition()

	if currentPos > 3000 {
		p.positionMS = 0
		p.pausedPosition = 0
		p.playStartedAt = time.Now()
		if p.status == StatusPlaying {
			p.startChapterLocked()
		}
		return
	}
	if p.chapterIndex > 0 {
		p.chapterIndex--
		p.positionMS = 0
		p.pausedPosition = 0
		p.playStartedAt = time.Now()
		if p.status == StatusPlaying {
			p.startChapterLocked()
		}
	}
}

// SkipForward adds d milliseconds to the current position.
// No upper clamping is applied — callers are responsible for chapter boundaries.
func (p *Player) SkipForward(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	pos := p.livePosition()
	pos += d.Milliseconds()
	p.positionMS = pos
	p.pausedPosition = pos
	p.playStartedAt = time.Now()

	if p.mpv != nil && p.mpv.IsRunning() {
		_ = p.mpv.SeekTo(pos)
	}
}

// SkipBackward subtracts d milliseconds from the current position, clamped to 0.
func (p *Player) SkipBackward(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	pos := p.livePosition()
	pos -= d.Milliseconds()
	if pos < 0 {
		pos = 0
	}
	p.positionMS = pos
	p.pausedPosition = pos
	p.playStartedAt = time.Now()

	if p.mpv != nil && p.mpv.IsRunning() {
		_ = p.mpv.SeekTo(pos)
	}
}

// SetSpeed sets the playback speed, clamped to [0.5, 3.0], and updates the engine.
func (p *Player) SetSpeed(speed float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if speed < 0.5 {
		speed = 0.5
	} else if speed > 3.0 {
		speed = 3.0
	}
	p.speed = speed
	if p.mpv != nil && p.mpv.IsRunning() {
		_ = p.mpv.SetSpeed(speed)
	}
	if p.engine != nil {
		p.engine.SetSpeed(speed)
	}
}

// SetVolume sets the playback volume, clamped to [0.0, 1.0], and updates the engine.
func (p *Player) SetVolume(vol float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if vol < 0.0 {
		vol = 0.0
	} else if vol > 1.0 {
		vol = 1.0
	}
	p.volume = vol
	if p.mpv != nil && p.mpv.IsRunning() {
		_ = p.mpv.SetVolume(vol)
	}
	if p.engine != nil {
		p.engine.SetVolume(vol)
	}
}

// SetSleepTimer schedules a Pause after duration d.
// If d <= 0, any existing timer is cancelled.
func (p *Player) SetSleepTimer(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Cancel any existing timer.
	if p.sleepTimer != nil {
		p.sleepTimer.Stop()
		p.sleepTimer = nil
		p.sleepRemainMS = 0
	}

	if d <= 0 {
		return
	}

	p.sleepRemainMS = d.Milliseconds()
	p.sleepTimer = time.AfterFunc(d, func() {
		p.mu.Lock()
		p.sleepRemainMS = 0
		p.sleepTimer = nil
		p.mu.Unlock()
		p.Pause()
	})
}

// GetStatus returns a snapshot of the current player status.
func (p *Player) GetStatus() PlayerStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	posMS := p.livePosition()

	s := PlayerStatus{
		Status:        p.status,
		Speed:         p.speed,
		Volume:        p.volume,
		SleepRemainMS: p.sleepRemainMS,
		ChapterIndex:  p.chapterIndex,
		PositionMS:    posMS,
	}

	if p.playlist != nil {
		s.AudiobookTitle = p.playlist.Title
		s.Author = p.playlist.Author
		s.Narrator = p.playlist.Narrator
		s.TotalChapters = len(p.playlist.Chapters)

		if p.chapterIndex >= 0 && p.chapterIndex < len(p.playlist.Chapters) {
			ch := p.playlist.Chapters[p.chapterIndex]
			s.ChapterTitle = ch.Title
			s.ChapterDurationMS = ch.Duration.Milliseconds()
		}
	}

	return s
}

// JumpToChapter jumps directly to the given chapter index.
func (p *Player) JumpToChapter(index int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.playlist == nil {
		return fmt.Errorf("no playlist loaded")
	}
	if index < 0 || index >= len(p.playlist.Chapters) {
		return fmt.Errorf("chapter index %d out of range [0, %d)", index, len(p.playlist.Chapters))
	}

	p.chapterIndex = index
	p.positionMS = 0
	p.pausedPosition = 0
	p.playStartedAt = time.Now()

	if p.status == StatusPlaying {
		p.startChapterLocked()
	}
	return nil
}

// GetPlaylist returns the current playlist.
func (p *Player) GetPlaylist() *Playlist {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.playlist
}

// startChapterLocked starts playing the current chapter. Caller must hold p.mu write lock.
// Lock ordering: caller holds p.mu; this may acquire mpv.mu. Never acquire mpv.mu before p.mu.
func (p *Player) startChapterLocked() {
	p.chapterGen++
	ch := p.playlist.Chapters[p.chapterIndex]
	if p.mpv != nil {
		p.mpv.Stop()
		_ = p.mpv.Start(ch.FilePath, p.positionMS)
		_ = p.mpv.SetSpeed(p.speed)
		_ = p.mpv.SetVolume(p.volume)
		gen := p.chapterGen
		p.mpv.SetOnEndFile(func() {
			p.onChapterEnd(gen)
		})
	} else if p.engine != nil {
		p.engine.Stop()
		_ = p.engine.PlayFile(ch.FilePath, p.positionMS)
	} else {
		playExternal(ch.FilePath)
	}
}

// onChapterEnd is called by mpv when the current file finishes playing.
// It advances to the next chapter or stops if at the end.
// The gen parameter identifies which chapter start this callback belongs to;
// stale callbacks from a previous chapter are silently discarded.
func (p *Player) onChapterEnd(gen uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.chapterGen != gen {
		return // stale callback from a previous chapter
	}

	if p.status != StatusPlaying || p.playlist == nil {
		return
	}

	last := len(p.playlist.Chapters) - 1
	if p.chapterIndex < last {
		p.chapterIndex++
		p.positionMS = 0
		p.pausedPosition = 0
		p.playStartedAt = time.Now()
		p.startChapterLocked()
	} else {
		// End of book — stop playback.
		p.status = StatusStopped
		p.stopSaveLoop()
		p.saveState()
	}
}

// livePosition returns the current position. Caller must hold p.mu (read or write).
// Lock ordering: may acquire mpv.mu.
func (p *Player) livePosition() int64 {
	if p.mpv != nil && p.mpv.IsRunning() {
		if pos, err := p.mpv.GetPosition(); err == nil {
			return pos
		}
	}
	if p.status == StatusPlaying && !p.playStartedAt.IsZero() {
		elapsed := time.Since(p.playStartedAt).Milliseconds()
		return p.pausedPosition + int64(float64(elapsed)*p.speed)
	}
	return p.positionMS
}

// startSaveLoop launches the periodic save goroutine.
// Caller must hold p.mu (write lock).
func (p *Player) startSaveLoop() {
	if p.stopChan != nil {
		// already running
		return
	}
	p.stopChan = make(chan struct{})
	p.saveTicker = time.NewTicker(5 * time.Second)
	go p.saveLoop(p.stopChan, p.saveTicker)
}

// stopSaveLoop stops the periodic save goroutine.
// Caller must hold p.mu (write lock).
func (p *Player) stopSaveLoop() {
	if p.stopChan == nil {
		return
	}
	close(p.stopChan)
	p.stopChan = nil
	if p.saveTicker != nil {
		p.saveTicker.Stop()
		p.saveTicker = nil
	}
}

// saveLoop runs in a goroutine and persists state every tick.
func (p *Player) saveLoop(stop <-chan struct{}, ticker *time.Ticker) {
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			p.mu.Lock()
			p.saveState()
			p.mu.Unlock()
		}
	}
}

// saveState persists the current playback state.
// Caller must hold p.mu (at least read lock; write lock preferred for consistency).
func (p *Player) saveState() {
	if p.database == nil || p.playlist == nil {
		return
	}
	_ = SaveState(p.database, &State{
		AudiobookID:  p.playlist.AudiobookID,
		ChapterIndex: p.chapterIndex,
		PositionMS:   p.positionMS,
		Speed:        p.speed,
	})
}
