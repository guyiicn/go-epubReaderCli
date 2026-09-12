package store

import "testing"

func TestParseLocatorHistoricalShapes(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want Locator
	}{
		{
			name: "shared format written by this client",
			raw:  `{"href":"/OEBPS/Text/part0005.xhtml","type":"application/xhtml+xml","title":"Ch 5","locations":{"progression":0.25,"totalProgression":0.1}}`,
			want: Locator{
				Href: "/OEBPS/Text/part0005.xhtml", Title: "Ch 5",
				Progression: 0.25, HasProgression: true,
				TotalProgression: 0.1, HasTotal: true,
			},
		},
		{
			name: "readium without cfi, as android writes it",
			raw:  `{"href":"/OEBPS/Text/part0005.xhtml","type":"application/xhtml+xml","locations":{"progression":0.1578,"position":15,"totalProgression":0.0466}}`,
			want: Locator{
				Href:        "/OEBPS/Text/part0005.xhtml",
				Progression: 0.1578, HasProgression: true,
				TotalProgression: 0.0466, HasTotal: true,
			},
		},
		{
			name: "foliate style carrying a cfi",
			raw:  `{"href":"/OEBPS/Text/part0005.xhtml","type":"application/xhtml+xml","locations":{"cfi":"epubcfi(/6/12!/4/2/2)","progression":0.5,"totalProgression":0.3}}`,
			want: Locator{
				Href: "/OEBPS/Text/part0005.xhtml", CFI: "epubcfi(/6/12!/4/2/2)",
				Progression: 0.5, HasProgression: true,
				TotalProgression: 0.3, HasTotal: true,
			},
		},
		{
			name: "legacy cli 1.x shape",
			raw:  `{"sectionIndex":5,"linePos":50}`,
			want: Locator{SectionIndex: 5, LinePos: 50, HasLegacy: true},
		},
		{name: "empty string", raw: ``, want: Locator{}},
		{name: "not json", raw: `garbage`, want: Locator{}},
		{
			name: "out of range values are clamped",
			raw:  `{"href":"/a.xhtml","locations":{"progression":-3,"totalProgression":7}}`,
			want: Locator{
				Href: "/a.xhtml", Progression: 0, HasProgression: true,
				TotalProgression: 1, HasTotal: true,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParseLocator(tc.raw); got != tc.want {
				t.Fatalf("ParseLocator(%q)\n got = %#v\nwant = %#v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestBuildLocatorRoundTrip(t *testing.T) {
	raw := BuildLocator("/OEBPS/Text/part0005.xhtml", "Ch 5", 0.25, 0.1)
	got := ParseLocator(raw)
	want := Locator{
		Href: "/OEBPS/Text/part0005.xhtml", Title: "Ch 5",
		Progression: 0.25, HasProgression: true,
		TotalProgression: 0.1, HasTotal: true,
	}
	if got != want {
		t.Fatalf("round trip\n got = %#v\nwant = %#v\nraw = %s", got, want, raw)
	}
}

// Readers must be able to consume a locator whose href was written by any
// engine, so a blank progression must still be distinguishable from zero.
func TestBuildLocatorAlwaysCarriesRequiredFields(t *testing.T) {
	got := ParseLocator(BuildLocator("/a.xhtml", "", 0, 0))
	if !got.HasProgression || !got.HasTotal {
		t.Fatalf("required locations fields missing: %#v", got)
	}
}
