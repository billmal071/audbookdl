package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/billmal071/audbookdl/internal/archive"
	"github.com/billmal071/audbookdl/internal/config"
	"github.com/billmal071/audbookdl/internal/db"
	"github.com/billmal071/audbookdl/internal/downloader"
	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/librivox"
	"github.com/billmal071/audbookdl/internal/loyalbooks"
	"github.com/billmal071/audbookdl/internal/openlibrary"
	"github.com/billmal071/audbookdl/internal/source"
	"github.com/spf13/cobra"
)

var (
	downloadSource string
	downloadOutput string
)

var downloadCmd = &cobra.Command{
	Use:   "download [audiobook-id]",
	Short: "Download an audiobook by ID",
	Args:  cobra.ExactArgs(1),
	RunE:  runDownload,
}

func init() {
	downloadCmd.Flags().StringVarP(&downloadSource, "source", "s", "librivox", "source to download from (librivox, archive, loyalbooks, openlibrary)")
	downloadCmd.Flags().StringVarP(&downloadOutput, "output", "o", "", "output directory (default: config download directory)")
}

func runDownload(cmd *cobra.Command, args []string) error {
	bookID := args[0]
	cfg := config.Get()

	http := httpclient.New(
		httpclient.WithTimeout(30*time.Second),
		httpclient.WithUserAgent(cfg.Network.UserAgent),
	)

	src := getSource(http, downloadSource)
	if src == nil {
		return fmt.Errorf("unknown source %q; valid sources: librivox, archive, loyalbooks, openlibrary", downloadSource)
	}

	ctx := context.Background()

	// Fetch book metadata
	fmt.Printf("Fetching metadata for %q from %s...\n", bookID, src.Name())
	book, err := fetchBookMetadata(ctx, src, bookID)
	if err != nil {
		// Use stub if metadata lookup fails
		book = &source.Audiobook{ID: bookID, Title: bookID, Author: "Unknown", Source: src.Name()}
	}

	fmt.Printf("Fetching chapters for %q by %s...\n", book.Title, book.Author)
	chapters, err := src.GetChapters(ctx, bookID)
	if err != nil {
		return fmt.Errorf("fetch chapters: %w", err)
	}

	if len(chapters) == 0 {
		return fmt.Errorf("no chapters found for audiobook %q", bookID)
	}

	book.ChapterCount = len(chapters)
	fmt.Printf("Found %d chapter(s). Starting download...\n", len(chapters))

	outDir := downloadOutput
	if outDir == "" {
		outDir = cfg.Download.Directory
	}

	mgr := downloader.NewManager(db.DB(), outDir, cfg.Download.MaxConcurrent)

	progressFn := func(chapterIndex, totalChapters int, chapterBytes int64) {
		if verbose {
			fmt.Printf("\r  Chapter %d/%d — %s downloaded",
				chapterIndex, totalChapters, bytesFormatted(chapterBytes))
		}
	}

	if err := mgr.DownloadAudiobook(ctx, book, chapters, progressFn); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	fmt.Printf("\nDownload complete. Files saved to: %s/%s/%s\n", outDir, book.Author, book.Title)
	return nil
}

// getSource returns the source.Source implementation for the given name.
func getSource(http *httpclient.Client, name string) source.Source {
	switch name {
	case "librivox":
		return librivox.NewClient("", http)
	case "archive":
		return archive.NewClient("", http)
	case "loyalbooks":
		return loyalbooks.NewClient("", http)
	case "openlibrary":
		return openlibrary.NewClient("", "", http)
	default:
		return nil
	}
}

// fetchBookMetadata tries to get audiobook metadata for a given ID.
func fetchBookMetadata(ctx context.Context, src source.Source, bookID string) (*source.Audiobook, error) {
	// LibriVox has a dedicated GetBook endpoint
	if lv, ok := src.(*librivox.Client); ok {
		return lv.GetBook(ctx, bookID)
	}
	// For other sources, search with the ID as query
	results, err := src.Search(ctx, bookID, source.SearchOptions{Limit: 10})
	if err != nil {
		return nil, err
	}
	for _, b := range results {
		if b.ID == bookID {
			return b, nil
		}
	}
	if len(results) > 0 {
		return results[0], nil
	}
	return nil, fmt.Errorf("no metadata found for %s", bookID)
}

// bytesFormatted returns a human-readable byte count string.
func bytesFormatted(b int64) string {
	const KB = 1024
	const MB = 1024 * KB
	switch {
	case b < KB:
		return fmt.Sprintf("%d B", b)
	case b < MB:
		return fmt.Sprintf("%.1f KB", float64(b)/KB)
	default:
		return fmt.Sprintf("%.1f MB", float64(b)/MB)
	}
}
