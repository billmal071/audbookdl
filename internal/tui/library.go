package tui

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/billmal071/audbookdl/internal/db"
)

// refreshLibraryMsg carries completed downloads and bookmarks from the DB.
type refreshLibraryMsg struct {
	downloads []*db.AudiobookDownload
	bookmarks []*db.Bookmark
	err       error
}

// LibraryTab shows completed downloads grouped by author and bookmarks.
type LibraryTab struct {
	db        *sql.DB
	baseDir   string
	downloads []*db.AudiobookDownload
	bookmarks []*db.Bookmark
	cursor    int
	err       error
	width     int
	height    int
}

// NewLibraryTab constructs a LibraryTab.
func NewLibraryTab(database *sql.DB, baseDir string) *LibraryTab {
	return &LibraryTab{db: database, baseDir: baseDir}
}

func (t *LibraryTab) TabName() string { return "Library" }

func (t *LibraryTab) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	}
}

func (t *LibraryTab) Init() tea.Cmd {
	return t.refresh()
}

func (t *LibraryTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			totalItems := len(t.downloads) + len(t.bookmarks)
			if t.cursor < totalItems-1 {
				t.cursor++
			}
		}
		return t, nil

	case refreshLibraryMsg:
		t.err = msg.err
		t.downloads = msg.downloads
		t.bookmarks = msg.bookmarks
		totalItems := len(t.downloads) + len(t.bookmarks)
		if t.cursor >= totalItems {
			t.cursor = 0
		}
		return t, nil
	}

	return t, nil
}

func (t *LibraryTab) View() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render("  Library"))
	sb.WriteString("\n\n")

	if t.err != nil {
		sb.WriteString(failedStyle.Render("  Error: "+t.err.Error()) + "\n")
		return sb.String()
	}

	// Build a flat list of renderable rows for uniform cursor navigation.
	type row struct {
		text    string
		isTitle bool
	}
	var rows []row
	globalIdx := 0

	// --- Downloaded section ---
	if len(t.downloads) > 0 {
		rows = append(rows, row{text: subtitleStyle.Render("  Downloaded"), isTitle: true})

		// Group by author.
		type group struct {
			author string
			books  []*db.AudiobookDownload
		}
		groupMap := make(map[string]*group)
		var authorOrder []string
		for _, dl := range t.downloads {
			if _, ok := groupMap[dl.Author]; !ok {
				groupMap[dl.Author] = &group{author: dl.Author}
				authorOrder = append(authorOrder, dl.Author)
			}
			groupMap[dl.Author].books = append(groupMap[dl.Author].books, dl)
		}

		for _, author := range authorOrder {
			g := groupMap[author]
			rows = append(rows, row{text: "  " + sourceStyle.Render(g.author), isTitle: true})
			for _, dl := range g.books {
				cursor := "    "
				ts := titleStyle
				if globalIdx == t.cursor {
					cursor = "  " + cursorStyle.Render("> ")
					ts = selectedStyle
				}
				rows = append(rows, row{text: fmt.Sprintf("%s%s", cursor, ts.Render(dl.Title))})
				globalIdx++
			}
		}
	} else {
		rows = append(rows, row{text: subtitleStyle.Render("  No completed downloads yet."), isTitle: true})
	}

	// --- Bookmarks section ---
	rows = append(rows, row{text: ""})
	rows = append(rows, row{text: subtitleStyle.Render("  Bookmarks"), isTitle: true})

	if len(t.bookmarks) == 0 {
		rows = append(rows, row{text: subtitleStyle.Render("    No bookmarks yet."), isTitle: true})
	} else {
		for _, bm := range t.bookmarks {
			cursor := "    "
			ts := titleStyle
			if globalIdx == t.cursor {
				cursor = "  " + cursorStyle.Render("> ")
				ts = selectedStyle
			}
			rows = append(rows, row{text: fmt.Sprintf("%s%s  %s",
				cursor,
				ts.Render(bm.Title),
				subtitleStyle.Render(bm.Author),
			)})
			globalIdx++
		}
	}

	for _, r := range rows {
		sb.WriteString(r.text + "\n")
	}

	sb.WriteString(fmt.Sprintf("\n  %d downloaded  ·  %d bookmarked  |  r to refresh",
		len(t.downloads), len(t.bookmarks)))

	return sb.String()
}

// refresh queries the DB for completed downloads and bookmarks.
func (t *LibraryTab) refresh() tea.Cmd {
	return func() tea.Msg {
		rows, err := t.db.Query(
			`SELECT id, audiobook_id, title, author, narrator, source, status,
			        base_path, total_size, downloaded_size, created_at, updated_at, completed_at
			   FROM audiobook_downloads WHERE status = 'completed' ORDER BY author, title`,
		)
		if err != nil {
			return refreshLibraryMsg{err: err}
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

		bmRows, err := t.db.Query(
			`SELECT id, audiobook_id, title, author, narrator, source, page_url, note, created_at
			   FROM bookmarks ORDER BY created_at DESC`,
		)
		if err != nil {
			return refreshLibraryMsg{downloads: downloads, err: err}
		}
		defer bmRows.Close()

		var bookmarks []*db.Bookmark
		for bmRows.Next() {
			var bm db.Bookmark
			if err := bmRows.Scan(
				&bm.ID, &bm.AudiobookID, &bm.Title, &bm.Author,
				&bm.Narrator, &bm.Source, &bm.PageURL, &bm.Note, &bm.CreatedAt,
			); err != nil {
				continue
			}
			bookmarks = append(bookmarks, &bm)
		}

		return refreshLibraryMsg{downloads: downloads, bookmarks: bookmarks}
	}
}
