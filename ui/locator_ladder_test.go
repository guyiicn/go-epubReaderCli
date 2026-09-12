package ui

import (
	"testing"

	"epub-reader/epub"
)

func ladderBook() *epub.Book {
	return &epub.Book{Sections: []epub.Section{
		{Href: "Text/part0001.xhtml", FullHref: "/OEBPS/Text/part0001.xhtml", Index: 0},
		{Href: "Text/part0002.xhtml", FullHref: "/OEBPS/Text/part0002.xhtml", Index: 1},
		{Href: "Text/part0003.xhtml", FullHref: "/OEBPS/Text/part0003.xhtml", Index: 2},
		{Href: "Text/part0004.xhtml", FullHref: "/OEBPS/Text/part0004.xhtml", Index: 3},
	}}
}

// The ladder has to land on the right chapter for every locator shape sitting
// in stored data, not just the one this client writes.
func TestResolveLocatorHistoricalShapes(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		ownSection int
		ownLine    int
		ownPercent float64
		want       locatorTarget
	}{
		{
			name: "shared format resolves href and parks the chapter fraction",
			raw:  `{"href":"/OEBPS/Text/part0003.xhtml","type":"application/xhtml+xml","locations":{"progression":0.5,"totalProgression":0.6}}`,
			want: locatorTarget{SectionIdx: 2, LinePos: 0, LineFrac: 0.5},
		},
		{
			name: "readium without cfi, as android writes it",
			raw:  `{"href":"/OEBPS/Text/part0002.xhtml","type":"application/xhtml+xml","locations":{"progression":0.25,"position":15,"totalProgression":0.3}}`,
			want: locatorTarget{SectionIdx: 1, LinePos: 0, LineFrac: 0.25},
		},
		{
			name: "foliate href without a leading slash still matches",
			raw:  `{"href":"OEBPS/Text/part0004.xhtml","type":"application/xhtml+xml","locations":{"progression":0.1,"totalProgression":0.9}}`,
			want: locatorTarget{SectionIdx: 3, LinePos: 0, LineFrac: 0.1},
		},
		{
			name: "a cfi this client cannot resolve falls through to href",
			raw:  `{"href":"/OEBPS/Text/part0003.xhtml","locations":{"cfi":"epubcfi(/6/12!/4/2/2)","progression":0.4,"totalProgression":0.5}}`,
			want: locatorTarget{SectionIdx: 2, LinePos: 0, LineFrac: 0.4},
		},
		{
			name: "legacy cli 1.x shape keeps its exact line",
			raw:  `{"sectionIndex":1,"linePos":50}`,
			want: locatorTarget{SectionIdx: 1, LinePos: 50, LineFrac: -1},
		},
		{
			name:       "empty locator prefers this client's own exact columns",
			raw:        ``,
			ownSection: 3, ownLine: 12, ownPercent: 0.1,
			want: locatorTarget{SectionIdx: 3, LinePos: 12, LineFrac: -1},
		},
		{
			name:       "no locator and no local position falls back to the percentage",
			raw:        ``,
			ownPercent: 0.5,
			want:       locatorTarget{SectionIdx: 2, LinePos: 0, LineFrac: 0},
		},
		{
			name: "unknown href falls back to the locator's own percentage",
			raw:  `{"href":"/OEBPS/Text/nowhere.xhtml","locations":{"progression":0.9,"totalProgression":0.25}}`,
			want: locatorTarget{SectionIdx: 1, LinePos: 0, LineFrac: 0},
		},
		{
			name: "garbage yields the start of the book",
			raw:  `not json`,
			want: locatorTarget{SectionIdx: 0, LinePos: 0, LineFrac: -1},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveLocator(ladderBook(), tc.raw, tc.ownSection, tc.ownLine, tc.ownPercent)
			if got != tc.want {
				t.Fatalf("resolveLocator(%q)\n got = %#v\nwant = %#v", tc.raw, got, tc.want)
			}
		})
	}
}

// The percentage rung must never index past the last chapter.
func TestResolveLocatorClampsFullProgress(t *testing.T) {
	got := resolveLocator(ladderBook(), `{"locations":{"totalProgression":1}}`, 0, 0, 0)
	if got.SectionIdx != 3 {
		t.Fatalf("SectionIdx = %d, want 3 (last chapter): %#v", got.SectionIdx, got)
	}
}
