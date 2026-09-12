package epub

import "testing"

// The three clients each emit a different href shape for the same chapter;
// all of them must resolve to the same spine index.
func TestSectionIndexByHrefAcrossEngines(t *testing.T) {
	book := &Book{Sections: []Section{
		{Href: "Text/part0001.xhtml", FullHref: "/OEBPS/Text/part0001.xhtml", Index: 0},
		{Href: "Text/part0005.xhtml", FullHref: "/OEBPS/Text/part0005.xhtml", Index: 1},
	}}

	for _, tc := range []struct {
		name string
		href string
		want int
	}{
		{"readium, leading slash", "/OEBPS/Text/part0005.xhtml", 1},
		{"foliate, no leading slash", "OEBPS/Text/part0005.xhtml", 1},
		{"legacy cli, no opf dir", "Text/part0005.xhtml", 1},
		{"percent encoded", "/OEBPS/Text/part0005%2Exhtml", 1},
		{"with fragment", "/OEBPS/Text/part0005.xhtml#p3", 1},
		{"first chapter", "/OEBPS/Text/part0001.xhtml", 0},
		{"unknown", "/OEBPS/Text/nope.xhtml", -1},
		{"empty", "", -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := book.SectionIndexByHref(tc.href); got != tc.want {
				t.Fatalf("SectionIndexByHref(%q) = %d, want %d", tc.href, got, tc.want)
			}
		})
	}
}

func TestCanonicalHref(t *testing.T) {
	for _, tc := range []struct {
		baseDir, item, want string
	}{
		{"OEBPS", "Text/part0005.xhtml", "/OEBPS/Text/part0005.xhtml"},
		{"", "s0.txt", "/s0.txt"},
		{".", "a.xhtml", "/a.xhtml"},
		{"OEBPS", "Text/a%20b.xhtml", "/OEBPS/Text/a b.xhtml"},
	} {
		if got := CanonicalHref(tc.baseDir, tc.item); got != tc.want {
			t.Fatalf("CanonicalHref(%q,%q) = %q, want %q", tc.baseDir, tc.item, got, tc.want)
		}
	}
}
