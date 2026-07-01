package epub

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	stdhtml "html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

func Load(path string) (*Book, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".epub":
		return loadEPUB(path)
	case ".txt", ".md", ".markdown":
		return loadTextBook(path, strings.TrimPrefix(ext, "."))
	case ".mobi", ".azw3":
		converted, err := convertToEPUB(path)
		if err != nil {
			return nil, err
		}
		return loadEPUB(converted)
	case ".pdf":
		converted, err := convertPDFToText(path)
		if err != nil {
			return nil, err
		}
		return loadTextBook(converted, "txt")
	default:
		return nil, fmt.Errorf("unsupported format %s", ext)
	}
}

func loadEPUB(path string) (*Book, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open epub: %w", err)
	}
	defer r.Close()

	book := &Book{}

	// 1. Parse container.xml
	containerFile := findFileInZip(r.File, "META-INF/container.xml")
	if containerFile == nil {
		return nil, fmt.Errorf("no container.xml found")
	}
	var container Container
	if err := decodeZipFile(containerFile, &container); err != nil {
		return nil, fmt.Errorf("parse container: %w", err)
	}
	if len(container.Rootfiles.Rootfile) == 0 {
		return nil, fmt.Errorf("no rootfile in container")
	}
	opfPath := container.Rootfiles.Rootfile[0].FullPath
	baseDir := filepath.Dir(opfPath)

	// 2. Parse OPF
	opfFile := findFileInZip(r.File, opfPath)
	if opfFile == nil {
		return nil, fmt.Errorf("no opf file: %s", opfPath)
	}
	var opf OPF
	if err := decodeZipFile(opfFile, &opf); err != nil {
		return nil, fmt.Errorf("parse opf: %w", err)
	}

	book.Title = opf.Metadata.Title
	book.Author = opf.Metadata.Author
	book.Meta = Metadata{
		Title:       opf.Metadata.Title,
		Author:      opf.Metadata.Author,
		Language:    opf.Metadata.Language,
		Publisher:   opf.Metadata.Publisher,
		Description: opf.Metadata.Description,
		Date:        opf.Metadata.Date,
	}

	// 3. Build manifest map: id → href
	manifest := make(map[string]ManifestItem)
	for _, item := range opf.Manifest.Items {
		manifest[item.ID] = item
	}

	// 4. Load sections from spine
	book.Sections = make([]Section, 0, len(opf.Spine.Items))
	for i, ref := range opf.Spine.Items {
		item, ok := manifest[ref.IDRef]
		if !ok {
			continue
		}
		fullPath := filepath.Join(baseDir, item.Href)
		f := findFileInZip(r.File, fullPath)
		if f == nil {
			// Try with forward slashes
			f = findFileInZip(r.File, strings.ReplaceAll(fullPath, `\`, "/"))
			if f == nil {
				continue
			}
		}

		data, err := readZipFile(f)
		if err != nil {
			continue
		}

		book.Sections = append(book.Sections, Section{
			ID:    ref.IDRef,
			Href:  item.Href,
			Index: i,
			HTML:  string(data),
		})
	}

	// 5. Parse TOC
	toc := parseTOC(r, &opf, manifest, baseDir)
	book.TOC = toc

	// 6. Resolve section titles
	resolveTitles(book)

	return book, nil
}

func loadTextBook(path, format string) (*Book, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read text book: %w", err)
	}
	title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	chunks := splitTextSections(string(data))
	sections := make([]Section, 0, len(chunks))
	toc := make([]TOCEntry, 0, len(chunks))
	for i, chunk := range chunks {
		sectionTitle := fmt.Sprintf("Part %d", i+1)
		htmlText := textToHTML(chunk)
		sections = append(sections, Section{
			ID:    fmt.Sprintf("s%d", i),
			Href:  fmt.Sprintf("s%d.%s", i, format),
			Title: sectionTitle,
			Index: i,
			HTML:  htmlText,
		})
		toc = append(toc, TOCEntry{Title: sectionTitle, SectionID: sections[i].Href, SectionIndex: i})
	}
	if len(sections) == 0 {
		sections = []Section{{ID: "s0", Href: "s0.txt", Title: "Text", HTML: ""}}
	}
	return &Book{
		Title:    title,
		Sections: sections,
		TOC:      toc,
		Meta: Metadata{
			Title: title,
		},
	}, nil
}

func splitTextSections(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	paragraphs := strings.Split(s, "\n\n")
	var sections []string
	var current strings.Builder
	for _, p := range paragraphs {
		if current.Len() > 80_000 {
			sections = append(sections, current.String())
			current.Reset()
		}
		current.WriteString(p)
		current.WriteString("\n\n")
	}
	if strings.TrimSpace(current.String()) != "" {
		sections = append(sections, current.String())
	}
	return sections
}

func textToHTML(s string) string {
	parts := strings.Split(s, "\n\n")
	var sb strings.Builder
	sb.WriteString("<html><body>")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		lines := strings.Split(stdhtml.EscapeString(p), "\n")
		sb.WriteString("<p>")
		sb.WriteString(strings.Join(lines, "<br/>"))
		sb.WriteString("</p>")
	}
	sb.WriteString("</body></html>")
	return sb.String()
}

func convertToEPUB(path string) (string, error) {
	if _, err := exec.LookPath("ebook-convert"); err != nil {
		return "", fmt.Errorf("MOBI/AZW3 support requires Calibre ebook-convert in PATH")
	}
	dst := convertedPath(path, ".epub")
	if isFresh(dst, path) {
		return dst, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return "", err
	}
	cmd := exec.Command("ebook-convert", path, dst)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ebook-convert failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return dst, nil
}

func convertPDFToText(path string) (string, error) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return "", fmt.Errorf("PDF support requires pdftotext in PATH")
	}
	dst := convertedPath(path, ".txt")
	if isFresh(dst, path) {
		return dst, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return "", err
	}
	cmd := exec.Command("pdftotext", "-layout", path, dst)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pdftotext failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return dst, nil
}

func convertedPath(path, ext string) string {
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		if home, err := os.UserHomeDir(); err == nil {
			cacheHome = filepath.Join(home, ".cache")
		} else {
			cacheHome = os.TempDir()
		}
	}
	info, _ := os.Stat(path)
	key := fmt.Sprintf("%s:%d:%d", path, infoSize(info), infoModUnix(info))
	raw := sha256.Sum256([]byte(key))
	sum := fmt.Sprintf("%x", raw[:])
	return filepath.Join(cacheHome, "epub-reader-term", "converted", sum+ext)
}

func isFresh(dst, src string) bool {
	dstInfo, err := os.Stat(dst)
	if err != nil {
		return false
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		return true
	}
	return !dstInfo.ModTime().Before(srcInfo.ModTime())
}

func infoSize(info os.FileInfo) int64 {
	if info == nil {
		return 0
	}
	return info.Size()
}

func infoModUnix(info os.FileInfo) int64 {
	if info == nil {
		return time.Now().Unix()
	}
	return info.ModTime().Unix()
}

// resolveTitles sets Section.Title using TOC, <title>, <h1>, or fallback.
func resolveTitles(book *Book) {
	// Build href → section index map
	hrefToIdx := make(map[string]int)
	for i := range book.Sections {
		hrefToIdx[book.Sections[i].Href] = i
		if book.Sections[i].Title == "" {
			book.Sections[i].Title = fmt.Sprintf("Chapter %d", i+1)
		}
	}

	// Match TOC entries to sections
	for _, entry := range book.TOC {
		href := stripFragment(entry.SectionID)
		if idx, ok := hrefToIdx[href]; ok {
			if entry.Title != "" {
				book.Sections[idx].Title = entry.Title
			}
			entry.SectionIndex = idx
		}
	}

	// Fallback: extract from HTML
	for i := range book.Sections {
		if book.Sections[i].Title != fmt.Sprintf("Chapter %d", i+1) {
			continue
		}
		if t := extractHTMLTitle(book.Sections[i].HTML); t != "" {
			book.Sections[i].Title = t
		}
	}
}

func stripFragment(href string) string {
	if idx := strings.Index(href, "#"); idx >= 0 {
		return href[:idx]
	}
	return href
}

func extractHTMLTitle(htmlStr string) string {
	// Try <title>
	re := regexp.MustCompile(`(?i)<title[^>]*>([^<]+)`)
	if m := re.FindStringSubmatch(htmlStr); len(m) > 1 {
		t := strings.TrimSpace(m[1])
		if t != "" {
			return t
		}
	}

	// Try <h1>, <h2>
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return ""
	}
	for _, tag := range []string{"h1", "h2"} {
		if t := findFirstTagText(doc, tag); t != "" {
			return t
		}
	}
	return ""
}

func findFirstTagText(n *html.Node, tag string) string {
	if n.Type == html.ElementNode && n.Data == tag {
		return textContent(n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if t := findFirstTagText(c, tag); t != "" {
			return t
		}
	}
	return ""
}

func textContent(n *html.Node) string {
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
	return strings.TrimSpace(sb.String())
}

// parseTOC tries NCX first, then nav.xhtml (EPUB 3).
func parseTOC(r *zip.ReadCloser, opf *OPF, manifest map[string]ManifestItem, baseDir string) []TOCEntry {
	// Try NCX (EPUB 2, also present in many EPUB 3)
	if opf.Spine.Toc != "" {
		// spine toc is a manifest item ID, resolve to href
		tocHref := opf.Spine.Toc
		if item, ok := manifest[opf.Spine.Toc]; ok {
			tocHref = item.Href
		}
		tocPath := filepath.Join(baseDir, tocHref)
		f := findFileInZip(r.File, tocPath)
		if f == nil {
			f = findFileInZip(r.File, strings.ReplaceAll(tocPath, `\`, "/"))
		}
		if f == nil {
			// Try the href directly
			f = findFileInZip(r.File, tocHref)
		}
		if f != nil {
			data, err := readZipFile(f)
			if err == nil {
				var ncx NCX
				if err := xml.Unmarshal(data, &ncx); err == nil && len(ncx.NavMap.NavPoints) > 0 {
					return ncxToEntries(ncx.NavMap.NavPoints)
				}
			}
		}
	}

	// Try nav.xhtml (EPUB 3)
	for _, item := range manifest {
		if item.Href == "nav.xhtml" || item.Href == "nav.html" ||
			strings.Contains(item.Properties, "nav") {
			navPath := filepath.Join(baseDir, item.Href)
			f := findFileInZip(r.File, navPath)
			if f == nil {
				f = findFileInZip(r.File, strings.ReplaceAll(navPath, `\`, "/"))
			}
			if f != nil {
				data, err := readZipFile(f)
				if err == nil {
					entries := parseNavXHTML(data)
					if len(entries) > 0 {
						return entries
					}
				}
			}
		}
	}

	return nil
}

func ncxToEntries(points []NavPoint) []TOCEntry {
	var entries []TOCEntry
	ncxWalk(points, 0, &entries)
	return entries
}

func ncxWalk(points []NavPoint, depth int, entries *[]TOCEntry) {
	for _, p := range points {
		*entries = append(*entries, TOCEntry{
			Title:     strings.TrimSpace(p.NavLabel.Text),
			SectionID: p.Content.Href,
			Depth:     depth,
		})
		if len(p.Children) > 0 {
			ncxWalk(p.Children, depth+1, entries)
		}
	}
}

// --- ZIP helpers ---

func findFileInZip(files []*zip.File, name string) *zip.File {
	for _, f := range files {
		if f.Name == name {
			return f
		}
	}
	return nil
}

func readZipFile(f *zip.File) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeZipFile(f *zip.File, v interface{}) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	return xml.NewDecoder(r).Decode(v)
}
