package ui

import (
	"epub-reader/epub"
	"epub-reader/render"
	"epub-reader/store"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Mode represents the current UI mode.
type Mode int

const (
	ModeLibrary Mode = iota
	ModeReader
	ModeTOC
	ModeBookmarks
	ModeHelp
	ModeAddBook
	ModeSearch
	ModeSearchResults
	ModeInfo
	ModeFileBrowser
	ModeBookmarkNote
	ModeAnnotationNote
)

// App is the main application.
type App struct {
	tapp     *tview.Application
	screen   tcell.Screen
	pages    *tview.Pages
	store    *store.Store
	renderer *render.Renderer

	// State
	mode     Mode
	prevMode Mode
	book     *epub.Book
	bookPath string
	library  []epub.LibraryEntry

	// Library widgets
	libFlex  *tview.Flex
	libList  *tview.List
	libTitle *tview.TextView

	// Reader widgets
	readerFlex  *tview.Flex
	readerTitle *tview.TextView
	readerView  *tview.TextView
	statusView  *tview.TextView

	// TOC popup
	tocList *tview.List

	// Bookmarks popup
	bmList *tview.List

	// Help popup
	helpView *tview.TextView

	// Info popup
	infoView *tview.TextView

	// Add book popup
	addInput *tview.InputField

	// File browser
	fileList    *tview.List
	fileEntries []fileEntry
	currentDir  string

	// Search
	searchInput      *tview.InputField
	searchResults    *tview.List
	searchTerm       string
	searchMatches    []int
	searchAllMode    bool // true = search all chapters
	searchAllResults []searchResult

	// Reader state
	sectionIdx int
	lines      []string
	scrollPos  int
	pageHeight int
	colWidth   int
	columns    int

	// Cache
	cachedSection int
	cachedWidth   int

	// Config
	config epub.Config

	// Bookmark note input
	bmNoteInput         *tview.InputField
	annotationNoteInput *tview.InputField
}

// NewApp creates the application.
func NewApp(s *store.Store) *App {
	a := &App{
		tapp:     tview.NewApplication(),
		store:    s,
		renderer: render.NewRenderer(),
		config:   s.Config(),
		library:  s.Library(),
	}

	a.setupUI()
	a.setupKeys()
	return a
}

// Run starts the application.
func (a *App) Run(args []string) error {
	if len(args) > 0 {
		go func() {
			a.tapp.QueueUpdateDraw(func() {
				a.openBookByPath(args[0])
			})
		}()
	} else {
		a.refreshLibrary()
	}
	a.startRealtimeSync()

	a.tapp.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		a.screen = screen
		return false
	})

	return a.tapp.Run()
}

func (a *App) setupUI() {
	a.pages = tview.NewPages()

	a.setupLibrary()
	a.setupReader()
	a.setupTOC()
	a.setupBookmarks()
	a.setupHelp()
	a.setupInfo()
	a.setupAddBook()
	a.setupSearch()
	a.setupBookmarkNote()
	a.setupAnnotationNote()

	a.pages.AddPage("library", a.libFlex, true, true)
	a.pages.AddPage("reader", a.readerFlex, true, false)
	a.pages.AddPage("toc", a.tocList, true, false)
	a.pages.AddPage("bookmarks", a.bmList, true, false)
	a.pages.AddPage("help", a.helpView, true, false)
	a.pages.AddPage("info", a.infoView, true, false)
	a.pages.AddPage("addbook", a.centerWidget(a.addInput, 60, 3), true, false)
	a.pages.AddPage("search", a.centerWidget(a.searchInput, 60, 3), true, false)
	a.pages.AddPage("searchresults", a.searchResults, true, false)
	a.pages.AddPage("filebrowser", a.fileList, true, false)
	a.pages.AddPage("bmnote", a.centerWidget(a.bmNoteInput, 60, 3), true, false)
	a.pages.AddPage("annotationnote", a.centerWidget(a.annotationNoteInput, 70, 3), true, false)

	a.tapp.SetRoot(a.pages, true)
	a.tapp.SetFocus(a.libList)
}

func (a *App) centerWidget(w tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().
				AddItem(nil, 0, 1, false).
				AddItem(w, width, 0, true).
				AddItem(nil, 0, 1, false),
			height, 0, true,
		).
		AddItem(nil, 0, 1, false)
}

func (a *App) switchPage(name string, focus tview.Primitive) {
	a.pages.SwitchToPage(name)
	if focus != nil {
		a.tapp.SetFocus(focus)
	}
}

// setupKeys sets up the global key handler.
func (a *App) setupKeys() {
	a.tapp.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyCtrlC {
			a.tapp.Stop()
			return nil
		}

		// h 呼出帮助，任何模式下都可以（除了输入框模式）
		if ev.Rune() == 'h' && a.mode != ModeAddBook && a.mode != ModeSearch && a.mode != ModeBookmarkNote && a.mode != ModeAnnotationNote {
			a.showHelp(a.mode)
			return nil
		}

		switch a.mode {
		case ModeLibrary:
			return a.handleLibraryKey(ev)
		case ModeReader:
			a.handleReaderKey(ev)
			return nil
		case ModeTOC:
			return a.handleTOCKey(ev)
		case ModeBookmarks:
			return a.handleBookmarksKey(ev)
		case ModeHelp:
			return a.handleHelpKey(ev)
		case ModeInfo:
			return a.handleInfoKey(ev)
		case ModeAddBook:
			return ev
		case ModeSearch:
			return ev
		case ModeBookmarkNote:
			return ev
		case ModeAnnotationNote:
			return ev
		case ModeSearchResults:
			return a.handleSearchResultsKey(ev)
		case ModeFileBrowser:
			return a.handleFileBrowserKey(ev)
		}
		return ev
	})
}

// handleLibraryKey handles keys in library mode.
func (a *App) handleLibraryKey(ev *tcell.EventKey) *tcell.EventKey {
	switch {
	case ev.Rune() == 'a':
		a.showAddBook()
		return nil
	case ev.Rune() == 'd':
		a.removeBook()
		return nil
	case ev.Rune() == 's':
		a.syncNow()
		return nil
	case ev.Rune() == 'q':
		a.tapp.Stop()
		return nil
	}
	return ev
}

// handleReaderKey handles keys in reader mode.
func (a *App) handleReaderKey(ev *tcell.EventKey) {
	_, termHeight := a.getScreenSize()
	a.pageHeight = termHeight - 4

	r := ev.Rune()
	key := ev.Key()

	switch {
	// 翻页 (章节无缝衔接)
	case key == tcell.KeyRight || key == tcell.KeyPgDn || r == ' ':
		a.pageForward()

	case key == tcell.KeyLeft || key == tcell.KeyPgUp || key == tcell.KeyBackspace || key == tcell.KeyBackspace2:
		a.pageBackward()

	case r == 'g':
		a.scrollPos = 0
		a.updateReaderDisplay()

	case r == 'e':
		ps := a.pageSize()
		totalLines := len(a.lines)
		if totalLines > 0 {
			a.scrollPos = ((totalLines - 1) / ps) * ps
		}
		a.updateReaderDisplay()

	case r == 'n':
		a.nextSection()

	case r == 'p':
		a.prevSection()

	case r == 't':
		a.showTOC()

	case r == 'i':
		a.showInfo()

	case r == 'c':
		a.toggleColumns()

	case r == 'b':
		a.showBookmarks()

	case r == 'a':
		a.addBookmark()

	case r == 'm':
		a.addAnnotation()

	case r == '/':
		a.showSearch()

	case r == 'x':
		a.showSearchAll()

	case r == '.':
		a.nextSearchMatch()

	case r == 'o' || r == 'q' || key == tcell.KeyEsc:
		a.backToLibrary()
	}
}

func (a *App) handleTOCKey(ev *tcell.EventKey) *tcell.EventKey {
	if ev.Key() == tcell.KeyEsc || ev.Rune() == 't' {
		a.closeTOC()
		return nil
	}
	return ev
}

func (a *App) handleBookmarksKey(ev *tcell.EventKey) *tcell.EventKey {
	if ev.Key() == tcell.KeyEsc {
		a.closeBookmarks()
		return nil
	}
	if ev.Rune() == 'd' {
		a.deleteBookmark()
		return nil
	}
	return ev
}

func (a *App) handleHelpKey(ev *tcell.EventKey) *tcell.EventKey {
	if ev.Key() == tcell.KeyEsc || ev.Rune() == 'h' || ev.Rune() == 'q' {
		a.closeHelp()
		return nil
	}
	return ev
}

func (a *App) handleInfoKey(ev *tcell.EventKey) *tcell.EventKey {
	if ev.Key() == tcell.KeyEsc || ev.Rune() == 'q' || ev.Rune() == 'i' {
		a.closeInfo()
		return nil
	}
	return ev
}

func (a *App) handleSearchResultsKey(ev *tcell.EventKey) *tcell.EventKey {
	if ev.Key() == tcell.KeyEsc {
		a.closeSearchResults()
		return nil
	}
	return ev
}

func (a *App) resolveColumns(termWidth int) int {
	cfg := a.config
	switch cfg.Columns {
	case 1:
		return 1
	case 2:
		return 2
	default:
		if termWidth >= cfg.ColumnThreshold {
			return 2
		}
		return 1
	}
}
