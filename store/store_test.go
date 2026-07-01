package store

import (
	"os"
	"path/filepath"
	"testing"

	"epub-reader/epub"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
	st, err := NewAt(Paths{
		ConfigDir: filepath.Join(root, "config"),
		DataDir:   filepath.Join(root, "data"),
		CacheDir:  filepath.Join(root, "cache"),
		OldDir:    filepath.Join(root, "old"),
		DBPath:    filepath.Join(root, "data", "reader.db"),
		BooksDir:  filepath.Join(root, "data", "books"),
	})
	if err != nil {
		t.Fatalf("NewAt: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestStoreAddProgressAndBookmark(t *testing.T) {
	st := testStore(t)
	bookPath := filepath.Join(t.TempDir(), "book.txt")
	if err := os.WriteFile(bookPath, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := st.AddBook(bookPath, "Book", "Author"); err != nil {
		t.Fatalf("AddBook: %v", err)
	}
	if got := st.Library(); len(got) != 1 || got[0].Title != "Book" {
		t.Fatalf("Library = %#v", got)
	}
	if err := st.SaveProgress(bookPath, epub.Progress{SectionIndex: 2, LinePos: 30, Percent: 0.5}); err != nil {
		t.Fatalf("SaveProgress: %v", err)
	}
	p, err := st.LoadProgress(bookPath)
	if err != nil {
		t.Fatalf("LoadProgress: %v", err)
	}
	if p == nil || p.SectionIndex != 2 || p.LinePos != 30 || p.Percent != 0.5 || !p.Dirty {
		t.Fatalf("progress = %#v", p)
	}
	bm := []epub.Bookmark{{ID: "11111111-1111-4111-8111-111111111111", SectionIndex: 2, LinePos: 30, Note: "note"}}
	if err := st.SaveBookmarks(bookPath, bm); err != nil {
		t.Fatalf("SaveBookmarks: %v", err)
	}
	bms, err := st.LoadBookmarks(bookPath)
	if err != nil {
		t.Fatalf("LoadBookmarks: %v", err)
	}
	if len(bms) != 1 || bms[0].Color != "#FFC107" || !bms[0].Dirty {
		t.Fatalf("bookmarks = %#v", bms)
	}
	if err := st.DeleteBookmark(bookPath, bm[0].ID); err != nil {
		t.Fatalf("DeleteBookmark: %v", err)
	}
	bms, err = st.LoadBookmarks(bookPath)
	if err != nil {
		t.Fatalf("LoadBookmarks after delete: %v", err)
	}
	if len(bms) != 0 {
		t.Fatalf("deleted bookmark still visible: %#v", bms)
	}
}

func TestAuthStateDefaults(t *testing.T) {
	st := testStore(t)
	auth, err := st.AuthState()
	if err != nil {
		t.Fatal(err)
	}
	if auth.DeviceID == "" || auth.DeviceName == "" || auth.Platform != "CLI" {
		t.Fatalf("auth defaults = %#v", auth)
	}
}
