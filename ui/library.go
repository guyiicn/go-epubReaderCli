package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"epub-reader/epub"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (a *App) setupLibrary() {
	a.libTitle = tview.NewTextView().
		SetDynamicColors(true).
		SetText("[::b]epub-reader — 书库[::-]")

	a.libList = tview.NewList().
		ShowSecondaryText(true)
	a.libList.
		SetMainTextColor(tcell.ColorDefault).
		SetSelectedTextColor(tcell.ColorDefault).
		SetSelectedBackgroundColor(tcell.ColorDarkCyan).
		SetBorder(true)

	a.libList.SetSelectedFunc(func(idx int, _, _ string, _ rune) {
		if idx >= len(a.library) {
			return
		}
		a.openBookByPath(a.library[idx].Path)
	})

	statusText := "[Enter]打开 [a]添加 [d]删除 [h]帮助 [q]退出"

	statusBar := tview.NewTextView().
		SetDynamicColors(true).
		SetText(statusText).
		SetTextAlign(tview.AlignCenter)

	a.libFlex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.libTitle, 1, 0, false).
		AddItem(a.libList, 0, 1, true).
		AddItem(statusBar, 1, 0, false)
}

func (a *App) refreshLibrary() {
	a.library = a.store.Library()
	a.libList.Clear()

	// Sort by LastOpened (most recent first)
	for i := 0; i < len(a.library); i++ {
		for j := i + 1; j < len(a.library); j++ {
			if a.library[i].LastOpened.Before(a.library[j].LastOpened) {
				a.library[i], a.library[j] = a.library[j], a.library[i]
			}
		}
	}

	if len(a.library) == 0 {
		a.libList.AddItem("按 [a] 添加第一本书", "", 0, nil)
		return
	}

	for _, entry := range a.library {
		mainText := entry.Title
		if entry.Author != "" {
			mainText = fmt.Sprintf("%-40s %s", truncatePad(entry.Title, 38), entry.Author)
		}

		secondary := ""
		if !entry.LastOpened.IsZero() {
			pct := a.loadProgressPercent(entry.Path)
			bar := progressBar(pct)
			secondary = fmt.Sprintf("%s  %s %d%%",
				entry.LastOpened.Format("2006-01-02"),
				bar,
				int(pct*100),
			)
		}

		// Check if file exists
		if _, err := os.Stat(entry.Path); err != nil {
			mainText = "[文件缺失] " + mainText
		}

		a.libList.AddItem(mainText, secondary, 0, nil)
	}
}

func (a *App) loadProgressPercent(path string) float64 {
	p, err := a.store.LoadProgress(path)
	if err != nil || p == nil {
		return 0
	}
	return p.Percent
}

func (a *App) showAddBook() {
	a.mode = ModeAddBook
	a.addInput.SetText("")
	a.switchPage("addbook", a.addInput)
}

func (a *App) addBookByPath(path string) {
	// Expand ~
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		path = home + path[1:]
	}

	// Check file exists
	if _, err := os.Stat(path); err != nil {
		a.showError(fmt.Sprintf("文件不存在: %s", path))
		return
	}

	// Parse to get metadata
	book, err := epub.Load(path)
	if err != nil {
		a.showError(fmt.Sprintf("无法打开: %v", err))
		return
	}

	title := book.Title
	if title == "" {
		title = filepath.Base(path)
	}

	if err := a.store.AddBook(path, title, book.Author); err != nil {
		a.showError(fmt.Sprintf("添加失败: %v", err))
		return
	}

	a.refreshLibrary()
	a.mode = ModeLibrary
	a.switchPage("library", a.libList)
}

func (a *App) removeBook() {
	idx := a.libList.GetCurrentItem()
	if idx >= len(a.library) {
		return
	}
	entry := a.library[idx]
	a.store.RemoveBook(entry.Path)
	a.refreshLibrary()
}

func (a *App) showError(msg string) {
	a.libTitle.SetText(fmt.Sprintf("[red]%s[::-]", msg))
	go func() {
		time.Sleep(3 * time.Second)
		a.tapp.QueueUpdateDraw(func() {
			a.libTitle.SetText("[::b]epub-reader — 书库[::-]")
		})
	}()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

func truncatePad(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s + strings.Repeat(" ", maxLen-len(s))
	}
	return s[:maxLen-1] + "…"
}

func progressBar(pct float64) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * 10)
	return strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)
}
