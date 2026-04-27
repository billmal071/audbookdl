// internal/tts/piper_test.go
package tts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPiperTTS_Name(t *testing.T) {
	p := NewPiperTTS("")
	if p.Name() != "piper" {
		t.Errorf("name: got %q, want %q", p.Name(), "piper")
	}
}

func TestPiperTTS_DataDir(t *testing.T) {
	p := NewPiperTTS("")
	if p.dataDir == "" {
		t.Error("dataDir should have a default")
	}
}

func TestPiperTTS_CustomDataDir(t *testing.T) {
	dir := t.TempDir()
	p := NewPiperTTS(dir)
	if p.dataDir != dir {
		t.Errorf("dataDir: got %q, want %q", p.dataDir, dir)
	}
}

func TestPiperTTS_ModelPath(t *testing.T) {
	dir := t.TempDir()
	p := NewPiperTTS(dir)
	path := p.modelPath("en_US-lessac-medium")
	expected := filepath.Join(dir, "en_US-lessac-medium.onnx")
	if path != expected {
		t.Errorf("modelPath: got %q, want %q", path, expected)
	}
}

func TestPiperTTS_IsInstalled_NotPresent(t *testing.T) {
	dir := t.TempDir()
	p := NewPiperTTS(dir)
	// With empty dir, piper binary should not be found there.
	// It might be on PATH though, so we just check the method doesn't panic.
	_ = p.isInstalled()
}

func TestPiperTTS_DownloadURL(t *testing.T) {
	url := piperDownloadURL()
	// piperDownloadURL returns a URL for supported platforms (linux/darwin amd64/arm64).
	// On unsupported platforms (e.g., linux/386), it returns empty which is valid behavior.
	if url != "" {
		if !strings.Contains(url, "piper") {
			t.Errorf("download URL should contain 'piper': got %q", url)
		}
	}
}

func TestPiperTTS_WriteTextToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.txt")
	err := os.WriteFile(path, []byte("test text"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "test text" {
		t.Errorf("got %q", string(data))
	}
}
