// internal/converter/manager_test.go
package converter

import (
	"context"
	"testing"

	"github.com/billmal071/audbookdl/internal/extractor"
	"github.com/billmal071/audbookdl/internal/tts"
)

// mockEngine implements tts.Engine for testing.
type mockEngine struct{}

func (m *mockEngine) Name() string { return "mock" }
func (m *mockEngine) Synthesize(ctx context.Context, text string, opts tts.SynthOptions) ([]byte, error) {
	// Return a minimal valid MP3 frame (silence).
	return []byte{0xFF, 0xFB, 0x90, 0x00}, nil
}
func (m *mockEngine) ListVoices(ctx context.Context) ([]tts.Voice, error) {
	return []tts.Voice{{ID: "test", Name: "Test", Language: "en", Gender: "Male"}}, nil
}

func TestNewManager(t *testing.T) {
	m := NewManager(&mockEngine{}, nil)
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if m.engine.Name() != "mock" {
		t.Errorf("engine: got %q", m.engine.Name())
	}
}

func TestManager_Convert_EmptyBook(t *testing.T) {
	m := NewManager(&mockEngine{}, nil)
	book := &extractor.Book{
		Title:    "Test",
		Author:   "Author",
		Chapters: nil,
	}
	err := m.Convert(context.Background(), book, Options{
		OutputDir: t.TempDir(),
		Voice:     "test",
		SkipConfirm: true,
	})
	if err == nil {
		t.Error("expected error for empty book")
	}
}

func TestManager_Convert_SingleChapter(t *testing.T) {
	m := NewManager(&mockEngine{}, nil)
	book := &extractor.Book{
		Title:  "Test Book",
		Author: "Test Author",
		Chapters: []extractor.Chapter{
			{Index: 1, Title: "Chapter 1", Text: "Hello world this is a test."},
		},
	}
	outDir := t.TempDir()
	err := m.Convert(context.Background(), book, Options{
		OutputDir:   outDir,
		Voice:       "test",
		SkipConfirm: true,
	})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
}
