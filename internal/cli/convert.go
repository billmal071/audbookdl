// internal/cli/convert.go
package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/billmal071/audbookdl/internal/config"
	"github.com/billmal071/audbookdl/internal/converter"
	"github.com/billmal071/audbookdl/internal/db"
	"github.com/billmal071/audbookdl/internal/extractor"
	"github.com/billmal071/audbookdl/internal/tts"
	"github.com/spf13/cobra"
)

var (
	convertEngine string
	convertVoice  string
	convertRate   string
	convertAuthor string
	convertTitle  string
	convertOutput string
	convertYes    bool
)

var convertCmd = &cobra.Command{
	Use:   "convert <file>",
	Short: "Convert a PDF, EPUB, TXT, or DOCX file to an audiobook using TTS",
	Long: `Convert an ebook file to an audiobook using text-to-speech.

Supported formats: PDF, EPUB, TXT, DOCX

Examples:
  audbookdl convert book.pdf
  audbookdl convert book.epub --voice en-US-GuyNeural
  audbookdl convert book.txt --engine piper --voice en_US-lessac-medium
  audbookdl convert book.docx --rate "+20%" --yes`,
	Args: cobra.ExactArgs(1),
	RunE: runConvert,
}

func init() {
	convertCmd.Flags().StringVarP(&convertEngine, "engine", "e", "edge", "TTS engine: edge or piper")
	convertCmd.Flags().StringVar(&convertVoice, "voice", "en-US-AriaNeural", "voice ID")
	convertCmd.Flags().StringVarP(&convertRate, "rate", "r", "+0%", "speech rate (e.g., +20%, -10%)")
	convertCmd.Flags().StringVarP(&convertAuthor, "author", "a", "", "override book author")
	convertCmd.Flags().StringVarP(&convertTitle, "title", "t", "", "override book title")
	convertCmd.Flags().StringVarP(&convertOutput, "output", "o", "", "output directory (default: ~/Audiobooks/Author/Title/)")
	convertCmd.Flags().BoolVarP(&convertYes, "yes", "y", false, "skip chapter review confirmation")
}

func runConvert(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	fmt.Printf("Extracting text from %s...\n", filePath)
	book, err := extractor.Extract(filePath)
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	// Apply overrides.
	if convertAuthor != "" {
		book.Author = convertAuthor
	}
	if convertTitle != "" {
		book.Title = convertTitle
	}

	fmt.Printf("Found %d chapters in %q by %s\n\n", len(book.Chapters), book.Title, book.Author)

	// Show chapters for review.
	for _, ch := range book.Chapters {
		words := len(strings.Fields(ch.Text))
		fmt.Printf("  %2d. %-40s (%d words)\n", ch.Index, ch.Title, words)
	}
	fmt.Println()

	if !convertYes {
		fmt.Print("Proceed with conversion? [Y/n] ")
		var answer string
		fmt.Scanln(&answer)
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "" && answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Create TTS engine.
	var engine tts.Engine
	switch convertEngine {
	case "edge":
		engine = tts.NewEdgeTTS()
	case "piper":
		engine = tts.NewPiperTTS("")
	default:
		return fmt.Errorf("unknown engine: %s (supported: edge, piper)", convertEngine)
	}

	// Determine output directory.
	outDir := convertOutput
	if outDir == "" {
		outDir = config.Get().Download.Directory
	}

	mgr := converter.NewManager(engine, db.DB())
	ctx := context.Background()

	return mgr.Convert(ctx, book, converter.Options{
		OutputDir:   outDir,
		Voice:       convertVoice,
		Rate:        convertRate,
		SkipConfirm: true, // Already confirmed above.
	})
}
