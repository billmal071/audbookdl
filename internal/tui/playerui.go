package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/billmal071/audbookdl/internal/player"
)

// playerTickMsg is sent every second to refresh the player display.
type playerTickMsg time.Time

// playerTick returns a command that fires playerTickMsg after one second.
func playerTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return playerTickMsg(t)
	})
}

// speedSteps is the cycle order for playback speed.
var speedSteps = []float64{1.0, 1.25, 1.5, 1.75, 2.0, 0.75}

// sleepPresets defines the sleep timer cycle in minutes: Off → 15m → 30m → 45m → 60m → 90m → Off.
var sleepPresets = []int{0, 15, 30, 45, 60, 90}

// PlayerTab is the audio player tab.
type PlayerTab struct {
	player       *player.Player
	width        int
	height       int
	seekFlash    string
	sleepIndex   int
	showChapters bool
	chapterList  []player.ChapterInfo
	chCursor     int
	chScroll     int
}

// NewPlayerTab constructs a PlayerTab wired to the given Player.
func NewPlayerTab(p *player.Player) *PlayerTab {
	return &PlayerTab{player: p}
}

func (t *PlayerTab) TabName() string { return "Player" }

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

func (t *PlayerTab) Init() tea.Cmd {
	return playerTick()
}

func (t *PlayerTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
		return t, nil

	case playerTickMsg:
		// Clear seek flash on each tick so it only shows for ~1 second.
		t.seekFlash = ""
		// Keep ticking while the tab is alive.
		return t, playerTick()

	case tea.KeyMsg:
		if t.player == nil {
			return t, nil
		}
		// When chapter list is open, intercept all keys there.
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

// cycleSpeed advances through the speed steps.
func (t *PlayerTab) cycleSpeed() {
	st := t.player.GetStatus()
	current := st.Speed
	next := speedSteps[0] // default fallback
	for i, sp := range speedSteps {
		if abs(sp-current) < 0.01 {
			next = speedSteps[(i+1)%len(speedSteps)]
			break
		}
	}
	t.player.SetSpeed(next)
}

// cycleVolume increments volume by 10%, wrapping from 100% back to 0%.
func (t *PlayerTab) cycleVolume() {
	st := t.player.GetStatus()
	vol := st.Volume + 0.1
	if vol > 1.0+0.001 {
		vol = 0.0
	}
	if vol > 1.0 {
		vol = 1.0
	}
	t.player.SetVolume(vol)
}

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
		sb.WriteString(t.viewChapterList(st))
		return sb.String()
	}

	// Build player content for the panel
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
	progressLine := fmt.Sprintf("%s %s %s",
		posStr,
		styledProgressBar(barProgress, barWidth),
		durStr,
	)
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
		label := fmt.Sprintf("%dm", sleepPresets[t.sleepIndex])
		remaining := ""
		if st.SleepRemainMS > 0 {
			remaining = " (" + formatMS(st.SleepRemainMS) + " left)"
		}
		content.WriteString(fmt.Sprintf("\n%s  %s%s\n",
			subtitleStyle.Render("Sleep timer:"),
			pausedStyle.Render(label),
			pausedStyle.Render(remaining),
		))
	} else if st.SleepRemainMS > 0 {
		content.WriteString(fmt.Sprintf("\n%s  %s\n",
			subtitleStyle.Render("Sleep timer:"),
			pausedStyle.Render(formatMS(st.SleepRemainMS)),
		))
	}

	content.WriteString("\n")
	content.WriteString(helpStyle.Render("space play/pause  n/p chapter  \u2190/\u2192 seek  s speed  v vol  t sleep  c chapters"))

	// Wrap in a centered detail panel
	panelWidth := t.width - 4
	if panelWidth > 70 {
		panelWidth = 70
	}
	if panelWidth < 40 {
		panelWidth = 40
	}

	panel := detailPanelStyle.Width(panelWidth).Render(content.String())

	// Center horizontally
	if t.width > panelWidth+4 {
		padding := (t.width - panelWidth - 4) / 2
		panel = lipgloss.NewStyle().MarginLeft(padding).Render(panel)
	}

	sb.WriteString(panel)
	sb.WriteString("\n")

	return sb.String()
}

// formatMS formats a millisecond value as "H:MM:SS" or "M:SS".
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

// cycleSleepTimer advances through the sleep presets and sets the timer.
func (t *PlayerTab) cycleSleepTimer() {
	t.sleepIndex = (t.sleepIndex + 1) % len(sleepPresets)
	minutes := sleepPresets[t.sleepIndex]
	t.player.SetSleepTimer(time.Duration(minutes) * time.Minute)
}

// openChapterList populates the chapter list and opens the overlay.
func (t *PlayerTab) openChapterList() {
	pl := t.player.GetPlaylist()
	if pl == nil {
		return
	}
	t.chapterList = pl.Chapters
	st := t.player.GetStatus()
	t.chCursor = st.ChapterIndex
	t.showChapters = true

	// Center scroll around the current chapter.
	visible := t.chapterListHeight()
	t.chScroll = t.chCursor - visible/2
	if t.chScroll < 0 {
		t.chScroll = 0
	}
	max := len(t.chapterList) - visible
	if max < 0 {
		max = 0
	}
	if t.chScroll > max {
		t.chScroll = max
	}
}

// updateChapterList handles key events when the chapter list modal is open.
func (t *PlayerTab) updateChapterList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		t.showChapters = false
	case "enter":
		if t.chCursor >= 0 && t.chCursor < len(t.chapterList) {
			_ = t.player.JumpToChapter(t.chCursor)
			t.showChapters = false
		}
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
			visible := t.chapterListHeight()
			if t.chCursor >= t.chScroll+visible {
				t.chScroll = t.chCursor - visible + 1
			}
		}
	}
	return t, nil
}

// chapterListHeight returns the visible height for the chapter list.
func (t *PlayerTab) chapterListHeight() int {
	h := t.height - 10
	if h < 5 {
		h = 5
	}
	return h
}

// viewChapterList renders the chapter selection overlay.
func (t *PlayerTab) viewChapterList(st player.PlayerStatus) string {
	var content strings.Builder

	content.WriteString(titleStyle.Render("Select Chapter") + "\n")
	content.WriteString(subtitleStyle.Render(st.AudiobookTitle) + "\n\n")

	visible := t.chapterListHeight()

	// Scroll up indicator
	if t.chScroll > 0 {
		content.WriteString(subtitleStyle.Render("  ↑ more") + "\n")
	}

	end := t.chScroll + visible
	if end > len(t.chapterList) {
		end = len(t.chapterList)
	}

	for i := t.chScroll; i < end; i++ {
		ch := t.chapterList[i]
		prefix := "  "
		isCurrent := i == st.ChapterIndex
		isCursor := i == t.chCursor

		if isCurrent && isCursor {
			prefix = cursorStyle.Render("▶ ")
		} else if isCursor {
			prefix = cursorStyle.Render("> ")
		} else if isCurrent {
			prefix = "♪ "
		}

		dur := formatDuration(ch.Duration)
		line := fmt.Sprintf("%s%d. %s  %s", prefix, ch.Index+1, ch.Title, subtitleStyle.Render(dur))

		if isCursor {
			line = selectedStyle.Render(line)
		}

		content.WriteString(line + "\n")
	}

	// Scroll down indicator
	if end < len(t.chapterList) {
		content.WriteString(subtitleStyle.Render("  ↓ more") + "\n")
	}

	content.WriteString("\n")
	content.WriteString(helpStyle.Render("j/k navigate  enter select  esc close"))

	// Wrap in panel
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

	return "\n" + panel + "\n"
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

// abs returns the absolute value of a float64.
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
