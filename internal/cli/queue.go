package cli

import (
	"fmt"
	"strconv"

	"github.com/billmal071/audbookdl/internal/db"
	"github.com/spf13/cobra"
)

var queueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Manage download queue",
}

var queueListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List queued downloads",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		downloads, err := db.ListDownloads(db.DB())
		if err != nil {
			return err
		}
		var queued []*db.AudiobookDownload
		for _, dl := range downloads {
			if dl.Status == db.StatusPending {
				queued = append(queued, dl)
			}
		}
		if len(queued) == 0 {
			fmt.Println("Queue is empty.")
			return nil
		}
		for i, dl := range queued {
			fmt.Printf("%d. #%d %s — %s\n", i+1, dl.ID, dl.Title, dl.Author)
		}
		return nil
	},
}

var queueClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all pending downloads from queue",
	RunE: func(cmd *cobra.Command, args []string) error {
		downloads, err := db.ListDownloads(db.DB())
		if err != nil {
			return err
		}
		count := 0
		for _, dl := range downloads {
			if dl.Status == db.StatusPending {
				db.UpdateDownloadStatus(db.DB(), dl.ID, db.StatusFailed)
				count++
			}
		}
		fmt.Printf("Cleared %d queued download(s).\n", count)
		return nil
	},
}

var queueRemoveCmd = &cobra.Command{
	Use:     "remove [download-id]",
	Short:   "Remove a download from queue",
	Aliases: []string{"rm"},
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid download ID: %s", args[0])
		}
		dl, err := db.GetDownload(db.DB(), id)
		if err != nil {
			return fmt.Errorf("download not found: %w", err)
		}
		if dl.Status != db.StatusPending {
			return fmt.Errorf("download #%d is %s, not queued", id, dl.Status)
		}
		if err := db.UpdateDownloadStatus(db.DB(), id, db.StatusFailed); err != nil {
			return err
		}
		fmt.Printf("Removed #%d from queue.\n", id)
		return nil
	},
}

func init() {
	queueCmd.AddCommand(queueListCmd)
	queueCmd.AddCommand(queueClearCmd)
	queueCmd.AddCommand(queueRemoveCmd)
}
