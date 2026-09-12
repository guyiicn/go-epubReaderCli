package store

import (
	"os"
	"path/filepath"
	"testing"

	"epub-reader/epub"
)

// seedBook registers a book and returns its path, marked as synced so
// DirtyAnnotations (which requires a server_id) can read rows back.
func seedBook(t *testing.T, st *Store) string {
	t.Helper()
	bookPath := filepath.Join(t.TempDir(), "book.txt")
	if err := os.WriteFile(bookPath, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := st.AddBook(bookPath, "Book", "Author"); err != nil {
		t.Fatalf("AddBook: %v", err)
	}
	lib := st.Library()
	if len(lib) != 1 {
		t.Fatalf("Library = %#v", lib)
	}
	if err := st.MarkBookSynced(lib[0].ID, "22222222-2222-4222-8222-222222222222", ""); err != nil {
		t.Fatalf("MarkBookSynced: %v", err)
	}
	return bookPath
}

func TestSaveBookmarksEmitsSharedLocator(t *testing.T) {
	st := testStore(t)
	bookPath := seedBook(t, st)

	err := st.SaveBookmarks(bookPath, []epub.Bookmark{{
		ID: "11111111-1111-4111-8111-111111111111", SectionIndex: 2, LinePos: 30,
		Href: "/OEBPS/Text/part0003.xhtml", Title: "Ch 3", Progression: 0.75, Percent: 0.5,
	}})
	if err != nil {
		t.Fatalf("SaveBookmarks: %v", err)
	}
	bms, err := st.LoadBookmarks(bookPath)
	if err != nil || len(bms) != 1 {
		t.Fatalf("LoadBookmarks = %#v, %v", bms, err)
	}
	got := ParseLocator(bms[0].Locator)
	want := Locator{
		Href: "/OEBPS/Text/part0003.xhtml", Title: "Ch 3",
		Progression: 0.75, HasProgression: true,
		TotalProgression: 0.5, HasTotal: true,
	}
	if got != want {
		t.Fatalf("bookmark locator\n got = %#v\nwant = %#v\nraw = %s", got, want, bms[0].Locator)
	}
}

// SaveBookmarks replaces every row, and rows coming back from LoadBookmarks
// carry only the raw locator. Re-saving them must not erase a locator another
// client wrote, whatever shape it is in.
func TestSaveBookmarksPreservesForeignLocator(t *testing.T) {
	tests := []struct {
		name   string
		stored string
	}{
		{"shared format", `{"href":"/OEBPS/Text/part0005.xhtml","type":"application/xhtml+xml","title":"Ch 5","locations":{"progression":0.25,"totalProgression":0.1}}`},
		{"readium without cfi, as android writes it", `{"href":"/OEBPS/Text/part0005.xhtml","type":"application/xhtml+xml","locations":{"progression":0.1578,"position":15,"totalProgression":0.0466}}`},
		{"legacy cli 1.x shape", `{"sectionIndex":5,"linePos":50}`},
		{"empty string", ``},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st := testStore(t)
			bookPath := seedBook(t, st)
			id := "11111111-1111-4111-8111-111111111111"

			// Arrive from a peer: locator set, position fields empty.
			if err := st.SaveBookmarks(bookPath, []epub.Bookmark{{
				ID: id, Locator: tc.stored, Note: "note",
			}}); err != nil {
				t.Fatalf("seed SaveBookmarks: %v", err)
			}
			bms, err := st.LoadBookmarks(bookPath)
			if err != nil || len(bms) != 1 {
				t.Fatalf("LoadBookmarks = %#v, %v", bms, err)
			}
			if bms[0].Locator != tc.stored {
				t.Fatalf("locator changed on seed: got %q want %q", bms[0].Locator, tc.stored)
			}

			// Adding another bookmark re-saves the whole set.
			next := append(bms, epub.Bookmark{
				ID: "33333333-3333-4333-8333-333333333333", SectionIndex: 1, LinePos: 5,
				Href: "/OEBPS/Text/part0002.xhtml", Title: "Ch 2", Progression: 0.1, Percent: 0.2,
			})
			if err := st.SaveBookmarks(bookPath, next); err != nil {
				t.Fatalf("SaveBookmarks: %v", err)
			}
			after, err := st.LoadBookmarks(bookPath)
			if err != nil || len(after) != 2 {
				t.Fatalf("LoadBookmarks after = %#v, %v", after, err)
			}
			for _, bm := range after {
				if bm.ID == id && bm.Locator != tc.stored {
					t.Fatalf("foreign locator clobbered: got %q want %q", bm.Locator, tc.stored)
				}
			}
		})
	}
}

func TestAddAnnotationEmitsSharedLocator(t *testing.T) {
	st := testStore(t)
	bookPath := seedBook(t, st)

	err := st.AddAnnotation(bookPath, epub.Annotation{
		SelectedText: "text", Note: "note", SectionIndex: 2, LinePos: 30,
		Href: "/OEBPS/Text/part0003.xhtml", Title: "Ch 3", Progression: 0.75, Percent: 0.5,
	})
	if err != nil {
		t.Fatalf("AddAnnotation: %v", err)
	}
	rows, err := st.DirtyAnnotations()
	if err != nil || len(rows) != 1 {
		t.Fatalf("DirtyAnnotations = %#v, %v", rows, err)
	}
	got := ParseLocator(rows[0].Locator)
	want := Locator{
		Href: "/OEBPS/Text/part0003.xhtml", Title: "Ch 3",
		Progression: 0.75, HasProgression: true,
		TotalProgression: 0.5, HasTotal: true,
	}
	if got != want {
		t.Fatalf("annotation locator\n got = %#v\nwant = %#v\nraw = %s", got, want, rows[0].Locator)
	}
}
