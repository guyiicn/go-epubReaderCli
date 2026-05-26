package epub

import (
	"encoding/xml"
	"time"
)

// Book represents an EPUB file loaded into memory.
type Book struct {
	Title    string
	Author   string
	Sections []Section
	TOC      []TOCEntry
	Meta     Metadata
}

// Section is a single chapter/section of an EPUB.
type Section struct {
	ID    string
	Href  string
	Title string
	Index int
	HTML  string
	CSS   string
}

// TOCEntry is a table-of-contents entry.
type TOCEntry struct {
	Title        string
	SectionID    string
	SectionIndex int
	Depth        int
}

// Metadata holds book metadata.
type Metadata struct {
	Title       string
	Author      string
	Language    string
	Publisher   string
	Description string
	Date        string
}

// --- XML types ---

type Container struct {
	XMLName   xml.Name  `xml:"container"`
	Rootfiles Rootfiles `xml:"rootfiles"`
}

type Rootfiles struct {
	Rootfile []Rootfile `xml:"rootfile"`
}

type Rootfile struct {
	FullPath  string `xml:"full-path,attr"`
	MediaType string `xml:"media-type,attr"`
}

type OPF struct {
	XMLName  xml.Name   `xml:"package"`
	Metadata OPFMetadata `xml:"metadata"`
	Manifest Manifest   `xml:"manifest"`
	Spine    Spine      `xml:"spine"`
}

type OPFMetadata struct {
	Title       string `xml:"http://purl.org/dc/elements/1.1/ title"`
	Author      string `xml:"http://purl.org/dc/elements/1.1/ creator"`
	Language    string `xml:"http://purl.org/dc/elements/1.1/ language"`
	Publisher   string `xml:"http://purl.org/dc/elements/1.1/ publisher"`
	Description string `xml:"http://purl.org/dc/elements/1.1/ description"`
	Date        string `xml:"http://purl.org/dc/elements/1.1/ date"`
}

type Manifest struct {
	Items []ManifestItem `xml:"item"`
}

type ManifestItem struct {
	ID        string `xml:"id,attr"`
	Href      string `xml:"href,attr"`
	MediaType string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
}

type Spine struct {
	Toc   string    `xml:"toc,attr"`
	Items []ItemRef `xml:"itemref"`
}

type ItemRef struct {
	IDRef string `xml:"idref,attr"`
}

type NCX struct {
	XMLName  xml.Name `xml:"ncx"`
	DocTitle string   `xml:"docTitle>text"`
	NavMap   NavMap   `xml:"navMap"`
}

type NavMap struct {
	NavPoints []NavPoint `xml:"navPoint"`
}

type NavPoint struct {
	ID       string     `xml:"id,attr"`
	NavLabel NavLabel   `xml:"navLabel"`
	Content  Content    `xml:"content"`
	Children []NavPoint `xml:"navPoint"`
}

type NavLabel struct {
	Text string `xml:"text"`
}

type Content struct {
	Href string `xml:"src,attr"`
}

// --- Store types (avoid circular import, defined here for sharing) ---

type LibraryEntry struct {
	Path        string    `json:"path"`
	Title       string    `json:"title"`
	Author      string    `json:"author"`
	LastOpened  time.Time `json:"last_opened"`
	AddedAt     time.Time `json:"added_at"`
}

type Progress struct {
	SectionIndex int       `json:"section_index"`
	LinePos      int       `json:"line_pos"`
	Percent      float64   `json:"percent"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Bookmark struct {
	ID           string    `json:"id"`
	SectionIndex int       `json:"section_index"`
	LinePos      int       `json:"line_pos"`
	Note         string    `json:"note"`
	CreatedAt    time.Time `json:"created_at"`
}

type Config struct {
	Columns          int `json:"columns"`
	ColumnThreshold  int `json:"column_threshold"`
	LineSpacing      int `json:"line_spacing"`
	RecentBooksLimit int `json:"recent_books_limit"`
}
