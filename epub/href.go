package epub

import (
	"net/url"
	"path"
	"strings"
)

// CanonicalHref renders a publication-root-relative resource path in the shape
// the cross-client locator spec mandates: leading slash, forward slashes,
// URL-decoded. See epub-reader/contracts/LOCATOR.md.
func CanonicalHref(baseDir, itemHref string) string {
	p := itemHref
	if baseDir != "" && baseDir != "." {
		p = path.Join(baseDir, itemHref)
	}
	return "/" + normalizeHref(p)
}

// OPFDir returns the directory holding the OPF, always forward-slashed.
func OPFDir(opfPath string) string {
	d := path.Dir(strings.ReplaceAll(opfPath, `\`, "/"))
	if d == "." || d == "/" {
		return ""
	}
	return d
}

// normalizeHref strips the leading slash, any fragment, and percent-encoding so
// hrefs written by different engines compare equal.
func normalizeHref(href string) string {
	h := strings.ReplaceAll(href, `\`, "/")
	if i := strings.IndexByte(h, '#'); i >= 0 {
		h = h[:i]
	}
	if decoded, err := url.PathUnescape(h); err == nil {
		h = decoded
	}
	return strings.TrimPrefix(path.Clean("/"+h), "/")
}

// SectionIndexByHref resolves a locator href to a spine index. Stored data
// carries three historical href shapes, so an exact match is tried first, then
// progressively looser suffix and basename matches. Returns -1 when unmatched.
func (b *Book) SectionIndexByHref(href string) int {
	want := normalizeHref(href)
	if want == "" {
		return -1
	}
	// FullHref is the canonical shape, so it is what a spec-compliant peer
	// sends; matching it first keeps the looser tiers below as a safety net
	// for legacy rows rather than the primary path.
	for i, s := range b.Sections {
		if s.FullHref != "" && normalizeHref(s.FullHref) == want {
			return i
		}
	}
	for i, s := range b.Sections {
		if normalizeHref(s.Href) == want {
			return i
		}
	}
	for i, s := range b.Sections {
		got := normalizeHref(s.Href)
		if strings.HasSuffix(got, "/"+want) || strings.HasSuffix(want, "/"+got) {
			return i
		}
	}
	wantBase := path.Base(want)
	for i, s := range b.Sections {
		if path.Base(normalizeHref(s.Href)) == wantBase {
			return i
		}
	}
	return -1
}
