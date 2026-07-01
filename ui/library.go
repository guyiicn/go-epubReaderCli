package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"epub-reader/epub"
	"epub-reader/internal/server"
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
		if a.library[idx].RemoteOnly || a.library[idx].Path == "" {
			a.showError("远端书籍需要先用 download 下载")
			return
		}
		a.openBookByPath(a.library[idx].Path)
	})

	statusText := "[Enter]打开 [a]添加 [s]同步 [d]删除 [h]帮助 [q]退出"

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

		state := "local"
		if entry.RemoteOnly {
			state = "remote"
		}
		if entry.Dirty {
			state += " dirty"
		}

		secondary := state
		if !entry.LastOpened.IsZero() {
			pct := a.loadProgressPercent(entry.Path)
			bar := progressBar(pct)
			secondary = fmt.Sprintf("%s  %s %d%%  %s",
				entry.LastOpened.Format("2006-01-02"),
				bar,
				int(pct*100),
				state,
			)
		}

		// Check if file exists
		if entry.RemoteOnly {
			mainText = "[远端] " + mainText
		} else if _, err := os.Stat(entry.Path); err != nil {
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

// library.go 中的 showAddBook 已移到 filebrowser.go

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
	if entry.RemoteOnly || entry.Path == "" {
		a.showError("远端书籍需要先用命令行 download 下载")
		return
	}
	msg := fmt.Sprintf("从书库删除 \"%s\"?", entry.Title)
	modal := tview.NewModal().
		SetText(msg).
		AddButtons([]string{"取消", "删除"}).
		SetFocus(0)
	modal.SetDoneFunc(func(buttonIdx int, _ string) {
		if buttonIdx == 1 {
			a.store.RemoveBook(entry.Path)
			a.refreshLibrary()
		}
		a.mode = ModeLibrary
		a.pages.SwitchToPage("library")
		a.tapp.SetFocus(a.libList)
	})
	a.pages.AddAndSwitchToPage("deletelib", modal, true)
	a.tapp.SetFocus(modal)
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

func (a *App) syncNow() {
	a.libTitle.SetText("[yellow]同步中...[::-]")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		auth, _ := a.store.AuthState()
		client, err := server.NewClient(auth.ServerURL, a.store)
		if err == nil {
			err = server.NewEngine(a.store, client).Sync(ctx)
		}
		a.tapp.QueueUpdateDraw(func() {
			if err != nil {
				a.showError(fmt.Sprintf("同步失败: %v", err))
				return
			}
			a.libTitle.SetText("[green]同步完成[::-]")
			a.refreshLibrary()
			go func() {
				time.Sleep(2 * time.Second)
				a.tapp.QueueUpdateDraw(func() {
					a.libTitle.SetText("[::b]epub-reader — 书库[::-]")
				})
			}()
		})
	}()
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
