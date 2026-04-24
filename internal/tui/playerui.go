package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// PlayerTab is a stub placeholder for the audio player (Plan 6).
type PlayerTab struct {
	width  int
	height int
}

// NewPlayerTab constructs the player stub.
func NewPlayerTab() *PlayerTab {
	return &PlayerTab{}
}

func (t *PlayerTab) TabName() string { return "Player" }

func (t *PlayerTab) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "play/pause")),
		key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next chapter")),
		key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prev chapter")),
	}
}

func (t *PlayerTab) Init() tea.Cmd { return nil }

func (t *PlayerTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		t.width = ws.Width
		t.height = ws.Height
	}
	return t, nil
}

func (t *PlayerTab) View() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(subtitleStyle.Render("  No audiobook loaded. Select from Library tab."))
	sb.WriteString("\n")
	return sb.String()
}
