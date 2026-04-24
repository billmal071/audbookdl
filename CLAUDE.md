# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**audbookdl** is a Go CLI tool for searching and downloading free audiobooks from LibriVox, Internet Archive, Loyal Books, and Open Library. It supports interactive TUI selection, resumable chunked downloads, and SQLite-backed state tracking.

## Build & Development Commands

```bash
make build          # Build to ./build/audbookdl (CGO_ENABLED=0)
make install        # Install to GOPATH/bin
make test           # go test -v ./...
make fmt            # Format code
make lint           # golangci-lint (must be installed separately)
make run ARGS="..." # Build and run with arguments
make deps           # go mod tidy && go mod download
make build-all      # Cross-compile for Linux, macOS (amd64+arm64), Windows
```

CGO is disabled — SQLite uses `modernc.org/sqlite` (pure Go). Do not introduce cgo-dependent SQLite drivers.

## Architecture

### Multi-Source Search (`internal/search/`)
Unified `Searcher` aggregates results from multiple audiobook sources in parallel.

### Source Clients (`internal/librivox/`, `internal/archive/`, `internal/loyalbooks/`, `internal/openlibrary/`)
Each source follows the same strategy pattern:
- **ScraperClient** — web scraping fallback via colly/goquery
- **APIClient** — uses source API when available

### Download Manager (`internal/downloader/`)
- **Chunked downloads** with configurable chunk size (default 5MB)
- Per-chunk progress tracked in SQLite for pause/resume
- Exponential backoff retry with jitter (`retry.go`)
- Post-download MD5 verification (`verify.go`)

### Database (`internal/db/`)
SQLite with WAL mode. Tables: `downloads`, `chunks`, `bookmarks`, `playback`, `search_history`, `search_cache`. Schema migrations run at startup in `db.go`.

### CLI (`internal/cli/`)
Cobra-based with subcommands for search, download, bookmarks, history, and playback. `root.go` handles init (config + DB) in `PersistentPreRunE` and cleanup in `PersistentPostRun`.

### TUI (`internal/tui/`)
Bubbletea-based interactive selectors for audiobook picking and search history browsing.

### Config (`internal/config/`)
Viper-based YAML config at `~/.config/audbookdl/config.yaml`. Environment overrides via `AUDBOOKDL_*` prefix.

### Version Injection
LDFLAGS inject `Version` and `Commit` into `internal/cli` package variables at build time.
