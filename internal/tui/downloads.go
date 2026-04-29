package tui

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/billmal071/audbookdl/internal/config"
	"github.com/billmal071/audbookdl/internal/db"
	"github.com/billmal071/audbookdl/internal/downloader"
	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/librivox"
	"github.com/billmal071/audbookdl/internal/loyalbooks"
	"github.com/billmal071/audbookdl/internal/openlibrary"
	"github.com/billmal071/audbookdl/internal/source"

	"github.com/billmal071/audbookdl/internal/archive"
)

// tickMsg triggers a periodic refresh of the downloads list.
type tickMsg struct{}

// refreshDownloadsMsg carries a fresh download list from the DB.
type refreshDownloadsMsg struct {
	downloads []*db.AudiobookDownload
	err       error
}

// resumeFinishedMsg signals that a resume attempt completed.
type resumeFinishedMsg struct {
	dlID  int64
	title string
	err   error
}

// DownloadsTab shows all audiobook downloads and their status.
type DownloadsTab struct {
	db        *sql.DB
	downloads []*db.AudiobookDownload
	cursor    int
	err       error
	width     int
	height    int
	statusMsg string
}

// NewDownloadsTab constructs a DownloadsTab.
func NewDownloadsTab(database *sql.DB) *DownloadsTab {
	return &DownloadsTab{db: database}
}

func (t *DownloadsTab) TabName() string { return "Downloads" }

func (t *DownloadsTab) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "resume")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	}
}

func (t *DownloadsTab) Init() tea.Cmd {
	return tea.Batch(t.refresh(), t.tick())
}

// tick returns a command that sends a tickMsg after 2 seconds.
func (t *DownloadsTab) tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
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
		case "R":
			if t.cursor >= 0 && t.cursor < len(t.downloads) {
				dl := t.downloads[t.cursor]
				if dl.Status == db.StatusFailed || dl.Status == db.StatusPaused {
					t.statusMsg = fmt.Sprintf("Resuming %s...", dl.Title)
					return t, t.resumeDownload(dl)
				}
				t.statusMsg = "Only failed or paused downloads can be resumed"
			}
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

	case resumeFinishedMsg:
		if msg.err != nil {
			t.statusMsg = fmt.Sprintf("Resume failed: %v", msg.err)
		} else {
			t.statusMsg = fmt.Sprintf("Resumed: %s", msg.title)
		}
		return t, t.refresh()

	case tickMsg:
		return t, tea.Batch(t.refresh(), t.tick())

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

		statusIcon := renderStatus(dl.Status)
		titleLine := statusIcon + " " + titleStyle.Render(dl.Title)
		if dl.Author != "" {
			titleLine += subtitleStyle.Render(" — " + dl.Author)
		}

		var progressLine string
		if dl.TotalSize > 0 {
			pct := float64(dl.DownloadedSize) / float64(dl.TotalSize) * 100
			barWidth := 30
			if t.width > 80 {
				barWidth = 40
			}
			progressLine = styledProgressBar(pct, barWidth) + fmt.Sprintf(" %.0f%%", pct)
		} else if chapters, err := db.ListChapterDownloads(t.db, dl.ID); err == nil && len(chapters) > 0 {
			done := 0
			for _, c := range chapters {
				if c.Status == db.StatusCompleted {
					done++
				}
			}
			pct := float64(done) / float64(len(chapters)) * 100
			barWidth := 30
			if t.width > 80 {
				barWidth = 40
			}
			progressLine = styledProgressBar(pct, barWidth) + fmt.Sprintf(" %d/%d chapters", done, len(chapters))
		} else {
			progressLine = subtitleStyle.Render("size unknown")
		}

		content := titleLine + "\n" + progressLine

		cardWidth := t.width - 4
		if cardWidth < 40 {
			cardWidth = 40
		}

		var card string
		if i == t.cursor {
			card = selectedCardStyle.Width(cardWidth).Render(content)
		} else {
			card = cardStyle.Width(cardWidth).Render(content)
		}

		sb.WriteString(card)
		sb.WriteString("\n")
	}

	footer := fmt.Sprintf("\n  %d download(s)  |  R resume  |  r refresh", len(t.downloads))
	if t.statusMsg != "" {
		footer += "  |  " + t.statusMsg
	}
	sb.WriteString(footer)

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
			var completedAt sql.NullTime
			if err := rows.Scan(
				&d.ID, &d.AudiobookID, &d.Title, &d.Author, &d.Narrator,
				&d.Source, &d.Status, &d.BasePath, &d.TotalSize, &d.DownloadedSize,
				&d.CreatedAt, &d.UpdatedAt, &completedAt,
			); err != nil {
				continue
			}
			if completedAt.Valid {
				d.CompletedAt = &completedAt.Time
			}

			// Auto-clean: if completed but files deleted, remove from DB
			if d.Status == db.StatusCompleted && d.BasePath != "" {
				if _, err := os.Stat(d.BasePath); os.IsNotExist(err) {
					t.db.Exec("DELETE FROM audiobook_downloads WHERE id = ?", d.ID)
					continue
				}
			}

			downloads = append(downloads, &d)
		}
		return refreshDownloadsMsg{downloads: downloads}
	}
}

// resumeDownload re-downloads failed/paused chapters for the given download.
func (t *DownloadsTab) resumeDownload(dl *db.AudiobookDownload) tea.Cmd {
	database := t.db
	return func() tea.Msg {
		cfg := config.Get()
		hc := httpclient.New(
			httpclient.WithTimeout(30*time.Second),
			httpclient.WithUserAgent(cfg.Network.UserAgent),
		)

		var src source.Source
		switch dl.Source {
		case "librivox":
			src = librivox.NewClient("", hc)
		case "archive":
			src = archive.NewClient("", hc)
		case "loyalbooks":
			src = loyalbooks.NewClient("", hc)
		case "openlibrary":
			src = openlibrary.NewClient("", "", hc)
		default:
			return resumeFinishedMsg{dlID: dl.ID, title: dl.Title, err: fmt.Errorf("unknown source: %s", dl.Source)}
		}

		ctx := context.Background()
		chapters, err := src.GetChapters(ctx, dl.AudiobookID)
		if err != nil {
			return resumeFinishedMsg{dlID: dl.ID, title: dl.Title, err: err}
		}

		mgr := downloader.NewManager(database, cfg.Download.Directory, cfg.Download.MaxConcurrent)
		err = mgr.ResumeDownload(ctx, dl.ID, chapters, nil)
		return resumeFinishedMsg{dlID: dl.ID, title: dl.Title, err: err}
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
