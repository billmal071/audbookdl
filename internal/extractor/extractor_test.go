package extractor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectChapters_HeadingPattern(t *testing.T) {
	text := "Chapter 1: The Beginning\nSome text here.\n\nChapter 2: The Middle\nMore text here.\n\nChapter 3: The End\nFinal text."
	chapters := DetectChapters(text)
	if len(chapters) != 3 {
		t.Fatalf("expected 3 chapters, got %d", len(chapters))
	}
	if chapters[0].Title != "Chapter 1: The Beginning" {
		t.Errorf("chapter 0 title: got %q", chapters[0].Title)
	}
	if !strings.Contains(chapters[0].Text, "Some text here") {
		t.Errorf("chapter 0 should contain text")
	}
	if chapters[1].Title != "Chapter 2: The Middle" {
		t.Errorf("chapter 1 title: got %q", chapters[1].Title)
	}
}

func TestDetectChapters_PartPattern(t *testing.T) {
	text := "PART ONE\nFirst part text.\n\nPART TWO\nSecond part text."
	chapters := DetectChapters(text)
	if len(chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(chapters))
	}
	if chapters[0].Title != "PART ONE" {
		t.Errorf("chapter 0 title: got %q", chapters[0].Title)
	}
}

func TestDetectChapters_NumberedPattern(t *testing.T) {
	text := "1. Introduction\nIntro text.\n\n2. Main Body\nBody text.\n\n3. Conclusion\nEnd text."
	chapters := DetectChapters(text)
	if len(chapters) != 3 {
		t.Fatalf("expected 3 chapters, got %d", len(chapters))
	}
}

func TestDetectChapters_NoPattern_FallbackSplit(t *testing.T) {
	words := make([]string, 12000)
	for i := range words {
		words[i] = "word"
	}
	text := strings.Join(words, " ")
	chapters := DetectChapters(text)
	if len(chapters) < 2 {
		t.Fatalf("expected at least 2 chapters from fallback split, got %d", len(chapters))
	}
	for _, ch := range chapters {
		if ch.Title == "" {
			t.Error("fallback chapters should have a title")
		}
	}
}

func TestDetectChapters_Empty(t *testing.T) {
	chapters := DetectChapters("")
	if len(chapters) != 0 {
		t.Errorf("expected 0 chapters for empty text, got %d", len(chapters))
	}
}

func TestExtract_TXT(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "Chapter 1: Hello\nThis is chapter one.\n\nChapter 2: World\nThis is chapter two."
	os.WriteFile(path, []byte(content), 0644)

	book, err := Extract(path)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(book.Chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(book.Chapters))
	}
	if book.Title != "test" {
		t.Errorf("title: got %q, want %q", book.Title, "test")
	}
}

func TestExtract_UnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.xyz")
	os.WriteFile(path, []byte("data"), 0644)

	_, err := Extract(path)
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

func TestExtract_FileNotFound(t *testing.T) {
	_, err := Extract("/nonexistent/file.txt")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestExtractTXT(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "book.txt")
	os.WriteFile(path, []byte("Hello world. This is a test book with enough content."), 0644)

	book, err := extractTXT(path)
	if err != nil {
		t.Fatalf("extractTXT: %v", err)
	}
	if book.Title != "book" {
		t.Errorf("title: got %q", book.Title)
	}
	if len(book.Chapters) == 0 {
		t.Error("expected at least 1 chapter")
	}
}

func TestExtractDOCX_InvalidZip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.docx")
	os.WriteFile(path, []byte("not a zip"), 0644)

	_, err := extractDOCX(path)
	if err == nil {
		t.Error("expected error for invalid docx")
	}
}

func TestExtractEPUB_InvalidZip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.epub")
	os.WriteFile(path, []byte("not a zip"), 0644)

	_, err := extractEPUB(path)
	if err == nil {
		t.Error("expected error for invalid epub")
	}
}
