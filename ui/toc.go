package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (a *App) setupTOC() {
	a.tocList = tview.NewList().
		ShowSecondaryText(false)
	a.tocList.
		SetMainTextColor(tcell.ColorDefault).
		SetSelectedTextColor(tcell.ColorDefault).
		SetSelectedBackgroundColor(tcell.ColorDarkCyan)
	a.tocList.SetBorder(true).SetTitle(" Table of Contents ")

	a.tocList.SetSelectedFunc(func(idx int, _ string, _ string, _ rune) {
		if a.book == nil {
			return
		}
		// Map TOC index to section
		tocEntries := a.book.TOC
		if idx >= len(tocEntries) {
			return
		}
		entry := tocEntries[idx]
		// Find section by href
		targetIdx := -1
		href := stripFragment(entry.SectionID)
		for i, s := range a.book.Sections {
			if s.Href == href || s.ID == href {
				targetIdx = i
				break
			}
		}
		if targetIdx == -1 && entry.SectionIndex > 0 && entry.SectionIndex < len(a.book.Sections) {
			targetIdx = entry.SectionIndex
		}
		if targetIdx >= 0 {
			a.goToSection(targetIdx)
		}
		a.closeTOC()
	})
}

func (a *App) showTOC() {
	if a.book == nil {
		return
	}
	a.mode = ModeTOC
	a.buildTOCList()
	a.switchPage("toc", a.tocList)
}

func (a *App) closeTOC() {
	a.mode = ModeReader
	a.switchPage("reader", a.readerView)
}

func (a *App) buildTOCList() {
	a.tocList.Clear()
	for _, entry := range a.book.TOC {
		indent := strings.Repeat("  ", entry.Depth)
		marker := "  "
		if a.sectionIdx == entry.SectionIndex {
			marker = "> "
		}
		a.tocList.AddItem(marker+indent+entry.Title, "", 0, nil)
	}

	// If no TOC entries, show section list
	if len(a.book.TOC) == 0 {
		for i, s := range a.book.Sections {
			marker := "  "
			if i == a.sectionIdx {
				marker = "> "
			}
			a.tocList.AddItem(fmt.Sprintf("%s%s", marker, s.Title), "", 0, nil)
		}
	}
}

func stripFragment(href string) string {
	if idx := strings.Index(href, "#"); idx >= 0 {
		return href[:idx]
	}
	return href
}
