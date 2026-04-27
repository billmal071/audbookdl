# TTS Convert Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `convert` CLI command that transforms PDF/EPUB/TXT/DOCX files into audiobooks using Edge TTS (pure Go WebSocket) or Piper TTS (auto-downloaded binary).

**Architecture:** Three new packages — `extractor` (text extraction with chapter detection), `tts` (engine interface with Edge/Piper implementations), `converter` (orchestration pipeline). Two new CLI commands (`convert`, `voices`). Config extended with `conversion` section. Reuses existing DB tables with `source: "converted"`.

**Tech Stack:** Go 1.22+, `nhooyr.io/websocket` (Edge TTS), `archive/zip` + `encoding/xml` (EPUB/DOCX), `ledongthuc/pdf` (PDF fallback), `github.com/bogem/id3v2/v2` (ID3 tagging).

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/extractor/types.go` | Create | Book, Chapter types |
| `internal/extractor/extractor.go` | Create | Extract() dispatch by file extension |
| `internal/extractor/pdf.go` | Create | PDF text extraction (pdftotext + pure Go fallback) |
| `internal/extractor/epub.go` | Create | EPUB parsing from zip |
| `internal/extractor/txt.go` | Create | Plain text reading |
| `internal/extractor/docx.go` | Create | DOCX XML extraction from zip |
| `internal/extractor/chapters.go` | Create | Chapter auto-detection from text |
| `internal/extractor/extractor_test.go` | Create | Tests for all extractors + chapter detection |
| `internal/tts/engine.go` | Create | Engine interface, SynthOptions, Voice types |
| `internal/tts/edge.go` | Create | Edge TTS WebSocket client |
| `internal/tts/edge_test.go` | Create | Edge TTS tests |
| `internal/tts/piper.go` | Create | Piper binary integration + auto-download |
| `internal/tts/piper_test.go` | Create | Piper tests |
| `internal/converter/manager.go` | Create | Conversion pipeline orchestration |
| `internal/converter/manager_test.go` | Create | Converter tests |
| `internal/cli/convert.go` | Create | convert command |
| `internal/cli/voices.go` | Create | voices command |
| `internal/cli/root.go` | Modify | Register convert + voices commands |
| `internal/config/config.go` | Modify | Add ConversionConfig |

---

### Task 1: Extractor Types and Chapter Detection

**Files:**
- Create: `internal/extractor/types.go`
- Create: `internal/extractor/chapters.go`
- Create: `internal/extractor/extractor_test.go`

- [ ] **Step 1: Create types.go**

```go
// internal/extractor/types.go
package extractor

// Book represents extracted text content from an ebook file.
type Book struct {
	Title    string
	Author   string
	Chapters []Chapter
}

// Chapter represents a single chapter of extracted text.
type Chapter struct {
	Index int
	Title string
	Text  string
}
```

- [ ] **Step 2: Write chapter detection tests**

```go
// internal/extractor/extractor_test.go
package extractor

import (
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
	// Generate text with no chapter markers — should fall back to word-count splitting.
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
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd ~/Documents/personal/audbookdl && PATH="/usr/local/go/bin:$PATH" CGO_ENABLED=0 go test ./internal/extractor/ -v`
Expected: FAIL — `DetectChapters` not defined

- [ ] **Step 4: Implement chapters.go**

```go
// internal/extractor/chapters.go
package extractor

import (
	"fmt"
	"regexp"
	"strings"
)

// chapterPatterns are regex patterns that match common chapter headings.
// They are tried in order; the first one that produces 2+ matches wins.
var chapterPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^(Chapter\s+\d+[^\n]*)$`),
	regexp.MustCompile(`(?m)^(CHAPTER\s+[IVXLCDM]+[^\n]*)$`),
	regexp.MustCompile(`(?m)^(PART\s+[A-Z]+[^\n]*)$`),
	regexp.MustCompile(`(?m)^(\d+\.\s+[^\n]+)$`),
}

const fallbackWordLimit = 5000

// DetectChapters splits text into chapters by detecting heading patterns.
// If no patterns match, falls back to splitting every ~5000 words.
func DetectChapters(text string) []Chapter {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	for _, pat := range chapterPatterns {
		chapters := splitByPattern(text, pat)
		if len(chapters) >= 2 {
			return chapters
		}
	}

	return splitByWordCount(text, fallbackWordLimit)
}

// splitByPattern splits text at each match of pat, using the match as the chapter title.
func splitByPattern(text string, pat *regexp.Regexp) []Chapter {
	locs := pat.FindAllStringIndex(text, -1)
	if len(locs) < 2 {
		return nil
	}

	var chapters []Chapter
	for i, loc := range locs {
		title := strings.TrimSpace(text[loc[0]:loc[1]])
		var body string
		if i+1 < len(locs) {
			body = text[loc[1]:locs[i+1][0]]
		} else {
			body = text[loc[1]:]
		}
		body = strings.TrimSpace(body)
		if body == "" {
			continue
		}
		chapters = append(chapters, Chapter{
			Index: len(chapters) + 1,
			Title: title,
			Text:  body,
		})
	}
	return chapters
}

// splitByWordCount splits text into chunks of approximately wordLimit words.
func splitByWordCount(text string, wordLimit int) []Chapter {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var chapters []Chapter
	for i := 0; i < len(words); i += wordLimit {
		end := i + wordLimit
		if end > len(words) {
			end = len(words)
		}
		chunk := strings.Join(words[i:end], " ")
		idx := len(chapters) + 1
		chapters = append(chapters, Chapter{
			Index: idx,
			Title: fmt.Sprintf("Part %d", idx),
			Text:  chunk,
		})
	}
	return chapters
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd ~/Documents/personal/audbookdl && PATH="/usr/local/go/bin:$PATH" CGO_ENABLED=0 go test ./internal/extractor/ -v`
Expected: All 5 tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/extractor/
git commit -m "feat(extractor): add types and chapter auto-detection"
```

---

### Task 2: Text Extractors (TXT, EPUB, DOCX, PDF)

**Files:**
- Create: `internal/extractor/extractor.go`
- Create: `internal/extractor/txt.go`
- Create: `internal/extractor/epub.go`
- Create: `internal/extractor/docx.go`
- Create: `internal/extractor/pdf.go`
- Modify: `internal/extractor/extractor_test.go`

- [ ] **Step 1: Add extractor tests**

Append to `internal/extractor/extractor_test.go`:

```go
import (
	"os"
	"path/filepath"
)

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
```

- [ ] **Step 2: Implement extractor.go (dispatch)**

```go
// internal/extractor/extractor.go
package extractor

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Extract reads an ebook file and returns a Book with chapters.
// Supported formats: .txt, .pdf, .epub, .docx
func Extract(filePath string) (*Book, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".txt":
		return extractTXT(filePath)
	case ".pdf":
		return extractPDF(filePath)
	case ".epub":
		return extractEPUB(filePath)
	case ".docx":
		return extractDOCX(filePath)
	default:
		return nil, fmt.Errorf("unsupported format: %s (supported: .txt, .pdf, .epub, .docx)", ext)
	}
}
```

- [ ] **Step 3: Implement txt.go**

```go
// internal/extractor/txt.go
package extractor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func extractTXT(filePath string) (*Book, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read txt: %w", err)
	}

	text := string(data)
	title := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	chapters := DetectChapters(text)
	if len(chapters) == 0 {
		// Entire file as one chapter.
		chapters = []Chapter{{Index: 1, Title: title, Text: strings.TrimSpace(text)}}
	}

	return &Book{
		Title:    title,
		Author:   "Unknown",
		Chapters: chapters,
	}, nil
}
```

- [ ] **Step 4: Implement epub.go**

```go
// internal/extractor/epub.go
package extractor

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
)

// opfPackage represents the OPF package document.
type opfPackage struct {
	Metadata opfMetadata `xml:"metadata"`
	Spine    opfSpine    `xml:"spine"`
	Manifest opfManifest `xml:"manifest"`
}

type opfMetadata struct {
	Title   string `xml:"title"`
	Creator string `xml:"creator"`
}

type opfSpine struct {
	ItemRefs []opfItemRef `xml:"itemref"`
}

type opfItemRef struct {
	IDRef string `xml:"idref,attr"`
}

type opfManifest struct {
	Items []opfItem `xml:"item"`
}

type opfItem struct {
	ID        string `xml:"id,attr"`
	Href      string `xml:"href,attr"`
	MediaType string `xml:"media-type,attr"`
}

// containerXML locates the OPF file inside the EPUB.
type containerXML struct {
	Rootfiles []rootfile `xml:"rootfiles>rootfile"`
}

type rootfile struct {
	FullPath string `xml:"full-path,attr"`
}

var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

func extractEPUB(filePath string) (*Book, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("open epub: %w", err)
	}
	defer r.Close()

	// Find OPF path from META-INF/container.xml.
	opfPath, err := findOPFPath(r)
	if err != nil {
		return nil, err
	}

	// Parse OPF.
	opfDir := filepath.Dir(opfPath)
	pkg, err := parseOPF(r, opfPath)
	if err != nil {
		return nil, err
	}

	// Build href map from manifest.
	hrefByID := make(map[string]string)
	for _, item := range pkg.Manifest.Items {
		hrefByID[item.ID] = item.Href
	}

	// Read spine items in order.
	var chapters []Chapter
	for _, itemRef := range pkg.Spine.ItemRefs {
		href, ok := hrefByID[itemRef.IDRef]
		if !ok {
			continue
		}
		fullHref := href
		if opfDir != "." {
			fullHref = opfDir + "/" + href
		}
		text, err := readZipFileText(r, fullHref)
		if err != nil {
			continue
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		title := filepath.Base(href)
		title = strings.TrimSuffix(title, filepath.Ext(title))
		chapters = append(chapters, Chapter{
			Index: len(chapters) + 1,
			Title: title,
			Text:  text,
		})
	}

	if len(chapters) == 0 {
		return nil, fmt.Errorf("no text content found in epub")
	}

	return &Book{
		Title:    pkg.Metadata.Title,
		Author:   pkg.Metadata.Creator,
		Chapters: chapters,
	}, nil
}

func findOPFPath(r *zip.ReadCloser) (string, error) {
	for _, f := range r.File {
		if f.Name == "META-INF/container.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()
			var c containerXML
			if err := xml.NewDecoder(rc).Decode(&c); err != nil {
				return "", fmt.Errorf("parse container.xml: %w", err)
			}
			if len(c.Rootfiles) > 0 {
				return c.Rootfiles[0].FullPath, nil
			}
			return "", fmt.Errorf("no rootfile in container.xml")
		}
	}
	return "", fmt.Errorf("META-INF/container.xml not found")
}

func parseOPF(r *zip.ReadCloser, path string) (*opfPackage, error) {
	for _, f := range r.File {
		if f.Name == path {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			var pkg opfPackage
			if err := xml.NewDecoder(rc).Decode(&pkg); err != nil {
				return nil, fmt.Errorf("parse opf: %w", err)
			}
			return &pkg, nil
		}
	}
	return nil, fmt.Errorf("opf file not found: %s", path)
}

func readZipFileText(r *zip.ReadCloser, name string) (string, error) {
	for _, f := range r.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				return "", err
			}
			return stripHTML(string(data)), nil
		}
	}
	return "", fmt.Errorf("file not found in epub: %s", name)
}

func stripHTML(s string) string {
	text := htmlTagRegex.ReplaceAllString(s, " ")
	// Collapse whitespace.
	fields := strings.Fields(text)
	return strings.Join(fields, " ")
}
```

- [ ] **Step 5: Implement docx.go**

```go
// internal/extractor/docx.go
package extractor

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// docxBody represents the body of a DOCX document.xml.
type docxDocument struct {
	Body docxBody `xml:"body"`
}

type docxBody struct {
	Paragraphs []docxParagraph `xml:"p"`
}

type docxParagraph struct {
	Runs []docxRun `xml:"r"`
}

type docxRun struct {
	Text string `xml:"t"`
}

func extractDOCX(filePath string) (*Book, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("open docx: %w", err)
	}
	defer r.Close()

	var docFile *zip.File
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return nil, fmt.Errorf("word/document.xml not found in docx")
	}

	rc, err := docFile.Open()
	if err != nil {
		return nil, fmt.Errorf("open document.xml: %w", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read document.xml: %w", err)
	}

	var doc docxDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse document.xml: %w", err)
	}

	var paragraphs []string
	for _, p := range doc.Body.Paragraphs {
		var parts []string
		for _, r := range p.Runs {
			if r.Text != "" {
				parts = append(parts, r.Text)
			}
		}
		line := strings.Join(parts, "")
		if line != "" {
			paragraphs = append(paragraphs, line)
		}
	}

	text := strings.Join(paragraphs, "\n\n")
	title := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	chapters := DetectChapters(text)
	if len(chapters) == 0 {
		chapters = []Chapter{{Index: 1, Title: title, Text: strings.TrimSpace(text)}}
	}

	return &Book{
		Title:    title,
		Author:   "Unknown",
		Chapters: chapters,
	}, nil
}
```

- [ ] **Step 6: Implement pdf.go**

```go
// internal/extractor/pdf.go
package extractor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	gopdf "github.com/ledongthuc/pdf"
)

func extractPDF(filePath string) (*Book, error) {
	// Try pdftotext first (better quality).
	text, err := extractPDFWithPdftotext(filePath)
	if err != nil {
		// Fall back to pure Go.
		text, err = extractPDFWithGoLib(filePath)
		if err != nil {
			return nil, fmt.Errorf("extract pdf: %w", err)
		}
	}

	title := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	chapters := DetectChapters(text)
	if len(chapters) == 0 {
		chapters = []Chapter{{Index: 1, Title: title, Text: strings.TrimSpace(text)}}
	}

	return &Book{
		Title:    title,
		Author:   "Unknown",
		Chapters: chapters,
	}, nil
}

func extractPDFWithPdftotext(filePath string) (string, error) {
	pdftotext, err := exec.LookPath("pdftotext")
	if err != nil {
		return "", fmt.Errorf("pdftotext not found: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, pdftotext, "-layout", filePath, "-")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext: %w", err)
	}
	return out.String(), nil
}

func extractPDFWithGoLib(filePath string) (string, error) {
	f, r, err := gopdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}
	defer f.Close()

	var sb strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(text)
		sb.WriteString("\n\n")
	}
	return sb.String(), nil
}
```

- [ ] **Step 7: Add the pdf dependency**

Run: `cd ~/Documents/personal/audbookdl && PATH="/usr/local/go/bin:$PATH" go get github.com/ledongthuc/pdf`

- [ ] **Step 8: Run all extractor tests**

Run: `cd ~/Documents/personal/audbookdl && PATH="/usr/local/go/bin:$PATH" CGO_ENABLED=0 go test ./internal/extractor/ -v`
Expected: All tests PASS

- [ ] **Step 9: Commit**

```bash
git add internal/extractor/ go.mod go.sum
git commit -m "feat(extractor): add PDF, EPUB, TXT, DOCX text extraction"
```

---

### Task 3: TTS Engine Interface and Edge TTS

**Files:**
- Create: `internal/tts/engine.go`
- Create: `internal/tts/edge.go`
- Create: `internal/tts/edge_test.go`

- [ ] **Step 1: Create engine.go (interface and types)**

```go
// internal/tts/engine.go
package tts

import "context"

// Engine synthesizes text into audio.
type Engine interface {
	Name() string
	Synthesize(ctx context.Context, text string, opts SynthOptions) ([]byte, error)
	ListVoices(ctx context.Context) ([]Voice, error)
}

// SynthOptions configures speech synthesis.
type SynthOptions struct {
	Voice  string // e.g., "en-US-AriaNeural"
	Rate   string // e.g., "+20%", "-10%"
	Volume string // e.g., "+0%"
	Format string // "audio-24khz-48kbitrate-mono-mp3"
}

// Voice describes an available TTS voice.
type Voice struct {
	ID       string // "en-US-AriaNeural"
	Name     string // "Aria"
	Language string // "en-US"
	Gender   string // "Female"
}

// DefaultSynthOptions returns sensible defaults for Edge TTS.
func DefaultSynthOptions() SynthOptions {
	return SynthOptions{
		Voice:  "en-US-AriaNeural",
		Rate:   "+0%",
		Volume: "+0%",
		Format: "audio-24khz-48kbitrate-mono-mp3",
	}
}
```

- [ ] **Step 2: Write Edge TTS tests**

```go
// internal/tts/edge_test.go
package tts

import (
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
	if !contains(ssml, "Hello world") {
		t.Error("SSML should contain the text")
	}
	if !contains(ssml, "en-US-AriaNeural") {
		t.Error("SSML should contain the voice name")
	}
	if !contains(ssml, "+0%") {
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

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3: Implement Edge TTS**

```go
// internal/tts/edge.go
package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"nhooyr.io/websocket"
)

const (
	edgeWSURL     = "wss://speech.platform.bing.com/consumer/speech/synthesize/readaloud/edge/v1?TrustedClientToken=6A5AA1D4EAFF4E9FB37E23D68491D6F4"
	edgeVoiceURL  = "https://speech.platform.bing.com/consumer/speech/synthesize/readaloud/voices/list?trustedclienttoken=6A5AA1D4EAFF4E9FB37E23D68491D6F4"
	edgeChunkSize = 3000 // characters per WebSocket request
)

// EdgeTTS implements the Edge TTS WebSocket protocol in pure Go.
type EdgeTTS struct{}

// NewEdgeTTS creates a new Edge TTS engine.
func NewEdgeTTS() *EdgeTTS {
	return &EdgeTTS{}
}

func (e *EdgeTTS) Name() string { return "edge" }

// ListVoices fetches available voices from the Edge TTS API.
func (e *EdgeTTS) ListVoices(ctx context.Context) ([]Voice, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", edgeVoiceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch voices: %w", err)
	}
	defer resp.Body.Close()

	var raw []struct {
		ShortName string `json:"ShortName"`
		FriendlyName string `json:"FriendlyName"`
		Locale    string `json:"Locale"`
		Gender    string `json:"Gender"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode voices: %w", err)
	}

	voices := make([]Voice, len(raw))
	for i, v := range raw {
		name := v.FriendlyName
		if idx := strings.LastIndex(name, " - "); idx > 0 {
			name = name[idx+3:]
		}
		voices[i] = Voice{
			ID:       v.ShortName,
			Name:     name,
			Language: v.Locale,
			Gender:   v.Gender,
		}
	}
	return voices, nil
}

// Synthesize converts text to audio using Edge TTS.
func (e *EdgeTTS) Synthesize(ctx context.Context, text string, opts SynthOptions) ([]byte, error) {
	if opts.Voice == "" {
		opts.Voice = "en-US-AriaNeural"
	}
	if opts.Rate == "" {
		opts.Rate = "+0%"
	}
	if opts.Volume == "" {
		opts.Volume = "+0%"
	}
	if opts.Format == "" {
		opts.Format = "audio-24khz-48kbitrate-mono-mp3"
	}

	chunks := e.chunkText(text, edgeChunkSize)
	var audio bytes.Buffer

	for _, chunk := range chunks {
		data, err := e.synthesizeChunk(ctx, chunk, opts)
		if err != nil {
			return nil, fmt.Errorf("synthesize chunk: %w", err)
		}
		audio.Write(data)
	}

	return audio.Bytes(), nil
}

func (e *EdgeTTS) synthesizeChunk(ctx context.Context, text string, opts SynthOptions) ([]byte, error) {
	connCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(connCtx, edgeWSURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"User-Agent": []string{"Mozilla/5.0"},
			"Origin":     []string{"chrome-extension://jdiccldimpdaibmpdkjnbmckianbfold"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer conn.CloseNow()

	// Send config message.
	configMsg := fmt.Sprintf(
		"Content-Type:application/json; charset=utf-8\r\nPath:speech.config\r\n\r\n"+
			`{"context":{"synthesis":{"audio":{"metadataoptions":{"sentenceBoundaryEnabled":"false","wordBoundaryEnabled":"false"},"outputFormat":"%s"}}}}`,
		opts.Format,
	)
	if err := conn.Write(connCtx, websocket.MessageText, []byte(configMsg)); err != nil {
		return nil, fmt.Errorf("send config: %w", err)
	}

	// Send SSML message.
	ssml := e.buildSSML(text, opts)
	ssmlMsg := "Content-Type:application/ssml+xml\r\nPath:ssml\r\n\r\n" + ssml
	if err := conn.Write(connCtx, websocket.MessageText, []byte(ssmlMsg)); err != nil {
		return nil, fmt.Errorf("send ssml: %w", err)
	}

	// Read audio responses.
	var audio bytes.Buffer
	for {
		msgType, data, err := conn.Read(connCtx)
		if err != nil {
			// Connection closed — done.
			break
		}

		if msgType == websocket.MessageBinary {
			// Binary messages contain audio data after a header.
			// Header ends with "Path:audio\r\n" followed by audio bytes.
			marker := []byte("Path:audio\r\n")
			idx := bytes.Index(data, marker)
			if idx >= 0 {
				audio.Write(data[idx+len(marker):])
			}
		} else if msgType == websocket.MessageText {
			// Check for turn.end signal.
			if strings.Contains(string(data), "turn.end") {
				break
			}
		}
	}

	conn.Close(websocket.StatusNormalClosure, "done")
	return audio.Bytes(), nil
}

// buildSSML creates an SSML document for the given text and options.
func (e *EdgeTTS) buildSSML(text string, opts SynthOptions) string {
	// Escape XML special characters.
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")

	return fmt.Sprintf(
		`<speak version='1.0' xmlns='http://www.w3.org/2001/10/synthesis' xml:lang='en-US'>`+
			`<voice name='%s'>`+
			`<prosody rate='%s' volume='%s'>%s</prosody>`+
			`</voice></speak>`,
		opts.Voice, opts.Rate, opts.Volume, text,
	)
}

// chunkText splits text into chunks of at most maxLen characters,
// preferring to split at sentence boundaries.
func (e *EdgeTTS) chunkText(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	remaining := text

	for len(remaining) > 0 {
		if len(remaining) <= maxLen {
			chunks = append(chunks, remaining)
			break
		}

		// Find the last sentence-ending punctuation within maxLen.
		cutoff := remaining[:maxLen]
		splitIdx := -1
		for _, sep := range []string{". ", "! ", "? ", ".\n", "!\n", "?\n"} {
			idx := strings.LastIndex(cutoff, sep)
			if idx > splitIdx {
				splitIdx = idx + len(sep)
			}
		}

		if splitIdx <= 0 {
			// No sentence boundary — split at last space.
			splitIdx = strings.LastIndex(cutoff, " ")
			if splitIdx <= 0 {
				splitIdx = maxLen
			}
		}

		chunks = append(chunks, strings.TrimSpace(remaining[:splitIdx]))
		remaining = strings.TrimSpace(remaining[splitIdx:])
	}

	return chunks
}
```

- [ ] **Step 4: Add the websocket dependency**

Run: `cd ~/Documents/personal/audbookdl && PATH="/usr/local/go/bin:$PATH" go get nhooyr.io/websocket`

- [ ] **Step 5: Run tests**

Run: `cd ~/Documents/personal/audbookdl && PATH="/usr/local/go/bin:$PATH" CGO_ENABLED=0 go test ./internal/tts/ -v`
Expected: All tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/tts/ go.mod go.sum
git commit -m "feat(tts): add Engine interface and Edge TTS WebSocket client"
```

---

### Task 4: Piper TTS Engine

**Files:**
- Create: `internal/tts/piper.go`
- Create: `internal/tts/piper_test.go`

- [ ] **Step 1: Write Piper tests**

```go
// internal/tts/piper_test.go
package tts

import (
	"os"
	"path/filepath"
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
	if url == "" {
		t.Error("download URL should not be empty")
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
```

- [ ] **Step 2: Implement piper.go**

```go
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
```

- [ ] **Step 3: Run tests**

Run: `cd ~/Documents/personal/audbookdl && PATH="/usr/local/go/bin:$PATH" CGO_ENABLED=0 go test ./internal/tts/ -v`
Expected: All tests PASS

- [ ] **Step 4: Commit**

```bash
git add internal/tts/piper.go internal/tts/piper_test.go
git commit -m "feat(tts): add Piper TTS engine with auto-download"
```

---

### Task 5: Config Extension

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add ConversionConfig to config.go**

Add the new struct after `NotificationsConfig`:

```go
// ConversionConfig holds text-to-speech conversion settings
type ConversionConfig struct {
	DefaultEngine string `mapstructure:"default_engine"`
	DefaultVoice  string `mapstructure:"default_voice"`
	SpeechRate    string `mapstructure:"speech_rate"`
}
```

Add the field to `Config`:

```go
type Config struct {
	Download      DownloadConfig      `mapstructure:"download"`
	Player        PlayerConfig        `mapstructure:"player"`
	Search        SearchConfig        `mapstructure:"search"`
	Network       NetworkConfig       `mapstructure:"network"`
	Notifications NotificationsConfig `mapstructure:"notifications"`
	Conversion    ConversionConfig    `mapstructure:"conversion"`
}
```

Add defaults in `Init()` after the notifications defaults:

```go
	viper.SetDefault("conversion.default_engine", "edge")
	viper.SetDefault("conversion.default_voice", "en-US-AriaNeural")
	viper.SetDefault("conversion.speech_rate", "+0%")
```

- [ ] **Step 2: Run config tests**

Run: `cd ~/Documents/personal/audbookdl && PATH="/usr/local/go/bin:$PATH" CGO_ENABLED=0 go test ./internal/config/ -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): add conversion section for TTS settings"
```

---

### Task 6: Conversion Manager

**Files:**
- Create: `internal/converter/manager.go`
- Create: `internal/converter/manager_test.go`

- [ ] **Step 1: Write manager tests**

```go
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
```

- [ ] **Step 2: Implement manager.go**

```go
// internal/converter/manager.go
package converter

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/billmal071/audbookdl/internal/db"
	"github.com/billmal071/audbookdl/internal/extractor"
	"github.com/billmal071/audbookdl/internal/tts"
)

// Options configures a conversion run.
type Options struct {
	OutputDir   string
	Voice       string
	Rate        string
	Volume      string
	SkipConfirm bool
}

// Manager orchestrates the conversion pipeline.
type Manager struct {
	engine   tts.Engine
	database *sql.DB
}

// NewManager creates a conversion manager.
func NewManager(engine tts.Engine, database *sql.DB) *Manager {
	return &Manager{engine: engine, database: database}
}

// Convert runs the full pipeline: synthesize each chapter, save MP3, create DB records.
func (m *Manager) Convert(ctx context.Context, book *extractor.Book, opts Options) error {
	if len(book.Chapters) == 0 {
		return fmt.Errorf("no chapters to convert")
	}

	// Create output directory: OutputDir/Author/Title/
	outDir := filepath.Join(opts.OutputDir, book.Author, book.Title)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	// Create DB record for the audiobook.
	var downloadID int64
	if m.database != nil {
		audiobookID := fmt.Sprintf("converted-%s-%s", book.Author, book.Title)
		id, err := db.CreateDownload(m.database, &db.AudiobookDownload{
			AudiobookID: audiobookID,
			Title:       book.Title,
			Author:      book.Author,
			Narrator:    fmt.Sprintf("%s (TTS)", opts.Voice),
			Source:      "converted",
			Status:      db.StatusDownloading,
			BasePath:    outDir,
		})
		if err != nil {
			return fmt.Errorf("create download record: %w", err)
		}
		downloadID = id
	}

	synthOpts := tts.SynthOptions{
		Voice:  opts.Voice,
		Rate:   opts.Rate,
		Volume: opts.Volume,
		Format: "audio-24khz-48kbitrate-mono-mp3",
	}

	total := len(book.Chapters)
	succeeded := 0
	var failures []string
	startTime := time.Now()

	for i, ch := range book.Chapters {
		chStart := time.Now()
		fmt.Printf("[%d/%d] Converting %q...", i+1, total, ch.Title)

		fileName := fmt.Sprintf("%02d - %s.mp3", ch.Index, sanitizeFilename(ch.Title))
		filePath := filepath.Join(outDir, fileName)

		// Create chapter DB record.
		var chapterID int64
		if m.database != nil {
			cid, err := db.CreateChapterDownload(m.database, &db.ChapterDownload{
				DownloadID:   downloadID,
				ChapterIndex: ch.Index,
				Title:        ch.Title,
				FilePath:     filePath,
				Status:       db.StatusDownloading,
			})
			if err == nil {
				chapterID = cid
			}
		}

		// Synthesize with one retry.
		audio, err := m.synthesizeWithRetry(ctx, ch.Text, synthOpts)
		if err != nil {
			fmt.Printf(" FAILED: %v\n", err)
			failures = append(failures, fmt.Sprintf("Chapter %d (%s): %v", ch.Index, ch.Title, err))
			if m.database != nil && chapterID > 0 {
				db.UpdateChapterStatus(m.database, chapterID, db.StatusFailed)
			}
			continue
		}

		// Write MP3 file.
		if err := os.WriteFile(filePath, audio, 0644); err != nil {
			fmt.Printf(" FAILED: %v\n", err)
			failures = append(failures, fmt.Sprintf("Chapter %d (%s): write: %v", ch.Index, ch.Title, err))
			continue
		}

		elapsed := time.Since(chStart).Round(time.Second)
		fmt.Printf(" done (%s)\n", elapsed)
		succeeded++

		if m.database != nil && chapterID > 0 {
			db.UpdateChapterProgress(m.database, chapterID, int64(len(audio)))
			db.UpdateChapterStatus(m.database, chapterID, db.StatusCompleted)
		}
	}

	// Update download status.
	if m.database != nil {
		if succeeded == total {
			db.UpdateDownloadStatus(m.database, downloadID, db.StatusCompleted)
		} else if succeeded == 0 {
			db.UpdateDownloadStatus(m.database, downloadID, db.StatusFailed)
		} else {
			db.UpdateDownloadStatus(m.database, downloadID, db.StatusCompleted)
		}
	}

	// Summary.
	totalElapsed := time.Since(startTime).Round(time.Second)
	fmt.Printf("\nConversion complete: %d/%d chapters in %s\n", succeeded, total, totalElapsed)
	if len(failures) > 0 {
		fmt.Println("Failed chapters:")
		for _, f := range failures {
			fmt.Printf("  - %s\n", f)
		}
	}
	fmt.Printf("Output: %s\n", outDir)

	return nil
}

func (m *Manager) synthesizeWithRetry(ctx context.Context, text string, opts tts.SynthOptions) ([]byte, error) {
	audio, err := m.engine.Synthesize(ctx, text, opts)
	if err != nil {
		// Retry once after 2 seconds.
		time.Sleep(2 * time.Second)
		audio, err = m.engine.Synthesize(ctx, text, opts)
		if err != nil {
			return nil, err
		}
	}
	return audio, nil
}

// sanitizeFilename removes characters that are invalid in filenames.
func sanitizeFilename(name string) string {
	replacer := []string{
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	}
	r := name
	for i := 0; i < len(replacer); i += 2 {
		r = filepath.Clean(r)
		for j := 0; j < len(r); j++ {
			if string(r[j]) == replacer[i] {
				r = r[:j] + replacer[i+1] + r[j+1:]
			}
		}
	}
	if len(r) > 100 {
		r = r[:100]
	}
	return r
}
```

- [ ] **Step 3: Run tests**

Run: `cd ~/Documents/personal/audbookdl && PATH="/usr/local/go/bin:$PATH" CGO_ENABLED=0 go test ./internal/converter/ -v`
Expected: All tests PASS

- [ ] **Step 4: Commit**

```bash
git add internal/converter/
git commit -m "feat(converter): add conversion manager pipeline"
```

---

### Task 7: CLI Commands (convert + voices)

**Files:**
- Create: `internal/cli/convert.go`
- Create: `internal/cli/voices.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Create convert.go**

```go
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
	cfg := config.Get()
	convertCmd.Flags().StringVarP(&convertEngine, "engine", "e", cfg.Conversion.DefaultEngine, "TTS engine: edge or piper")
	convertCmd.Flags().StringVarP(&convertVoice, "voice", "v", cfg.Conversion.DefaultVoice, "voice ID")
	convertCmd.Flags().StringVarP(&convertRate, "rate", "r", cfg.Conversion.SpeechRate, "speech rate (e.g., +20%, -10%)")
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
```

- [ ] **Step 2: Create voices.go**

```go
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
```

- [ ] **Step 3: Register commands in root.go**

Add to `init()` in `internal/cli/root.go`, after the `playCmd` line:

```go
	rootCmd.AddCommand(convertCmd)
	rootCmd.AddCommand(voicesCmd)
```

- [ ] **Step 4: Build and verify**

Run: `cd ~/Documents/personal/audbookdl && PATH="/usr/local/go/bin:$PATH" CGO_ENABLED=0 go build ./...`
Expected: Clean build

- [ ] **Step 5: Commit**

```bash
git add internal/cli/convert.go internal/cli/voices.go internal/cli/root.go
git commit -m "feat(cli): add convert and voices commands"
```

---

### Task 8: Final Integration Test and Cleanup

**Files:**
- All modified files

- [ ] **Step 1: Run full test suite**

Run: `cd ~/Documents/personal/audbookdl && PATH="/usr/local/go/bin:$PATH" CGO_ENABLED=0 go test ./... -v`
Expected: All tests PASS

- [ ] **Step 2: Run go vet and fmt**

Run: `cd ~/Documents/personal/audbookdl && PATH="/usr/local/go/bin:$PATH" CGO_ENABLED=0 go vet ./... && PATH="/usr/local/go/bin:$PATH" gofmt -l .`
Expected: No issues

- [ ] **Step 3: Build binary**

Run: `cd ~/Documents/personal/audbookdl && PATH="/usr/local/go/bin:$PATH" CGO_ENABLED=0 go build -o ./build/audbookdl ./cmd/audbookdl`
Expected: Clean build

- [ ] **Step 4: Smoke test convert command help**

Run: `cd ~/Documents/personal/audbookdl && ./build/audbookdl convert --help`
Expected: Shows usage with all flags

- [ ] **Step 5: Smoke test voices command**

Run: `cd ~/Documents/personal/audbookdl && ./build/audbookdl voices --engine piper`
Expected: Lists piper voices

- [ ] **Step 6: Fix any formatting or vet issues, commit**

```bash
git add -A
git commit -m "chore: final cleanup for TTS convert feature"
```
