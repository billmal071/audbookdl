package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	// Reset viper state and cached config
	resetForTest()

	if err := Init(""); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	cfg := Get()

	home, _ := os.UserHomeDir()
	expectedDir := filepath.Join(home, "Audiobooks")

	if cfg.Download.Directory != expectedDir {
		t.Errorf("Download.Directory = %q, want %q", cfg.Download.Directory, expectedDir)
	}

	const fiveMB = int64(5 * 1024 * 1024)
	if cfg.Download.ChunkSize != fiveMB {
		t.Errorf("Download.ChunkSize = %d, want %d", cfg.Download.ChunkSize, fiveMB)
	}

	if cfg.Download.MaxConcurrent != 3 {
		t.Errorf("Download.MaxConcurrent = %d, want 3", cfg.Download.MaxConcurrent)
	}

	if cfg.Download.PreferredFormat != "mp3" {
		t.Errorf("Download.PreferredFormat = %q, want \"mp3\"", cfg.Download.PreferredFormat)
	}

	if cfg.Player.DefaultSpeed != 1.0 {
		t.Errorf("Player.DefaultSpeed = %f, want 1.0", cfg.Player.DefaultSpeed)
	}

	if cfg.Player.SkipSeconds != 15 {
		t.Errorf("Player.SkipSeconds = %d, want 15", cfg.Player.SkipSeconds)
	}

	if cfg.Search.DefaultLimit != 10 {
		t.Errorf("Search.DefaultLimit = %d, want 10", cfg.Search.DefaultLimit)
	}

	if cfg.Search.CacheTTL != 1*time.Hour {
		t.Errorf("Search.CacheTTL = %v, want 1h", cfg.Search.CacheTTL)
	}

	if len(cfg.Search.Sources) != 4 {
		t.Errorf("len(Search.Sources) = %d, want 4", len(cfg.Search.Sources))
	}
}

func TestGetConfigDir(t *testing.T) {
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "audbookdl")

	got := GetConfigDir()
	if got != expected {
		t.Errorf("GetConfigDir() = %q, want %q", got, expected)
	}
}

func TestGetDBPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "audbookdl", "audbookdl.db")

	got := GetDBPath()
	if got != expected {
		t.Errorf("GetDBPath() = %q, want %q", got, expected)
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "tilde expansion",
			input: "~/Music",
			want:  filepath.Join(home, "Music"),
		},
		{
			name:  "absolute path unchanged",
			input: "/absolute/path",
			want:  "/absolute/path",
		},
		{
			name:  "relative path unchanged",
			input: "relative/path",
			want:  "relative/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandPath(tt.input)
			if got != tt.want {
				t.Errorf("expandPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
