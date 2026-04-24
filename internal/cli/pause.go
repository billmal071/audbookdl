package cli

import (
	"fmt"
	"strconv"

	"github.com/billmal071/audbookdl/internal/db"
	"github.com/spf13/cobra"
)

var pauseCmd = &cobra.Command{
	Use:   "pause [download-id]",
	Short: "Pause an active download",
	Args:  cobra.ExactArgs(1),
	RunE:  runPause,
}

func runPause(cmd *cobra.Command, args []string) error {
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid download ID %q: %w", args[0], err)
	}

	if err := db.UpdateDownloadStatus(db.DB(), id, db.StatusPaused); err != nil {
		return fmt.Errorf("pause download %d: %w", id, err)
	}

	fmt.Printf("Download %d paused.\n", id)
	return nil
}
