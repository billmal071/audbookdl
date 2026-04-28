package cli

import (
	"fmt"

	"github.com/billmal071/audbookdl/internal/db"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all downloads with status and progress",
	RunE:  runList,
}

func runList(cmd *cobra.Command, args []string) error {
	downloads, err := db.ListDownloads(db.DB())
	if err != nil {
		return fmt.Errorf("list downloads: %w", err)
	}

	if len(downloads) == 0 {
		fmt.Println("No downloads found.")
		return nil
	}

	fmt.Printf("%-6s  %-40s  %-25s  %-12s  %s\n", "ID", "Title", "Author", "Status", "Progress")
	fmt.Printf("%-6s  %-40s  %-25s  %-12s  %s\n",
		"------", "----------------------------------------",
		"-------------------------", "------------", "--------")

	for _, d := range downloads {
		title := d.Title
		if len(title) > 40 {
			title = title[:37] + "..."
		}
		author := d.Author
		if len(author) > 25 {
			author = author[:22] + "..."
		}

		progress := "n/a"
		if d.TotalSize > 0 {
			pct := float64(d.DownloadedSize) / float64(d.TotalSize) * 100
			progress = fmt.Sprintf("%.1f%%", pct)
		} else if chapters, err := db.ListChapterDownloads(db.DB(), d.ID); err == nil && len(chapters) > 0 {
			done := 0
			for _, c := range chapters {
				if c.Status == db.StatusCompleted {
					done++
				}
			}
			progress = fmt.Sprintf("%d/%d chapters", done, len(chapters))
		}

		fmt.Printf("%-6d  %-40s  %-25s  %-12s  %s\n",
			d.ID, title, author, string(d.Status), progress)
	}

	return nil
}
