package cli

import (
	"fmt"
	"strconv"

	"github.com/billmal071/audbookdl/internal/db"
	"github.com/spf13/cobra"
)

var bookmarkCmd = &cobra.Command{
	Use:   "bookmark",
	Short: "Manage audiobook bookmarks",
}

var bookmarkListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all bookmarks",
	RunE:    runBookmarkList,
}

var bookmarkDeleteCmd = &cobra.Command{
	Use:     "delete [bookmark-id]",
	Aliases: []string{"rm"},
	Short:   "Delete a bookmark by ID",
	Args:    cobra.ExactArgs(1),
	RunE:    runBookmarkDelete,
}

func init() {
	bookmarkCmd.AddCommand(bookmarkListCmd)
	bookmarkCmd.AddCommand(bookmarkDeleteCmd)
}

func runBookmarkList(cmd *cobra.Command, args []string) error {
	bookmarks, err := db.ListBookmarks(db.DB())
	if err != nil {
		return fmt.Errorf("list bookmarks: %w", err)
	}

	if len(bookmarks) == 0 {
		fmt.Println("No bookmarks found.")
		return nil
	}

	fmt.Printf("%-6s  %-40s  %-25s  %s\n", "ID", "Title", "Author", "Source")
	fmt.Printf("%-6s  %-40s  %-25s  %s\n",
		"------", "----------------------------------------",
		"-------------------------", "----------")

	for _, bm := range bookmarks {
		title := bm.Title
		if len(title) > 40 {
			title = title[:37] + "..."
		}
		author := bm.Author
		if len(author) > 25 {
			author = author[:22] + "..."
		}
		fmt.Printf("%-6d  %-40s  %-25s  %s\n", bm.ID, title, author, bm.Source)
	}

	return nil
}

func runBookmarkDelete(cmd *cobra.Command, args []string) error {
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid bookmark ID %q: %w", args[0], err)
	}

	if err := db.DeleteBookmark(db.DB(), id); err != nil {
		return fmt.Errorf("delete bookmark %d: %w", id, err)
	}

	fmt.Printf("Bookmark %d deleted.\n", id)
	return nil
}
