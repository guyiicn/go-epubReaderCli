package render

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

// Renderer converts XHTML content to terminal text lines.
type Renderer struct{}

// NewRenderer creates a new Renderer.
func NewRenderer() *Renderer {
	return &Renderer{}
}

// textBlock represents a logical block of text to be rendered.
type textBlock struct {
	text   string
	indent string // prefix for continuation lines
	prefix string // prefix for first line (bullet, "> ")
	blank  bool   // add blank line before
	heading string // h1-h6, empty for non-heading
}

// Render converts HTML to a slice of display lines wrapped to width.
func (r *Renderer) Render(htmlStr string, width int) []string {
	if width < 10 {
		width = 10
	}

	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		// Fallback: return raw text
		return strings.Split(htmlStr, "\n")
	}

	// Find <body>
	body := findBody(doc)
	if body == nil {
		body = doc
	}

	blocks := extractBlocks(body)
	return renderBlocks(blocks, width)
}

func findBody(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.Data == "body" {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findBody(c); found != nil {
			return found
		}
	}
	return nil
}

func extractBlocks(n *html.Node) []textBlock {
	var blocks []textBlock

	if n.Type == html.TextNode {
		text := normalizeWhitespace(n.Data)
		if text != "" {
			return []textBlock{{text: text}}
		}
		return nil
	}

	if n.Type != html.ElementNode {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			blocks = append(blocks, extractBlocks(c)...)
		}
		return blocks
	}

	tag := n.Data

	switch tag {
	case "head", "style", "script", "link", "meta":
		return nil

	case "body", "div", "section", "article", "main":
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			blocks = append(blocks, extractBlocks(c)...)
		}

	case "p":
		text := collectInlineText(n)
		if text != "" {
			blocks = append(blocks, textBlock{text: text, blank: true})
		}

	case "h1", "h2", "h3", "h4", "h5", "h6":
		text := collectInlineText(n)
		if text != "" {
			blocks = append(blocks, textBlock{text: text, blank: true, heading: tag})
		}

	case "blockquote":
		var inner []textBlock
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			inner = append(inner, extractBlocks(c)...)
		}
		for i := range inner {
			if inner[i].prefix == "" {
				inner[i].prefix = "│ "
				inner[i].indent = "│ "
			} else {
				inner[i].prefix = "│ " + inner[i].prefix
				inner[i].indent = "│ " + inner[i].indent
			}
			inner[i].blank = false
		}
		if len(inner) > 0 {
			inner[0].blank = true
		}
		blocks = append(blocks, inner...)

	case "ul":
		counter := 0
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "li" {
				counter++
				text := collectInlineText(c)
				if text != "" {
					blocks = append(blocks, textBlock{
						text:   text,
						prefix: "  • ",
						indent: "    ",
						blank:  false,
					})
				}
			}
		}

	case "ol":
		counter := 0
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "li" {
				counter++
				text := collectInlineText(c)
				if text != "" {
					prefix := fmt.Sprintf("  %d. ", counter)
					indent := fmt.Sprintf("  %s  ", strings.Repeat(" ", len(fmt.Sprintf("%d", counter))))
					blocks = append(blocks, textBlock{
						text:   text,
						prefix: prefix,
						indent: indent,
						blank:  false,
					})
				}
			}
		}

	case "pre":
		text := collectPreText(n)
		for _, line := range strings.Split(text, "\n") {
			blocks = append(blocks, textBlock{text: line, blank: false})
		}

	case "hr":
		blocks = append(blocks, textBlock{text: "────────────────────────────────", blank: true})

	case "img":
		alt := getHTMLAttr(n, "alt")
		src := getHTMLAttr(n, "src")
		text := alt
		if text == "" {
			text = src
		}
		if text != "" {
			text = fmt.Sprintf("[图片: %s]", text)
		} else {
			text = "[图片]"
		}
		blocks = append(blocks, textBlock{text: text, blank: true})

	case "table":
		blocks = append(blocks, renderSimpleTable(n)...)

	case "br":
		blocks = append(blocks, textBlock{text: "", blank: false})

	case "dl":
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode {
				switch c.Data {
				case "dt":
					text := collectInlineText(c)
					if text != "" {
						blocks = append(blocks, textBlock{text: text, blank: true})
					}
				case "dd":
					text := collectInlineText(c)
					if text != "" {
						blocks = append(blocks, textBlock{
							text:   text,
							prefix: "  ",
							indent: "  ",
							blank:  false,
						})
					}
				}
			}
		}

	default:
		// Inline or unknown elements: recurse into children
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			blocks = append(blocks, extractBlocks(c)...)
		}
	}

	return blocks
}

// collectTextOnly recursively collects raw text from children, no markup wrapping.
func collectTextOnly(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		} else if n.Type == html.ElementNode {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
	}
	walk(n)
	return strings.TrimSpace(sb.String())
}

func collectInlineText(n *html.Node) string {
	var parts []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			text := normalizeWhitespace(n.Data)
			if text != "" {
				parts = append(parts, text)
			}
		} else if n.Type == html.ElementNode {
			switch n.Data {
			case "br":
				parts = append(parts, "\n")
			case "img":
				alt := getHTMLAttr(n, "alt")
				if alt == "" {
					alt = getHTMLAttr(n, "src")
				}
				parts = append(parts, fmt.Sprintf("[图片: %s]", alt))
			case "code":
				inner := collectTextOnly(n)
				if inner != "" {
					parts = append(parts, "`"+inner+"`")
				}
			case "strong", "b":
				inner := collectTextOnly(n)
				if inner != "" {
					parts = append(parts, "*"+inner+"*")
				}
			case "em", "i":
				inner := collectTextOnly(n)
				if inner != "" {
					parts = append(parts, "_"+inner+"_")
				}
			default:
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
			}
		}
	}
	walk(n)
	return strings.Join(parts, " ")
}

func collectPreText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

func renderSimpleTable(table *html.Node) []textBlock {
	var rows [][]string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			var cells []string
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
					cells = append(cells, collectInlineText(c))
				}
			}
			if len(cells) > 0 {
				rows = append(rows, cells)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(table)

	var blocks []textBlock
	for _, row := range rows {
		blocks = append(blocks, textBlock{text: strings.Join(row, " │ "), blank: true})
	}
	return blocks
}

func renderBlocks(blocks []textBlock, width int) []string {
	var lines []string
	for i, b := range blocks {
		if b.blank && i > 0 {
			lines = append(lines, "")
		}

		if b.text == "" {
			lines = append(lines, "")
			continue
		}

		// Handle heading decoration
		text := b.text
		switch b.heading {
		case "h1":
			lines = append(lines, "")
			lines = append(lines, "══ "+text+" ══")
			lines = append(lines, "")
			continue
		case "h2":
			lines = append(lines, "")
			lines = append(lines, "── "+text+" ──")
			continue
		case "h3":
			text = "▸ " + text
		case "h4":
			text = "  " + text
		}

		prefix := b.prefix
		indent := b.indent
		if prefix == "" && indent == "" {
			prefix = ""
			indent = ""
		}

		prefixWidth := StringWidth(prefix)
		indentWidth := StringWidth(indent)
		_ = indentWidth

		contentWidth := width - prefixWidth
		if contentWidth < 10 {
			contentWidth = 10
		}

		wrapped := wrapText(text, contentWidth)
		for j, line := range wrapped {
			if j == 0 {
				lines = append(lines, prefix+line)
			} else {
				pad := ""
				if indent == "" {
					pad = strings.Repeat(" ", prefixWidth)
				}
				lines = append(lines, pad+indent+line)
			}
		}
	}
	return lines
}

// wrapText wraps text to fit within width, handling CJK characters.
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	// Split by explicit newlines first
	parts := strings.Split(text, "\n")
	var result []string
	for _, part := range parts {
		result = append(result, wrapParagraph(part, width)...)
	}
	return result
}

func wrapParagraph(text string, width int) []string {
	tokens := tokenize(text)
	if len(tokens) == 0 {
		return nil
	}

	var lines []string
	var line strings.Builder
	lineWidth := 0

	for _, tok := range tokens {
		tokWidth := StringWidth(tok)

		if lineWidth > 0 && lineWidth+tokWidth > width && line.Len() > 0 {
			lines = append(lines, line.String())
			line.Reset()
			lineWidth = 0
		}

		// If token doesn't fit on a line, force it
		if lineWidth == 0 {
			line.WriteString(tok)
			lineWidth = tokWidth
		} else {
			// Add space between non-CJK tokens
			if !isCJKString(tok) && lineWidth > 0 {
				line.WriteString(" ")
				lineWidth++
			}
			line.WriteString(tok)
			lineWidth += tokWidth
		}
	}

	if line.Len() > 0 {
		lines = append(lines, line.String())
	}

	return lines
}

// tokenize breaks text into words/tokens, splitting CJK chars individually.
func tokenize(text string) []string {
	var tokens []string
	var buf strings.Builder
	inCJK := false

	for _, r := range text {
		if r == ' ' || r == '\t' {
			if buf.Len() > 0 {
				tokens = append(tokens, buf.String())
				buf.Reset()
			}
			inCJK = false
			continue
		}

		cjk := isCJKRune(r)
		if cjk && buf.Len() > 0 && !inCJK {
			tokens = append(tokens, buf.String())
			buf.Reset()
		} else if !cjk && inCJK && buf.Len() > 0 {
			tokens = append(tokens, buf.String())
			buf.Reset()
		}

		if cjk && buf.Len() > 0 {
			tokens = append(tokens, buf.String())
			buf.Reset()
		}

		buf.WriteRune(r)
		inCJK = cjk
	}

	if buf.Len() > 0 {
		tokens = append(tokens, buf.String())
	}

	return tokens
}

func isCJKRune(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r) ||
		isCJKPunct(r)
}

func isCJKPunct(r rune) bool {
	switch r {
	case '。', '，', '、', '：', '；', '！', '？',
		'（', '）', '【', '】', '《', '》', '「', '」',
		'『', '』', '—', '…', '·',
		'〖', '〗', '〈', '〉', 'ㄧ':
		return true
	}
	return false
}

func isCJKString(s string) bool {
	for _, r := range s {
		return isCJKRune(r)
	}
	return false
}

func normalizeWhitespace(s string) string {
	// Replace newlines and tabs with spaces
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	// Collapse multiple spaces into one
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

// StringWidth returns the display width of a string in terminal columns.
func StringWidth(s string) int {
	w := 0
	for _, r := range s {
		if r == '\n' {
			continue
		} else if isCJKRune(r) {
			w += 2
		} else {
			w++
		}
	}
	return w
}

func getHTMLAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}
