package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"epub-reader/epub"

	"github.com/rivo/tview"
)

func (a *App) setupReader() {
	a.readerTitle = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)

	a.readerView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false).
		SetWrap(false).
		SetChangedFunc(func() { a.tapp.Draw() })

	a.statusView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	a.readerFlex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.readerTitle, 1, 0, false).
		AddItem(a.readerView, 0, 1, true).
		AddItem(a.statusView, 1, 0, false)
}

func (a *App) openBookByPath(path string) {
	if path == "" {
		a.showError("远端书籍需要先下载")
		return
	}
	book, err := epub.Load(path)
	if err != nil {
		a.showError(fmt.Sprintf("无法打开: %v", err))
		return
	}

	a.book = book
	a.bookPath = path
	a.mode = ModeReader

	a.store.UpdateLastOpened(path)

	// Join book into library
	title := book.Title
	if title == "" {
		title = filepath.Base(path)
	}
	a.store.AddBook(path, title, book.Author)

	// Restore progress
	a.sectionIdx = 0
	a.scrollPos = 0
	if p, err := a.store.LoadProgress(path); err == nil && p != nil {
		if p.SectionIndex >= 0 && p.SectionIndex < len(book.Sections) {
			a.sectionIdx = p.SectionIndex
			a.scrollPos = p.LinePos
		}
	}

	a.cachedSection = -1
	a.cachedWidth = -1

	termWidth, _ := a.getScreenSize()
	a.columns = a.resolveColumns(termWidth)

	a.renderCurrentSection()
	a.switchPage("reader", a.readerView)
}

func (a *App) getScreenSize() (int, int) {
	if a.screen == nil {
		return 80, 24
	}
	w, h := a.screen.Size()
	if w < 10 {
		w = 10
	}
	if h < 4 {
		h = 4
	}
	return w, h
}

// pageSize: how many source lines one page consumes.
func (a *App) pageSize() int {
	if a.columns == 2 {
		return a.pageHeight * 2
	}
	return a.pageHeight
}

// totalPages returns the number of pages in current section.
func (a *App) totalPages() int {
	ps := a.pageSize()
	totalLines := len(a.lines)
	if ps <= 0 || totalLines == 0 {
		return 1
	}
	return (totalLines + ps - 1) / ps
}

// currentPage returns 0-based page index.
func (a *App) currentPage() int {
	ps := a.pageSize()
	if ps <= 0 {
		return 0
	}
	return a.scrollPos / ps
}

func (a *App) renderCurrentSection() {
	if a.book == nil || a.sectionIdx >= len(a.book.Sections) {
		return
	}

	termWidth, termHeight := a.getScreenSize()
	a.pageHeight = termHeight - 4
	if a.pageHeight < 1 {
		a.pageHeight = 1
	}

	gap := 0
	if a.columns == 2 {
		gap = 2
	}
	newColWidth := (termWidth - gap) / a.columns
	if newColWidth < 10 {
		newColWidth = 10
	}

	if newColWidth != a.colWidth {
		a.cachedSection = -1
		a.cachedWidth = -1
	}
	a.colWidth = newColWidth

	if a.cachedSection != a.sectionIdx || a.cachedWidth != a.colWidth {
		section := a.book.Sections[a.sectionIdx]
		a.lines = a.renderer.Render(section.HTML, a.colWidth)
		a.cachedSection = a.sectionIdx
		a.cachedWidth = a.colWidth
	}

	totalLines := len(a.lines)
	if totalLines == 0 {
		a.scrollPos = 0
	} else {
		// Align to page boundary
		ps := a.pageSize()
		a.scrollPos = (a.scrollPos / ps) * ps
		if a.scrollPos >= totalLines {
			a.scrollPos = ((totalLines - 1) / ps) * ps
		}
		if a.scrollPos < 0 {
			a.scrollPos = 0
		}
	}

	a.updateReaderDisplay()
	a.saveProgress()
}

func (a *App) updateReaderDisplay() {
	section := a.book.Sections[a.sectionIdx]
	title := a.book.Title
	if section.Title != "" {
		title = fmt.Sprintf("%s — %s", a.book.Title, section.Title)
	}
	a.readerTitle.SetText(fmt.Sprintf("[::b]%s[::-]", title))

	totalLines := len(a.lines)

	if totalLines == 0 {
		a.readerView.SetText("(此章节为空)")
		a.statusView.SetText(fmt.Sprintf(" Ch%d/%d %s │ 空 │ %s ",
			a.sectionIdx+1, len(a.book.Sections), section.Title,
			time.Now().Format("15:04")))
		return
	}

	ps := a.pageSize()
	start := a.scrollPos
	if start < 0 {
		start = 0
	}
	end := start + ps
	if end > totalLines {
		end = totalLines
	}

	var displayText string
	if start < end {
		if a.columns == 2 {
			displayText = a.buildTwoColumnDisplay(start, end)
		} else {
			displayText = strings.Join(a.lines[start:end], "\n")
		}
	} else if totalLines > 0 {
		displayText = a.lines[0]
	}

	visibleLines := strings.Count(displayText, "\n") + 1
	for i := visibleLines; i < a.pageHeight; i++ {
		displayText += "\n"
	}

	a.readerView.SetText(displayText)

	// Status bar
	pct := 0.0
	if totalLines > 0 {
		pct = float64(a.scrollPos) / float64(totalLines) * 100
	}
	bar := progressBar(pct / 100)
	now := time.Now().Format("15:04")

	colInfo := "单栏"
	if a.columns == 2 {
		colInfo = "双栏"
	}

	tp := a.totalPages()
	cp := a.currentPage() + 1

	status := fmt.Sprintf(" Ch%d/%d %s │ %d/%d页 │ %s %d%% │ %s │ %s ",
		a.sectionIdx+1, len(a.book.Sections),
		section.Title,
		cp, tp,
		bar, int(pct),
		colInfo,
		now,
	)
	a.statusView.SetText(status)
}

func (a *App) buildTwoColumnDisplay(start, end int) string {
	pageLines := a.lines[start:end]
	if len(pageLines) == 0 {
		return ""
	}

	leftEnd := a.pageHeight
	if leftEnd > len(pageLines) {
		leftEnd = len(pageLines)
	}

	var sb strings.Builder
	for i := 0; i < a.pageHeight; i++ {
		left := ""
		if i < leftEnd {
			left = padRight(pageLines[i], a.colWidth)
		}
		right := ""
		if leftEnd+i < len(pageLines) {
			right = pageLines[leftEnd+i]
		}
		sb.WriteString(left)
		sb.WriteString("  ")
		sb.WriteString(right)
		if i < a.pageHeight-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// pageForward goes to next page, crossing chapter boundaries.
func (a *App) pageForward() {
	totalLines := len(a.lines)
	ps := a.pageSize()

	newPos := a.scrollPos + ps
	if newPos < totalLines {
		a.scrollPos = newPos
		a.updateReaderDisplay()
		a.saveProgress()
		return
	}

	// End of chapter, try next
	if a.sectionIdx+1 < len(a.book.Sections) {
		a.sectionIdx++
		a.scrollPos = 0
		a.cachedSection = -1
		a.renderCurrentSection()
		return
	}

	// Last page of last chapter, do nothing
}

// pageBackward goes to previous page, crossing chapter boundaries.
func (a *App) pageBackward() {
	if a.scrollPos > 0 {
		ps := a.pageSize()
		a.scrollPos -= ps
		if a.scrollPos < 0 {
			a.scrollPos = 0
		}
		a.updateReaderDisplay()
		a.saveProgress()
		return
	}

	// Start of chapter, try previous chapter → go to its last page
	if a.sectionIdx > 0 {
		a.sectionIdx--
		a.cachedSection = -1
		// Need to render first to know total lines
		section := a.book.Sections[a.sectionIdx]
		a.lines = a.renderer.Render(section.HTML, a.colWidth)
		a.cachedSection = a.sectionIdx
		a.cachedWidth = a.colWidth

		ps := a.pageSize()
		totalLines := len(a.lines)
		if totalLines == 0 {
			a.scrollPos = 0
		} else {
			a.scrollPos = ((totalLines - 1) / ps) * ps
		}
		a.updateReaderDisplay()
		a.saveProgress()
		return
	}

	// First page of first chapter, do nothing
}

func (a *App) nextSection() {
	if a.sectionIdx+1 < len(a.book.Sections) {
		a.sectionIdx++
		a.scrollPos = 0
		a.cachedSection = -1
		a.renderCurrentSection()
	}
}

func (a *App) prevSection() {
	if a.sectionIdx > 0 {
		a.sectionIdx--
		a.scrollPos = 0
		a.cachedSection = -1
		a.renderCurrentSection()
	}
}

func (a *App) goToSection(idx int) {
	if idx >= 0 && idx < len(a.book.Sections) {
		a.sectionIdx = idx
		a.scrollPos = 0
		a.cachedSection = -1
		a.renderCurrentSection()
	}
}

func (a *App) toggleColumns() {
	if a.columns == 1 {
		a.columns = 2
	} else {
		a.columns = 1
	}
	a.cachedSection = -1
	a.cachedWidth = -1
	a.renderCurrentSection()
}

func (a *App) saveProgress() {
	if a.book == nil {
		return
	}
	total := len(a.lines)
	pct := 0.0
	if total > 0 {
		pct = float64(a.scrollPos) / float64(total)
	}
	overallPct := (float64(a.sectionIdx) + pct) / float64(len(a.book.Sections))
	a.store.SaveProgress(a.bookPath, epub.Progress{
		SectionIndex: a.sectionIdx,
		LinePos:      a.scrollPos,
		Percent:      overallPct,
	})
}

func (a *App) backToLibrary() {
	a.saveProgress()
	a.book = nil
	a.bookPath = ""
	a.mode = ModeLibrary
	a.refreshLibrary()
	a.switchPage("library", a.libList)
}

func padRight(s string, width int) string {
	w := 0
	for _, r := range s {
		if r >= 0x1100 && (r <= 0x115F || r == 0x2329 || r == 0x232A ||
			(r >= 0x2E80 && r <= 0xA4CF && r != 0x303F) ||
			(r >= 0xAC00 && r <= 0xD7A3) ||
			(r >= 0xF900 && r <= 0xFAFF) ||
			(r >= 0xFE10 && r <= 0xFE19) ||
			(r >= 0xFE30 && r <= 0xFE6F) ||
			(r >= 0xFF01 && r <= 0xFF60) ||
			(r >= 0xFFE0 && r <= 0xFFE6) ||
			(r >= 0x20000 && r <= 0x2FFFD) ||
			(r >= 0x30000 && r <= 0x3FFFD)) {
			w += 2
		} else {
			w++
		}
	}
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}
