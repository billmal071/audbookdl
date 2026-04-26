package cli

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/billmal071/audbookdl/internal/db"
	"github.com/billmal071/audbookdl/internal/player"
	"github.com/spf13/cobra"
)

var (
	playSpeed  float64
	playVolume float64
)

var playCmd = &cobra.Command{
	Use:   "play [download-id or path]",
	Short: "Play a downloaded audiobook",
	Long: `Play a downloaded audiobook using mpv (IPC-controlled) with state persistence.

Examples:
  audbookdl play 1                              # Play download #1
  audbookdl play ~/Audiobooks/Author/Title/     # Play from directory
  audbookdl play 1 --speed 1.5 --volume 0.7     # Custom speed and volume`,
	Args: cobra.ExactArgs(1),
	RunE: runPlay,
}

func init() {
	playCmd.Flags().Float64Var(&playSpeed, "speed", 0, "playback speed (0.5-3.0, default: saved or 1.0)")
	playCmd.Flags().Float64Var(&playVolume, "volume", 0, "volume level (0.0-1.0, default: saved or 0.8)")
}

func runPlay(cmd *cobra.Command, args []string) error {
	arg := args[0]

	var audioDir string
	var audiobookID string
	var title string

	if info, err := os.Stat(arg); err == nil && info.IsDir() {
		audioDir = arg
		audiobookID = filepath.Base(arg)
		title = audiobookID
	} else {
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
		audiobookID = dl.AudiobookID
		title = dl.Title
	}

	entries, err := os.ReadDir(audioDir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

	var chapters []player.ChapterInfo
	idx := 0
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".mp3" || ext == ".m4b" || ext == ".m4a" || ext == ".ogg" {
			chapters = append(chapters, player.ChapterInfo{
				Index:    idx,
				Title:    strings.TrimSuffix(e.Name(), ext),
				FilePath: filepath.Join(audioDir, e.Name()),
			})
			idx++
		}
	}

	if len(chapters) == 0 {
		return fmt.Errorf("no audio files found in %s", audioDir)
	}

	chapters = player.ProbeChapterDurations(chapters)

	p := player.NewPlayer(db.DB())
	p.Load(&player.Playlist{
		AudiobookID: audiobookID,
		Title:       title,
		Chapters:    chapters,
	})

	if playSpeed > 0 {
		p.SetSpeed(playSpeed)
	}
	if playVolume > 0 {
		p.SetVolume(playVolume)
	}

	st := p.GetStatus()
	fmt.Printf("Playing %s (%d chapters)\n", title, len(chapters))
	if st.ChapterIndex > 0 || st.PositionMS > 0 {
		fmt.Printf("Resuming from chapter %d at %s\n", st.ChapterIndex+1, formatPlayPosition(st.PositionMS))
	}

	p.Play()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	fmt.Println("\nStopping playback...")
	p.Stop()
	return nil
}

func formatPlayPosition(ms int64) string {
	total := ms / 1000
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
