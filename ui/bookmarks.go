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
	a.bmList.SetBorder(true).SetTitle(" 书签 ")

	a.bmList.SetSelectedFunc(func(idx int, _ string, _ string, _ rune) {
		bms, _ := a.store.LoadBookmarks(a.bookPath)
		if idx >= len(bms) {
			return
		}
		bm := bms[idx]
		a.sectionIdx = bm.SectionIndex
		a.scrollPos = bm.LinePos
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
		a.bmList.AddItem("(无书签)", "按 [a] 添加书签", 0, nil)
		return
	}
	for _, bm := range bms {
		title := "(未知章节)"
		if bm.SectionIndex >= 0 && bm.SectionIndex < len(a.book.Sections) {
			title = a.book.Sections[bm.SectionIndex].Title
		}
		note := bm.Note
		if note == "" {
			note = fmt.Sprintf("行 %d", bm.LinePos)
		}
		timeStr := bm.CreatedAt.Format("2006-01-02 15:04")
		a.bmList.AddItem(
			fmt.Sprintf("%s — %s", title, note),
			timeStr,
			0, nil,
		)
	}
}

func (a *App) addBookmark() {
	if a.book == nil {
		return
	}
	bms, _ := a.store.LoadBookmarks(a.bookPath)
	bms = append(bms, epub.Bookmark{
		ID:           fmt.Sprintf("bm-%d", time.Now().UnixNano()),
		SectionIndex: a.sectionIdx,
		LinePos:      a.scrollPos,
		Note:         "",
		CreatedAt:    time.Now(),
	})
	a.store.SaveBookmarks(a.bookPath, bms)
	a.updateReaderStatus("书签已添加")
}

func (a *App) deleteBookmark() {
	bms, _ := a.store.LoadBookmarks(a.bookPath)
	idx := a.bmList.GetCurrentItem()
	if idx >= len(bms) {
		return
	}
	bms = append(bms[:idx], bms[idx+1:]...)
	a.store.SaveBookmarks(a.bookPath, bms)
	a.buildBookmarksList()
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
