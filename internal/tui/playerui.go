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

// PlayerTab is the audio player tab.
type PlayerTab struct {
	player *player.Player
	width  int
	height int
}

// NewPlayerTab constructs a PlayerTab wired to the given Player.
func NewPlayerTab(p *player.Player) *PlayerTab {
	return &PlayerTab{player: p}
}

func (t *PlayerTab) TabName() string { return "Player" }

func (t *PlayerTab) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "play/pause")),
		key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next chapter")),
		key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prev chapter")),
		key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "skip -15s")),
		key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "skip +15s")),
		key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "cycle speed")),
		key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "cycle volume")),
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
		// Keep ticking while the tab is alive.
		return t, playerTick()

	case tea.KeyMsg:
		if t.player == nil {
			return t, nil
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
		case "right", "l":
			t.player.SkipForward(15 * time.Second)
		case "s":
			t.cycleSpeed()
		case "v":
			t.cycleVolume()
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
	content.WriteString(fmt.Sprintf("%s %s %s\n\n",
		posStr,
		styledProgressBar(barProgress, barWidth),
		durStr,
	))

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
	if st.SleepRemainMS > 0 {
		content.WriteString(fmt.Sprintf("\n%s  %s\n",
			subtitleStyle.Render("Sleep timer:"),
			pausedStyle.Render(formatMS(st.SleepRemainMS)),
		))
	}

	content.WriteString("\n")
	content.WriteString(helpStyle.Render("space play/pause  n next  p prev  ←/h -15s  →/l +15s  s speed  v volume"))

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
	if ms < 0 {
		ms = 0
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

// abs returns the absolute value of a float64.
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
