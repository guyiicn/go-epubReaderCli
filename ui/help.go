package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (a *App) setupHelp() {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	tv.SetBorder(true).SetTitle(" Help (h/Esc Close) ")
	a.helpView = tv
}

func (a *App) showHelp(from Mode) {
	a.prevMode = from
	a.mode = ModeHelp
	a.buildHelpText()
	a.switchPage("help", a.helpView)
}

func (a *App) closeHelp() {
	switch a.prevMode {
	case ModeLibrary, ModeFileBrowser:
		a.mode = ModeLibrary
		a.switchPage("library", a.libList)
	case ModeReader:
		a.mode = ModeReader
		a.switchPage("reader", a.readerView)
	case ModeTOC:
		a.mode = ModeTOC
		a.switchPage("toc", a.tocList)
	case ModeBookmarks:
		a.mode = ModeBookmarks
		a.switchPage("bookmarks", a.bmList)
	case ModeInfo:
		a.mode = ModeInfo
		a.switchPage("info", a.infoView)
	case ModeAnnotationNote:
		a.mode = ModeReader
		a.switchPage("reader", a.readerView)
	default:
		a.mode = ModeReader
		a.switchPage("reader", a.readerView)
	}
}

func (a *App) buildHelpText() {
	helpText := strings.Join([]string{
		"[::b]epub-reader Shortcuts[::-]",
		"",
		"[::b]Global[::-]",
		"  h         Help (from non-input screens)",
		"  Esc       Back / Close",
		"  Ctrl+C    Force quit",
		"",
		"[::b]Library[::-]",
		"  j/Down    Next book",
		"  k/Up      Previous book",
		"  Enter     Open book; download first for remote books",
		"  a         Add book (file browser)",
		"  f         Find a book and add it to the server library",
		"  s         Sync",
		"  d         Delete book (with confirmation)",
		"  q         Quit",
		"",
		"[::b]Reader[::-]",
		"  Right/Sp/PgDn  Next page (crosses chapters)",
		"  Left/Bs/PgUp   Previous page (crosses chapters)",
		"  g         Chapter start",
		"  e         Chapter end",
		"  n         Next chapter",
		"  p         Previous chapter",
		"  t         Table of contents",
		"  b         Bookmarks",
		"  a         Add bookmark (optional note)",
		"  m         Add note/annotation at current position",
		"  i         Book info",
		"  c         Toggle single/two-column layout",
		"  /         Search current chapter",
		"  x         Search full book",
		"  .         Next search result",
		"  o/q/Esc   Back to library",
		"",
		"[::b]File Browser[::-]",
		"  j/k/Up/Down  Move",
		"  Enter        Open directory / Select file",
		"  Esc          Cancel and return to library",
		"",
		"[::b]Popups (TOC / Bookmarks / Help / Info)[::-]",
		"  j/k/Up/Down  Move",
		"  Enter        Jump / Confirm",
		"  d            Delete (bookmarks, with confirmation)",
		"  Esc          Close popup",
	}, "\n")

	a.helpView.SetText(helpText)
}

// --- Info ---

func (a *App) setupInfo() {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	tv.SetBorder(true).SetTitle(" Book Info (i/Esc Close) ")
	a.infoView = tv
}

func (a *App) showInfo() {
	if a.book == nil {
		return
	}
	a.mode = ModeInfo

	m := a.book.Meta
	var sb strings.Builder
	sb.WriteString("[::b]Book Info[::-]\n\n")
	if m.Title != "" {
		sb.WriteString(fmt.Sprintf("  Title: %s\n", m.Title))
	}
	if m.Author != "" {
		sb.WriteString(fmt.Sprintf("  Author: %s\n", m.Author))
	}
	if m.Language != "" {
		sb.WriteString(fmt.Sprintf("  Language: %s\n", m.Language))
	}
	if m.Publisher != "" {
		sb.WriteString(fmt.Sprintf("  Publisher: %s\n", m.Publisher))
	}
	if m.Date != "" {
		sb.WriteString(fmt.Sprintf("  Date: %s\n", m.Date))
	}
	if m.Description != "" {
		sb.WriteString(fmt.Sprintf("\n  Description:\n  %s\n", m.Description))
	}
	sb.WriteString(fmt.Sprintf("\n  Chapters: %d\n", len(a.book.Sections)))

	totalChars := 0
	for _, s := range a.book.Sections {
		totalChars += len(s.HTML)
	}
	sb.WriteString(fmt.Sprintf("  Approx. characters: %d\n", totalChars/2))

	sb.WriteString(fmt.Sprintf("\n  File: %s\n", a.bookPath))

	a.infoView.SetText(sb.String())
	a.switchPage("info", a.infoView)
}

func (a *App) closeInfo() {
	a.mode = ModeReader
	a.switchPage("reader", a.readerView)
}

// --- Bookmark Note Input ---

func (a *App) setupBookmarkNote() {
	input := tview.NewInputField().
		SetLabel("Note: ").
		SetFieldWidth(40)
	input.SetBorder(true).SetTitle(" Add Bookmark ")

	input.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			note := input.GetText()
			a.doAddBookmark(note)
			a.mode = ModeReader
			a.switchPage("reader", a.readerView)
		case tcell.KeyEsc:
			a.mode = ModeReader
			a.switchPage("reader", a.readerView)
		}
	})
	a.bmNoteInput = input
}

func (a *App) setupAnnotationNote() {
	input := tview.NewInputField().
		SetLabel("Note: ").
		SetFieldWidth(50)
	input.SetBorder(true).SetTitle(" Add Note at Current Position ")

	input.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			note := input.GetText()
			a.doAddAnnotation(note)
			a.mode = ModeReader
			a.switchPage("reader", a.readerView)
		case tcell.KeyEsc:
			a.mode = ModeReader
			a.switchPage("reader", a.readerView)
		}
	})
	a.annotationNoteInput = input
}

func (a *App) showAnnotationNoteInput() {
	a.mode = ModeAnnotationNote
	a.annotationNoteInput.SetText("")
	a.switchPage("annotationnote", a.annotationNoteInput)
}

func (a *App) showBookmarkNoteInput() {
	a.mode = ModeBookmarkNote
	a.bmNoteInput.SetText("")
	a.switchPage("bmnote", a.bmNoteInput)
}

// --- Search ---

func (a *App) setupSearch() {
	input := tview.NewInputField().
		SetLabel("Search: ").
		SetFieldWidth(40)
	input.SetBorder(true).SetTitle(" Search ")
	input.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			a.searchTerm = input.GetText()
			a.searchAllMode = false
			a.executeSearch()
		case tcell.KeyEsc:
			a.mode = ModeReader
			a.switchPage("reader", a.readerView)
		}
	})
	a.searchInput = input

	a.searchResults = tview.NewList().
		ShowSecondaryText(true)
	a.searchResults.
		SetMainTextColor(tcell.ColorDefault).
		SetSelectedTextColor(tcell.ColorDefault).
		SetSelectedBackgroundColor(tcell.ColorDarkCyan)
	a.searchResults.SetBorder(true).SetTitle(" Search Results ")
	a.searchResults.SetSelectedFunc(func(idx int, _ string, _ string, _ rune) {
		if idx >= len(a.searchAllResults) {
			return
		}
		r := a.searchAllResults[idx]
		a.sectionIdx = r.sectionIdx
		a.scrollPos = r.linePos
		a.cachedSection = -1
		a.mode = ModeReader
		a.switchPage("reader", a.readerView)
		a.renderCurrentSection()
	})
}

func (a *App) showSearch() {
	a.mode = ModeSearch
	a.searchInput.SetBorder(true).SetTitle(" Search Current Chapter ")
	a.searchInput.SetText("")
	a.switchPage("search", a.searchInput)
}

func (a *App) showSearchAll() {
	a.mode = ModeSearch
	a.searchInput.SetBorder(true).SetTitle(" Search Full Book ")
	a.searchInput.SetText("")
	a.switchPage("search", a.searchInput)
}

type searchResult struct {
	sectionIdx int
	linePos    int
	line       string
}

func (a *App) executeSearch() {
	if a.searchTerm == "" {
		a.mode = ModeReader
		a.switchPage("reader", a.readerView)
		return
	}

	term := strings.ToLower(a.searchTerm)

	if a.searchAllMode {
		a.executeSearchAll(term)
		return
	}

	// Current chapter search
	a.searchMatches = nil
	for i, line := range a.lines {
		if strings.Contains(strings.ToLower(line), term) {
			a.searchMatches = append(a.searchMatches, i)
		}
	}

	if len(a.searchMatches) == 0 {
		a.updateReaderStatus("Not found: " + a.searchTerm)
		a.mode = ModeReader
		a.switchPage("reader", a.readerView)
		return
	}

	a.scrollPos = a.searchMatches[0]
	a.mode = ModeReader
	a.switchPage("reader", a.readerView)
	a.updateReaderDisplay()
	a.updateReaderStatus(fmt.Sprintf("Found %d matches", len(a.searchMatches)))
}

func (a *App) executeSearchAll(term string) {
	a.searchAllResults = nil
	a.searchResults.Clear()

	for si := 0; si < len(a.book.Sections); si++ {
		section := a.book.Sections[si]
		lines := a.renderer.Render(section.HTML, a.colWidth)
		for li, line := range lines {
			if strings.Contains(strings.ToLower(line), term) {
				a.searchAllResults = append(a.searchAllResults, searchResult{
					sectionIdx: si,
					linePos:    li,
					line:       line,
				})
			}
		}
	}

	if len(a.searchAllResults) == 0 {
		a.updateReaderStatus("Not found in book: " + a.searchTerm)
		a.mode = ModeReader
		a.switchPage("reader", a.readerView)
		return
	}

	// Show results list
	for _, r := range a.searchAllResults {
		title := fmt.Sprintf("Ch%d", r.sectionIdx+1)
		if r.sectionIdx < len(a.book.Sections) {
			title = a.book.Sections[r.sectionIdx].Title
		}
		a.searchResults.AddItem(
			fmt.Sprintf("%s: %s", title, truncate(r.line, 60)),
			fmt.Sprintf("Line %d", r.linePos),
			0, nil,
		)
	}

	a.mode = ModeSearchResults
	a.searchResults.SetTitle(fmt.Sprintf(" Search Results (%d) ", len(a.searchAllResults)))
	a.switchPage("searchresults", a.searchResults)
}

func truncate(s string, maxLen int) string {
	// byte-based truncation for display
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

func (a *App) nextSearchMatch() {
	if len(a.searchMatches) == 0 {
		return
	}
	for _, pos := range a.searchMatches {
		if pos > a.scrollPos {
			a.scrollPos = pos
			a.updateReaderDisplay()
			return
		}
	}
	a.scrollPos = a.searchMatches[0]
	a.updateReaderDisplay()
	a.updateReaderStatus("Search wrapped to the start")
}

func (a *App) closeSearchResults() {
	a.mode = ModeReader
	a.switchPage("reader", a.readerView)
}
