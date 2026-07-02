# Repository Guidelines

## Project Structure & Module Organization

This Go CLI/TUI EPUB reader module is named `epub-reader`. The entry point is `main.go`. Core packages are organized by responsibility:

- `epub/`: EPUB, nav, and text parsing.
- `render/`: terminal text rendering and layout helpers.
- `store/`: SQLite-backed library, progress, bookmarks, auth, and migration logic.
- `ui/`: tview/tcell screens for the library, reader, search, file browser, help, TOC, and bookmarks.
- `internal/server/`: sync API client, WebSocket invalidation, and related types.
- `samples/`: sample EPUB files for manual testing.

Tests live beside the package they cover, for example `store/store_test.go`.

## Build, Test, and Development Commands

- `go build -o epub-reader .`: build the local CLI binary.
- `go run .`: launch the library TUI without installing.
- `go run . paths`: print XDG config, data, cache, DB, and book paths.
- `go test ./...`: run all package tests.
- `go test ./store -run TestStoreAddProgressAndBookmark`: run a focused store test.
- `go fmt ./...`: format Go source before committing.

Optional document conversion uses external tools such as Calibre `ebook-convert` or `pdftotext`; the Go build has no CGO requirement.

## Coding Style & Naming Conventions

Use standard Go formatting: tabs for indentation, `gofmt`/`go fmt` for layout, and idiomatic short names in narrow scopes. Keep exported identifiers clear and documented when they form package APIs. File names should be lowercase and descriptive, such as `filebrowser.go` or `store_test.go`.

Prefer small package-level helpers that match existing package boundaries. Keep UI behavior in `ui/`, persistence in `store/`, parsing in `epub/`, and sync concerns in `internal/server/`.

## Testing Guidelines

Use Go's built-in `testing` package. Name tests `Test<Behavior>` and isolate fixtures with `t.TempDir()` where filesystem or database state is involved. Add tests beside the changed package, especially for storage migrations, parsing edge cases, progress/bookmark behavior, and sync serialization logic. Run `go test ./...` before opening a PR.

## Commit & Pull Request Guidelines

Recent history uses concise Conventional Commit-style subjects, for example `feat: add tui find book flow`, `fix: serialize sync and avoid duplicate local books`, and `docs: add sample EPUB files for testing`. Follow that pattern with a lowercase type like `feat`, `fix`, `docs`, or `refactor`.

Pull requests should include a short description, testing performed, and any user-visible behavior changes. For TUI changes, include screenshots or a brief terminal reproduction path when helpful. Link related issues when applicable and call out configuration or data migration impact.
