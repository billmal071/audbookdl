package tui

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/billmal071/audbookdl/internal/db"
)

// refreshDownloadsMsg carries a fresh download list from the DB.
type refreshDownloadsMsg struct {
	downloads []*db.AudiobookDownload
	err       error
}

// DownloadsTab shows all audiobook downloads and their status.
type DownloadsTab struct {
	db        *sql.DB
	downloads []*db.AudiobookDownload
	cursor    int
	err       error
	width     int
	height    int
}

// NewDownloadsTab constructs a DownloadsTab.
func NewDownloadsTab(database *sql.DB) *DownloadsTab {
	return &DownloadsTab{db: database}
}

func (t *DownloadsTab) TabName() string { return "Downloads" }

func (t *DownloadsTab) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	}
}

func (t *DownloadsTab) Init() tea.Cmd {
	return t.refresh()
}

func (t *DownloadsTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
		return t, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			return t, t.refresh()
		case "up", "k":
			if t.cursor > 0 {
				t.cursor--
			}
		case "down", "j":
			if t.cursor < len(t.downloads)-1 {
				t.cursor++
			}
		}
		return t, nil

	case refreshDownloadsMsg:
		t.err = msg.err
		t.downloads = msg.downloads
		if t.cursor >= len(t.downloads) {
			t.cursor = 0
		}
		return t, nil
	}

	return t, nil
}

func (t *DownloadsTab) View() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render("  Downloads"))
	sb.WriteString("\n\n")

	if t.err != nil {
		sb.WriteString(failedStyle.Render("  Error: "+t.err.Error()) + "\n")
		return sb.String()
	}

	if len(t.downloads) == 0 {
		sb.WriteString(subtitleStyle.Render("  No downloads yet. Use the Search tab to find audiobooks.") + "\n")
		return sb.String()
	}

	maxRows := t.height - 6
	if maxRows < 1 {
		maxRows = 10
	}

	start := 0
	if t.cursor >= maxRows {
		start = t.cursor - maxRows + 1
	}
	end := start + maxRows
	if end > len(t.downloads) {
		end = len(t.downloads)
	}

	for i := start; i < end; i++ {
		dl := t.downloads[i]
		cursor := "  "
		if i == t.cursor {
			cursor = cursorStyle.Render("> ")
		}

		var pct float64
		var barStr string
		if dl.TotalSize > 0 {
			pct = float64(dl.DownloadedSize) / float64(dl.TotalSize) * 100
			barStr = " " + progressBar(pct, 20) + fmt.Sprintf(" %.0f%%", pct)
		}

		statusIcon := renderStatus(dl.Status)
		line := fmt.Sprintf("%s%s %s  %s%s",
			cursor,
			statusIcon,
			titleStyle.Render(dl.Title),
			subtitleStyle.Render(dl.Author),
			subtitleStyle.Render(barStr),
		)
		sb.WriteString(line + "\n")
	}

	sb.WriteString(fmt.Sprintf("\n  %d download(s)  |  r to refresh", len(t.downloads)))

	return sb.String()
}

// progressBar renders a text progress bar of the given width.
// progress is in the range [0, 100].
func progressBar(progress float64, width int) string {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	filled := int(progress / 100 * float64(width))
	empty := width - filled
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", empty) + "]"
}

// refresh queries the DB for all downloads.
func (t *DownloadsTab) refresh() tea.Cmd {
	return func() tea.Msg {
		rows, err := t.db.Query(
			`SELECT id, audiobook_id, title, author, narrator, source, status,
			        base_path, total_size, downloaded_size, created_at, updated_at, completed_at
			   FROM audiobook_downloads ORDER BY created_at DESC`,
		)
		if err != nil {
			return refreshDownloadsMsg{err: err}
		}
		defer rows.Close()

		var downloads []*db.AudiobookDownload
		for rows.Next() {
			var d db.AudiobookDownload
			if err := rows.Scan(
				&d.ID, &d.AudiobookID, &d.Title, &d.Author, &d.Narrator,
				&d.Source, &d.Status, &d.BasePath, &d.TotalSize, &d.DownloadedSize,
				&d.CreatedAt, &d.UpdatedAt, &d.CompletedAt,
			); err != nil {
				continue
			}
			downloads = append(downloads, &d)
		}
		return refreshDownloadsMsg{downloads: downloads}
	}
}

// renderStatus returns a styled status icon string.
func renderStatus(status db.DownloadStatus) string {
	switch status {
	case db.StatusCompleted:
		return completedStyle.Render("[done]")
	case db.StatusDownloading:
		return downloadingStyle.Render("[....]")
	case db.StatusPaused:
		return pausedStyle.Render("[stop]")
	case db.StatusFailed:
		return failedStyle.Render("[fail]")
	default:
		return pendingStyle.Render("[wait]")
	}
}
