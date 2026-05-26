package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"epub-reader/epub"
)

// Store manages persistent data for the epub reader.
type Store struct {
	mu       sync.Mutex
	configDir string
	config    epub.Config
	library   []epub.LibraryEntry
}

// New creates or loads a Store. Default config dir: ~/.config/epub-reader
func New() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}
	return NewAt(filepath.Join(home, ".config", "epub-reader"))
}

// NewAt creates or loads a Store at the given directory.
func NewAt(dir string) (*Store, error) {
	s := &Store{configDir: dir}

	// Ensure directories exist
	for _, sub := range []string{"", "progress", "bookmarks"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
			return nil, fmt.Errorf("create dir %s: %w", sub, err)
		}
	}

	// Load config
	s.config = defaultConfig()
	if data, err := os.ReadFile(filepath.Join(dir, "config.json")); err == nil {
		json.Unmarshal(data, &s.config)
	}

	// Load library
	s.library = nil
	if data, err := os.ReadFile(filepath.Join(dir, "library.json")); err == nil {
		json.Unmarshal(data, &s.library)
	}

	return s, nil
}

func defaultConfig() epub.Config {
	return epub.Config{
		Columns:          0,
		ColumnThreshold:  120,
		LineSpacing:      0,
		RecentBooksLimit: 100,
	}
}

// Config returns the current config.
func (s *Store) Config() epub.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

// SaveConfig persists the config.
func (s *Store) SaveConfig(c epub.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = c
	return s.writeJSON("config.json", c)
}

// Library returns all library entries.
func (s *Store) Library() []epub.LibraryEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]epub.LibraryEntry, len(s.library))
	copy(out, s.library)
	return out
}

// AddBook adds a book to the library.
func (s *Store) AddBook(path, title, author string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already exists
	for i, e := range s.library {
		if e.Path == path {
			s.library[i].Title = title
			s.library[i].Author = author
			s.library[i].LastOpened = time.Now()
			return s.writeJSON("library.json", s.library)
		}
	}

	s.library = append(s.library, epub.LibraryEntry{
		Path:       path,
		Title:      title,
		Author:     author,
		LastOpened: time.Now(),
		AddedAt:    time.Now(),
	})

	return s.writeJSON("library.json", s.library)
}

// RemoveBook removes a book from the library.
func (s *Store) RemoveBook(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, e := range s.library {
		if e.Path == path {
			s.library = append(s.library[:i], s.library[i+1:]...)
			break
		}
	}

	// Also remove progress and bookmarks
	hash := bookHash(path)
	os.Remove(filepath.Join(s.configDir, "progress", hash+".json"))
	os.Remove(filepath.Join(s.configDir, "bookmarks", hash+".json"))

	return s.writeJSON("library.json", s.library)
}

// UpdateLastOpened updates the last opened time for a book.
func (s *Store) UpdateLastOpened(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, e := range s.library {
		if e.Path == path {
			s.library[i].LastOpened = time.Now()
			return s.writeJSON("library.json", s.library)
		}
	}
	return nil
}

// LoadProgress reads the reading progress for a book.
func (s *Store) LoadProgress(path string) (*epub.Progress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	hash := bookHash(path)
	data, err := os.ReadFile(filepath.Join(s.configDir, "progress", hash+".json"))
	if err != nil {
		return nil, nil // no progress saved
	}
	var p epub.Progress
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// SaveProgress persists reading progress for a book.
func (s *Store) SaveProgress(path string, p epub.Progress) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p.UpdatedAt = time.Now()
	hash := bookHash(path)
	return s.writeJSON(filepath.Join("progress", hash+".json"), p)
}

// LoadBookmarks reads bookmarks for a book.
func (s *Store) LoadBookmarks(path string) ([]epub.Bookmark, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	hash := bookHash(path)
	data, err := os.ReadFile(filepath.Join(s.configDir, "bookmarks", hash+".json"))
	if err != nil {
		return nil, nil
	}
	var bm []epub.Bookmark
	if err := json.Unmarshal(data, &bm); err != nil {
		return nil, err
	}
	return bm, nil
}

// SaveBookmarks persists bookmarks for a book.
func (s *Store) SaveBookmarks(path string, bm []epub.Bookmark) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hash := bookHash(path)
	return s.writeJSON(filepath.Join("bookmarks", hash+".json"), bm)
}

func bookHash(path string) string {
	h := sha256.Sum256([]byte(path))
	return hex.EncodeToString(h[:8])
}

func (s *Store) writeJSON(name string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.configDir, name), data, 0644)
}
