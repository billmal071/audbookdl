package cli

import (
	"fmt"

	"github.com/billmal071/audbookdl/internal/db"
	"github.com/spf13/cobra"
)

var historyLimit int

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show search history",
	RunE:  runHistory,
}

var historyClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all search history",
	RunE:  runHistoryClear,
}

func init() {
	historyCmd.Flags().IntVarP(&historyLimit, "limit", "n", 20, "number of entries to show")
	historyCmd.AddCommand(historyClearCmd)
}

func runHistory(cmd *cobra.Command, args []string) error {
	entries, err := db.ListSearchHistory(db.DB(), historyLimit)
	if err != nil {
		return fmt.Errorf("list search history: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No search history found.")
		return nil
	}

	fmt.Printf("%-6s  %-30s  %-20s  %-8s  %s\n", "ID", "Query", "Source", "Results", "Date")
	fmt.Printf("%-6s  %-30s  %-20s  %-8s  %s\n",
		"------", "------------------------------",
		"--------------------", "-------", "-------------------")

	for _, e := range entries {
		query := e.Query
		if len(query) > 30 {
			query = query[:27] + "..."
		}
		src := e.Source
		if len(src) > 20 {
			src = src[:17] + "..."
		}
		fmt.Printf("%-6d  %-30s  %-20s  %-8d  %s\n",
			e.ID, query, src, e.ResultCount,
			e.CreatedAt.Format("2006-01-02 15:04:05"))
	}

	return nil
}

func runHistoryClear(cmd *cobra.Command, args []string) error {
	if err := db.ClearSearchHistory(db.DB()); err != nil {
		return fmt.Errorf("clear search history: %w", err)
	}
	fmt.Println("Search history cleared.")
	return nil
}
