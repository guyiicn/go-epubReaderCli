package ui

import (
	"fmt"
	"strings"
	"time"

	"epub-reader/epub"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (a *App) setupBookmarks() {
	a.bmList = tview.NewList().ShowSecondaryText(true)
	a.bmList.
		SetMainTextColor(tcell.ColorDefault).
		SetSelectedTextColor(tcell.ColorDefault).
		SetSelectedBackgroundColor(tcell.ColorDarkCyan)
	a.bmList.SetBorder(true).SetTitle(" Bookmarks ")

	a.bmList.SetSelectedFunc(func(idx int, _ string, _ string, _ rune) {
		bms, _ := a.store.LoadBookmarks(a.bookPath)
		if idx >= len(bms) {
			return
		}
		bm := bms[idx]
		// A bookmark written by another client carries only a locator; its
		// section/line columns are 0 until resolved against this book.
		t := resolveLocator(a.book, bm.Locator, bm.SectionIndex, bm.LinePos, bm.Percent)
		a.sectionIdx = t.SectionIdx
		a.scrollPos = t.LinePos
		a.pendingLineFrac = t.LineFrac
		a.cachedSection = -1
		a.renderCurrentSection()
		a.closeBookmarks()
	})
}

func (a *App) showBookmarks() {
	if a.book == nil {
		return
	}
	a.mode = ModeBookmarks
	a.buildBookmarksList()
	a.switchPage("bookmarks", a.bmList)
}

func (a *App) closeBookmarks() {
	a.mode = ModeReader
	a.switchPage("reader", a.readerView)
}

func (a *App) buildBookmarksList() {
	a.bmList.Clear()
	bms, _ := a.store.LoadBookmarks(a.bookPath)
	if len(bms) == 0 {
		a.bmList.AddItem("(No bookmarks)", "Press [a] to add a bookmark", 0, nil)
		return
	}
	for _, bm := range bms {
		title := "(Unknown chapter)"
		sectionIdx := resolveLocator(a.book, bm.Locator, bm.SectionIndex, bm.LinePos, bm.Percent).SectionIdx
		if sectionIdx >= 0 && sectionIdx < len(a.book.Sections) {
			title = a.book.Sections[sectionIdx].Title
		}
		note := bm.Note
		if note == "" {
			note = fmt.Sprintf("Line %d", bm.LinePos)
		}
		timeStr := bm.CreatedAt.Format("2006-01-02 15:04")
		a.bmList.AddItem(
			fmt.Sprintf("%s — %s", title, note),
			timeStr,
			0, nil,
		)
	}
}

// addBookmark shows a note input dialog, then saves.
func (a *App) addBookmark() {
	if a.book == nil {
		return
	}
	a.showBookmarkNoteInput()
}

// doAddBookmark actually saves the bookmark (called after note input).
func (a *App) doAddBookmark(note string) {
	if a.book == nil {
		return
	}
	bms, _ := a.store.LoadBookmarks(a.bookPath)
	href, title, progression, percent := a.currentPos()
	bms = append(bms, epub.Bookmark{
		ID:           fmt.Sprintf("bm-%d", time.Now().UnixNano()),
		SectionIndex: a.sectionIdx,
		LinePos:      a.scrollPos,
		Href:         href,
		Title:        title,
		Progression:  progression,
		Percent:      percent,
		Note:         note,
		CreatedAt:    time.Now(),
	})
	a.store.SaveBookmarks(a.bookPath, bms)
	a.updateReaderStatus("Bookmark added")
}

func (a *App) deleteBookmark() {
	bms, _ := a.store.LoadBookmarks(a.bookPath)
	idx := a.bmList.GetCurrentItem()
	if idx >= len(bms) {
		return
	}
	// Confirm deletion — use a modal
	a.showDeleteConfirm(fmt.Sprintf("Delete bookmark \"%s\"?", bms[idx].Note), func() {
		a.store.DeleteBookmark(a.bookPath, bms[idx].ID)
		a.buildBookmarksList()
	})
}

func (a *App) showDeleteConfirm(msg string, onConfirm func()) {
	modal := tview.NewModal().
		SetText(msg).
		AddButtons([]string{"Cancel", "Delete"}).
		SetFocus(0)
	modal.SetDoneFunc(func(idx int, _ string) {
		if idx == 1 {
			onConfirm()
		}
		a.mode = ModeBookmarks
		a.switchPage("bookmarks", a.bmList)
	})
	a.pages.AddAndSwitchToPage("deleteconfirm", modal, true)
	a.tapp.SetFocus(modal)
	a.mode = ModeBookmarks // keep mode as bookmarks
}

func (a *App) updateReaderStatus(msg string) {
	status := fmt.Sprintf(" %s ", msg)
	a.statusView.SetText(status)
	go func() {
		time.Sleep(2 * time.Second)
		a.tapp.QueueUpdateDraw(func() {
			if a.mode == ModeReader {
				a.updateReaderDisplay()
			}
		})
	}()
	_ = strings.TrimSpace(status)
}

func (a *App) addAnnotation() {
	if a.book == nil {
		return
	}
	a.showAnnotationNoteInput()
}

func (a *App) doAddAnnotation(note string) {
	if a.book == nil {
		return
	}
	selected := "position note"
	if a.scrollPos >= 0 && a.scrollPos < len(a.lines) {
		selected = strings.TrimSpace(a.lines[a.scrollPos])
	}
	if selected == "" {
		selected = "position note"
	}
	if len(selected) > 500 {
		selected = selected[:500]
	}
	href, title, progression, percent := a.currentPos()
	if err := a.store.AddAnnotation(a.bookPath, epub.Annotation{
		SelectedText: selected,
		Note:         note,
		SectionIndex: a.sectionIdx,
		LinePos:      a.scrollPos,
		Href:         href,
		Title:        title,
		Progression:  progression,
		Percent:      percent,
	}); err != nil {
		a.updateReaderStatus(fmt.Sprintf("Annotation failed: %v", err))
		return
	}
	a.updateReaderStatus("Note added at current position")
}
