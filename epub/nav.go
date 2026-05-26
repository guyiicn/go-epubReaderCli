package epub

import (
	"strings"

	"golang.org/x/net/html"
)

// parseNavXHTML parses an EPUB 3 nav.xhtml and extracts TOC entries.
func parseNavXHTML(data []byte) []TOCEntry {
	doc, err := html.Parse(strings.NewReader(string(data)))
	if err != nil {
		return nil
	}

	// Find the <nav> with epub:type="toc"
	navNode := findNavTOC(doc)
	if navNode == nil {
		return nil
	}

	// Find the first <ol> inside the nav
	olNode := findFirstTag(navNode, "ol")
	if olNode == nil {
		return nil
	}

	var entries []TOCEntry
	parseNavOL(olNode, 0, &entries)
	return entries
}

func findNavTOC(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.Data == "nav" {
		for _, attr := range n.Attr {
			if attr.Key == "type" && attr.Val == "toc" {
				return n
			}
			// epub:type stored as separate attribute in some parsers
			if attr.Key == "epub:type" && attr.Val == "toc" {
				return n
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findNavTOC(c); found != nil {
			return found
		}
	}
	return nil
}

func findFirstTag(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findFirstTag(c, tag); found != nil {
			return found
		}
	}
	return nil
}

func parseNavOL(n *html.Node, depth int, entries *[]TOCEntry) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || c.Data != "li" {
			continue
		}
		parseNavLI(c, depth, entries)
	}
}

func parseNavLI(n *html.Node, depth int, entries *[]TOCEntry) {
	// Find <a> for title and href
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "a" {
			title := textContent(c)
			href := getAttr(c, "href")
			*entries = append(*entries, TOCEntry{
				Title:     strings.TrimSpace(title),
				SectionID: href,
				Depth:     depth,
			})
			break
		}
		if c.Type == html.ElementNode && c.Data == "span" {
			// EPUB 3 can use <span> for unlinked group titles
			title := textContent(c)
			if strings.TrimSpace(title) != "" {
				*entries = append(*entries, TOCEntry{
					Title: strings.TrimSpace(title),
					Depth: depth,
				})
			}
		}
	}

	// Nested <ol>
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "ol" {
			parseNavOL(c, depth+1, entries)
		}
	}
}

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}
