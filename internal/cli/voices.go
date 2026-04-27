// internal/cli/voices.go
package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/billmal071/audbookdl/internal/tts"
	"github.com/spf13/cobra"
)

var (
	voicesEngine string
	voicesLang   string
)

var voicesCmd = &cobra.Command{
	Use:   "voices",
	Short: "List available TTS voices",
	Long: `List available text-to-speech voices for the specified engine.

Examples:
  audbookdl voices                    # List Edge TTS voices
  audbookdl voices --engine piper     # List Piper voices
  audbookdl voices --lang en          # Filter by language`,
	RunE: runVoices,
}

func init() {
	voicesCmd.Flags().StringVarP(&voicesEngine, "engine", "e", "edge", "TTS engine: edge or piper")
	voicesCmd.Flags().StringVarP(&voicesLang, "lang", "l", "", "filter by language code (e.g., en, fr)")
}

func runVoices(cmd *cobra.Command, args []string) error {
	var engine tts.Engine
	switch voicesEngine {
	case "edge":
		engine = tts.NewEdgeTTS()
	case "piper":
		engine = tts.NewPiperTTS("")
	default:
		return fmt.Errorf("unknown engine: %s", voicesEngine)
	}

	ctx := context.Background()
	voices, err := engine.ListVoices(ctx)
	if err != nil {
		return fmt.Errorf("list voices: %w", err)
	}

	// Filter by language.
	if voicesLang != "" {
		var filtered []tts.Voice
		for _, v := range voices {
			if strings.HasPrefix(strings.ToLower(v.Language), strings.ToLower(voicesLang)) {
				filtered = append(filtered, v)
			}
		}
		voices = filtered
	}

	if len(voices) == 0 {
		fmt.Println("No voices found.")
		return nil
	}

	fmt.Printf("%-30s %-20s %-10s %s\n", "ID", "NAME", "LANG", "GENDER")
	fmt.Println(strings.Repeat("-", 75))
	for _, v := range voices {
		fmt.Printf("%-30s %-20s %-10s %s\n", v.ID, v.Name, v.Language, v.Gender)
	}
	fmt.Printf("\n%d voice(s)\n", len(voices))

	return nil
}
