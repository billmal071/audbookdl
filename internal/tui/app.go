package tui

import (
	"database/sql"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
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
	return &App{
		tabs: []Tab{
			NewSearchTab(database),
			NewDownloadsTab(database),
			NewLibraryTab(database, baseDir),
			NewPlayerTab(),
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
	sb.WriteString(tabBarStyle.Render(strings.Join(tabLabels, "")))
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
