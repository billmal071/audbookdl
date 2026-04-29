# Beginner's Guide to audbookdl

A step-by-step guide to finding, downloading, and listening to free audiobooks from your terminal.

## Table of Contents

- [What is audbookdl?](#what-is-audbookdl)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Using the TUI (Recommended)](#using-the-tui-recommended)
- [Using the CLI](#using-the-cli)
- [Playing Audiobooks](#playing-audiobooks)
- [Managing Downloads](#managing-downloads)
- [Converting Ebooks to Audiobooks](#converting-ebooks-to-audiobooks)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)

---

## What is audbookdl?

audbookdl is a free, open-source terminal tool that lets you search, download, and listen to public domain audiobooks. It pulls from four sources:

- **LibriVox** — volunteer-read public domain books
- **Internet Archive** — massive audiobook collection
- **Loyal Books** — curated public domain audiobooks
- **Open Library** — metadata bridge to Internet Archive

All content is free and legal (public domain).

## Installation

### Prerequisites

- **Go 1.22+** (to build from source)
- **mpv** (for the built-in audio player)

Install mpv if you don't have it:

```bash
# Ubuntu / Debian
sudo apt install mpv

# macOS
brew install mpv

# Fedora
sudo dnf install mpv

# Arch
sudo pacman -S mpv
```

### Install audbookdl

**Option 1: Download a pre-built binary** from the [latest release](https://github.com/billmal071/audbookdl/releases/latest), then put it somewhere on your PATH:

```bash
chmod +x audbookdl-linux-amd64
sudo mv audbookdl-linux-amd64 /usr/local/bin/audbookdl
```

**Option 2: Build from source:**

```bash
git clone https://github.com/billmal071/audbookdl.git
cd audbookdl
make install
```

Verify it's working:

```bash
audbookdl version
```

## Quick Start

The fastest way to get going:

```bash
# Launch the interactive TUI
audbookdl

# Or do everything from the command line
audbookdl search "pride and prejudice"
audbookdl download 314 -s librivox
audbookdl play 1
```

## Using the TUI (Recommended)

The TUI is the easiest way to use audbookdl. Launch it with:

```bash
audbookdl
```

You'll see four tabs across the top. Press `tab` to move between them and `shift+tab` to go back. Press `?` to see keybindings for the current tab.

### Search Tab

1. Type your search query (e.g., "sherlock holmes") and press `Enter`
2. Use `j`/`k` or arrow keys to scroll through results
3. Press `Enter` on a result to see details
4. In the detail view:
   - Press `d` to download
   - Press `b` to bookmark for later
   - Press `esc` to go back

### Downloads Tab

Shows all your downloads with progress bars.

| Key | Action |
|-----|--------|
| `j`/`k` or arrows | Navigate up/down |
| `R` (shift+r) | Resume a failed or paused download |
| `r` | Manual refresh |

The tab auto-refreshes every 2 seconds, so you can watch progress in real time.

### Library Tab

Shows your completed downloads organized by author. Press `Enter` on an audiobook to start playing it.

### Player Tab

The built-in audio player. Plays directly in your terminal using mpv.

| Key | Action |
|-----|--------|
| `space` | Play / Pause |
| `n` | Next chapter |
| `p` | Previous chapter (restarts current if >3s in) |
| `left` or `h` | Skip back 15 seconds |
| `right` or `l` | Skip forward 15 seconds |
| `s` | Cycle speed (1.0x, 1.25x, 1.5x, 1.75x, 2.0x, 0.75x) |
| `v` | Cycle volume (+10%, wraps around) |
| `t` | Cycle sleep timer (off, 15m, 30m, 45m, 60m, 90m) |
| `c` | Open chapter list (pick any chapter to jump to) |

When a chapter finishes, the next one starts automatically. Your position is saved and restored when you come back.

Press `q` or `ctrl+c` to exit the TUI. Audio will stop cleanly.

## Using the CLI

If you prefer commands over the TUI, everything is available via subcommands.

### Searching

```bash
# Search all sources
audbookdl search "jane austen"

# Search a specific source
audbookdl search "moby dick" -s librivox

# Limit results
audbookdl search "dickens" -n 5

# Filter by author
audbookdl search "christmas carol" -a "dickens"

# Paginate
audbookdl search "dickens" -p 1
```

Each result shows an **ID** and a **source** — you'll need both to download.

### Downloading

```bash
# Download from LibriVox (default source)
audbookdl download 314

# Download from Internet Archive
audbookdl download adventures_sherlock_holmes_0711_librivox -s archive

# Download to a specific folder
audbookdl download 314 -o ~/my-audiobooks
```

Files are saved as:

```
~/Audiobooks/
  Jane Austen/
    Pride and Prejudice/
      01 - Ch. 01-04.mp3
      02 - Ch. 05-08.mp3
      ...
```

### Listing Downloads

```bash
audbookdl list
```

Shows all downloads with their ID, status, and progress:

```
ID      Title                                     Author                     Status        Progress
------  ----------------------------------------  -------------------------  ------------  --------
1       Pride and Prejudice (version 2)           Jane Austen                completed     100.0%
2       A Christmas Carol                         Charles Dickens            failed        3/5 chapters
```

### Pausing and Resuming

```bash
# Pause a running download
audbookdl pause 1

# Resume a failed or paused download
audbookdl resume 2
```

Resume is smart: it skips chapters that already downloaded successfully and only retries the ones that failed. If a chapter was partially downloaded, it picks up from where it left off.

### Playing from CLI

```bash
# Play by download ID
audbookdl play 1

# Play from a directory
audbookdl play ~/Audiobooks/Jane\ Austen/Pride\ and\ Prejudice/

# Play at 1.5x speed
audbookdl play 1 --speed 1.5

# Play at lower volume
audbookdl play 1 --volume 0.5
```

### Bookmarks

```bash
# List bookmarks
audbookdl bookmark list

# Delete a bookmark
audbookdl bookmark delete 1
```

Bookmarks are created from the TUI search detail view (press `b`).

### Search History

```bash
# View past searches
audbookdl history

# Clear history
audbookdl history clear
```

### Download Queue

```bash
# View pending downloads
audbookdl queue list

# Remove a queued download
audbookdl queue remove 3

# Clear the entire queue
audbookdl queue clear
```

## Playing Audiobooks

audbookdl uses [mpv](https://mpv.io/) for audio playback. Make sure mpv is installed (see [Installation](#installation)).

Playback state (position, chapter, speed) is saved automatically every 5 seconds. When you come back to an audiobook, it picks up right where you left off.

### From the TUI

1. Go to the **Library** tab
2. Navigate to an audiobook and press `Enter`
3. You're now in the **Player** tab with full controls

### From the CLI

```bash
audbookdl play 1
```

This opens an interactive player in your terminal with the same controls as the TUI player tab.

## Converting Ebooks to Audiobooks

Got a PDF, EPUB, TXT, or DOCX file? Convert it to an audiobook with text-to-speech:

```bash
# Basic conversion (uses Microsoft Edge TTS, free)
audbookdl convert book.pdf

# Choose a different voice
audbookdl convert book.epub --voice en-US-GuyNeural

# Use Piper TTS engine instead
audbookdl convert book.txt --engine piper --voice en_US-lessac-medium

# Speed up the speech
audbookdl convert book.docx --rate "+20%"

# Set custom title and author
audbookdl convert book.pdf --title "My Book" --author "Some Author"

# Skip the chapter review prompt
audbookdl convert book.pdf --yes
```

List available voices:

```bash
# Edge TTS voices
audbookdl voices

# Piper TTS voices
audbookdl voices --engine piper
```

The converter:
1. Extracts text from your file
2. Auto-detects chapter boundaries (headings, "Chapter X" patterns, etc.)
3. Shows you the detected chapters for review
4. Converts each chapter to MP3 using TTS
5. Saves them in the same `Author/Title/` structure as downloaded audiobooks

## Configuration

Config lives at `~/.config/audbookdl/config.yaml`. Edit it directly or use the CLI:

```bash
# See where the config file is
audbookdl config path

# Get a value
audbookdl config get download.directory

# Set a value
audbookdl config set download.directory ~/my-audiobooks
audbookdl config set download.max_concurrent 5
audbookdl config set player.default_speed 1.25
```

### Common settings

| Setting | Default | Description |
|---------|---------|-------------|
| `download.directory` | `~/Audiobooks` | Where audiobooks are saved |
| `download.max_concurrent` | `3` | Parallel chapter downloads |
| `player.default_speed` | `1.0` | Default playback speed |
| `player.skip_seconds` | `15` | Seconds to skip with arrow keys |
| `search.default_limit` | `10` | Results per search |
| `search.sources` | all four | Which sources to search |
| `notifications.enabled` | `true` | Desktop notifications on download complete |

### Environment variables

Any setting can be overridden with an environment variable using the `AUDBOOKDL_` prefix:

```bash
export AUDBOOKDL_DOWNLOAD_DIRECTORY=~/my-audiobooks
export AUDBOOKDL_DOWNLOAD_MAX_CONCURRENT=5
```

### Shell completions

Enable tab completion for your shell:

```bash
# Bash
audbookdl completion bash >> ~/.bashrc

# Zsh
audbookdl completion zsh > "${fpath[1]}/_audbookdl"

# Fish
audbookdl completion fish > ~/.config/fish/completions/audbookdl.fish
```

## Troubleshooting

### "mpv not found"

Install mpv — see [Installation](#installation). The player won't work without it.

### Audio keeps playing after closing

This was fixed in v0.3.0. Update to the latest version. If you still have orphaned audio:

```bash
pkill -f 'mpv.*audbookdl-mpv'
```

### Download stuck or failed

```bash
# Check status
audbookdl list

# Resume failed downloads
audbookdl resume <id>
```

Or in the TUI, go to the Downloads tab and press `R` on the failed download.

### "No chapters found"

Some audiobook IDs are source-specific. Make sure you're using the right `--source` flag:

```bash
# This ID is from LibriVox
audbookdl download 314 -s librivox

# This ID is from Internet Archive
audbookdl download adventures_sherlock_holmes_0711_librivox -s archive
```

### Search returns no results

Try a different source or a simpler query:

```bash
# Search all sources
audbookdl search "sherlock"

# Try a specific source
audbookdl search "sherlock" -s archive
```

### Config file issues

Reset to defaults by deleting the config:

```bash
rm ~/.config/audbookdl/config.yaml
```

A fresh one will be created on next run.

### Where are my audiobooks stored?

By default: `~/Audiobooks/`. Check with:

```bash
audbookdl config get download.directory
```

### Where is the database?

SQLite database is at `~/.local/share/audbookdl/audbookdl.db`. It stores download state, bookmarks, playback positions, and search history.
