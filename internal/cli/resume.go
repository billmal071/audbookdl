package cli

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/billmal071/audbookdl/internal/config"
	"github.com/billmal071/audbookdl/internal/db"
	"github.com/billmal071/audbookdl/internal/downloader"
	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/spf13/cobra"
)

var resumeCmd = &cobra.Command{
	Use:   "resume [download-id]",
	Short: "Resume a paused or failed download",
	Args:  cobra.ExactArgs(1),
	RunE:  runResume,
}

func runResume(cmd *cobra.Command, args []string) error {
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid download ID %q: %w", args[0], err)
	}

	download, err := db.GetDownload(db.DB(), id)
	if err != nil {
		return fmt.Errorf("get download %d: %w", id, err)
	}

	if download.Status != db.StatusPaused && download.Status != db.StatusFailed {
		return fmt.Errorf("download %d has status %q; only paused or failed downloads can be resumed", id, download.Status)
	}

	cfg := config.Get()
	http := httpclient.New(
		httpclient.WithTimeout(30*time.Second),
		httpclient.WithUserAgent(cfg.Network.UserAgent),
	)

	src := getSource(http, download.Source)
	if src == nil {
		return fmt.Errorf("unknown source %q for download %d", download.Source, id)
	}

	ctx := context.Background()

	fmt.Printf("Fetching chapters for %q by %s from %s...\n", download.Title, download.Author, download.Source)
	chapters, err := src.GetChapters(ctx, download.AudiobookID)
	if err != nil {
		return fmt.Errorf("fetch chapters: %w", err)
	}
	if len(chapters) == 0 {
		return fmt.Errorf("no chapters found for audiobook %q", download.AudiobookID)
	}

	outDir := cfg.Download.Directory
	mgr := downloader.NewManager(db.DB(), outDir, cfg.Download.MaxConcurrent)

	chapterRecords, _ := db.ListChapterDownloads(db.DB(), id)
	completedCount := 0
	for _, c := range chapterRecords {
		if c.Status == db.StatusCompleted {
			completedCount++
		}
	}
	fmt.Printf("Resuming download #%d: %d/%d chapters already completed. Downloading remaining...\n",
		id, completedCount, len(chapters))

	progressFn := func(chapterIndex, totalChapters int, chapterBytes int64) {
		if verbose {
			fmt.Printf("\r  Chapter %d/%d — %s downloaded",
				chapterIndex, totalChapters, bytesFormatted(chapterBytes))
		}
	}

	if err := mgr.ResumeDownload(ctx, id, chapters, progressFn); err != nil {
		return fmt.Errorf("resume failed: %w", err)
	}

	fmt.Printf("\nDownload complete. Files saved to: %s/%s/%s\n", outDir, download.Author, download.Title)
	return nil
}
