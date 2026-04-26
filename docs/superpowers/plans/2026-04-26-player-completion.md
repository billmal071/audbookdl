# Player Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the audbookdl audio player with proper mpv IPC control, real-time position tracking, chapter duration detection, and TUI enhancements.

**Architecture:** Replace the fire-and-forget external player subprocess with an mpv IPC controller that communicates over a Unix socket. The Player struct delegates to this controller for all audio operations (pause, seek, speed, volume, position queries). Duration detection uses ffprobe. The TUI gains sleep timer cycling, a chapter jump modal, and seek feedback.

**Tech Stack:** Go 1.22+, mpv (IPC via `--input-ipc-server`), ffprobe, bubbletea/bubbles/lipgloss, Unix domain sockets.

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/player/mpv.go` | Create | mpv IPC controller — launch, command, query |
| `internal/player/mpv_test.go` | Create | Tests for mpv controller (mocked socket) |
| `internal/player/probe.go` | Create | ffprobe duration detection |
| `internal/player/probe_test.go` | Create | Tests for probe (mocked exec) |
| `internal/player/player.go` | Modify | Wire mpv controller, add JumpToChapter, fix seek |
| `internal/player/player_test.go` | Modify | Add tests for JumpToChapter, mpv-based seek |
| `internal/player/engine_speaker_stub.go` | Modify | Remove playExternal/stopExternal bodies (mpv.go replaces them) |
| `internal/player/engine_speaker_cgo.go` | Modify | Remove playExternal/stopExternal stubs |
| `internal/tui/playerui.go` | Modify | Sleep timer cycle, chapter jump modal, seek feedback, help line |
| `internal/tui/library.go` | Modify | Call ProbeChapterDurations in buildPlaylist |
| `internal/tui/search.go` | Modify | Gate j/k behind input focus check |
| `internal/cli/play.go` | Modify | Rewrite to use MpvController with state persistence |

---

### Task 1: mpv IPC Controller

**Files:**
- Create: `internal/player/mpv.go`
- Create: `internal/player/mpv_test.go`

- [ ] **Step 1: Write the mpv controller**

```go
// internal/player/mpv.go
package player

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// MpvController manages an mpv subprocess via JSON IPC over a Unix socket.
type MpvController struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	conn       net.Conn
	socketPath string
	requestID  atomic.Int64
	responses  map[int64]chan json.RawMessage
	respMu     sync.Mutex
	running    bool
}

type mpvCommand struct {
	Command   []interface{} `json:"command"`
	RequestID int64         `json:"request_id"`
}

type mpvResponse struct {
	Data      json.RawMessage `json:"data"`
	RequestID int64           `json:"request_id"`
	Error     string          `json:"error"`
}

// NewMpvController returns a controller if mpv is on PATH, nil otherwise.
func NewMpvController() *MpvController {
	if _, err := exec.LookPath("mpv"); err != nil {
		return nil
	}
	return &MpvController{
		socketPath: fmt.Sprintf("/tmp/audbookdl-mpv-%d.sock", os.Getpid()),
		responses:  make(map[int64]chan json.RawMessage),
	}
}

// Start launches mpv playing filePath, seeking to positionMS.
func (m *MpvController) Start(filePath string, positionMS int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop any existing instance.
	m.stopLocked()

	// Remove stale socket.
	os.Remove(m.socketPath)

	startSec := fmt.Sprintf("--start=%d", positionMS/1000)
	m.cmd = exec.Command("mpv",
		"--no-video",
		"--really-quiet",
		"--idle=no",
		"--input-ipc-server="+m.socketPath,
		startSec,
		filePath,
	)
	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("start mpv: %w", err)
	}
	m.running = true

	// Wait for the socket to appear (mpv needs a moment).
	var conn net.Conn
	var err error
	for i := 0; i < 50; i++ {
		conn, err = net.Dial("unix", m.socketPath)
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		m.stopLocked()
		return fmt.Errorf("connect to mpv socket: %w", err)
	}
	m.conn = conn

	// Start response reader.
	go m.readResponses()

	return nil
}

// readResponses reads JSON lines from the socket and dispatches to waiting callers.
func (m *MpvController) readResponses() {
	dec := json.NewDecoder(m.conn)
	for {
		var resp mpvResponse
		if err := dec.Decode(&resp); err != nil {
			return
		}
		// Only dispatch responses with a request_id (skip events).
		if resp.RequestID == 0 {
			continue
		}
		m.respMu.Lock()
		ch, ok := m.responses[resp.RequestID]
		if ok {
			delete(m.responses, resp.RequestID)
		}
		m.respMu.Unlock()
		if ok {
			ch <- resp.Data
		}
	}
}

// sendCommand sends a command and waits for the response (up to 2 seconds).
func (m *MpvController) sendCommand(args ...interface{}) (json.RawMessage, error) {
	m.mu.Lock()
	if m.conn == nil {
		m.mu.Unlock()
		return nil, errors.New("mpv not connected")
	}
	conn := m.conn
	m.mu.Unlock()

	id := m.requestID.Add(1)
	ch := make(chan json.RawMessage, 1)

	m.respMu.Lock()
	m.responses[id] = ch
	m.respMu.Unlock()

	cmd := mpvCommand{Command: args, RequestID: id}
	data, _ := json.Marshal(cmd)
	data = append(data, '\n')

	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(data); err != nil {
		m.respMu.Lock()
		delete(m.responses, id)
		m.respMu.Unlock()
		return nil, fmt.Errorf("write to mpv: %w", err)
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(2 * time.Second):
		m.respMu.Lock()
		delete(m.responses, id)
		m.respMu.Unlock()
		return nil, errors.New("mpv command timeout")
	}
}

// Pause pauses mpv playback.
func (m *MpvController) Pause() error {
	_, err := m.sendCommand("set_property", "pause", true)
	return err
}

// Resume resumes mpv playback.
func (m *MpvController) Resume() error {
	_, err := m.sendCommand("set_property", "pause", false)
	return err
}

// Seek seeks to an absolute position in milliseconds.
func (m *MpvController) Seek(positionMS int64) error {
	_, err := m.sendCommand("seek", float64(positionMS)/1000.0, "absolute")
	return err
}

// SetSpeed sets the playback speed.
func (m *MpvController) SetSpeed(rate float64) error {
	_, err := m.sendCommand("set_property", "speed", rate)
	return err
}

// SetVolume sets volume (0.0-1.0 mapped to 0-100).
func (m *MpvController) SetVolume(vol float64) error {
	_, err := m.sendCommand("set_property", "volume", vol*100)
	return err
}

// GetPosition returns the current playback position in milliseconds.
func (m *MpvController) GetPosition() (int64, error) {
	data, err := m.sendCommand("get_property", "time-pos")
	if err != nil {
		return 0, err
	}
	var secs float64
	if err := json.Unmarshal(data, &secs); err != nil {
		return 0, err
	}
	return int64(secs * 1000), nil
}

// GetDuration returns the current file's duration in milliseconds.
func (m *MpvController) GetDuration() (int64, error) {
	data, err := m.sendCommand("get_property", "duration")
	if err != nil {
		return 0, err
	}
	var secs float64
	if err := json.Unmarshal(data, &secs); err != nil {
		return 0, err
	}
	return int64(secs * 1000), nil
}

// Stop stops mpv and cleans up.
func (m *MpvController) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked()
}

func (m *MpvController) stopLocked() {
	if m.conn != nil {
		// Try graceful quit first.
		cmd := mpvCommand{Command: []interface{}{"quit"}, RequestID: 0}
		data, _ := json.Marshal(cmd)
		data = append(data, '\n')
		m.conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
		m.conn.Write(data)
		m.conn.Close()
		m.conn = nil
	}
	if m.cmd != nil && m.cmd.Process != nil {
		m.cmd.Process.Kill()
		m.cmd.Wait()
		m.cmd = nil
	}
	os.Remove(m.socketPath)
	m.running = false

	// Drain pending response channels.
	m.respMu.Lock()
	for id, ch := range m.responses {
		close(ch)
		delete(m.responses, id)
	}
	m.respMu.Unlock()
}

// IsRunning reports whether mpv is currently running.
func (m *MpvController) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}
```

- [ ] **Step 2: Write tests for the mpv controller**

```go
// internal/player/mpv_test.go
package player

import (
	"encoding/json"
	"os/exec"
	"testing"
)

func TestNewMpvController_NoMpv(t *testing.T) {
	// This test verifies the constructor doesn't panic.
	// If mpv is not installed, it returns nil gracefully.
	ctrl := NewMpvController()
	if ctrl != nil {
		// mpv is on PATH — verify socketPath is set.
		if ctrl.socketPath == "" {
			t.Error("socketPath should be set when mpv is available")
		}
	}
}

func TestMpvController_SendCommand_NotConnected(t *testing.T) {
	ctrl := &MpvController{
		responses: make(map[int64]chan json.RawMessage),
	}
	// Should return error when not connected.
	_, err := ctrl.sendCommand("get_property", "time-pos")
	if err == nil {
		t.Error("expected error when mpv not connected")
	}
}

func TestMpvController_StopWhenNotRunning(t *testing.T) {
	ctrl := &MpvController{
		responses: make(map[int64]chan json.RawMessage),
	}
	// Should not panic.
	ctrl.Stop()
	if ctrl.IsRunning() {
		t.Error("should not be running after Stop")
	}
}

func TestMpvController_IsRunning_Default(t *testing.T) {
	ctrl := &MpvController{
		responses: make(map[int64]chan json.RawMessage),
	}
	if ctrl.IsRunning() {
		t.Error("new controller should not be running")
	}
}

func TestMpvController_Integration(t *testing.T) {
	// Skip if mpv is not installed.
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("mpv not installed, skipping integration test")
	}
	// Integration tests with real mpv require audio files,
	// which are not available in CI. Skip for now.
	t.Skip("integration tests require audio files")
}
```

- [ ] **Step 3: Run tests to verify they pass**

Run: `cd ~/Documents/personal/audbookdl && go test ./internal/player/ -run TestMpv -v`
Expected: PASS (tests that need mpv skip gracefully)

- [ ] **Step 4: Commit**

```bash
git add internal/player/mpv.go internal/player/mpv_test.go
git commit -m "feat(player): add mpv IPC controller for audio playback"
```

---

### Task 2: Wire MpvController into Player

**Files:**
- Modify: `internal/player/player.go`
- Modify: `internal/player/engine_speaker_stub.go`
- Modify: `internal/player/engine_speaker_cgo.go`
- Modify: `internal/player/player_test.go`

- [ ] **Step 1: Write test for JumpToChapter**

Add to `internal/player/player_test.go`:

```go
func TestPlayer_JumpToChapter(t *testing.T) {
	p := NewPlayer(nil)
	p.Load(makePlaylist(5))
	p.Play()

	// Jump to chapter 3.
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

	// Invalid index.
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd ~/Documents/personal/audbookdl && go test ./internal/player/ -run TestPlayer_JumpToChapter -v`
Expected: FAIL — `JumpToChapter` method does not exist yet

- [ ] **Step 3: Modify player.go — add mpv field, JumpToChapter, and rewire methods**

In `internal/player/player.go`, add the `mpv` field to the Player struct (after the `engine` field at line 53):

```go
	engine         *Engine
	mpv            *MpvController
```

Modify `NewPlayer` (line 58-66) to create the mpv controller:

```go
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
```

Replace the `Play` method (lines 91-111) with:

```go
func (p *Player) Play() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.playlist == nil {
		return
	}

	p.status = StatusPlaying
	p.playStartedAt = time.Now()
	p.pausedPosition = p.positionMS
	p.startSaveLoop()

	idx := p.chapterIndex
	if idx >= 0 && idx < len(p.playlist.Chapters) {
		ch := p.playlist.Chapters[idx]
		if p.mpv != nil {
			_ = p.mpv.Start(ch.FilePath, p.positionMS)
			_ = p.mpv.SetSpeed(p.speed)
			_ = p.mpv.SetVolume(p.volume)
		} else if p.engine != nil {
			_ = p.engine.PlayFile(ch.FilePath, p.positionMS)
		}
	}
}
```

Replace the `Pause` method (lines 114-128) with:

```go
func (p *Player) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.mpv != nil && p.mpv.IsRunning() {
		if pos, err := p.mpv.GetPosition(); err == nil {
			p.positionMS = pos
		}
		p.mpv.Pause()
	} else {
		// Fallback: snapshot position from elapsed time
		if p.status == StatusPlaying && !p.playStartedAt.IsZero() {
			elapsed := time.Since(p.playStartedAt).Milliseconds()
			p.positionMS = p.pausedPosition + int64(float64(elapsed)*p.speed)
		}
		if p.engine != nil && p.engine.IsPlaying() {
			p.engine.PauseResume()
		}
	}
	p.pausedPosition = p.positionMS
	p.status = StatusPaused
}
```

Replace the `Stop` method (lines 132-143) with:

```go
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.mpv != nil && p.mpv.IsRunning() {
		if pos, err := p.mpv.GetPosition(); err == nil {
			p.positionMS = pos
		}
		p.mpv.Stop()
	}
	p.status = StatusStopped
	p.stopSaveLoop()
	p.saveState()
	if p.engine != nil {
		p.engine.Stop()
	}
}
```

Replace `NextChapter` (lines 147-167) with:

```go
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
```

Replace `PrevChapter` (lines 171-204) with:

```go
func (p *Player) PrevChapter() {
	p.mu.Lock()
	defer p.mu.Unlock()

	currentPos := p.livePositionLocked()

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
```

Replace `SkipForward` (lines 208-213) with:

```go
func (p *Player) SkipForward(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.positionMS = p.livePositionLocked() + d.Milliseconds()
	p.pausedPosition = p.positionMS
	p.playStartedAt = time.Now()

	if p.mpv != nil && p.mpv.IsRunning() {
		p.mpv.Seek(p.positionMS)
	}
}
```

Replace `SkipBackward` (lines 216-224) with:

```go
func (p *Player) SkipBackward(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.positionMS = p.livePositionLocked() - d.Milliseconds()
	if p.positionMS < 0 {
		p.positionMS = 0
	}
	p.pausedPosition = p.positionMS
	p.playStartedAt = time.Now()

	if p.mpv != nil && p.mpv.IsRunning() {
		p.mpv.Seek(p.positionMS)
	}
}
```

Replace `SetSpeed` (lines 227-240) with:

```go
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
		p.mpv.SetSpeed(speed)
	}
	if p.engine != nil {
		p.engine.SetSpeed(speed)
	}
}
```

Replace `SetVolume` (lines 243-256) with:

```go
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
		p.mpv.SetVolume(vol)
	}
	if p.engine != nil {
		p.engine.SetVolume(vol)
	}
}
```

Replace `GetStatus` (lines 286-320) — change the position calculation:

```go
func (p *Player) GetStatus() PlayerStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	posMS := p.livePositionRLocked()

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
```

Add these new helper methods after `GetStatus`:

```go
// JumpToChapter jumps to the specified chapter index and starts playback at 0.
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

// startChapterLocked starts playing the current chapter.
// Caller must hold p.mu write lock.
func (p *Player) startChapterLocked() {
	ch := p.playlist.Chapters[p.chapterIndex]
	if p.mpv != nil {
		p.mpv.Stop()
		_ = p.mpv.Start(ch.FilePath, p.positionMS)
		_ = p.mpv.SetSpeed(p.speed)
		_ = p.mpv.SetVolume(p.volume)
	} else if p.engine != nil {
		p.engine.Stop()
		_ = p.engine.PlayFile(ch.FilePath, p.positionMS)
	}
}

// livePositionLocked returns the current position, querying mpv if available.
// Caller must hold p.mu write lock.
func (p *Player) livePositionLocked() int64 {
	if p.mpv != nil && p.mpv.IsRunning() {
		if pos, err := p.mpv.GetPosition(); err == nil {
			return pos
		}
	}
	// Fallback: elapsed-time estimation.
	if p.status == StatusPlaying && !p.playStartedAt.IsZero() {
		elapsed := time.Since(p.playStartedAt).Milliseconds()
		return p.pausedPosition + int64(float64(elapsed)*p.speed)
	}
	return p.positionMS
}

// livePositionRLocked returns the current position using read-lock-safe operations.
// Caller must hold p.mu read lock.
func (p *Player) livePositionRLocked() int64 {
	// mpv.GetPosition is thread-safe on its own, so safe to call under RLock.
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
```

Add `"fmt"` to the imports at the top of player.go.

- [ ] **Step 4: Remove playExternal/stopExternal from engine_speaker_stub.go**

Replace `internal/player/engine_speaker_stub.go` (lines 46-77) — remove the `playExternal` and `stopExternal` function bodies, keeping only stubs:

```go
// playExternal is a no-op. Audio is handled by MpvController.
func playExternal(filePath string) error { return nil }

// stopExternal is a no-op. Audio is handled by MpvController.
func stopExternal() {}
```

Also remove the unused `extPlayer`, `extPlayerMu`, and `extPaused` variables (lines 14-17), and the `"sync"` import since it's no longer needed. Remove the old body from `speakerClear` that referenced `extPlayer`:

```go
func speakerClear() {}
```

The full replacement for `engine_speaker_stub.go`:

```go
//go:build !cgo

package player

import (
	"github.com/gopxl/beep/v2"
)

func speakerPlay(format beep.Format, streamer beep.Streamer) error {
	return nil
}

func speakerPauseResume() {}
func speakerClear()       {}
func speakerLock()        {}
func speakerUnlock()      {}

// playExternal is a no-op. Audio is handled by MpvController.
func playExternal(filePath string) error { return nil }

// stopExternal is a no-op. Audio is handled by MpvController.
func stopExternal() {}
```

- [ ] **Step 5: Run all player tests**

Run: `cd ~/Documents/personal/audbookdl && go test ./internal/player/ -v`
Expected: All tests PASS including the new `TestPlayer_JumpToChapter`

- [ ] **Step 6: Commit**

```bash
git add internal/player/player.go internal/player/player_test.go internal/player/engine_speaker_stub.go
git commit -m "feat(player): wire mpv IPC controller into Player, add JumpToChapter"
```

---

### Task 3: Chapter Duration Detection with ffprobe

**Files:**
- Create: `internal/player/probe.go`
- Create: `internal/player/probe_test.go`
- Modify: `internal/tui/library.go`

- [ ] **Step 1: Write probe.go**

```go
// internal/player/probe.go
package player

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ProbeAudioDuration uses ffprobe to get the duration of an audio file.
// Returns 0 and an error if ffprobe is not available or the file can't be probed.
func ProbeAudioDuration(filePath string) (time.Duration, error) {
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0, fmt.Errorf("ffprobe not found: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffprobe,
		"-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		filePath,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe %s: %w", filePath, err)
	}

	secs, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, fmt.Errorf("parse duration: %w", err)
	}

	return time.Duration(secs * float64(time.Second)), nil
}

// ProbeChapterDurations probes all chapters in parallel and populates their Duration fields.
// Errors are non-fatal: chapters with probe failures keep Duration == 0.
func ProbeChapterDurations(chapters []ChapterInfo) []ChapterInfo {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4) // limit concurrency

	for i := range chapters {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			d, err := ProbeAudioDuration(chapters[idx].FilePath)
			if err == nil {
				chapters[idx].Duration = d
			}
		}(i)
	}

	wg.Wait()
	return chapters
}
```

- [ ] **Step 2: Write probe tests**

```go
// internal/player/probe_test.go
package player

import (
	"os/exec"
	"testing"
	"time"
)

func TestProbeAudioDuration_NoFfprobe(t *testing.T) {
	// If ffprobe is not installed, should return error.
	if _, err := exec.LookPath("ffprobe"); err != nil {
		_, probeErr := ProbeAudioDuration("/nonexistent.mp3")
		if probeErr == nil {
			t.Error("expected error when ffprobe not available")
		}
		return
	}
	t.Skip("ffprobe is installed — this test checks the no-ffprobe path")
}

func TestProbeAudioDuration_FileNotFound(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	_, err := ProbeAudioDuration("/nonexistent/file.mp3")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestProbeChapterDurations_Empty(t *testing.T) {
	// Should handle empty slice without error.
	result := ProbeChapterDurations(nil)
	if len(result) != 0 {
		t.Errorf("expected nil, got %d chapters", len(result))
	}
}

func TestProbeChapterDurations_NonexistentFiles(t *testing.T) {
	chapters := []ChapterInfo{
		{Index: 0, Title: "Ch1", FilePath: "/nonexistent/ch1.mp3"},
		{Index: 1, Title: "Ch2", FilePath: "/nonexistent/ch2.mp3"},
	}
	result := ProbeChapterDurations(chapters)
	// All should have Duration == 0 (errors are non-fatal).
	for _, ch := range result {
		if ch.Duration != 0 {
			t.Errorf("chapter %d: expected Duration 0, got %v", ch.Index, ch.Duration)
		}
	}
}

func TestProbeChapterDurations_PreservesDuration(t *testing.T) {
	// Chapters that already have duration should not be overwritten on error.
	chapters := []ChapterInfo{
		{Index: 0, Title: "Ch1", FilePath: "/nonexistent.mp3", Duration: 5 * time.Minute},
	}
	result := ProbeChapterDurations(chapters)
	// ffprobe will fail, so Duration stays at the probed value (0) since we overwrite on success only.
	// Actually: our implementation only writes on success, so Duration should stay 5m.
	if result[0].Duration != 5*time.Minute {
		t.Errorf("expected Duration to be preserved at 5m, got %v", result[0].Duration)
	}
}
```

- [ ] **Step 3: Run probe tests**

Run: `cd ~/Documents/personal/audbookdl && go test ./internal/player/ -run TestProbe -v`
Expected: PASS

- [ ] **Step 4: Integrate into library.go buildPlaylist**

In `internal/tui/library.go`, modify `buildPlaylist` (starting at line 260). After the loop that builds `chapters` and before the `if len(chapters) == 0` check (line 289), add the probe call:

```go
		// Probe durations from audio files (non-blocking, errors ignored)
		chapters = player.ProbeChapterDurations(chapters)
```

The modified section (lines 286-293) becomes:

```go
			// Probe durations from audio files (non-blocking, errors ignored)
			chapters = player.ProbeChapterDurations(chapters)

			if len(chapters) == 0 {
				return refreshLibraryMsg{err: fmt.Errorf("no audio files found in %s", dir)}
			}
```

- [ ] **Step 5: Update playerui.go to handle zero duration**

In `internal/tui/playerui.go`, modify `formatMS` (line 240) to handle zero/negative:

Replace lines 240-252:
```go
func formatMS(ms int64) string {
	if ms <= 0 {
		return "--:--"
	}
	total := ms / 1000
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
```

- [ ] **Step 6: Run full test suite**

Run: `cd ~/Documents/personal/audbookdl && go test ./... -v`
Expected: All tests PASS

- [ ] **Step 7: Commit**

```bash
git add internal/player/probe.go internal/player/probe_test.go internal/tui/library.go internal/tui/playerui.go
git commit -m "feat(player): add ffprobe chapter duration detection"
```

---

### Task 4: TUI Player Enhancements — Sleep Timer, Chapter Jump, Seek Feedback

**Files:**
- Modify: `internal/tui/playerui.go`

- [ ] **Step 1: Add sleep timer presets and chapter jump state to PlayerTab**

Replace the `PlayerTab` struct and add new types in `internal/tui/playerui.go` (lines 28-37):

```go
// sleepPresets is the cycle order for sleep timer duration (minutes).
var sleepPresets = []int{0, 15, 30, 45, 60, 90}

// PlayerTab is the audio player tab.
type PlayerTab struct {
	player       *player.Player
	width        int
	height       int
	seekFlash    string    // "+15s" or "-15s", cleared after 1 tick
	sleepIndex   int       // index into sleepPresets
	showChapters bool      // whether the chapter list overlay is visible
	chapterList  []player.ChapterInfo
	chCursor     int       // cursor within chapter list
	chScroll     int       // scroll offset for chapter list
}
```

- [ ] **Step 2: Add sleep timer cycling, chapter jump, and seek flash to Update**

Replace the `Update` method (lines 58-98) with:

```go
func (t *PlayerTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
		return t, nil

	case playerTickMsg:
		// Clear seek flash on tick.
		t.seekFlash = ""
		return t, playerTick()

	case tea.KeyMsg:
		if t.player == nil {
			return t, nil
		}

		// Chapter list modal intercepts all keys.
		if t.showChapters {
			return t.updateChapterList(msg)
		}

		switch msg.String() {
		case " ":
			st := t.player.GetStatus()
			if st.Status == player.StatusPlaying {
				t.player.Pause()
			} else {
				t.player.Play()
			}
		case "n":
			t.player.NextChapter()
		case "p":
			t.player.PrevChapter()
		case "left", "h":
			t.player.SkipBackward(15 * time.Second)
			t.seekFlash = "-15s"
		case "right", "l":
			t.player.SkipForward(15 * time.Second)
			t.seekFlash = "+15s"
		case "s":
			t.cycleSpeed()
		case "v":
			t.cycleVolume()
		case "t":
			t.cycleSleepTimer()
		case "c":
			t.openChapterList()
		}
		return t, nil
	}

	return t, nil
}

// cycleSleepTimer cycles through sleep presets: off → 15m → 30m → 45m → 60m → 90m → off.
func (t *PlayerTab) cycleSleepTimer() {
	t.sleepIndex = (t.sleepIndex + 1) % len(sleepPresets)
	minutes := sleepPresets[t.sleepIndex]
	t.player.SetSleepTimer(time.Duration(minutes) * time.Minute)
}

// openChapterList opens the chapter jump overlay.
func (t *PlayerTab) openChapterList() {
	st := t.player.GetStatus()
	pl := t.player.GetPlaylist()
	if pl == nil {
		return
	}
	t.chapterList = pl.Chapters
	t.chCursor = st.ChapterIndex
	t.showChapters = true
	// Center scroll on current chapter.
	maxVisible := t.chapterListHeight()
	t.chScroll = t.chCursor - maxVisible/2
	if t.chScroll < 0 {
		t.chScroll = 0
	}
}

// updateChapterList handles keys while the chapter list is open.
func (t *PlayerTab) updateChapterList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		t.showChapters = false
		return t, nil
	case "enter":
		_ = t.player.JumpToChapter(t.chCursor)
		t.showChapters = false
		return t, nil
	case "up", "k":
		if t.chCursor > 0 {
			t.chCursor--
			if t.chCursor < t.chScroll {
				t.chScroll = t.chCursor
			}
		}
	case "down", "j":
		if t.chCursor < len(t.chapterList)-1 {
			t.chCursor++
			maxVisible := t.chapterListHeight()
			if t.chCursor >= t.chScroll+maxVisible {
				t.chScroll = t.chCursor - maxVisible + 1
			}
		}
	}
	return t, nil
}

func (t *PlayerTab) chapterListHeight() int {
	h := t.height - 10
	if h < 5 {
		h = 5
	}
	return h
}
```

- [ ] **Step 3: Add GetPlaylist method to Player**

In `internal/player/player.go`, add after `GetStatus`:

```go
// GetPlaylist returns the currently loaded playlist, or nil.
func (p *Player) GetPlaylist() *Playlist {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.playlist
}
```

- [ ] **Step 4: Update the View to render sleep timer, seek flash, chapter list, and new help line**

Replace the `View` method in `internal/tui/playerui.go` (lines 127-237) with:

```go
func (t *PlayerTab) View() string {
	var sb strings.Builder
	sb.WriteString("\n")

	if t.player == nil {
		sb.WriteString(subtitleStyle.Render("  No audiobook loaded. Select from Library tab."))
		sb.WriteString("\n")
		return sb.String()
	}

	st := t.player.GetStatus()

	if st.AudiobookTitle == "" {
		sb.WriteString(subtitleStyle.Render("  No audiobook loaded. Select from Library tab."))
		sb.WriteString("\n")
		return sb.String()
	}

	// Chapter list overlay
	if t.showChapters {
		return t.viewChapterList(st)
	}

	var content strings.Builder

	// Title / Author / Narrator
	content.WriteString(titleStyle.Render(st.AudiobookTitle) + "\n")
	if st.Author != "" {
		content.WriteString(subtitleStyle.Render("by " + st.Author))
		if st.Narrator != "" {
			content.WriteString(subtitleStyle.Render("  ·  narrated by " + st.Narrator))
		}
		content.WriteString("\n")
	}
	content.WriteString("\n")

	// Chapter info
	chapterLine := fmt.Sprintf("Chapter %d / %d", st.ChapterIndex+1, st.TotalChapters)
	if st.ChapterTitle != "" {
		chapterLine += "  —  " + st.ChapterTitle
	}
	content.WriteString(sourceStyle.Render(chapterLine) + "\n\n")

	// Position / duration progress bar
	posStr := formatMS(st.PositionMS)
	durStr := formatMS(st.ChapterDurationMS)

	barWidth := 30
	if t.width > 60 {
		barWidth = t.width/2 - 20
		if barWidth < 20 {
			barWidth = 20
		}
	}
	var barProgress float64
	if st.ChapterDurationMS > 0 {
		barProgress = float64(st.PositionMS) / float64(st.ChapterDurationMS) * 100
	}

	progressLine := fmt.Sprintf("%s %s %s", posStr, styledProgressBar(barProgress, barWidth), durStr)
	if t.seekFlash != "" {
		progressLine += "  " + downloadingStyle.Render(t.seekFlash)
	}
	content.WriteString(progressLine + "\n\n")

	// Playback state
	statusLabel := "■ Stopped"
	switch st.Status {
	case player.StatusPlaying:
		statusLabel = "▶ Playing"
	case player.StatusPaused:
		statusLabel = "‖ Paused"
	}
	content.WriteString(downloadingStyle.Render(statusLabel) + "\n\n")

	// Speed and volume
	content.WriteString(fmt.Sprintf("%s  %.2fx    %s  %.0f%%\n",
		subtitleStyle.Render("Speed:"),
		st.Speed,
		subtitleStyle.Render("Volume:"),
		st.Volume*100,
	))

	// Sleep timer
	if sleepPresets[t.sleepIndex] > 0 {
		remaining := formatMS(st.SleepRemainMS)
		if st.SleepRemainMS <= 0 {
			remaining = fmt.Sprintf("%dm", sleepPresets[t.sleepIndex])
		}
		content.WriteString(fmt.Sprintf("\n%s  %s\n",
			subtitleStyle.Render("Sleep timer:"),
			pausedStyle.Render(remaining),
		))
	}

	content.WriteString("\n")
	content.WriteString(helpStyle.Render("space play/pause  n/p chapter  ←/→ seek  s speed  v vol  t sleep  c chapters"))

	// Wrap in a centered detail panel
	panelWidth := t.width - 4
	if panelWidth > 70 {
		panelWidth = 70
	}
	if panelWidth < 40 {
		panelWidth = 40
	}

	panel := detailPanelStyle.Width(panelWidth).Render(content.String())

	if t.width > panelWidth+4 {
		padding := (t.width - panelWidth - 4) / 2
		panel = lipgloss.NewStyle().MarginLeft(padding).Render(panel)
	}

	sb.WriteString(panel)
	sb.WriteString("\n")

	return sb.String()
}

// viewChapterList renders the chapter selection overlay.
func (t *PlayerTab) viewChapterList(st player.PlayerStatus) string {
	var sb strings.Builder

	sb.WriteString("\n")
	var content strings.Builder
	content.WriteString(titleStyle.Render("Select Chapter") + "\n")
	content.WriteString(subtitleStyle.Render(st.AudiobookTitle) + "\n\n")

	maxVisible := t.chapterListHeight()
	end := t.chScroll + maxVisible
	if end > len(t.chapterList) {
		end = len(t.chapterList)
	}

	for i := t.chScroll; i < end; i++ {
		ch := t.chapterList[i]
		prefix := "  "
		style := subtitleStyle
		if i == t.chCursor {
			prefix = cursorStyle.Render("> ")
			style = selectedStyle
		}
		if i == st.ChapterIndex {
			prefix = downloadingStyle.Render("♪ ")
			if i == t.chCursor {
				prefix = cursorStyle.Render("▶ ")
			}
		}

		dur := ""
		if ch.Duration > 0 {
			dur = "  " + tagStyle.Render(formatDuration(ch.Duration))
		}
		content.WriteString(fmt.Sprintf("%s%s%s\n", prefix, style.Render(fmt.Sprintf("%d. %s", i+1, ch.Title)), dur))
	}

	// Scroll indicators
	if t.chScroll > 0 {
		content.WriteString(subtitleStyle.Render("  ↑ more") + "\n")
	}
	if end < len(t.chapterList) {
		content.WriteString(subtitleStyle.Render("  ↓ more") + "\n")
	}

	content.WriteString("\n")
	content.WriteString(helpStyle.Render("j/k navigate  enter select  esc close"))

	panelWidth := t.width - 4
	if panelWidth > 60 {
		panelWidth = 60
	}
	if panelWidth < 40 {
		panelWidth = 40
	}
	panel := detailPanelStyle.Width(panelWidth).Render(content.String())

	if t.width > panelWidth+4 {
		padding := (t.width - panelWidth - 4) / 2
		panel = lipgloss.NewStyle().MarginLeft(padding).Render(panel)
	}

	sb.WriteString(panel)
	sb.WriteString("\n")
	return sb.String()
}

// formatDuration formats a time.Duration as "H:MM:SS" or "M:SS".
func formatDuration(d time.Duration) string {
	total := int(d.Seconds())
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
```

- [ ] **Step 5: Update ShortHelp to include new keybindings**

Replace `ShortHelp` (lines 42-52) with:

```go
func (t *PlayerTab) ShortHelp() []key.Binding {
	if t.showChapters {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select chapter")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
			key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
			key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		}
	}
	return []key.Binding{
		key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "play/pause")),
		key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next chapter")),
		key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prev chapter")),
		key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "skip -15s")),
		key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "skip +15s")),
		key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "cycle speed")),
		key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "cycle volume")),
		key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "sleep timer")),
		key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "chapters")),
	}
}
```

- [ ] **Step 6: Build and verify**

Run: `cd ~/Documents/personal/audbookdl && go build ./...`
Expected: Clean build, no errors

- [ ] **Step 7: Commit**

```bash
git add internal/tui/playerui.go internal/player/player.go
git commit -m "feat(tui): add sleep timer cycling, chapter jump modal, seek feedback"
```

---

### Task 5: Fix Search Input j/k Bug

**Files:**
- Modify: `internal/tui/search.go`

- [ ] **Step 1: Gate j/k navigation behind input focus check**

In `internal/tui/search.go`, replace lines 180-190:

```go
		case "up", "k":
			if t.cursor > 0 {
				t.cursor--
			}
			return t, nil

		case "down", "j":
			if t.cursor < len(t.results)-1 {
				t.cursor++
			}
			return t, nil
```

with:

```go
		case "up":
			if t.cursor > 0 {
				t.cursor--
			}
			return t, nil

		case "down":
			if t.cursor < len(t.results)-1 {
				t.cursor++
			}
			return t, nil

		case "k":
			if !t.textinput.Focused() {
				if t.cursor > 0 {
					t.cursor--
				}
				return t, nil
			}

		case "j":
			if !t.textinput.Focused() {
				if t.cursor < len(t.results)-1 {
					t.cursor++
				}
				return t, nil
			}
```

- [ ] **Step 2: Build and verify**

Run: `cd ~/Documents/personal/audbookdl && go build ./...`
Expected: Clean build

- [ ] **Step 3: Commit**

```bash
git add internal/tui/search.go
git commit -m "fix(tui): allow typing j/k in search input when focused"
```

---

### Task 6: Rewrite CLI Play Command

**Files:**
- Modify: `internal/cli/play.go`

- [ ] **Step 1: Rewrite play.go to use MpvController with state persistence**

Replace the entire contents of `internal/cli/play.go`:

```go
package cli

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/billmal071/audbookdl/internal/db"
	"github.com/billmal071/audbookdl/internal/player"
	"github.com/spf13/cobra"
)

var (
	playSpeed  float64
	playVolume float64
)

var playCmd = &cobra.Command{
	Use:   "play [download-id or path]",
	Short: "Play a downloaded audiobook",
	Long: `Play a downloaded audiobook using mpv (IPC-controlled) with state persistence.

Examples:
  audbookdl play 1                              # Play download #1
  audbookdl play ~/Audiobooks/Author/Title/     # Play from directory
  audbookdl play 1 --speed 1.5 --volume 0.7     # Custom speed and volume`,
	Args: cobra.ExactArgs(1),
	RunE: runPlay,
}

func init() {
	playCmd.Flags().Float64Var(&playSpeed, "speed", 0, "playback speed (0.5-3.0, default: saved or 1.0)")
	playCmd.Flags().Float64Var(&playVolume, "volume", 0, "volume level (0.0-1.0, default: saved or 0.8)")
}

func runPlay(cmd *cobra.Command, args []string) error {
	arg := args[0]

	var audioDir string
	var audiobookID string
	var title string

	if info, err := os.Stat(arg); err == nil && info.IsDir() {
		audioDir = arg
		audiobookID = filepath.Base(arg)
		title = audiobookID
	} else {
		id, err := strconv.ParseInt(arg, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid download ID or path: %s", arg)
		}
		dl, err := db.GetDownload(db.DB(), id)
		if err != nil {
			return fmt.Errorf("download not found: %w", err)
		}
		if dl.Status != db.StatusCompleted {
			return fmt.Errorf("download #%d is %s, not completed", id, dl.Status)
		}
		audioDir = dl.BasePath
		audiobookID = dl.AudiobookID
		title = dl.Title
	}

	// Find and sort audio files.
	entries, err := os.ReadDir(audioDir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

	var chapters []player.ChapterInfo
	idx := 0
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".mp3" || ext == ".m4b" || ext == ".m4a" || ext == ".ogg" {
			chapters = append(chapters, player.ChapterInfo{
				Index:    idx,
				Title:    strings.TrimSuffix(e.Name(), ext),
				FilePath: filepath.Join(audioDir, e.Name()),
			})
			idx++
		}
	}

	if len(chapters) == 0 {
		return fmt.Errorf("no audio files found in %s", audioDir)
	}

	// Probe durations.
	chapters = player.ProbeChapterDurations(chapters)

	// Create player with DB for state persistence.
	p := player.NewPlayer(db.DB())
	p.Load(&player.Playlist{
		AudiobookID: audiobookID,
		Title:       title,
		Chapters:    chapters,
	})

	// Apply flags if set.
	if playSpeed > 0 {
		p.SetSpeed(playSpeed)
	}
	if playVolume > 0 {
		p.SetVolume(playVolume)
	}

	st := p.GetStatus()
	fmt.Printf("Playing %s (%d chapters)\n", title, len(chapters))
	if st.ChapterIndex > 0 || st.PositionMS > 0 {
		fmt.Printf("Resuming from chapter %d at %s\n", st.ChapterIndex+1, formatPosition(st.PositionMS))
	}

	p.Play()

	// Wait for interrupt to stop gracefully.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	fmt.Println("\nStopping playback...")
	p.Stop()
	return nil
}

func formatPosition(ms int64) string {
	total := ms / 1000
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
```

- [ ] **Step 2: Build and verify**

Run: `cd ~/Documents/personal/audbookdl && go build ./...`
Expected: Clean build

- [ ] **Step 3: Commit**

```bash
git add internal/cli/play.go
git commit -m "feat(cli): rewrite play command to use mpv IPC with state persistence"
```

---

### Task 7: Final Integration Test and Cleanup

**Files:**
- All modified files

- [ ] **Step 1: Run full test suite**

Run: `cd ~/Documents/personal/audbookdl && go test ./... -v`
Expected: All tests PASS

- [ ] **Step 2: Run go vet and fmt**

Run: `cd ~/Documents/personal/audbookdl && go vet ./... && gofmt -l .`
Expected: No issues, no unformatted files

- [ ] **Step 3: Build all platforms**

Run: `cd ~/Documents/personal/audbookdl && make build`
Expected: Clean build to `./build/audbookdl`

- [ ] **Step 4: Commit any remaining fixes**

```bash
git add -A
git commit -m "chore: final cleanup for player completion"
```
