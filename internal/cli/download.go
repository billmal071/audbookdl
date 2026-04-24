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

	fmt.Printf("Fetching chapters for %q from %s...\n", bookID, src.Name())

	ctx := context.Background()
	chapters, err := src.GetChapters(ctx, bookID)
	if err != nil {
		return fmt.Errorf("fetch chapters: %w", err)
	}

	if len(chapters) == 0 {
		return fmt.Errorf("no chapters found for audiobook %q", bookID)
	}

	fmt.Printf("Found %d chapter(s). Starting download...\n", len(chapters))

	// Build a minimal audiobook stub from what we know
	book := &source.Audiobook{
		ID:           bookID,
		Title:        bookID, // will be refined once we have metadata
		Source:       src.Name(),
		ChapterCount: len(chapters),
	}

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
