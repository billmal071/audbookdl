# Player Completion Design

Complete the audbookdl audio player with proper mpv IPC control, real-time position tracking, chapter duration detection, and TUI enhancements.

## Context

The player currently has working state management (play/stop, chapter nav, speed/volume, persistence) but broken or missing audio control:
- Pause/resume is a no-op (beep v2 has no pause API; non-CGO stub kills process)
- Seeking updates internal numbers but doesn't move the audio stream
- Chapter durations are never populated (always zero)
- Sleep timer has no UI
- No chapter jump/selection
- CLI play command is disconnected from the player package
- External player (mpv/ffplay) runs as fire-and-forget subprocess with no feedback

## Design

### 1. mpv IPC Controller

**New file: `internal/player/mpv.go`**

`MpvController` manages an mpv subprocess via JSON IPC over a Unix socket.

**Lifecycle:**
- `NewMpvController()` — checks mpv is on PATH, returns nil if not found
- `Start(filePath string, positionMS int64) error` — launches `mpv --no-video --really-quiet --idle=no --input-ipc-server=<socket> --start=<seconds> <file>`. Connects to the socket with retry (mpv needs a moment to create it).
- `Stop()` — sends quit command, kills process, removes socket file
- `IsRunning() bool` — checks process state

**IPC commands (all via `{"command": [...]}` JSON over Unix socket):**
- `Pause()` — `set_property pause true`
- `Resume()` — `set_property pause false`
- `Seek(positionMS int64)` — `seek <seconds> absolute`
- `SetSpeed(rate float64)` — `set_property speed <rate>`
- `SetVolume(vol float64)` — `set_property volume <pct>` (maps 0.0-1.0 to 0-100)
- `GetPosition() (int64, error)` — `get_property time-pos`, returns milliseconds
- `GetDuration() (int64, error)` — `get_property duration`, returns milliseconds

**IPC protocol:** Newline-delimited JSON. Each command includes a `request_id`. A background goroutine reads responses from the socket and dispatches them to waiting callers via a channel map keyed by request ID. Timeout after 2 seconds per command.

**Socket path:** `/tmp/audbookdl-mpv-<pid>.sock` to avoid collisions.

### 2. Player Integration

**Changes to `internal/player/player.go`:**

Add `mpv *MpvController` field. `NewPlayer()` creates the controller if mpv is available. Priority: mpv controller > beep engine > ffplay fallback.

Method changes when mpv is available:
- `Play()` — `mpv.Start(filePath, positionMS)`
- `Pause()` — `mpv.Pause()`. Gets real position via `mpv.GetPosition()`.
- A new internal `resume()` calls `mpv.Resume()` instead of restarting.
- `Stop()` — `mpv.Stop()`
- `SkipForward(d)` / `SkipBackward(d)` — compute new position, call `mpv.Seek(pos)`
- `SetSpeed()` / `SetVolume()` — forward to `mpv.SetSpeed()` / `mpv.SetVolume()`
- `NextChapter()` / `PrevChapter()` — `mpv.Stop()` then `mpv.Start(newFile, 0)`
- `JumpToChapter(index int)` — new method: validates index, sets chapterIndex, starts playback at 0
- `GetStatus()` — uses `mpv.GetPosition()` for real position instead of elapsed-time estimation

The `playStartedAt`/`pausedPosition` elapsed-time calculation becomes the fallback for when mpv is not available (ffplay mode).

### 3. Chapter Duration Detection

**New file: `internal/player/probe.go`**

- `ProbeAudioDuration(filePath string) (time.Duration, error)` — runs `ffprobe -v quiet -show_entries format=duration -of csv=p=0 <file>`, parses float seconds
- `ProbeChapterDurations(chapters []ChapterInfo) []ChapterInfo` — probes all chapters in parallel with a semaphore (concurrency 4). Errors are non-fatal (duration stays zero).

**Integration:**
- `LibraryTab.buildPlaylist()` calls `ProbeChapterDurations()` after scanning audio files
- TUI progress bar uses real duration data
- Zero duration displays as `--:--`

**Dependency:** ffprobe (from ffmpeg). Almost always present alongside mpv. Graceful degradation if missing.

### 4. TUI Player Enhancements

**Sleep timer preset cycle:**
- `t` key cycles: Off → 15m → 30m → 45m → 60m → 90m → Off
- Calls `Player.SetSleepTimer(duration)`
- Countdown already rendered in the status display

**Chapter jump modal:**
- `c` key opens a `bubbles/list` overlay
- Shows all chapters: index, title, duration
- Current chapter highlighted
- Navigate: `j`/`k`/`↑`/`↓`, select: `Enter`, dismiss: `Esc`
- Selection calls `Player.JumpToChapter(index)`
- All other player keys suppressed while modal is open

**Seek feedback:**
- `h`/`l`/`←`/`→` now actually seeks via mpv IPC
- Brief "+15s" / "-15s" flash indicator (clears after 1 tick)

**Updated help line:**
- `space` play/pause · `n`/`p` chapter · `←`/`→` seek · `s` speed · `v` volume · `t` sleep · `c` chapters

### 5. CLI Play Command Rewrite

**Changes to `internal/cli/play.go`:**
- Use `MpvController` instead of raw `exec.Command`
- Persist playback state via the player package (resume on next launch)
- Add `--speed` and `--volume` flags forwarded to mpv IPC
- Still accepts download ID or directory path

### 6. ffplay Fallback

**New file: `internal/player/ffplay.go`**
- Extract current kill-restart logic from `engine_speaker_stub.go`
- Same method signatures as relevant MpvController methods but no IPC
- Used only when mpv is not on PATH
- No position sync, no live speed/volume — degraded but functional

### 7. Search Input Fix

**Changes to `internal/tui/search.go`:**
- Gate `j`/`k` key handling behind `!t.textinput.Focused()`
- When input is focused, single-letter keys pass through to the text input
- Arrow keys (`up`/`down`) still work for navigation regardless

## Priority Order

1. mpv IPC Controller (foundation for everything else)
2. Player Integration (wires controller into existing player)
3. Duration Detection (enables progress display)
4. TUI Enhancements (sleep timer, chapter jump, seek feedback)
5. CLI Play Rewrite (uses new controller)
6. Search Input Fix (independent bugfix)

## Dependencies

- **mpv** — required for full player functionality. Most Linux distros and macOS (via Homebrew) have it available.
- **ffprobe** — required for duration detection. Part of ffmpeg, almost always co-installed with mpv.
- **ffplay** — degraded fallback only. Part of ffmpeg.

## Files Changed

| File | Action |
|------|--------|
| `internal/player/mpv.go` | New — mpv IPC controller |
| `internal/player/probe.go` | New — ffprobe duration detection |
| `internal/player/ffplay.go` | New — extracted ffplay fallback |
| `internal/player/player.go` | Modified — integrate mpv controller, add JumpToChapter |
| `internal/player/engine_speaker_stub.go` | Modified — remove playExternal/stopExternal (moved to ffplay.go) |
| `internal/tui/playerui.go` | Modified — sleep timer cycle, chapter jump modal, seek feedback |
| `internal/tui/library.go` | Modified — call ProbeChapterDurations in buildPlaylist |
| `internal/tui/search.go` | Modified — gate j/k behind input focus check |
| `internal/cli/play.go` | Modified — rewrite to use MpvController |
