package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/billmal071/audbookdl/internal/db"
	"github.com/spf13/cobra"
)

var playCmd = &cobra.Command{
	Use:   "play [download-id or path]",
	Short: "Play a downloaded audiobook",
	Long: `Play a downloaded audiobook using the system audio player (mpv or ffplay).

Examples:
  audbookdl play 1                              # Play download #1
  audbookdl play ~/Audiobooks/Author/Title/     # Play from directory`,
	Args: cobra.ExactArgs(1),
	RunE: runPlay,
}

func runPlay(cmd *cobra.Command, args []string) error {
	arg := args[0]

	// Check if it's a directory path
	var audioDir string
	if info, err := os.Stat(arg); err == nil && info.IsDir() {
		audioDir = arg
	} else {
		// Try as download ID
		id, err := strconv.ParseInt(arg, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid download ID or path: %s", arg)
		}
		dl, err := db.GetDownload(db.DB(), id)
		if err != nil {
			return fmt.Errorf("download not found: %w", err)
		}
		if dl.Status != db.StatusCompleted {
			return fmt.Errorf("download #%d is %s, not completed", id, dl.Status)
		}
		audioDir = dl.BasePath
	}

	// Find audio files
	entries, err := os.ReadDir(audioDir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

	var files []string
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".mp3" || ext == ".m4b" || ext == ".m4a" || ext == ".ogg" {
			files = append(files, filepath.Join(audioDir, e.Name()))
		}
	}
	sort.Strings(files)

	if len(files) == 0 {
		return fmt.Errorf("no audio files found in %s", audioDir)
	}

	fmt.Printf("Playing %d track(s) from %s\n", len(files), audioDir)

	// Try mpv first, then ffplay
	if mpv, err := exec.LookPath("mpv"); err == nil {
		playerArgs := append([]string{"--no-video"}, files...)
		playerCmd := exec.Command(mpv, playerArgs...)
		playerCmd.Stdin = os.Stdin
		playerCmd.Stdout = os.Stdout
		playerCmd.Stderr = os.Stderr
		return playerCmd.Run()
	}

	if ffplay, err := exec.LookPath("ffplay"); err == nil {
		// ffplay can only play one file at a time — play them sequentially
		for i, f := range files {
			fmt.Printf("[%d/%d] %s\n", i+1, len(files), filepath.Base(f))
			playerCmd := exec.Command(ffplay, "-nodisp", "-autoexit", f)
			playerCmd.Stdin = os.Stdin
			playerCmd.Stdout = os.Stdout
			playerCmd.Stderr = os.Stderr
			if err := playerCmd.Run(); err != nil {
				return err
			}
		}
		return nil
	}

	return fmt.Errorf("no audio player found — install mpv or ffplay")
}
