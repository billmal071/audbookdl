package player

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// mpvCommand is the JSON structure sent to mpv's IPC socket.
type mpvCommand struct {
	Command   []interface{} `json:"command"`
	RequestID int64         `json:"request_id"`
}

// mpvResponse is the JSON structure received from mpv's IPC socket.
// It also covers event messages which have an "event" field instead of request_id.
type mpvResponse struct {
	Data      json.RawMessage `json:"data"`
	RequestID int64           `json:"request_id"`
	Error     string          `json:"error"`
	Event     string          `json:"event"`
}

// mpvResult carries the data and error fields from an mpv IPC response.
type mpvResult struct {
	Data  json.RawMessage
	Error string
}

// MpvController manages an mpv subprocess via JSON IPC over a Unix socket.
type MpvController struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	conn       net.Conn
	socketPath string
	requestID  atomic.Int64
	responses  map[int64]chan mpvResult
	respMu     sync.Mutex
	running    bool
	onEndFile  func() // called when mpv finishes playing the current file
}

// NewMpvController returns a new MpvController, or nil if mpv is not on PATH.
// It also cleans up any orphaned mpv processes from previous audbookdl sessions.
func NewMpvController() *MpvController {
	if _, err := exec.LookPath("mpv"); err != nil {
		return nil
	}
	killOrphanedMpv()
	socketPath := fmt.Sprintf("/tmp/audbookdl-mpv-%d.sock", os.Getpid())
	os.Remove(socketPath) // clean stale socket from a previous crash
	return &MpvController{
		socketPath: socketPath,
		responses:  make(map[int64]chan mpvResult),
	}
}

// killOrphanedMpv finds and kills any mpv processes started by previous
// audbookdl sessions (identified by the --input-ipc-server flag pattern).
func killOrphanedMpv() {
	out, err := exec.Command("pgrep", "-f", "mpv.*audbookdl-mpv").Output()
	if err != nil || len(out) == 0 {
		return
	}
	// Kill each matching PID
	_ = exec.Command("pkill", "-f", "mpv.*audbookdl-mpv").Run()
	// Give processes a moment to die, then force-kill stragglers
	time.Sleep(100 * time.Millisecond)
	_ = exec.Command("pkill", "-9", "-f", "mpv.*audbookdl-mpv").Run()
}

// Start launches mpv with the given file and seeks to positionMS.
// It stops any existing instance first.
func (m *MpvController) Start(filePath string, positionMS int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopLocked()

	// Remove stale socket
	os.Remove(m.socketPath)

	startSec := float64(positionMS) / 1000.0

	m.cmd = exec.Command("mpv",
		"--no-video",
		"--really-quiet",
		"--idle=no",
		fmt.Sprintf("--input-ipc-server=%s", m.socketPath),
		fmt.Sprintf("--start=%f", startSec),
		filePath,
	)

	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start mpv: %w", err)
	}

	// Retry connecting to the IPC socket
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
		m.cmd.Process.Kill()
		m.cmd.Wait()
		m.cmd = nil
		os.Remove(m.socketPath)
		return fmt.Errorf("failed to connect to mpv socket: %w", err)
	}

	m.conn = conn
	m.running = true
	m.responses = make(map[int64]chan mpvResult)

	go m.readResponses()

	// Subscribe to end-file events so we know when playback finishes.
	m.observeEvent("end-file")

	return nil
}

// SetOnEndFile registers a callback invoked when mpv finishes playing a file.
func (m *MpvController) SetOnEndFile(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onEndFile = fn
}

// observeEvent tells mpv to send us events of the given type.
func (m *MpvController) observeEvent(name string) {
	cmd := mpvCommand{
		Command:   []interface{}{"enable_event", name},
		RequestID: 0,
	}
	data, err := json.Marshal(cmd)
	if err != nil {
		return
	}
	data = append(data, '\n')
	if m.conn != nil {
		m.conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
		m.conn.Write(data)
	}
}

// readResponses reads JSON responses from the mpv IPC socket and dispatches
// them to waiting channels by request_id. Events (request_id == 0) are skipped.
// When the connection closes (mpv exits), this marks the controller as not running.
func (m *MpvController) readResponses() {
	dec := json.NewDecoder(m.conn)
	for {
		var resp mpvResponse
		if err := dec.Decode(&resp); err != nil {
			// Connection closed — mpv exited. Mark as not running.
			m.mu.Lock()
			m.running = false
			m.mu.Unlock()

			// Clean up pending channels
			m.respMu.Lock()
			for id, ch := range m.responses {
				close(ch)
				delete(m.responses, id)
			}
			m.respMu.Unlock()
			return
		}

		// Handle events (request_id 0).
		if resp.Event == "end-file" {
			m.mu.Lock()
			fn := m.onEndFile
			m.mu.Unlock()
			if fn != nil {
				go fn()
			}
			continue
		}
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
			ch <- mpvResult{Data: resp.Data, Error: resp.Error}
		}
	}
}

// sendCommand sends a command to mpv and waits up to 2 seconds for the response.
func (m *MpvController) sendCommand(args ...interface{}) (json.RawMessage, error) {
	m.mu.Lock()
	if m.conn == nil || !m.running {
		m.mu.Unlock()
		return nil, fmt.Errorf("mpv not connected")
	}
	conn := m.conn
	m.mu.Unlock()

	id := m.requestID.Add(1)

	ch := make(chan mpvResult, 1)
	m.respMu.Lock()
	m.responses[id] = ch
	m.respMu.Unlock()

	cmd := mpvCommand{
		Command:   args,
		RequestID: id,
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		m.respMu.Lock()
		delete(m.responses, id)
		m.respMu.Unlock()
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}

	data = append(data, '\n')

	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(data); err != nil {
		m.respMu.Lock()
		delete(m.responses, id)
		m.respMu.Unlock()
		return nil, fmt.Errorf("failed to write command: %w", err)
	}

	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("mpv connection closed")
		}
		if resp.Error != "" && resp.Error != "success" {
			return nil, fmt.Errorf("mpv error: %s", resp.Error)
		}
		return resp.Data, nil
	case <-time.After(500 * time.Millisecond):
		m.respMu.Lock()
		delete(m.responses, id)
		m.respMu.Unlock()
		return nil, fmt.Errorf("mpv command timed out")
	}
}

// Pause pauses playback.
func (m *MpvController) Pause() error {
	_, err := m.sendCommand("set_property", "pause", true)
	return err
}

// Resume resumes playback.
func (m *MpvController) Resume() error {
	_, err := m.sendCommand("set_property", "pause", false)
	return err
}

// SeekTo seeks to the given position in milliseconds.
func (m *MpvController) SeekTo(positionMS int64) error {
	sec := float64(positionMS) / 1000.0
	_, err := m.sendCommand("seek", sec, "absolute")
	return err
}

// SetSpeed sets the playback speed.
func (m *MpvController) SetSpeed(rate float64) error {
	_, err := m.sendCommand("set_property", "speed", rate)
	return err
}

// SetVolume sets the volume. vol is in the range 0.0 to 1.0, mapped to 0-100.
func (m *MpvController) SetVolume(vol float64) error {
	_, err := m.sendCommand("set_property", "volume", vol*100)
	return err
}

// GetPosition returns the current playback position in milliseconds.
func (m *MpvController) GetPosition() (int64, error) {
	raw, err := m.sendCommand("get_property", "time-pos")
	if err != nil {
		return 0, err
	}
	var sec float64
	if err := json.Unmarshal(raw, &sec); err != nil {
		return 0, fmt.Errorf("failed to parse time-pos: %w", err)
	}
	return int64(sec * 1000), nil
}

// GetDuration returns the total duration of the current file in milliseconds.
func (m *MpvController) GetDuration() (int64, error) {
	raw, err := m.sendCommand("get_property", "duration")
	if err != nil {
		return 0, err
	}
	var sec float64
	if err := json.Unmarshal(raw, &sec); err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}
	return int64(sec * 1000), nil
}

// Stop stops mpv playback and cleans up all resources.
func (m *MpvController) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked()
}

// stopLocked is the internal stop method that assumes the lock is already held.
func (m *MpvController) stopLocked() {
	if !m.running {
		return
	}

	// Try to send quit command gracefully
	if m.conn != nil {
		cmd := mpvCommand{
			Command:   []interface{}{"quit"},
			RequestID: 0,
		}
		data, err := json.Marshal(cmd)
		if err == nil {
			data = append(data, '\n')
			m.conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
			m.conn.Write(data)
		}
	}

	// Close connection
	if m.conn != nil {
		m.conn.Close()
		m.conn = nil
	}

	// Kill process
	if m.cmd != nil && m.cmd.Process != nil {
		m.cmd.Process.Kill()
		m.cmd.Wait()
		m.cmd = nil
	}

	// Remove socket
	os.Remove(m.socketPath)

	// Drain pending response channels
	m.respMu.Lock()
	for id, ch := range m.responses {
		close(ch)
		delete(m.responses, id)
	}
	m.respMu.Unlock()

	m.running = false
}

// IsRunning returns whether mpv is currently running.
func (m *MpvController) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}
