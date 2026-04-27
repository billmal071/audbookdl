// internal/tts/edge_test.go
package tts

import (
	"strings"
	"testing"
)

func TestEdgeTTS_Name(t *testing.T) {
	e := NewEdgeTTS()
	if e.Name() != "edge" {
		t.Errorf("name: got %q, want %q", e.Name(), "edge")
	}
}

func TestEdgeTTS_BuildSSML(t *testing.T) {
	e := NewEdgeTTS()
	opts := DefaultSynthOptions()
	ssml := e.buildSSML("Hello world", opts)
	if ssml == "" {
		t.Fatal("expected non-empty SSML")
	}
	if !strings.Contains(ssml, "Hello world") {
		t.Error("SSML should contain the text")
	}
	if !strings.Contains(ssml, "en-US-AriaNeural") {
		t.Error("SSML should contain the voice name")
	}
	if !strings.Contains(ssml, "+0%") {
		t.Error("SSML should contain the rate")
	}
}

func TestEdgeTTS_ChunkText(t *testing.T) {
	e := NewEdgeTTS()
	// Short text — single chunk.
	chunks := e.chunkText("Hello world", 100)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != "Hello world" {
		t.Errorf("chunk: got %q", chunks[0])
	}

	// Text longer than limit — split at sentence boundaries.
	long := "First sentence. Second sentence. Third sentence. Fourth sentence."
	chunks = e.chunkText(long, 40)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}
}
