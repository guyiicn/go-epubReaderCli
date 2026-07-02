package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// --- File Browser ---

func (a *App) setupAddBook() {
	// Keep the old InputField for manual path entry
	a.addInput = tview.NewInputField().
		SetLabel("Book path: ").
		SetFieldWidth(50)
	a.addInput.SetBorder(true).SetTitle(" Add Book ")
	a.addInput.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			path := a.addInput.GetText()
			a.mode = ModeLibrary
			a.addBookByPath(path)
		case tcell.KeyEsc:
			a.mode = ModeLibrary
			a.switchPage("library", a.libList)
		}
	})

	// File browser list
	a.fileList = tview.NewList().ShowSecondaryText(false)
	a.fileList.
		SetMainTextColor(tcell.ColorDefault).
		SetSelectedTextColor(tcell.ColorDefault).
		SetSelectedBackgroundColor(tcell.ColorDarkCyan)
	a.fileList.SetBorder(true).SetTitle(" Select Book File ")
	a.fileList.SetSelectedFunc(func(idx int, mainText string, _ string, _ rune) {
		if idx >= len(a.fileEntries) {
			return
		}
		entry := a.fileEntries[idx]
		if entry.isDir {
			a.browseDir(entry.path)
		} else if entry.isParent {
			parent := filepath.Dir(a.currentDir)
			a.browseDir(parent)
		} else {
			// It's a file — add it
			a.mode = ModeLibrary
			a.addBookByPath(entry.path)
		}
	})
}

type fileEntry struct {
	path     string
	name     string
	isDir    bool
	isParent bool
}

func (a *App) browseDir(dir string) {
	a.currentDir = dir
	a.mode = ModeFileBrowser

	entries, err := os.ReadDir(dir)
	if err != nil {
		a.fileList.Clear()
		a.fileList.AddItem(fmt.Sprintf("Cannot read directory: %v", err), "", 0, nil)
		a.switchPage("filebrowser", a.fileList)
		return
	}

	a.fileEntries = nil

	// Parent directory
	if dir != "/" {
		a.fileEntries = append(a.fileEntries, fileEntry{
			name:     "..",
			path:     dir,
			isParent: true,
		})
	}

	// Sort: directories first, then files, both alphabetically
	var dirs, files []os.DirEntry
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, e)
		} else {
			lower := strings.ToLower(name)
			if isSupportedBookFile(lower) {
				files = append(files, e)
			}
		}
	}

	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name() < dirs[j].Name() })
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })

	for _, d := range dirs {
		fullPath := filepath.Join(dir, d.Name())
		a.fileEntries = append(a.fileEntries, fileEntry{
			name:  d.Name() + "/",
			path:  fullPath,
			isDir: true,
		})
	}
	for _, f := range files {
		fullPath := filepath.Join(dir, f.Name())
		a.fileEntries = append(a.fileEntries, fileEntry{
			name: f.Name(),
			path: fullPath,
		})
	}

	a.fileList.Clear()
	if len(a.fileEntries) == 0 {
		a.fileList.AddItem("(Empty directory)", "", 0, nil)
	} else {
		for _, e := range a.fileEntries {
			a.fileList.AddItem(e.name, "", 0, nil)
		}
	}

	a.fileList.SetTitle(fmt.Sprintf(" %s ", dir))
	a.switchPage("filebrowser", a.fileList)
}

func (a *App) handleFileBrowserKey(ev *tcell.EventKey) *tcell.EventKey {
	if ev.Key() == tcell.KeyEsc {
		a.mode = ModeLibrary
		a.switchPage("library", a.libList)
		return nil
	}
	return ev
}

func (a *App) showAddBook() {
	// Start file browser from home directory
	home, _ := os.UserHomeDir()
	a.browseDir(home)
}

func isSupportedBookFile(name string) bool {
	for _, ext := range []string{".epub", ".txt", ".md", ".markdown", ".mobi", ".azw3", ".pdf"} {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}
