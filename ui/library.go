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

const (
	libraryTitleText  = "[::b]epub-reader - Library[::-]"
	libraryStatusHelp = "[Enter] Open  [a] Add  [f] Find  [s] Sync  [d] Delete  [h] Help  [q] Quit"
)

func (a *App) setupLibrary() {
	a.libTitle = tview.NewTextView().
		SetDynamicColors(true).
		SetText(libraryTitleText)

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
		a.openLibraryEntry(a.library[idx])
	})

	a.libStatus = tview.NewTextView().
		SetDynamicColors(true).
		SetText(libraryStatusHelp).
		SetTextAlign(tview.AlignCenter)

	a.libFlex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.libTitle, 1, 0, false).
		AddItem(a.libList, 0, 1, true).
		AddItem(a.libStatus, 1, 0, false)
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
		a.libList.AddItem("No books. Press [a] to add one.", "", 0, nil)
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
			mainText = "[Remote] " + mainText
		} else if _, err := os.Stat(entry.Path); err != nil {
			mainText = "[Missing file] " + mainText
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

// showAddBook lives in filebrowser.go.

func (a *App) addBookByPath(path string) {
	// Expand ~
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		path = home + path[1:]
	}

	// Check file exists
	if _, err := os.Stat(path); err != nil {
		a.showError(fmt.Sprintf("File not found: %s", path))
		return
	}

	// Parse to get metadata
	book, err := epub.Load(path)
	if err != nil {
		a.showError(fmt.Sprintf("Open failed: %v", err))
		return
	}

	title := book.Title
	if title == "" {
		title = filepath.Base(path)
	}

	if err := a.store.AddBook(path, title, book.Author); err != nil {
		a.showError(fmt.Sprintf("Add failed: %v", err))
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
		a.showError("Remote books can be opened with [Enter] to download first.")
		return
	}
	msg := fmt.Sprintf("Remove \"%s\" from the library?", entry.Title)
	modal := tview.NewModal().
		SetText(msg).
		AddButtons([]string{"Cancel", "Delete"}).
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

func (a *App) openLibraryEntry(entry epub.LibraryEntry) {
	if !entry.RemoteOnly && entry.Path != "" {
		a.openBookByPath(entry.Path)
		return
	}
	serverID := entry.ServerID
	if serverID == "" {
		serverID = entry.ID
	}
	if serverID == "" {
		a.showError("Remote book is missing a server id.")
		return
	}
	a.setLibraryStatus("Downloading remote book: " + entry.Title)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		auth, _ := a.store.AuthState()
		client, err := server.NewClient(auth.ServerURL, a.store)
		var dst string
		var n int64
		if err == nil {
			dst = a.store.BookStoragePath(entry.ContentHash, serverID, entry.Format)
			n, err = client.DownloadBookFile(ctx, serverID, dst)
			if err == nil {
				err = a.store.MarkDownloaded(serverID, dst, entry.ContentHash, n)
			}
		}
		a.tapp.QueueUpdateDraw(func() {
			if err != nil {
				a.showError(fmt.Sprintf("Download failed: %v", err))
				return
			}
			a.setLibraryStatus(fmt.Sprintf("Download complete: %d bytes. Opening...", n))
			a.refreshLibrary()
			a.openBookByPath(dst)
		})
	}()
}

func (a *App) showError(msg string) {
	a.libTitle.SetText(fmt.Sprintf("[red]%s[::-]", msg))
	if a.libStatus != nil {
		a.libStatus.SetText(fmt.Sprintf("[red]%s[::-]  |  %s", msg, libraryStatusHelp))
	}
	go func() {
		time.Sleep(3 * time.Second)
		a.tapp.QueueUpdateDraw(func() {
			a.libTitle.SetText(libraryTitleText)
			if a.libStatus != nil {
				a.libStatus.SetText(libraryStatusHelp)
			}
		})
	}()
}

func (a *App) setLibraryStatus(msg string) {
	a.libTitle.SetText(fmt.Sprintf("[yellow]%s[::-]", msg))
	if a.libStatus != nil {
		a.libStatus.SetText(fmt.Sprintf("[yellow]%s[::-]  |  %s", msg, libraryStatusHelp))
	}
}

func (a *App) syncNow() {
	a.syncNowWithStatus(func(msg string) {
		a.tapp.QueueUpdateDraw(func() {
			a.setLibraryStatus(msg)
		})
	})
}

func (a *App) syncNowFromUI() {
	a.syncNowWithStatus(a.setLibraryStatus)
}

func (a *App) syncNowWithStatus(setStatus func(string)) {
	a.syncMu.Lock()
	if a.syncRunning {
		a.syncPending = true
		a.syncMu.Unlock()
		setStatus("Sync already running; queued another sync.")
		return
	}
	a.syncRunning = true
	a.syncMu.Unlock()

	setStatus("Syncing with server...")

	go func() {
		for {
			err := a.performSyncOnce()
			a.tapp.QueueUpdateDraw(func() {
				if err != nil {
					a.showError(fmt.Sprintf("Sync failed: %v", err))
				} else {
					a.libTitle.SetText("[green]Sync complete[::-]")
					if a.libStatus != nil {
						a.libStatus.SetText("[green]Sync complete. Library metadata refreshed.[::-]  |  " + libraryStatusHelp)
					}
					a.refreshLibrary()
				}
			})

			a.syncMu.Lock()
			if a.syncPending {
				a.syncPending = false
				a.syncMu.Unlock()
				continue
			}
			a.syncRunning = false
			a.syncMu.Unlock()
			return
		}
	}()
}

func (a *App) performSyncOnce() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	auth, _ := a.store.AuthState()
	client, err := server.NewClient(auth.ServerURL, a.store)
	if err == nil {
		err = server.NewEngine(a.store, client).Sync(ctx)
	}
	if err == nil {
		go func() {
			time.Sleep(2 * time.Second)
			a.tapp.QueueUpdateDraw(func() {
				if a.mode == ModeLibrary {
					a.libTitle.SetText(libraryTitleText)
				}
			})
		}()
	}
	return err
}

func (a *App) startRealtimeSync() {
	auth, _ := a.store.AuthState()
	if auth.ServerURL == "" || auth.AccessToken == "" || auth.DeviceID == "" {
		return
	}
	go func() {
		for {
			ctx, cancel := context.WithCancel(context.Background())
			ch := make(chan server.InvalidateMessage, 8)
			ws, err := server.NewWSClient(auth.ServerURL, a.store)
			if err == nil {
				go func() {
					for msg := range ch {
						if msg.Table != "" {
							a.tapp.QueueUpdateDraw(func() {
								a.setLibraryStatus("Remote update received; syncing...")
							})
							a.syncNow()
						}
					}
				}()
				err = ws.Listen(ctx, ch)
			}
			cancel()
			close(ch)
			time.Sleep(10 * time.Second)
			auth, _ = a.store.AuthState()
			if auth.ServerURL == "" || auth.AccessToken == "" {
				return
			}
		}
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
