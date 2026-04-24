package tui

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/billmal071/audbookdl/internal/archive"
	"github.com/billmal071/audbookdl/internal/config"
	"github.com/billmal071/audbookdl/internal/db"
	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/librivox"
	"github.com/billmal071/audbookdl/internal/loyalbooks"
	"github.com/billmal071/audbookdl/internal/openlibrary"
	"github.com/billmal071/audbookdl/internal/search"
	"github.com/billmal071/audbookdl/internal/source"
)

// searchResultMsg carries results from a background search.
type searchResultMsg struct {
	books []*source.Audiobook
	err   error
}

// bookmarkSavedMsg signals that a bookmark was saved (or failed).
type bookmarkSavedMsg struct {
	title string
	err   error
}

// SearchTab is the interactive search tab.
type SearchTab struct {
	db           *sql.DB
	textinput    textinput.Model
	spinner      spinner.Model
	results      []*source.Audiobook
	cursor       int
	loading      bool
	err          error
	width        int
	height       int
	showDetail   bool
	selectedBook *source.Audiobook
	statusMsg    string
}

// NewSearchTab constructs a SearchTab.
func NewSearchTab(database *sql.DB) *SearchTab {
	ti := textinput.New()
	ti.Placeholder = "Search audiobooks..."
	ti.Focus()
	ti.CharLimit = 200

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = downloadingStyle

	return &SearchTab{
		db:        database,
		textinput: ti,
		spinner:   sp,
	}
}

func (t *SearchTab) TabName() string { return "Search" }

func (t *SearchTab) ShortHelp() []key.Binding {
	if t.showDetail {
		return []key.Binding{
			key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "download")),
			key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "bookmark")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		}
	}
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "search / view detail")),
		key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	}
}

func (t *SearchTab) Init() tea.Cmd {
	return textinput.Blink
}

func (t *SearchTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
		return t, nil

	case tea.KeyMsg:
		// --- Detail view keybindings ---
		if t.showDetail {
			switch msg.String() {
			case "esc":
				t.showDetail = false
				t.selectedBook = nil
				t.statusMsg = ""
				return t, nil
			case "d":
				t.statusMsg = fmt.Sprintf("Download queued: %s", t.selectedBook.Title)
				return t, nil
			case "b":
				book := t.selectedBook
				return t, t.doBookmark(book)
			}
			return t, nil
		}

		if t.loading {
			// Absorb keys while loading.
			return t, t.spinner.Tick
		}

		switch msg.String() {
		case "enter":
			// If textinput is focused and has content, run search.
			if t.textinput.Focused() {
				query := strings.TrimSpace(t.textinput.Value())
				if query == "" {
					return t, nil
				}
				t.loading = true
				t.err = nil
				t.results = nil
				t.cursor = 0
				return t, tea.Batch(t.spinner.Tick, doSearch(query))
			}
			// Otherwise open detail view for the selected result.
			if len(t.results) > 0 {
				t.selectedBook = t.results[t.cursor]
				t.showDetail = true
				t.statusMsg = ""
				return t, nil
			}

		case "up", "k":
			if t.cursor > 0 {
				t.cursor--
			}
			return t, nil

		case "down", "j":
			if t.cursor < len(t.results)-1 {
				t.cursor++
			}
			return t, nil

		case "esc":
			// Blur the input so arrow keys navigate the list.
			if t.textinput.Focused() {
				t.textinput.Blur()
			} else {
				t.textinput.Focus()
			}
			return t, nil
		}

	case searchResultMsg:
		t.loading = false
		t.err = msg.err
		t.results = msg.books
		t.cursor = 0
		// Blur input so enter/arrows navigate results
		if len(msg.books) > 0 {
			t.textinput.Blur()
		}
		return t, nil

	case bookmarkSavedMsg:
		if msg.err != nil {
			t.statusMsg = "Bookmark failed: " + msg.err.Error()
		} else {
			t.statusMsg = "Bookmarked: " + msg.title
		}
		return t, nil

	case spinner.TickMsg:
		if t.loading {
			var cmd tea.Cmd
			t.spinner, cmd = t.spinner.Update(msg)
			return t, cmd
		}
	}

	var cmd tea.Cmd
	t.textinput, cmd = t.textinput.Update(msg)
	return t, cmd
}

func (t *SearchTab) View() string {
	var sb strings.Builder

	// --- Detail view ---
	if t.showDetail && t.selectedBook != nil {
		book := t.selectedBook
		sb.WriteString("\n")
		sb.WriteString(titleStyle.Render("  " + book.Title))
		sb.WriteString("\n\n")

		if book.Author != "" {
			sb.WriteString(fmt.Sprintf("  %s  %s\n",
				subtitleStyle.Render("Author:"),
				titleStyle.Render(book.Author),
			))
		}
		if book.Narrator != "" {
			sb.WriteString(fmt.Sprintf("  %s  %s\n",
				subtitleStyle.Render("Narrator:"),
				titleStyle.Render(book.Narrator),
			))
		}
		if book.Duration > 0 {
			sb.WriteString(fmt.Sprintf("  %s  %s\n",
				subtitleStyle.Render("Duration:"),
				titleStyle.Render(book.DurationFormatted()),
			))
		}
		if book.ChapterCount > 0 {
			sb.WriteString(fmt.Sprintf("  %s  %s\n",
				subtitleStyle.Render("Chapters:"),
				titleStyle.Render(fmt.Sprintf("%d", book.ChapterCount)),
			))
		}
		if book.Format != "" {
			sb.WriteString(fmt.Sprintf("  %s  %s\n",
				subtitleStyle.Render("Format:"),
				titleStyle.Render(book.Format),
			))
		}
		if book.Source != "" {
			sb.WriteString(fmt.Sprintf("  %s  %s\n",
				subtitleStyle.Render("Source:"),
				sourceStyle.Render(book.Source),
			))
		}

		if book.Description != "" {
			sb.WriteString("\n")
			sb.WriteString(subtitleStyle.Render("  Description:"))
			sb.WriteString("\n")
			// Wrap description to width.
			desc := book.Description
			maxWidth := t.width - 4
			if maxWidth < 40 {
				maxWidth = 40
			}
			if len(desc) > maxWidth*4 {
				desc = desc[:maxWidth*4] + "..."
			}
			sb.WriteString("  " + subtitleStyle.Render(desc))
			sb.WriteString("\n")
		}

		sb.WriteString("\n")
		if t.statusMsg != "" {
			sb.WriteString("  " + completedStyle.Render(t.statusMsg))
			sb.WriteString("\n\n")
		}
		sb.WriteString(helpStyle.Render("  d download  b bookmark  esc back"))
		sb.WriteString("\n")
		return sb.String()
	}

	// --- List view ---
	sb.WriteString("\n")
	sb.WriteString(t.textinput.View())
	sb.WriteString("\n\n")

	if t.loading {
		sb.WriteString(fmt.Sprintf("  %s Searching...\n", t.spinner.View()))
		return sb.String()
	}

	if t.err != nil {
		sb.WriteString(failedStyle.Render("  Error: "+t.err.Error()) + "\n")
		return sb.String()
	}

	if len(t.results) == 0 {
		sb.WriteString(subtitleStyle.Render("  No results. Type a query and press Enter.") + "\n")
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
	if end > len(t.results) {
		end = len(t.results)
	}

	for i := start; i < end; i++ {
		book := t.results[i]
		cursor := "  "
		titleS := titleStyle
		if i == t.cursor {
			cursor = cursorStyle.Render("> ")
			titleS = selectedStyle
		}

		dur := ""
		if book.Duration > 0 {
			dur = " · " + book.DurationFormatted()
		}
		chapters := ""
		if book.ChapterCount > 0 {
			chapters = fmt.Sprintf(" · %d ch", book.ChapterCount)
		}
		narrator := ""
		if book.Narrator != "" {
			narrator = " · " + book.Narrator
		}

		line := fmt.Sprintf("%s%s\n     %s%s%s%s%s",
			cursor,
			titleS.Render(book.Title),
			subtitleStyle.Render(book.Author+narrator),
			subtitleStyle.Render(dur),
			subtitleStyle.Render(chapters),
			"  ",
			sourceStyle.Render("["+book.Source+"]"),
		)
		sb.WriteString(line + "\n")
	}

	sb.WriteString(fmt.Sprintf("\n  %d result(s)  |  enter to view detail", len(t.results)))

	return sb.String()
}

// doBookmark saves a bookmark for the given audiobook.
func (t *SearchTab) doBookmark(book *source.Audiobook) tea.Cmd {
	database := t.db
	return func() tea.Msg {
		bm := &db.Bookmark{
			AudiobookID: book.ID,
			Title:       book.Title,
			Author:      book.Author,
			Narrator:    book.Narrator,
			Source:      book.Source,
			PageURL:     book.PageURL,
		}
		_, err := db.CreateBookmark(database, bm)
		return bookmarkSavedMsg{title: book.Title, err: err}
	}
}

// doSearch executes the search in the background and returns a Cmd.
func doSearch(query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		hc := httpclient.New(
			httpclient.WithTimeout(30*time.Second),
			httpclient.WithUserAgent(config.Get().Network.UserAgent),
		)

		searcher := buildDefaultSearcher(hc)
		opts := source.SearchOptions{Limit: config.Get().Search.DefaultLimit}
		books, err := searcher.Search(ctx, query, opts)
		return searchResultMsg{books: books, err: err}
	}
}

// buildDefaultSearcher constructs a Searcher from the configured sources.
func buildDefaultSearcher(hc *httpclient.Client) *search.Searcher {
	cfg := config.Get()
	var sources []source.Source

	sourceSet := make(map[string]bool)
	for _, s := range cfg.Search.Sources {
		sourceSet[s] = true
	}

	// Default to all sources if none configured.
	if len(sourceSet) == 0 {
		sourceSet = map[string]bool{
			"librivox":    true,
			"archive":     true,
			"loyalbooks":  true,
			"openlibrary": true,
		}
	}

	if sourceSet["librivox"] {
		sources = append(sources, librivox.NewClient("", hc))
	}
	if sourceSet["archive"] {
		sources = append(sources, archive.NewClient("", hc))
	}
	if sourceSet["loyalbooks"] {
		sources = append(sources, loyalbooks.NewClient("", hc))
	}
	if sourceSet["openlibrary"] {
		sources = append(sources, openlibrary.NewClient("", "", hc))
	}

	return search.New(sources...)
}
