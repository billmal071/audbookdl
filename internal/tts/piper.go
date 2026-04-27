// internal/tts/piper.go
package tts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	piperDefaultVoice = "en_US-lessac-medium"
	piperGitHubBase   = "https://github.com/rhasspy/piper/releases/latest/download"
	piperModelBase    = "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0"
)

// PiperTTS implements offline TTS using the Piper binary.
type PiperTTS struct {
	dataDir string
}

// NewPiperTTS creates a Piper TTS engine. If dataDir is empty, uses ~/.config/audbookdl/piper/.
func NewPiperTTS(dataDir string) *PiperTTS {
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".config", "audbookdl", "piper")
	}
	return &PiperTTS{dataDir: dataDir}
}

func (p *PiperTTS) Name() string { return "piper" }

// ListVoices returns a static list of popular Piper voices.
func (p *PiperTTS) ListVoices(ctx context.Context) ([]Voice, error) {
	return []Voice{
		{ID: "en_US-lessac-medium", Name: "Lessac", Language: "en-US", Gender: "Male"},
		{ID: "en_US-amy-medium", Name: "Amy", Language: "en-US", Gender: "Female"},
		{ID: "en_US-ryan-medium", Name: "Ryan", Language: "en-US", Gender: "Male"},
		{ID: "en_GB-alan-medium", Name: "Alan", Language: "en-GB", Gender: "Male"},
		{ID: "en_GB-alba-medium", Name: "Alba", Language: "en-GB", Gender: "Female"},
		{ID: "fr_FR-siwis-medium", Name: "Siwis", Language: "fr-FR", Gender: "Female"},
		{ID: "de_DE-thorsten-medium", Name: "Thorsten", Language: "de-DE", Gender: "Male"},
		{ID: "es_ES-davefx-medium", Name: "DaveFX", Language: "es-ES", Gender: "Male"},
	}, nil
}

// Synthesize converts text to audio using Piper.
func (p *PiperTTS) Synthesize(ctx context.Context, text string, opts SynthOptions) ([]byte, error) {
	if err := p.ensureInstalled(ctx); err != nil {
		return nil, err
	}

	voice := opts.Voice
	if voice == "" {
		voice = piperDefaultVoice
	}

	if err := p.ensureModel(ctx, voice); err != nil {
		return nil, err
	}

	piperBin := p.piperPath()
	model := p.modelPath(voice)

	// Write text to temp file (piper reads from stdin).
	tmpFile, err := os.CreateTemp("", "audbookdl-piper-*.txt")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(text)
	tmpFile.Close()

	// Output to temp wav file.
	outFile := tmpFile.Name() + ".wav"
	defer os.Remove(outFile)

	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, piperBin,
		"--model", model,
		"--output_file", outFile,
	)
	cmd.Stdin, _ = os.Open(tmpFile.Name())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("piper: %w: %s", err, stderr.String())
	}

	// Convert WAV to MP3 using ffmpeg if available, otherwise return WAV.
	if mp3Data, err := wavToMP3(ctx, outFile); err == nil {
		return mp3Data, nil
	}

	// Fallback: return raw WAV.
	return os.ReadFile(outFile)
}

func wavToMP3(ctx context.Context, wavPath string) ([]byte, error) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, err
	}

	mp3Path := wavPath + ".mp3"
	defer os.Remove(mp3Path)

	cmd := exec.CommandContext(ctx, ffmpeg,
		"-i", wavPath,
		"-codec:a", "libmp3lame",
		"-qscale:a", "2",
		"-y",
		mp3Path,
	)
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return os.ReadFile(mp3Path)
}

func (p *PiperTTS) piperPath() string {
	return filepath.Join(p.dataDir, "piper")
}

func (p *PiperTTS) modelPath(voice string) string {
	return filepath.Join(p.dataDir, voice+".onnx")
}

func (p *PiperTTS) isInstalled() bool {
	_, err := os.Stat(p.piperPath())
	if err == nil {
		return true
	}
	// Check system PATH.
	_, err = exec.LookPath("piper")
	return err == nil
}

func (p *PiperTTS) ensureInstalled(ctx context.Context) error {
	if p.isInstalled() {
		return nil
	}

	fmt.Println("Piper TTS not found. Downloading...")
	if err := os.MkdirAll(p.dataDir, 0755); err != nil {
		return err
	}

	url := piperDownloadURL()
	if url == "" {
		return fmt.Errorf("unsupported platform for piper auto-download: %s/%s — install piper manually", runtime.GOOS, runtime.GOARCH)
	}

	return downloadAndExtract(ctx, url, p.dataDir)
}

func (p *PiperTTS) ensureModel(ctx context.Context, voice string) error {
	modelFile := p.modelPath(voice)
	if _, err := os.Stat(modelFile); err == nil {
		return nil
	}

	fmt.Printf("Downloading Piper voice model: %s...\n", voice)

	// Model files are at: {base}/{lang}/{voice}/{quality}/{voice}.onnx
	parts := strings.SplitN(voice, "-", 2)
	lang := strings.ReplaceAll(parts[0], "_", "/")
	modelURL := fmt.Sprintf("%s/%s/%s/%s.onnx", piperModelBase, lang, voice, voice)
	configURL := fmt.Sprintf("%s/%s/%s/%s.onnx.json", piperModelBase, lang, voice, voice)

	if err := downloadFile(ctx, modelURL, modelFile); err != nil {
		return fmt.Errorf("download model: %w", err)
	}
	// Also download the config JSON.
	_ = downloadFile(ctx, configURL, modelFile+".json")

	return nil
}

func piperDownloadURL() string {
	switch runtime.GOOS {
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return piperGitHubBase + "/piper_linux_x86_64.tar.gz"
		case "arm64":
			return piperGitHubBase + "/piper_linux_aarch64.tar.gz"
		}
	case "darwin":
		switch runtime.GOARCH {
		case "amd64":
			return piperGitHubBase + "/piper_macos_x64.tar.gz"
		case "arm64":
			return piperGitHubBase + "/piper_macos_aarch64.tar.gz"
		}
	}
	return ""
}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

func downloadAndExtract(ctx context.Context, url, destDir string) error {
	tmpFile, err := os.CreateTemp("", "piper-*.tar.gz")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	if err := downloadFile(ctx, url, tmpFile.Name()); err != nil {
		return err
	}

	// Extract using tar.
	cmd := exec.CommandContext(ctx, "tar", "xzf", tmpFile.Name(), "-C", destDir, "--strip-components=1")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("extract piper: %w", err)
	}

	// Make binary executable.
	piperBin := filepath.Join(destDir, "piper")
	return os.Chmod(piperBin, 0755)
}
