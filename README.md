# go-epubReaderCli

A terminal EPUB reader written in Go. Works in any terminal, supports CJK text, dual-column layout, bookmarks, search, and more.

## Features

- **EPUB 2 & 3 support** — NCX and nav.xhtml parsing
- **CJK friendly** — proper Chinese/Japanese/Korean character width and line breaking
- **Dual-column layout** — auto-enable on wide terminals (≥120 cols), toggle with `c`
- **Library management** — add/remove books, reading progress with percentage bar
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
# Open library
epub-reader

# Open a specific book directly
epub-reader ~/books/three-body.epub
```

Data is stored in `~/.config/epub-reader/`:

```
~/.config/epub-reader/
├── config.json           # user preferences
├── library.json          # book list
├── progress/<hash>.json  # reading progress per book
└── bookmarks/<hash>.json # bookmarks per book
```

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
