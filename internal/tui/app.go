package tui

import (
	"database/sql"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/billmal071/audbookdl/internal/player"
)

// Tab is the interface every TUI tab must satisfy.
type Tab interface {
	tea.Model
	TabName() string
	ShortHelp() []key.Binding
}

// App is the root bubbletea model that owns all tabs.
type App struct {
	tabs      []Tab
	activeTab int
	width     int
	height    int
	help      help.Model
	showHelp  bool
	db        *sql.DB
}

// NewApp constructs the App with its four tabs.
func NewApp(database *sql.DB, baseDir string) *App {
	h := help.New()
	h.ShowAll = false
	p := player.NewPlayer(database)
	return &App{
		tabs: []Tab{
			NewSearchTab(database),
			NewDownloadsTab(database),
			NewLibraryTab(database, baseDir),
			NewPlayerTab(p),
		},
		activeTab: 0,
		help:      h,
		db:        database,
	}
}

func (a *App) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, t := range a.tabs {
		if cmd := t.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		// Forward a size message with adjusted height to all tabs
		inner := tea.WindowSizeMsg{Width: msg.Width, Height: msg.Height - 4}
		var cmds []tea.Cmd
		for i, t := range a.tabs {
			updated, cmd := t.Update(inner)
			a.tabs[i] = updated.(Tab)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return a, tea.Batch(cmds...)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			// Stop the player so mpv doesn't keep running after exit.
			if pt, ok := a.tabs[3].(*PlayerTab); ok && pt.player != nil {
				pt.player.Stop()
			}
			return a, tea.Quit
		case "tab":
			a.activeTab = (a.activeTab + 1) % len(a.tabs)
			return a, nil
		case "shift+tab":
			a.activeTab = (a.activeTab - 1 + len(a.tabs)) % len(a.tabs)
			return a, nil
		case "?":
			a.showHelp = !a.showHelp
			return a, nil
		}
	}

	// Handle playAudiobookMsg from Library tab — load into player, switch to Player tab
	if msg, ok := msg.(playAudiobookMsg); ok {
		if pt, ok := a.tabs[3].(*PlayerTab); ok && pt.player != nil {
			pt.player.Load(msg.playlist)
			pt.player.Play()
			a.activeTab = 3 // Switch to Player tab
		}
		return a, nil
	}

	// Always forward tick messages to the relevant tabs, even when inactive,
	// so they keep refreshing in the background.
	switch msg.(type) {
	case tickMsg:
		// Downloads tab (index 1) auto-refresh.
		for i, t := range a.tabs {
			if _, ok := t.(*DownloadsTab); ok && i != a.activeTab {
				updated, cmd := t.Update(msg)
				a.tabs[i] = updated.(Tab)
				if cmd != nil {
					// Forward to active tab too, then return.
					activeUpdated, activeCmd := a.tabs[a.activeTab].Update(msg)
					a.tabs[a.activeTab] = activeUpdated.(Tab)
					return a, tea.Batch(cmd, activeCmd)
				}
			}
		}
	case playerTickMsg:
		// Player tab (index 3) keeps ticking for position updates.
		for i, t := range a.tabs {
			if _, ok := t.(*PlayerTab); ok && i != a.activeTab {
				updated, cmd := t.Update(msg)
				a.tabs[i] = updated.(Tab)
				if cmd != nil {
					return a, cmd
				}
			}
		}
	}

	// Forward all other messages to the active tab.
	updated, cmd := a.tabs[a.activeTab].Update(msg)
	a.tabs[a.activeTab] = updated.(Tab)
	return a, cmd
}

func (a *App) View() string {
	var sb strings.Builder

	// Tab bar
	var tabLabels []string
	for i, t := range a.tabs {
		label := " " + t.TabName() + " "
		if i == a.activeTab {
			tabLabels = append(tabLabels, activeTabStyle.Render(label))
		} else {
			tabLabels = append(tabLabels, inactiveTabStyle.Render(label))
		}
	}
	tabLine := strings.Join(tabLabels, dividerStyle.Render(" │ "))
	sb.WriteString(tabLine)
	sb.WriteString("\n")
	sb.WriteString(dividerStyle.Render(strings.Repeat("─", a.width)))
	sb.WriteString("\n")

	// Active tab content
	sb.WriteString(a.tabs[a.activeTab].View())
	sb.WriteString("\n")

	// Status / help bar
	helpLine := helpStyle.Render("tab next  shift+tab prev  q quit  ? help")
	if a.showHelp {
		bindings := a.tabs[a.activeTab].ShortHelp()
		var parts []string
		for _, b := range bindings {
			parts = append(parts, b.Help().Key+"  "+b.Help().Desc)
		}
		if len(parts) > 0 {
			helpLine = helpStyle.Render(strings.Join(parts, "  |  "))
		}
	}
	sb.WriteString(statusBarStyle.Render(helpLine))

	return sb.String()
}

// Run creates a bubbletea program with AltScreen and runs it.
func Run(database *sql.DB, baseDir string) error {
	app := NewApp(database, baseDir)
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
