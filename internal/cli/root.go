package cli

import (
	"fmt"
	"os"

	"github.com/billmal071/audbookdl/internal/config"
	"github.com/billmal071/audbookdl/internal/db"
	"github.com/billmal071/audbookdl/internal/tui"
	"github.com/spf13/cobra"
)

var (
	Version = "dev"
	Commit  = "unknown"
	cfgFile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "audbookdl",
	Short: "Search and download free audiobooks",
	Long: `audbookdl is a CLI tool for searching and downloading free audiobooks
from LibriVox, Internet Archive, Loyal Books, and Open Library.

It features a full-screen TUI, built-in audio player, resumable downloads,
and SQLite-backed state tracking.

Run without arguments to launch the interactive TUI.

Examples:
  audbookdl                                Launch full TUI
  audbookdl search "sherlock holmes"        Search for audiobooks
  audbookdl download <id>                   Download an audiobook
  audbookdl play <id>                       Play a downloaded audiobook
  audbookdl list                            List all downloads`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Get()
		return tui.Run(db.DB(), cfg.Download.Directory)
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Init(cfgFile); err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}
		if err := db.Init(); err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		db.Close()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default $HOME/.config/audbookdl/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(pauseCmd)
	rootCmd.AddCommand(resumeCmd)
	rootCmd.AddCommand(bookmarkCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(completionCmd)
	rootCmd.AddCommand(queueCmd)
	rootCmd.AddCommand(playCmd)
}

func Verbose() bool { return verbose }

func Printf(format string, args ...interface{}) {
	if verbose {
		fmt.Printf(format, args...)
	}
}

func Errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}
