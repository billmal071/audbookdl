# TTS Convert Feature Design

Convert PDF, EPUB, TXT, and DOCX files into audiobooks using text-to-speech engines.

## Context

audbookdl currently searches and downloads free audiobooks from public domain sources. Users want to convert their own ebook files into audiobooks with natural-sounding TTS voices. This feature adds a `convert` CLI command and a `voices` command.

## Architecture Overview

Four new packages: `extractor` (text extraction), `tts` (speech synthesis engines), `converter` (orchestration manager), plus CLI commands. All integrate with the existing config, database, tagger, and library systems.

## 1. Text Extraction — `internal/extractor/`

Unified interface to extract structured text from multiple ebook formats.

### Interface

```go
type Book struct {
    Title    string
    Author   string
    Chapters []Chapter
}

type Chapter struct {
    Index int
    Title string
    Text  string // plain text content
}

func Extract(filePath string) (*Book, error)
```

### Format Handlers

- **PDF** — shell out to `pdftotext` (poppler-utils). Fallback to pure Go library (`ledongthuc/pdf`) if pdftotext not installed.
- **EPUB** — parse zip structure in pure Go, extract XHTML from spine, strip HTML tags. Chapter boundaries from EPUB TOC (table of contents).
- **TXT** — read file directly, split on blank-line patterns.
- **DOCX** — parse zip structure in pure Go, extract `word/document.xml`, strip XML tags.

Format detected by file extension. Unknown extensions return an error.

### Chapter Auto-Detection

Scan extracted text for patterns: `Chapter \d+`, `PART \w+`, `^\d+\.`, heading styles. EPUB uses its native TOC entries. If no chapters detected, fall back to splitting every ~5000 words.

Detected chapters are printed to stdout for user review. A `--yes` flag skips confirmation and proceeds immediately.

## 2. TTS Engines — `internal/tts/`

Pluggable engine interface with two implementations.

### Interface

```go
type Engine interface {
    Name() string
    Synthesize(ctx context.Context, text string, opts SynthOptions) ([]byte, error)
    ListVoices(ctx context.Context) ([]Voice, error)
}

type SynthOptions struct {
    Voice  string // e.g., "en-US-AriaNeural"
    Rate   string // e.g., "+20%", "-10%"
    Volume string // e.g., "+0%"
    Format string // "mp3" or "wav"
}

type Voice struct {
    ID       string // "en-US-AriaNeural"
    Name     string // "Aria"
    Language string // "en-US"
    Gender   string // "Female"
}
```

### Edge TTS — `internal/tts/edge.go`

Pure Go implementation of Microsoft Edge's TTS WebSocket protocol.

- Connects to `wss://speech.platform.bing.com/consumer/speech/synthesize/readaloud/edge/v1`
- Sends SSML payload, receives binary audio chunks
- Supports ~400 voices across multiple languages
- Chunks text into ~3000 character segments (Edge TTS limit per request), concatenates audio output
- No API key required, no Python dependency
- Default voice: `en-US-AriaNeural`

### Piper TTS — `internal/tts/piper.go`

Offline TTS via the Piper binary with auto-download.

- On first use, checks if `piper` is on PATH
- If not found, downloads piper binary + default English voice model to `~/.config/audbookdl/piper/`
- Pipes text via stdin: `echo "text" | piper --model <model> --output_raw`
- Converts raw PCM (s16le, 22050Hz, mono) to MP3 via ffmpeg/lame
- Default voice: `en_US-lessac-medium`

### Voice Selection

- `audbookdl voices` command lists available voices, filterable by language
- `--voice` flag on convert command selects voice
- Default voice stored in config (`conversion.default_voice`)

## 3. Conversion Manager — `internal/converter/`

Orchestrates the full pipeline.

### Flow

1. `extractor.Extract(filePath)` — get Book with chapters
2. Print detected chapters to stdout for review (skip with `--yes`)
3. User confirms or the flag bypasses
4. For each chapter sequentially:
   a. Call `engine.Synthesize(ctx, chapter.Text, opts)` — get audio bytes
   b. Save to `~/Audiobooks/Author/Title/01 - Chapter Title.mp3`
   c. Print progress: `[3/12] Converting "Chapter 3"... done (2m34s)`
5. Tag all MP3s with ID3 metadata via existing tagger package:
   - Artist: Author, Album: Title, Genre: "Audiobook"
   - Narrator field set to voice name (e.g., "Aria (Edge TTS)")
6. Create DB records: `audiobook_downloads` with `source: "converted"`, `chapter_downloads` entries

### Progress

- Per-chapter line: `[3/12] Converting "Chapter 3: The Return"... done (2m34s)`
- Estimated time remaining based on average chapter conversion time
- Final summary: total time, chapters succeeded/failed

### Error Handling

- Chapter failure: retry once with exponential backoff
- Second failure: skip chapter, log error, continue with remaining chapters
- Final report lists which chapters succeeded/failed
- `audbookdl convert --resume <id>` retries only failed chapters from a previous run

### Concurrency

Sequential chapter conversion. Edge TTS has rate limits; Piper is CPU-bound. Sequential avoids throttling and keeps resource usage predictable.

## 4. CLI Commands

### `audbookdl convert <file> [flags]`

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--engine` | `-e` | `edge` | TTS engine: `edge` or `piper` |
| `--voice` | `-v` | from config | Voice ID |
| `--rate` | `-r` | `+0%` | Speech rate adjustment |
| `--author` | `-a` | auto-detect | Override book author |
| `--title` | `-t` | auto-detect | Override book title |
| `--output` | `-o` | `~/Audiobooks/Author/Title/` | Output directory |
| `--yes` | `-y` | false | Skip chapter review |
| `--resume` | | | Resume failed conversion by download ID |

### `audbookdl voices [flags]`

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--engine` | `-e` | `edge` | Which engine's voices |
| `--lang` | `-l` | all | Filter by language code |

## 5. Config — `internal/config/`

New section added to Config struct:

```go
type ConversionConfig struct {
    DefaultEngine string `mapstructure:"default_engine"` // "edge" or "piper"
    DefaultVoice  string `mapstructure:"default_voice"`  // voice ID
    SpeechRate    string `mapstructure:"speech_rate"`     // e.g., "+0%"
}
```

Defaults: engine=`edge`, voice=`en-US-AriaNeural`, rate=`+0%`.

Config key prefix: `conversion.*`. Env override: `AUDBOOKDL_CONVERSION_*`.

## 6. Database

Converted audiobooks use the existing `audiobook_downloads` and `chapter_downloads` tables. Distinguished by `source = "converted"`. No new tables needed.

The `audiobook_id` for converted books is derived from a hash of the input file path, so resuming works correctly.

## 7. Dependencies

New Go dependencies:
- WebSocket library for Edge TTS (e.g., `nhooyr.io/websocket`)
- No new dependencies for EPUB/DOCX (standard `archive/zip` + `encoding/xml`)
- PDF fallback: `github.com/ledongthuc/pdf` (pure Go)

External tools (optional, with fallbacks):
- `pdftotext` (poppler-utils) — preferred PDF extractor, falls back to pure Go
- `piper` — auto-downloaded on first use for offline TTS
- `ffmpeg` — needed for Piper PCM-to-MP3 conversion

## Files Changed/Created

| File | Action |
|------|--------|
| `internal/extractor/extractor.go` | Create — Extract interface and format dispatch |
| `internal/extractor/pdf.go` | Create — PDF extraction |
| `internal/extractor/epub.go` | Create — EPUB extraction |
| `internal/extractor/txt.go` | Create — TXT extraction |
| `internal/extractor/docx.go` | Create — DOCX extraction |
| `internal/extractor/chapters.go` | Create — chapter auto-detection |
| `internal/tts/engine.go` | Create — Engine interface, SynthOptions, Voice types |
| `internal/tts/edge.go` | Create — Edge TTS WebSocket client |
| `internal/tts/piper.go` | Create — Piper binary integration with auto-download |
| `internal/converter/manager.go` | Create — conversion orchestration |
| `internal/cli/convert.go` | Create — convert command |
| `internal/cli/voices.go` | Create — voices command |
| `internal/cli/root.go` | Modify — register convert and voices commands |
| `internal/config/config.go` | Modify — add ConversionConfig |
