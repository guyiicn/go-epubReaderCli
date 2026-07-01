package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"epub-reader/epub"
	"epub-reader/internal/server"
	"epub-reader/store"
	"epub-reader/ui"

	"golang.org/x/term"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [file.epub]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nA terminal EPUB reader.\n")
	}
	s, err := store.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing store: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	if err := run(os.Args[1:], s); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, s *store.Store) error {
	if len(args) > 0 && isCommand(args[0]) {
		return runCommand(args, s)
	}
	flag.Parse()
	app := ui.NewApp(s)
	return app.Run(args)
}

func isCommand(s string) bool {
	switch s {
	case "login", "logout", "account", "sync", "list", "import", "open", "download", "search", "search-download", "paths", "help":
		return true
	default:
		return false
	}
}

func runCommand(args []string, st *store.Store) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	switch args[0] {
	case "help":
		printUsage()
		return nil
	case "paths":
		p := st.Paths()
		fmt.Printf("config: %s\ndata:   %s\ncache:  %s\ndb:     %s\nbooks:  %s\nlegacy: %s\n", p.ConfigDir, p.DataDir, p.CacheDir, p.DBPath, p.BooksDir, p.OldDir)
		return nil
	case "list":
		return cmdList(st)
	case "import":
		if len(args) < 2 {
			return fmt.Errorf("usage: epub-reader-term import <file>")
		}
		return cmdImport(st, args[1])
	case "open":
		if len(args) < 2 {
			return fmt.Errorf("usage: epub-reader-term open <book-id-or-query>")
		}
		b, err := st.BookByIDOrQuery(strings.Join(args[1:], " "))
		if err != nil {
			return err
		}
		if b.RemoteOnly || b.Path == "" {
			return fmt.Errorf("book is remote-only; run download %s first", firstNonEmpty(b.ServerID, b.ID))
		}
		app := ui.NewApp(st)
		return app.Run([]string{b.Path})
	case "login":
		return cmdLogin(ctx, st, args[1:])
	case "logout":
		return st.ClearAuthTokens()
	case "account":
		return cmdAccount(st)
	case "sync":
		return cmdSync(ctx, st)
	case "search":
		return cmdSearch(ctx, st, args[1:])
	case "search-download":
		return cmdSearchDownload(ctx, st, args[1:])
	case "download":
		if len(args) < 2 {
			return fmt.Errorf("usage: epub-reader-term download <server-book-id-or-query>")
		}
		return cmdDownload(ctx, st, strings.Join(args[1:], " "))
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage() {
	fmt.Fprintf(os.Stdout, `epub-reader-term

Usage:
  epub-reader-term                         open TUI library
  epub-reader-term <file.epub>             import/open a local EPUB
  epub-reader-term login --server URL      login and register CLI device
  epub-reader-term account                 show current account/device
  epub-reader-term sync                    pull server library metadata
  epub-reader-term list                    list local and remote books
  epub-reader-term import <file>           import a local book
  epub-reader-term open <id-or-query>      open a local book
  epub-reader-term download <id-or-query>  download a remote-only book
  epub-reader-term search <query>          search server find-book API
  epub-reader-term search-download --book-command CMD --title TITLE [--author AUTHOR]
  epub-reader-term paths                   show data paths
`)
}

func cmdList(st *store.Store) error {
	books, err := st.Books()
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTITLE\tAUTHOR\tPROGRESS\tSTATE")
	for _, b := range books {
		id := firstNonEmpty(b.ServerID, b.ID)
		if len(id) > 12 {
			id = id[:12]
		}
		state := "local"
		if b.RemoteOnly {
			state = "remote"
		}
		if b.Dirty {
			state += ",dirty"
		}
		progress := ""
		if p, _ := st.LoadProgress(b.Path); p != nil {
			progress = fmt.Sprintf("%d%%", int(p.Percent*100))
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", id, b.Title, b.Author, progress, state)
	}
	return w.Flush()
}

func cmdImport(st *store.Store, path string) error {
	book, err := epub.Load(path)
	if err != nil {
		return err
	}
	title := book.Title
	if title == "" {
		title = path
	}
	if err := st.AddBook(path, title, book.Author); err != nil {
		return err
	}
	fmt.Printf("imported: %s\n", title)
	return nil
}

func cmdLogin(ctx context.Context, st *store.Store, args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	serverURL := fs.String("server", "", "server base URL, e.g. https://us.guyii.net")
	username := fs.String("username", "", "username")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *serverURL == "" {
		return fmt.Errorf("login requires --server")
	}
	if *username == "" {
		fmt.Fprint(os.Stderr, "username: ")
		if _, err := fmt.Fscanln(os.Stdin, username); err != nil {
			return err
		}
	}
	fmt.Fprint(os.Stderr, "password: ")
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return err
	}
	auth, err := server.LoginAndRegister(ctx, st, *serverURL, *username, string(pass))
	if err != nil {
		return err
	}
	fmt.Printf("logged in user=%s device=%s platform=%s\n", auth.UserID, auth.DeviceID, auth.Platform)
	return nil
}

func cmdAccount(st *store.Store) error {
	auth, err := st.AuthState()
	if err != nil {
		return err
	}
	fmt.Printf("server:   %s\nuser:     %s\ndevice:   %s\nname:     %s\nplatform: %s\nexpires:  %d\n",
		auth.ServerURL, emptyDash(auth.UserID), auth.DeviceID, auth.DeviceName, auth.Platform, auth.AccessTokenExpiresAt)
	return nil
}

func cmdSync(ctx context.Context, st *store.Store) error {
	auth, _ := st.AuthState()
	client, err := server.NewClient(auth.ServerURL, st)
	if err != nil {
		return err
	}
	engine := server.NewEngine(st, client)
	if err := engine.Sync(ctx); err != nil {
		return err
	}
	fmt.Println("sync complete")
	return nil
}

func cmdSearch(ctx context.Context, st *store.Store, args []string) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	page := fs.Int("page", 1, "page")
	preference := fs.String("preference", "", "search preference")
	if err := fs.Parse(args); err != nil {
		return err
	}
	query := strings.Join(fs.Args(), " ")
	if query == "" {
		return fmt.Errorf("usage: epub-reader-term search <query>")
	}
	auth, _ := st.AuthState()
	client, err := server.NewClient(auth.ServerURL, st)
	if err != nil {
		return err
	}
	res, err := client.Search(ctx, query, *page, *preference)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "IDX\tFORMAT\tSIZE\tTITLE\tAUTHOR\tCOMMAND")
	for _, b := range res.Books {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n", b.Index, b.Format, b.Size, b.Title, b.Author, b.BookCommand)
	}
	return w.Flush()
}

func cmdSearchDownload(ctx context.Context, st *store.Store, args []string) error {
	fs := flag.NewFlagSet("search-download", flag.ContinueOnError)
	bookCommand := fs.String("book-command", "", "server book command, must start with /book_")
	title := fs.String("title", "", "title")
	author := fs.String("author", "", "author")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *bookCommand == "" || *title == "" {
		return fmt.Errorf("search-download requires --book-command and --title")
	}
	auth, _ := st.AuthState()
	client, err := server.NewClient(auth.ServerURL, st)
	if err != nil {
		return err
	}
	book, err := client.SearchDownload(ctx, server.SearchDownloadRequest{BookCommand: *bookCommand, Title: *title, Author: *author})
	if err != nil {
		return err
	}
	if err := st.UpsertRemoteBook(book.ID, book.Title, book.Author, book.Format, book.ContentHash, book.TotalChapters, 0); err != nil {
		return err
	}
	fmt.Printf("added to library: %s (%s)\n", book.Title, book.ID)
	return nil
}

func cmdDownload(ctx context.Context, st *store.Store, query string) error {
	b, err := st.BookByIDOrQuery(query)
	if err != nil {
		return err
	}
	serverID := firstNonEmpty(b.ServerID, b.ID)
	if serverID == "" {
		return fmt.Errorf("book has no server id")
	}
	auth, _ := st.AuthState()
	client, err := server.NewClient(auth.ServerURL, st)
	if err != nil {
		return err
	}
	dst := st.BookStoragePath(b.ContentHash, serverID, b.Format)
	n, err := client.DownloadBookFile(ctx, serverID, dst)
	if err != nil {
		return err
	}
	if err := st.MarkDownloaded(serverID, dst, b.ContentHash, n); err != nil {
		return err
	}
	fmt.Printf("downloaded: %s -> %s (%d bytes)\n", b.Title, dst, n)
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
