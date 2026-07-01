package ui

import (
	"context"
	"fmt"
	"time"

	"epub-reader/internal/server"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (a *App) setupFindBook() {
	a.findInput = tview.NewInputField().
		SetLabel("找书: ").
		SetFieldWidth(50)
	a.findInput.SetBorder(true).SetTitle(" 找书 ")
	a.findInput.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			query := a.findInput.GetText()
			a.executeFindBook(query)
		case tcell.KeyEsc:
			a.mode = ModeLibrary
			a.switchPage("library", a.libList)
		}
	})

	a.findResults = tview.NewList().ShowSecondaryText(true)
	a.findResults.
		SetMainTextColor(tcell.ColorDefault).
		SetSelectedTextColor(tcell.ColorDefault).
		SetSelectedBackgroundColor(tcell.ColorDarkCyan)
	a.findResults.SetBorder(true).SetTitle(" 找书结果 ")
	a.findResults.SetSelectedFunc(func(idx int, _ string, _ string, _ rune) {
		if idx < 0 || idx >= len(a.findBooks) {
			return
		}
		a.downloadFoundBook(a.findBooks[idx])
	})
}

func (a *App) showFindBook() {
	a.mode = ModeFindInput
	a.findInput.SetText("")
	a.switchPage("find", a.findInput)
}

func (a *App) executeFindBook(query string) {
	if query == "" {
		a.mode = ModeLibrary
		a.switchPage("library", a.libList)
		return
	}
	a.libTitle.SetText("[yellow]搜索中...[::-]")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		auth, _ := a.store.AuthState()
		client, err := server.NewClient(auth.ServerURL, a.store)
		var result server.SearchResult
		if err == nil {
			result, err = client.Search(ctx, query, 1, "")
		}
		a.tapp.QueueUpdateDraw(func() {
			if err != nil {
				a.showError(fmt.Sprintf("找书失败: %v", err))
				return
			}
			a.findBooks = result.Books
			a.findResults.Clear()
			if len(result.Books) == 0 {
				a.findResults.AddItem("(无结果)", "Esc 返回", 0, nil)
			}
			for _, b := range result.Books {
				secondary := fmt.Sprintf("%s  %s  %s", b.Author, b.Format, b.Size)
				a.findResults.AddItem(b.Title, secondary, 0, nil)
			}
			a.mode = ModeFindResults
			a.switchPage("findresults", a.findResults)
		})
	}()
}

func (a *App) downloadFoundBook(book server.SearchBook) {
	a.findResults.SetTitle(" 加入书库中... ")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		auth, _ := a.store.AuthState()
		client, err := server.NewClient(auth.ServerURL, a.store)
		var syncBook server.SyncBook
		if err == nil {
			syncBook, err = client.SearchDownload(ctx, server.SearchDownloadRequest{
				BookCommand: book.BookCommand,
				Title:       book.Title,
				Author:      book.Author,
			})
		}
		if err == nil {
			err = a.store.UpsertRemoteBook(syncBook.ID, syncBook.Title, syncBook.Author, syncBook.Format, syncBook.ContentHash, syncBook.TotalChapters, 0)
		}
		a.tapp.QueueUpdateDraw(func() {
			if err != nil {
				a.showError(fmt.Sprintf("加入书库失败: %v", err))
				return
			}
			a.refreshLibrary()
			a.mode = ModeLibrary
			a.switchPage("library", a.libList)
			a.libTitle.SetText("[green]已加入书库，可 Enter 下载打开[::-]")
		})
	}()
}

func (a *App) handleFindResultsKey(ev *tcell.EventKey) *tcell.EventKey {
	if ev.Key() == tcell.KeyEsc || ev.Rune() == 'q' {
		a.mode = ModeLibrary
		a.switchPage("library", a.libList)
		return nil
	}
	return ev
}
