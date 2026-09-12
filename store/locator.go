package store

import "encoding/json"

// sectionMediaType is the MIME every EPUB chapter resource carries.
const sectionMediaType = "application/xhtml+xml"

// Locator is a reading position decoded from the cross-client locator format
// (epub-reader/contracts/LOCATOR.md). Has* flags distinguish an absent field
// from a legitimate zero.
type Locator struct {
	Href  string
	Title string
	// CFI is recorded so it survives a round trip, but the CLI renders plain
	// text and has no DOM to resolve it against, so it never navigates by CFI.
	CFI              string
	Progression      float64
	HasProgression   bool
	TotalProgression float64
	HasTotal         bool

	// Legacy CLI 1.x shape {"sectionIndex":N,"linePos":M} — read-only compat.
	SectionIndex int
	LinePos      int
	HasLegacy    bool
}

type locatorOut struct {
	Href      string       `json:"href"`
	Type      string       `json:"type"`
	Title     string       `json:"title,omitempty"`
	Locations locationsOut `json:"locations"`
}

type locationsOut struct {
	Progression      float64 `json:"progression"`
	TotalProgression float64 `json:"totalProgression"`
}

type locatorIn struct {
	Href      string `json:"href"`
	Title     string `json:"title"`
	Locations *struct {
		CFI              string   `json:"cfi"`
		Progression      *float64 `json:"progression"`
		TotalProgression *float64 `json:"totalProgression"`
	} `json:"locations"`
	SectionIndex *int `json:"sectionIndex"`
	LinePos      *int `json:"linePos"`
}

// BuildLocator renders a position in the shared format. The CLI cannot produce
// an EPUB CFI, which the spec permits — readers fall back to href+progression.
func BuildLocator(href, title string, progression, totalProgression float64) string {
	b, err := json.Marshal(locatorOut{
		Href:  href,
		Type:  sectionMediaType,
		Title: title,
		Locations: locationsOut{
			Progression:      clamp01(progression),
			TotalProgression: clamp01(totalProgression),
		},
	})
	if err != nil {
		return ""
	}
	return string(b)
}

// ParseLocator decodes any locator shape seen in the wild: the shared format,
// Readium's (no cfi), the legacy CLI one, and empty/garbage input. It never
// errors — unparseable input yields a zero Locator so callers fall through the
// spec's ladder to the whole-book percentage.
func ParseLocator(raw string) Locator {
	var out Locator
	if raw == "" {
		return out
	}
	var in locatorIn
	if json.Unmarshal([]byte(raw), &in) != nil {
		return out
	}
	out.Href = in.Href
	out.Title = in.Title
	if in.Locations != nil {
		out.CFI = in.Locations.CFI
		if in.Locations.Progression != nil {
			out.Progression = clamp01(*in.Locations.Progression)
			out.HasProgression = true
		}
		if in.Locations.TotalProgression != nil {
			out.TotalProgression = clamp01(*in.Locations.TotalProgression)
			out.HasTotal = true
		}
	}
	if in.SectionIndex != nil || in.LinePos != nil {
		out.HasLegacy = true
		if in.SectionIndex != nil {
			out.SectionIndex = *in.SectionIndex
		}
		if in.LinePos != nil {
			out.LinePos = *in.LinePos
		}
	}
	return out
}

func clamp01(v float64) float64 {
	if v < 0 || v != v { // NaN compares false against everything
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
