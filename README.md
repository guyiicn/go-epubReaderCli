# go-epubReaderCli

A terminal EpubReader client written in Go. Works in any terminal, supports local reading, SQLite storage, server sync, find-book search, and keyboard-driven TUI reading.

## Features

- **EPUB 2 & 3 support** — NCX and nav.xhtml parsing
- **TXT/Markdown direct reading** — plain text content is split into readable sections
- **MOBI/AZW3/PDF support via tools** — uses Calibre `ebook-convert` or `pdftotext` when available
- **CJK friendly** — proper Chinese/Japanese/Korean character width and line breaking
- **Dual-column layout** — auto-enable on wide terminals (≥120 cols), toggle with `c`
- **SQLite library** — local/remote/dirty state, migrated from the old JSON layout
- **Server sync** — auth, device registration, books, progress, bookmarks, annotations, find-book, and WebSocket invalidate client
- **Library management** — add/remove books, remote metadata, reading progress with percentage bar
- **Auto progress save** — remembers your position, resumes on next open
- **Bookmarks** — add/delete/list/jump
- **Search** — search current chapter, jump between matches
- **Book info** — title, author, publisher, description, chapter count
- **Keyboard driven** — all lowercase shortcuts, no Shift required
- **Zero config** — works out of the box, stores data in `~/.config/epub-reader/`

## Installation

### From source (requires Go 1.21+)

```bash
git clone https://github.com/guyiicn/go-epubReaderCli.git
cd go-epubReaderCli
go build -o epub-reader .
sudo cp epub-reader /usr/local/bin/
```

### Arch Linux (AUR)

```bash
# TODO: AUR package coming soon
```

## Usage

```bash
# Open library TUI
epub-reader

# Open a specific book directly
epub-reader ~/books/three-body.epub

# Show storage paths
epub-reader paths

# Login and register CLI device
epub-reader login --server https://us.guyii.net --username your-name

# Pull/push server sync
epub-reader sync

# List local and remote books
epub-reader list

# Import/open/download
epub-reader import ~/books/three-body.epub
epub-reader open three
epub-reader download <server-book-id>

# Find-book API
epub-reader search "漫长的告别"
epub-reader search-download --book-command /book_xxx --title "漫长的告别" --author "Raymond Chandler"
```

Data is stored in XDG paths:

```
~/.config/epub-reader-term/config.json
~/.local/share/epub-reader-term/reader.db
~/.local/share/epub-reader-term/books/
~/.cache/epub-reader-term/converted/
```

The legacy `~/.config/epub-reader/` JSON library is migrated once on startup.

## Keyboard Shortcuts

### Global

| Key | Action |
|-----|--------|
| `h` | Help (works on any screen) |
| `Esc` | Back / Close / Cancel |
| `Ctrl+C` | Force quit |

### Library

| Key | Action |
|-----|--------|
| `j` / `↓` | Next book |
| `k` / `↑` | Previous book |
| `Enter` | Open book |
| `a` | Add book (enter file path) |
| `s` | Sync with server |
| `d` | Remove book from library |
| `q` | Quit |

### Reader

| Key | Action |
|-----|--------|
| `←` / `Space` / `PgDn` | Next page (crosses chapters) |
| `→` / `Backspace` / `PgUp` | Previous page (crosses chapters) |

Flipping past the last page of a chapter automatically enters the next chapter. Flipping past the first page goes to the previous chapter.

| Key | Action |
|-----|--------|
| `g` | Jump to chapter start |
| `e` | Jump to chapter end |
| `n` | Next chapter |
| `p` | Previous chapter |
| `t` | Table of contents |
| `b` | Bookmark list |
| `a` | Add bookmark at current position |
| `m` | Add annotation/note at current position |
| `i` | Book info |
| `c` | Toggle single/dual column |
| `/` | Search current chapter |
| `.` | Next search result |
| `o` / `q` / `Esc` | Back to library |

### Popups (TOC / Bookmarks / Help / Info)

| Key | Action |
|-----|--------|
| `j` `k` `↑` `↓` | Navigate |
| `Enter` | Confirm / Jump |
| `d` | Delete bookmark (in bookmark list only) |
| `Esc` | Close popup |

## Configuration

Config file: `~/.config/epub-reader/config.json`

```json
{
  "columns": 0,
  "column_threshold": 120,
  "line_spacing": 0,
  "recent_books_limit": 100
}
```

| Field | Default | Description |
|-------|---------|-------------|
| `columns` | `0` | `0` = auto, `1` = always single, `2` = always dual |
| `column_threshold` | `120` | Terminal width to trigger dual-column (when `columns=0`) |
| `line_spacing` | `0` | Extra blank lines between text lines |
| `recent_books_limit` | `100` | Max books in library |

## Tech Stack

- [tview](https://github.com/rivo/tview) — Terminal UI framework
- [tcell](https://github.com/gdamore/tcell) — Terminal cell library
- [x/net/html](https://golang.org/x/net/html) — HTML parsing
- Pure Go, no CGO dependencies

## License

MIT
