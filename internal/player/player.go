package player

import (
	"database/sql"
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
	mu            sync.RWMutex
	status        Status
	playlist      *Playlist
	chapterIndex  int
	positionMS    int64
	speed         float64
	volume        float64
	sleepTimer    *time.Timer
	sleepRemainMS int64
	database      *sql.DB
	saveTicker    *time.Ticker
	stopChan      chan struct{}
	engine        *Engine
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
// and delegates audio output to the engine.
func (p *Player) Play() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.status = StatusPlaying
	p.startSaveLoop()

	if p.playlist != nil {
		idx := p.chapterIndex
		if idx >= 0 && idx < len(p.playlist.Chapters) {
			ch := p.playlist.Chapters[idx]
			if p.engine != nil {
				_ = p.engine.PlayFile(ch.FilePath, p.positionMS)
			}
			// On non-CGO builds, use external player (mpv/ffplay)
			playExternal(ch.FilePath)
		}
	}
}

// Pause transitions the player to StatusPaused and pauses audio output.
func (p *Player) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.status = StatusPaused
	if p.engine != nil && p.engine.IsPlaying() {
		p.engine.PauseResume()
	}
}

// Stop halts playback, stops the save ticker, persists the final state, and
// releases engine resources.
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.status = StatusStopped
	p.stopSaveLoop()
	p.saveState()
	if p.engine != nil {
		p.engine.Stop()
	}
	stopExternal()
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
	}
}

// PrevChapter goes to the previous chapter unless positionMS > 3000ms,
// in which case it restarts the current chapter.
func (p *Player) PrevChapter() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.positionMS > 3000 {
		p.positionMS = 0
		return
	}
	if p.chapterIndex > 0 {
		p.chapterIndex--
		p.positionMS = 0
	}
}

// SkipForward adds d milliseconds to the current position.
// No upper clamping is applied — callers are responsible for chapter boundaries.
func (p *Player) SkipForward(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.positionMS += d.Milliseconds()
}

// SkipBackward subtracts d milliseconds from the current position, clamped to 0.
func (p *Player) SkipBackward(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.positionMS -= d.Milliseconds()
	if p.positionMS < 0 {
		p.positionMS = 0
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

	s := PlayerStatus{
		Status:        p.status,
		Speed:         p.speed,
		Volume:        p.volume,
		SleepRemainMS: p.sleepRemainMS,
		ChapterIndex:  p.chapterIndex,
		PositionMS:    p.positionMS,
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
