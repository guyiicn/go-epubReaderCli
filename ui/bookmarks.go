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
	bms = append(bms, epub.Bookmark{
		ID:           fmt.Sprintf("bm-%d", time.Now().UnixNano()),
		SectionIndex: a.sectionIdx,
		LinePos:      a.scrollPos,
		Note:         note,
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
	// Confirm deletion — use a modal
	a.showDeleteConfirm(fmt.Sprintf("删除书签 \"%s\"?", bms[idx].Note), func() {
		a.store.DeleteBookmark(a.bookPath, bms[idx].ID)
		a.buildBookmarksList()
	})
}

func (a *App) showDeleteConfirm(msg string, onConfirm func()) {
	modal := tview.NewModal().
		SetText(msg).
		AddButtons([]string{"取消", "删除"}).
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
	if err := a.store.AddAnnotation(a.bookPath, selected, note, a.sectionIdx, a.scrollPos); err != nil {
		a.updateReaderStatus(fmt.Sprintf("标注失败: %v", err))
		return
	}
	a.updateReaderStatus("当前位置笔记已添加")
}
