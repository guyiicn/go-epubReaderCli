package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (a *App) setupHelp() {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	tv.SetBorder(true).SetTitle(" 帮助 (h/ESC 关闭) ")
	a.helpView = tv
}

func (a *App) showHelp(from Mode) {
	a.prevMode = from
	a.mode = ModeHelp
	a.buildHelpText()
	a.switchPage("help", a.helpView)
}

func (a *App) closeHelp() {
	switch a.prevMode {
	case ModeLibrary:
		a.mode = ModeLibrary
		a.switchPage("library", a.libList)
	case ModeReader:
		a.mode = ModeReader
		a.switchPage("reader", a.readerView)
	case ModeTOC:
		a.mode = ModeTOC
		a.switchPage("toc", a.tocList)
	case ModeBookmarks:
		a.mode = ModeBookmarks
		a.switchPage("bookmarks", a.bmList)
	case ModeInfo:
		a.mode = ModeInfo
		a.switchPage("info", a.infoView)
	default:
		a.mode = ModeReader
		a.switchPage("reader", a.readerView)
	}
}

func (a *App) buildHelpText() {
	helpText := strings.Join([]string{
		"[::b]epub-reader 快捷键[::-]",
		"",
		"[::b]全局[::-]",
		"  h         帮助(任何界面)",
		"  Esc       返回/关闭",
		"  Ctrl+C    强制退出",
		"",
		"[::b]书库模式[::-]",
		"  j/↓       下一本",
		"  k/↑       上一本",
		"  Enter     打开书籍",
		"  a         添加书籍",
		"  d         删除书籍",
		"  q         退出程序",
		"",
		"[::b]阅读模式[::-]",
		"  j/↓       单栏:下滚1行  双栏:下翻1页",
		"  k/↑       单栏:上滚1行  双栏:上翻1页",
		"  Sp/→      单栏:下滚一屏 双栏:下翻1页",
		"  Bs/←      单栏:上滚一屏 双栏:上翻1页",
		"  翻过末尾  → 自动进入下一章",
		"  翻过开头  → 自动进入上一章",
		"  g         章节开头",
		"  e         章节末尾",
		"  n         下一章",
		"  p         上一章",
		"  t         目录弹窗",
		"  b         书签列表",
		"  a         添加书签",
		"  i         书籍信息",
		"  c         切换单栏/双栏",
		"  /         搜索当前章节",
		"  .         下一个搜索结果",
		"  o/q/Esc   返回书库",
		"",
		"[::b]弹窗通用 (目录/书签/帮助/信息)[::-]",
		"  j/k/↑/↓   上下移动",
		"  Enter     跳转/确认",
		"  d         删除书签(书签列表)",
		"  Esc       关闭弹窗",
	}, "\n")

	a.helpView.SetText(helpText)
}

// --- Info ---

func (a *App) setupInfo() {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	tv.SetBorder(true).SetTitle(" 书籍信息 (i/ESC 关闭) ")
	a.infoView = tv
}

func (a *App) showInfo() {
	if a.book == nil {
		return
	}
	a.mode = ModeInfo

	m := a.book.Meta
	var sb strings.Builder
	sb.WriteString("[::b]书籍信息[::-]\n\n")
	if m.Title != "" {
		sb.WriteString(fmt.Sprintf("  书名: %s\n", m.Title))
	}
	if m.Author != "" {
		sb.WriteString(fmt.Sprintf("  作者: %s\n", m.Author))
	}
	if m.Language != "" {
		sb.WriteString(fmt.Sprintf("  语言: %s\n", m.Language))
	}
	if m.Publisher != "" {
		sb.WriteString(fmt.Sprintf("  出版社: %s\n", m.Publisher))
	}
	if m.Date != "" {
		sb.WriteString(fmt.Sprintf("  日期: %s\n", m.Date))
	}
	if m.Description != "" {
		sb.WriteString(fmt.Sprintf("\n  简介:\n  %s\n", m.Description))
	}
	sb.WriteString(fmt.Sprintf("\n  章节数: %d\n", len(a.book.Sections)))

	totalChars := 0
	for _, s := range a.book.Sections {
		totalChars += len(s.HTML)
	}
	sb.WriteString(fmt.Sprintf("  总字数(约): %d\n", totalChars/2))

	sb.WriteString(fmt.Sprintf("\n  文件: %s\n", a.bookPath))

	a.infoView.SetText(sb.String())
	a.switchPage("info", a.infoView)
}

func (a *App) closeInfo() {
	a.mode = ModeReader
	a.switchPage("reader", a.readerView)
}

// --- Add Book ---

func (a *App) setupAddBook() {
	input := tview.NewInputField().
		SetLabel("EPUB 文件路径: ").
		SetFieldWidth(50)
	input.SetBorder(true).SetTitle(" 添加书籍 ")

	input.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			path := input.GetText()
			a.mode = ModeLibrary
			a.addBookByPath(path)
		case tcell.KeyEsc:
			a.mode = ModeLibrary
			a.switchPage("library", a.libList)
		}
	})
	a.addInput = input
}

// --- Search ---

func (a *App) setupSearch() {
	input := tview.NewInputField().
		SetLabel("搜索: ").
		SetFieldWidth(40)
	input.SetBorder(true).SetTitle(" 搜索 ")

	input.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			a.searchTerm = input.GetText()
			a.executeSearch()
		case tcell.KeyEsc:
			a.mode = ModeReader
			a.switchPage("reader", a.readerView)
		}
	})
	a.searchInput = input

	a.searchResults = tview.NewList().
		ShowSecondaryText(false)
	a.searchResults.
		SetMainTextColor(tcell.ColorDefault).
		SetSelectedTextColor(tcell.ColorDefault).
		SetSelectedBackgroundColor(tcell.ColorDarkCyan)
	a.searchResults.SetBorder(true).SetTitle(" 搜索结果 ")

	a.searchResults.SetSelectedFunc(func(idx int, _ string, _ string, _ rune) {
		if idx < len(a.searchMatches) {
			a.scrollPos = a.searchMatches[idx]
			a.mode = ModeReader
			a.switchPage("reader", a.readerView)
			a.updateReaderDisplay()
		}
	})
}

func (a *App) showSearch() {
	a.mode = ModeSearch
	a.searchInput.SetText("")
	a.switchPage("search", a.searchInput)
}

func (a *App) executeSearch() {
	if a.searchTerm == "" {
		a.mode = ModeReader
		a.switchPage("reader", a.readerView)
		return
	}

	a.searchMatches = nil
	term := strings.ToLower(a.searchTerm)
	for i, line := range a.lines {
		if strings.Contains(strings.ToLower(line), term) {
			a.searchMatches = append(a.searchMatches, i)
		}
	}

	if len(a.searchMatches) == 0 {
		a.updateReaderStatus("未找到: " + a.searchTerm)
		a.mode = ModeReader
		a.switchPage("reader", a.readerView)
		return
	}

	a.scrollPos = a.searchMatches[0]
	a.mode = ModeReader
	a.switchPage("reader", a.readerView)
	a.updateReaderDisplay()
	a.updateReaderStatus(fmt.Sprintf("找到 %d 处匹配", len(a.searchMatches)))
}

func (a *App) nextSearchMatch() {
	if len(a.searchMatches) == 0 {
		return
	}
	for _, pos := range a.searchMatches {
		if pos > a.scrollPos {
			a.scrollPos = pos
			a.updateReaderDisplay()
			return
		}
	}
	a.scrollPos = a.searchMatches[0]
	a.updateReaderDisplay()
	a.updateReaderStatus("搜索回到开头")
}

func (a *App) closeSearchResults() {
	a.mode = ModeReader
	a.switchPage("reader", a.readerView)
}
