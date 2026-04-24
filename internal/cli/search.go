package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/billmal071/audbookdl/internal/archive"
	"github.com/billmal071/audbookdl/internal/config"
	"github.com/billmal071/audbookdl/internal/db"
	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/librivox"
	"github.com/billmal071/audbookdl/internal/loyalbooks"
	"github.com/billmal071/audbookdl/internal/openlibrary"
	"github.com/billmal071/audbookdl/internal/search"
	"github.com/billmal071/audbookdl/internal/source"
	"github.com/spf13/cobra"
)

var (
	searchLimit    int
	searchSource   string
	searchLanguage string
	searchAuthor   string
	searchFormat   string
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search for audiobooks",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSearch,
}

func init() {
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "n", 10, "number of results to return")
	searchCmd.Flags().StringVarP(&searchSource, "source", "s", "", "filter by source (librivox, archive, loyalbooks, openlibrary)")
	searchCmd.Flags().StringVarP(&searchLanguage, "language", "l", "", "filter by language")
	searchCmd.Flags().StringVarP(&searchAuthor, "author", "a", "", "filter by author")
	searchCmd.Flags().StringVarP(&searchFormat, "format", "f", "", "filter by format (e.g. mp3)")
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := strings.Join(args, " ")

	cfg := config.Get()
	http := httpclient.New(
		httpclient.WithTimeout(30*time.Second),
		httpclient.WithUserAgent(cfg.Network.UserAgent),
	)

	searcher := buildSearcher(http, searchSource)

	opts := source.SearchOptions{
		Limit:    searchLimit,
		Language: searchLanguage,
		Author:   searchAuthor,
		Format:   searchFormat,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check search cache first.
	var books []*source.Audiobook
	cacheSource := searchSource
	if cacheSource == "" {
		cacheSource = "all"
	}
	if cached, err := db.GetCachedSearch(db.DB(), query, cacheSource); err == nil {
		var cachedBooks []*source.Audiobook
		if json.Unmarshal(cached, &cachedBooks) == nil && len(cachedBooks) > 0 {
			books = cachedBooks
		}
	}

	if len(books) == 0 {
		var err error
		books, err = searcher.Search(ctx, query, opts)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}
		// Cache results for 1 hour.
		if len(books) > 0 {
			if data, err := json.Marshal(books); err == nil {
				_ = db.SetCachedSearch(db.DB(), query, cacheSource, data, time.Hour)
			}
		}
	}

	if len(books) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	fmt.Printf("Found %d result(s) for %q:\n\n", len(books), query)
	for i, b := range books {
		fmt.Printf("%d. %s\n", i+1, b.Title)
		if b.Author != "" {
			fmt.Printf("   Author:   %s\n", b.Author)
		}
		if b.Narrator != "" {
			fmt.Printf("   Narrator: %s\n", b.Narrator)
		}
		if b.Duration > 0 {
			fmt.Printf("   Duration: %s\n", b.DurationFormatted())
		}
		if b.ChapterCount > 0 {
			fmt.Printf("   Chapters: %d\n", b.ChapterCount)
		}
		if b.Format != "" {
			fmt.Printf("   Format:   %s\n", b.Format)
		}
		fmt.Printf("   Source:   %s\n", b.Source)
		fmt.Printf("   ID:       %s\n", b.ID)
		fmt.Println()
	}

	// Save search to history
	sources := make([]string, 0, len(searcher.Sources()))
	for _, s := range searcher.Sources() {
		sources = append(sources, s.Name())
	}
	_ = db.AddSearchHistory(db.DB(), query, strings.Join(sources, ","), len(books))

	return nil
}

// buildSearcher creates a Searcher from the enabled sources in config, optionally
// filtered to a single source by sourceFilter.
func buildSearcher(http *httpclient.Client, sourceFilter string) *search.Searcher {
	cfg := config.Get()
	enabledSources := cfg.Search.Sources

	var sources []source.Source
	for _, name := range enabledSources {
		if sourceFilter != "" && name != sourceFilter {
			continue
		}
		switch name {
		case "librivox":
			sources = append(sources, librivox.NewClient("", http))
		case "archive":
			sources = append(sources, archive.NewClient("", http))
		case "loyalbooks":
			sources = append(sources, loyalbooks.NewClient("", http))
		case "openlibrary":
			sources = append(sources, openlibrary.NewClient("", "", http))
		}
	}

	// If a specific source was requested but not in config, add it anyway
	if sourceFilter != "" && len(sources) == 0 {
		switch sourceFilter {
		case "librivox":
			sources = append(sources, librivox.NewClient("", http))
		case "archive":
			sources = append(sources, archive.NewClient("", http))
		case "loyalbooks":
			sources = append(sources, loyalbooks.NewClient("", http))
		case "openlibrary":
			sources = append(sources, openlibrary.NewClient("", "", http))
		}
	}

	return search.New(sources...)
}
