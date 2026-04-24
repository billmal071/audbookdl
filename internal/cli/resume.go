package cli

import (
	"fmt"
	"strconv"

	"github.com/billmal071/audbookdl/internal/db"
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

	if err := db.UpdateDownloadStatus(db.DB(), id, db.StatusPending); err != nil {
		return fmt.Errorf("resume download %d: %w", id, err)
	}

	fmt.Printf("Download #%d (%s) marked for retry.\n", id, download.Title)
	fmt.Println("Use the TUI or re-run the download command to restart.")
	return nil
}
