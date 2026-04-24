# audbookdl

A Go CLI tool for searching and downloading free audiobooks from multiple public sources.

## Features

- **Multi-source search** — query LibriVox, Internet Archive, Loyal Books, and Open Library in parallel
- **Full-screen TUI** — browse, download, and play audiobooks from the terminal (bubbletea + bubbles + lipgloss)
- **Built-in audio player** — play/pause, chapter navigation, speed control, volume, sleep timer, position memory
- **Resumable downloads** — chunked downloads with per-chapter SQLite tracking for pause/resume
- **Album-aware** — treats audiobooks as multi-chapter albums, organized as `Author/Title/01 - Chapter.mp3`
- **ID3 tagging** — embeds title, author, narrator, track number, genre, and cover art into downloaded MP3s
- **Bookmarks & history** — save audiobooks for later, browse past searches
- **Cross-platform** — pure Go (CGO_ENABLED=0), builds for Linux, macOS, and Windows
- **Desktop notifications** — get notified when downloads complete

## Install

### From source

```bash
git clone https://github.com/billmal071/audbookdl.git
cd audbookdl
make install
```

Requires Go 1.22+.

### Build

```bash
make build          # Build to ./build/audbookdl
make build-all      # Cross-compile for all platforms
```

## Usage

### TUI Mode

Launch the full-screen terminal UI:

```bash
audbookdl
```

Navigate with `tab`/`shift+tab` between tabs:

| Tab | What it does |
|-----|-------------|
| **Search** | Type a query, press Enter to search all sources |
| **Downloads** | View download progress and status |
| **Library** | Browse completed downloads by author, press Enter to play |
| **Player** | Audio playback with controls |

### CLI Mode

```bash
# Search for audiobooks
audbookdl search "sherlock holmes"
audbookdl search "dickens" -s librivox -n 10
audbookdl search "jane austen" -s archive

# Download by ID (from search results)
audbookdl download 314 -s librivox
audbookdl download adventures_sherlock_holmes_0711_librivox -s archive

# Manage downloads
audbookdl list
audbookdl pause 1
audbookdl resume 1

# Bookmarks
audbookdl bookmark list
audbookdl bookmark delete 1

# Search history
audbookdl history
audbookdl history clear

# Configuration
audbookdl config get download.directory
audbookdl config set download.directory ~/Audiobooks
audbookdl config path

# Shell completions
audbookdl completion bash >> ~/.bashrc
audbookdl completion zsh > "${fpath[1]}/_audbookdl"
audbookdl completion fish > ~/.config/fish/completions/audbookdl.fish
```

## Sources

| Source | Method | Content |
|--------|--------|---------|
| [LibriVox](https://librivox.org) | XML API + RSS feeds | Public domain audiobooks read by volunteers |
| [Internet Archive](https://archive.org) | JSON API | Massive audiobook collection including LibriVox mirrors |
| [Loyal Books](https://loyalbooks.com) | HTML scraping | Curated public domain audiobooks |
| [Open Library](https://openlibrary.org) | JSON API | Metadata bridge to Internet Archive |

## Configuration

Config file at `~/.config/audbookdl/config.yaml`:

```yaml
download:
  directory: ~/Audiobooks
  chunk_size: 5242880       # 5MB
  max_concurrent: 3
  preferred_format: mp3

player:
  default_speed: 1.0
  skip_seconds: 15
  sleep_timer_minutes: 0

search:
  default_limit: 10
  cache_ttl: 3600
  sources:
    - librivox
    - archive
    - loyalbooks
    - openlibrary

notifications:
  enabled: true
  sound: true
```

Environment variable overrides with `AUDBOOKDL_` prefix (e.g., `AUDBOOKDL_DOWNLOAD_DIRECTORY`).

## File Organization

Downloaded audiobooks are saved in a nested structure:

```
~/Audiobooks/
└── Charles Dickens/
    └── Christmas Carol, A/
        ├── 01 - Preface and Stave 1.mp3
        ├── 02 - Stave 2.mp3
        ├── 03 - Stave 3.mp3
        ├── 04 - Stave 4.mp3
        ├── 05 - Stave 5.mp3
        └── cover.jpg
```

## Player Controls

| Key | Action |
|-----|--------|
| `space` | Play / Pause |
| `n` | Next chapter |
| `p` | Previous chapter (restart if >3s in) |
| `left` / `h` | Skip back 15s |
| `right` / `l` | Skip forward 15s |
| `s` | Cycle speed (1.0x → 1.25x → 1.5x → 1.75x → 2.0x → 0.75x) |
| `v` | Cycle volume (+10%, wraps) |

Playback position is saved automatically and restored when you return to an audiobook.

## Architecture

```
cmd/audbookdl/          Entry point
internal/
├── source/             Source interface, Audiobook/Chapter types
├── librivox/           LibriVox XML API + RSS client
├── archive/            Internet Archive search + metadata client
├── loyalbooks/         Loyal Books HTML scraper (goquery)
├── openlibrary/        Open Library API with IA delegation
├── search/             Multi-source concurrent orchestrator
├── downloader/         Album-aware chunked downloads with retry
├── player/             Audio engine with state persistence
├── tagger/             ID3v2 tag writing + cover art
├── tui/                Full bubbletea app (4 tabs)
├── cli/                Cobra commands
├── db/                 SQLite (WAL mode, 7 tables)
├── config/             Viper YAML configuration
├── httpclient/         Shared HTTP client
└── notify/             Desktop notifications
```

## Development

```bash
make test           # Run all tests
make fmt            # Format code
make lint           # Run golangci-lint
make ci             # Run all CI checks (format, vet, test, build)
```

## License

MIT
