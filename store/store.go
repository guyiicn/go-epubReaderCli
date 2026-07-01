package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"epub-reader/epub"

	_ "modernc.org/sqlite"
)

const (
	appName             = "epub-reader-term"
	schemaVersion       = "1"
	syncContractVersion = "1"
)

// Store manages persistent data for the epub reader.
type Store struct {
	mu        sync.Mutex
	configDir string
	dataDir   string
	cacheDir  string
	oldDir    string
	db        *sql.DB
	config    epub.Config
}

// Paths describes the filesystem layout used by the application.
type Paths struct {
	ConfigDir string
	DataDir   string
	CacheDir  string
	OldDir    string
	DBPath    string
	BooksDir  string
}

// AuthState is persisted auth/device state for server integration.
type AuthState struct {
	ServerURL            string `json:"server_url"`
	UserID               string `json:"user_id"`
	DeviceID             string `json:"device_id"`
	DeviceName           string `json:"device_name"`
	Platform             string `json:"platform"`
	AccessToken          string `json:"access_token"`
	RefreshToken         string `json:"refresh_token"`
	AccessTokenExpiresAt int64  `json:"access_token_expires_at"`
}

// BookRecord is the richer store model used by CLI and sync code.
type BookRecord struct {
	epub.LibraryEntry
	FileSize      int64
	TotalChapters int
	UpdatedAt     time.Time
	DeletedAt     time.Time
}

type ProgressRecord struct {
	BookID           string
	ServerBookID     string
	Locator          string
	SectionIndex     int
	LinePos          int
	TotalProgression float64
	UpdatedAt        int64
	UpdatedBy        string
}

type BookmarkRecord struct {
	ID           string
	BookID       string
	ServerBookID string
	Locator      string
	SectionIndex int
	LinePos      int
	Note         string
	Color        string
	CreatedAt    int64
	CreatedBy    string
	UpdatedAt    int64
	DeletedAt    int64
}

type AnnotationRecord struct {
	ID           string
	BookID       string
	ServerBookID string
	Locator      string
	SectionIndex int
	LinePos      int
	SelectedText string
	Note         string
	Color        string
	CreatedAt    int64
	CreatedBy    string
	UpdatedAt    int64
	DeletedAt    int64
}

// New creates or loads a Store.
func New() (*Store, error) {
	paths, err := DefaultPaths()
	if err != nil {
		return nil, err
	}
	return NewAt(paths)
}

// DefaultPaths returns the XDG-style application layout.
func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("get home dir: %w", err)
	}
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		cacheHome = filepath.Join(home, ".cache")
	}
	p := Paths{
		ConfigDir: filepath.Join(configHome, appName),
		DataDir:   filepath.Join(dataHome, appName),
		CacheDir:  filepath.Join(cacheHome, appName),
		OldDir:    filepath.Join(home, ".config", "epub-reader"),
	}
	p.DBPath = filepath.Join(p.DataDir, "reader.db")
	p.BooksDir = filepath.Join(p.DataDir, "books")
	return p, nil
}

// NewAt creates or loads a Store at the given application paths.
func NewAt(paths Paths) (*Store, error) {
	for _, dir := range []string{paths.ConfigDir, paths.DataDir, paths.CacheDir, paths.BooksDir, filepath.Join(paths.DataDir, "covers"), filepath.Join(paths.CacheDir, "converted")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create dir %s: %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite", paths.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	s := &Store{
		configDir: paths.ConfigDir,
		dataDir:   paths.DataDir,
		cacheDir:  paths.CacheDir,
		oldDir:    paths.OldDir,
		db:        db,
		config:    defaultConfig(),
	}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.loadConfig(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.migrateLegacyJSON(); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := s.EnsureDeviceID(); err != nil {
		db.Close()
		return nil, err
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

// Close closes the SQLite handle.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

// Paths returns the effective filesystem layout.
func (s *Store) Paths() Paths {
	return Paths{
		ConfigDir: s.configDir,
		DataDir:   s.dataDir,
		CacheDir:  s.cacheDir,
		OldDir:    s.oldDir,
		DBPath:    filepath.Join(s.dataDir, "reader.db"),
		BooksDir:  filepath.Join(s.dataDir, "books"),
	}
}

func (s *Store) initSchema() error {
	if _, err := s.db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("enable wal: %w", err)
	}
	if _, err := s.db.Exec(`PRAGMA foreign_keys=ON;`); err != nil {
		return fmt.Errorf("enable foreign keys: %w", err)
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS metadata (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS books (
			id TEXT PRIMARY KEY,
			server_id TEXT UNIQUE,
			content_hash TEXT,
			title TEXT NOT NULL,
			author TEXT NOT NULL DEFAULT '',
			format TEXT NOT NULL,
			original_format TEXT NOT NULL,
			file_path TEXT,
			file_size INTEGER,
			total_chapters INTEGER NOT NULL DEFAULT 0,
			cover_hash TEXT,
			cover_path TEXT,
			added_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			deleted_at INTEGER,
			last_read_at INTEGER,
			reading_status TEXT NOT NULL DEFAULT 'NONE',
			remote_only INTEGER NOT NULL DEFAULT 0,
			sync_opt_out INTEGER NOT NULL DEFAULT 0,
			dirty INTEGER NOT NULL DEFAULT 0,
			push_attempts INTEGER NOT NULL DEFAULT 0,
			conversion_status TEXT NOT NULL DEFAULT 'none',
			conversion_error TEXT,
			converted_path TEXT,
			readable_format TEXT,
			source TEXT NOT NULL DEFAULT 'local'
		);`,
		`CREATE INDEX IF NOT EXISTS idx_books_server_id ON books(server_id);`,
		`CREATE INDEX IF NOT EXISTS idx_books_content_hash ON books(content_hash);`,
		`CREATE INDEX IF NOT EXISTS idx_books_last_read_at ON books(last_read_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_books_dirty ON books(dirty);`,
		`CREATE TABLE IF NOT EXISTS progress (
			book_id TEXT PRIMARY KEY,
			section_index INTEGER NOT NULL DEFAULT 0,
			line_pos INTEGER NOT NULL DEFAULT 0,
			locator TEXT NOT NULL DEFAULT '',
			total_progression REAL NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL,
			updated_by TEXT,
			dirty INTEGER NOT NULL DEFAULT 0,
			push_attempts INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY(book_id) REFERENCES books(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS bookmarks (
			id TEXT PRIMARY KEY,
			book_id TEXT NOT NULL,
			section_index INTEGER NOT NULL DEFAULT 0,
			line_pos INTEGER NOT NULL DEFAULT 0,
			locator TEXT NOT NULL DEFAULT '',
			chapter_title TEXT,
			note TEXT,
			color TEXT NOT NULL DEFAULT '#FFC107',
			created_at INTEGER NOT NULL,
			created_by TEXT,
			updated_at INTEGER NOT NULL,
			deleted_at INTEGER,
			dirty INTEGER NOT NULL DEFAULT 0,
			push_attempts INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY(book_id) REFERENCES books(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS annotations (
			id TEXT PRIMARY KEY,
			book_id TEXT NOT NULL,
			section_index INTEGER NOT NULL DEFAULT 0,
			line_pos INTEGER NOT NULL DEFAULT 0,
			locator TEXT NOT NULL DEFAULT '',
			selected_text TEXT NOT NULL DEFAULT '',
			note TEXT,
			color TEXT NOT NULL DEFAULT '#FFC107',
			created_at INTEGER NOT NULL,
			created_by TEXT,
			updated_at INTEGER NOT NULL,
			deleted_at INTEGER,
			dirty INTEGER NOT NULL DEFAULT 0,
			push_attempts INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY(book_id) REFERENCES books(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS sync_cursors (
			table_name TEXT PRIMARY KEY,
			cursor INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS auth_state (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO metadata(key,value) VALUES('schema_version',?),('sync_contract_version',?)`, schemaVersion, syncContractVersion); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) loadConfig() error {
	if data, err := os.ReadFile(filepath.Join(s.configDir, "config.json")); err == nil {
		_ = json.Unmarshal(data, &s.config)
		return nil
	}
	if data, err := os.ReadFile(filepath.Join(s.oldDir, "config.json")); err == nil {
		_ = json.Unmarshal(data, &s.config)
		return s.SaveConfig(s.config)
	}
	return s.SaveConfig(s.config)
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
	return writeJSON(filepath.Join(s.configDir, "config.json"), c)
}

// Library returns all non-deleted library entries.
func (s *Store) Library() []epub.LibraryEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT id, COALESCE(server_id,''), COALESCE(file_path,''), title, author, format, original_format,
		COALESCE(readable_format,''), COALESCE(content_hash,''), remote_only, dirty, added_at, COALESCE(last_read_at,0)
		FROM books WHERE deleted_at IS NULL ORDER BY COALESCE(last_read_at, added_at) DESC, title COLLATE NOCASE`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []epub.LibraryEntry
	for rows.Next() {
		var e epub.LibraryEntry
		var addedAt, lastReadAt int64
		var remoteOnly, dirty int
		if err := rows.Scan(&e.ID, &e.ServerID, &e.Path, &e.Title, &e.Author, &e.Format, &e.OriginalFormat, &e.ReadableFormat, &e.ContentHash, &remoteOnly, &dirty, &addedAt, &lastReadAt); err != nil {
			continue
		}
		e.RemoteOnly = remoteOnly != 0
		e.Dirty = dirty != 0
		e.AddedAt = millisToTime(addedAt)
		e.LastOpened = millisToTime(lastReadAt)
		out = append(out, e)
	}
	return out
}

// Books returns all visible books with richer metadata.
func (s *Store) Books() ([]BookRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT id, COALESCE(server_id,''), COALESCE(file_path,''), title, author, format, original_format,
		COALESCE(readable_format,''), COALESCE(content_hash,''), COALESCE(file_size,0), total_chapters,
		remote_only, dirty, added_at, updated_at, COALESCE(last_read_at,0)
		FROM books WHERE deleted_at IS NULL ORDER BY COALESCE(last_read_at, added_at) DESC, title COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BookRecord
	for rows.Next() {
		var b BookRecord
		var addedAt, updatedAt, lastReadAt int64
		var remoteOnly, dirty int
		if err := rows.Scan(&b.ID, &b.ServerID, &b.Path, &b.Title, &b.Author, &b.Format, &b.OriginalFormat, &b.ReadableFormat,
			&b.ContentHash, &b.FileSize, &b.TotalChapters, &remoteOnly, &dirty, &addedAt, &updatedAt, &lastReadAt); err != nil {
			return nil, err
		}
		b.RemoteOnly = remoteOnly != 0
		b.Dirty = dirty != 0
		b.AddedAt = millisToTime(addedAt)
		b.UpdatedAt = millisToTime(updatedAt)
		b.LastOpened = millisToTime(lastReadAt)
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) DirtyBooksForUpload() ([]BookRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT id, COALESCE(server_id,''), COALESCE(file_path,''), title, author, format, original_format,
		COALESCE(readable_format,''), COALESCE(content_hash,''), COALESCE(file_size,0), total_chapters,
		remote_only, dirty, added_at, updated_at, COALESCE(last_read_at,0)
		FROM books
		WHERE dirty=1 AND deleted_at IS NULL AND remote_only=0 AND COALESCE(file_path,'') != ''
		ORDER BY added_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BookRecord
	for rows.Next() {
		var b BookRecord
		var addedAt, updatedAt, lastReadAt int64
		var remoteOnly, dirty int
		if err := rows.Scan(&b.ID, &b.ServerID, &b.Path, &b.Title, &b.Author, &b.Format, &b.OriginalFormat, &b.ReadableFormat,
			&b.ContentHash, &b.FileSize, &b.TotalChapters, &remoteOnly, &dirty, &addedAt, &updatedAt, &lastReadAt); err != nil {
			return nil, err
		}
		b.RemoteOnly = remoteOnly != 0
		b.Dirty = dirty != 0
		b.AddedAt = millisToTime(addedAt)
		b.UpdatedAt = millisToTime(updatedAt)
		b.LastOpened = millisToTime(lastReadAt)
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) MarkBookSynced(localID, serverID, contentHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE books SET server_id=?, content_hash=COALESCE(NULLIF(?,''), content_hash), dirty=0, push_attempts=0, updated_at=? WHERE id=?`,
		serverID, contentHash, time.Now().UnixMilli(), localID)
	return err
}

// BookByIDOrQuery returns a visible book by local id, server id, path, or title substring.
func (s *Store) BookByIDOrQuery(q string) (BookRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b BookRecord
	row := s.db.QueryRow(`SELECT id, COALESCE(server_id,''), COALESCE(file_path,''), title, author, format, original_format,
		COALESCE(readable_format,''), COALESCE(content_hash,''), COALESCE(file_size,0), total_chapters,
		remote_only, dirty, added_at, updated_at, COALESCE(deleted_at,0), COALESCE(last_read_at,0)
		FROM books
		WHERE deleted_at IS NULL AND (id=? OR server_id=? OR file_path=? OR title LIKE ?)
		ORDER BY CASE WHEN id=? OR server_id=? OR file_path=? THEN 0 ELSE 1 END, COALESCE(last_read_at, added_at) DESC
		LIMIT 1`, q, q, q, "%"+q+"%", q, q, q)
	var addedAt, updatedAt, deletedAt, lastReadAt int64
	var remoteOnly, dirty int
	if err := row.Scan(&b.ID, &b.ServerID, &b.Path, &b.Title, &b.Author, &b.Format, &b.OriginalFormat,
		&b.ReadableFormat, &b.ContentHash, &b.FileSize, &b.TotalChapters, &remoteOnly, &dirty, &addedAt, &updatedAt, &deletedAt, &lastReadAt); err != nil {
		return b, err
	}
	b.RemoteOnly = remoteOnly != 0
	b.Dirty = dirty != 0
	b.AddedAt = millisToTime(addedAt)
	b.UpdatedAt = millisToTime(updatedAt)
	b.DeletedAt = millisToTime(deletedAt)
	b.LastOpened = millisToTime(lastReadAt)
	return b, nil
}

// Cursor returns a sync cursor.
func (s *Store) Cursor(table string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var cursor int64
	err := s.db.QueryRow(`SELECT cursor FROM sync_cursors WHERE table_name=?`, table).Scan(&cursor)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return cursor, err
}

// SaveCursor stores a sync cursor.
func (s *Store) SaveCursor(table string, cursor int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT OR REPLACE INTO sync_cursors(table_name,cursor) VALUES(?,?)`, table, cursor)
	return err
}

func (s *Store) DirtyProgress() ([]ProgressRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT p.book_id, COALESCE(b.server_id,b.id), p.locator, p.section_index, p.line_pos,
		p.total_progression, p.updated_at, COALESCE(p.updated_by,'')
		FROM progress p JOIN books b ON b.id=p.book_id
		WHERE p.dirty=1 AND COALESCE(b.server_id,b.id) != '' AND b.deleted_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProgressRecord
	for rows.Next() {
		var r ProgressRecord
		if err := rows.Scan(&r.BookID, &r.ServerBookID, &r.Locator, &r.SectionIndex, &r.LinePos, &r.TotalProgression, &r.UpdatedAt, &r.UpdatedBy); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) MarkProgressSynced(bookID string, updatedAt int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE progress SET dirty=0, push_attempts=0 WHERE book_id=? AND updated_at<=?`, bookID, updatedAt)
	return err
}

func (s *Store) ApplyRemoteProgress(serverBookID, locator string, totalProgression float64, updatedAt int64, updatedBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	bookID, err := s.localBookIDByServerID(serverBookID)
	if err != nil {
		return nil
	}
	sectionIdx, linePos := parseLocator(locator)
	_, err = s.db.Exec(`INSERT INTO progress(book_id, section_index, line_pos, locator, total_progression, updated_at, updated_by, dirty)
		VALUES(?,?,?,?,?,?,?,0)
		ON CONFLICT(book_id) DO UPDATE SET
			section_index=excluded.section_index,
			line_pos=excluded.line_pos,
			locator=excluded.locator,
			total_progression=excluded.total_progression,
			updated_at=excluded.updated_at,
			updated_by=excluded.updated_by,
			dirty=0
		WHERE progress.updated_at <= excluded.updated_at`,
		bookID, sectionIdx, linePos, locator, totalProgression, updatedAt, updatedBy)
	return err
}

func (s *Store) DirtyBookmarks() ([]BookmarkRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT bm.id, bm.book_id, COALESCE(b.server_id,b.id), bm.locator, bm.section_index, bm.line_pos,
		COALESCE(bm.note,''), bm.color, bm.created_at, COALESCE(bm.created_by,''), bm.updated_at, COALESCE(bm.deleted_at,0)
		FROM bookmarks bm JOIN books b ON b.id=bm.book_id
		WHERE bm.dirty=1 AND COALESCE(b.server_id,b.id) != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BookmarkRecord
	for rows.Next() {
		var r BookmarkRecord
		if err := rows.Scan(&r.ID, &r.BookID, &r.ServerBookID, &r.Locator, &r.SectionIndex, &r.LinePos, &r.Note, &r.Color, &r.CreatedAt, &r.CreatedBy, &r.UpdatedAt, &r.DeletedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) MarkBookmarkSynced(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE bookmarks SET dirty=0, push_attempts=0 WHERE id=?`, id)
	return err
}

func (s *Store) DirtyAnnotations() ([]AnnotationRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT an.id, an.book_id, COALESCE(b.server_id,b.id), an.locator, an.section_index, an.line_pos,
		an.selected_text, COALESCE(an.note,''), an.color, an.created_at, COALESCE(an.created_by,''), an.updated_at, COALESCE(an.deleted_at,0)
		FROM annotations an JOIN books b ON b.id=an.book_id
		WHERE an.dirty=1 AND COALESCE(b.server_id,b.id) != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AnnotationRecord
	for rows.Next() {
		var r AnnotationRecord
		if err := rows.Scan(&r.ID, &r.BookID, &r.ServerBookID, &r.Locator, &r.SectionIndex, &r.LinePos, &r.SelectedText, &r.Note, &r.Color, &r.CreatedAt, &r.CreatedBy, &r.UpdatedAt, &r.DeletedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) MarkAnnotationSynced(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE annotations SET dirty=0, push_attempts=0 WHERE id=?`, id)
	return err
}

func (s *Store) ApplyRemoteAnnotation(serverBookID string, a AnnotationRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	bookID, err := s.localBookIDByServerID(serverBookID)
	if err != nil {
		return nil
	}
	sectionIdx, linePos := parseLocator(a.Locator)
	if a.Color == "" {
		a.Color = "#FFC107"
	}
	_, err = s.db.Exec(`INSERT INTO annotations(id, book_id, section_index, line_pos, locator, selected_text, note, color, created_at, created_by, updated_at, deleted_at, dirty)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,0)
		ON CONFLICT(id) DO UPDATE SET
			book_id=excluded.book_id,
			section_index=excluded.section_index,
			line_pos=excluded.line_pos,
			locator=excluded.locator,
			selected_text=excluded.selected_text,
			note=excluded.note,
			color=excluded.color,
			created_at=excluded.created_at,
			created_by=excluded.created_by,
			updated_at=excluded.updated_at,
			deleted_at=excluded.deleted_at,
			dirty=0
		WHERE annotations.updated_at <= excluded.updated_at`,
		a.ID, bookID, sectionIdx, linePos, a.Locator, a.SelectedText, a.Note, a.Color, a.CreatedAt, a.CreatedBy, a.UpdatedAt, nullMillis(a.DeletedAt))
	return err
}

func (s *Store) AddAnnotation(path, selectedText, note string, sectionIndex, linePos int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	bookID, err := s.bookIDByPath(path)
	if err != nil {
		return nil
	}
	now := time.Now().UnixMilli()
	if selectedText == "" {
		selectedText = "position note"
	}
	_, err = s.db.Exec(`INSERT INTO annotations(id, book_id, section_index, line_pos, locator, selected_text, note, color, created_at, created_by, updated_at, dirty)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,1)`,
		newID(), bookID, sectionIndex, linePos, locator(sectionIndex, linePos), selectedText, note, "#FFC107", now, s.deviceIDLocked(), now)
	return err
}

func (s *Store) ApplyRemoteBookmark(serverBookID string, b BookmarkRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	bookID, err := s.localBookIDByServerID(serverBookID)
	if err != nil {
		return nil
	}
	sectionIdx, linePos := parseLocator(b.Locator)
	if b.Color == "" {
		b.Color = "#FFC107"
	}
	_, err = s.db.Exec(`INSERT INTO bookmarks(id, book_id, section_index, line_pos, locator, note, color, created_at, created_by, updated_at, deleted_at, dirty)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,0)
		ON CONFLICT(id) DO UPDATE SET
			book_id=excluded.book_id,
			section_index=excluded.section_index,
			line_pos=excluded.line_pos,
			locator=excluded.locator,
			note=excluded.note,
			color=excluded.color,
			created_at=excluded.created_at,
			created_by=excluded.created_by,
			updated_at=excluded.updated_at,
			deleted_at=excluded.deleted_at,
			dirty=0
		WHERE bookmarks.updated_at <= excluded.updated_at`,
		b.ID, bookID, sectionIdx, linePos, b.Locator, b.Note, b.Color, b.CreatedAt, b.CreatedBy, b.UpdatedAt, nullMillis(b.DeletedAt))
	return err
}

// AddBook adds or updates a local book.
func (s *Store) AddBook(path, title, author string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	now := time.Now().UnixMilli()
	format := formatFromPath(path)
	if format == "" {
		format = "epub"
	}
	hash, size, _ := fileSHA256(path)
	id := hash
	if id == "" {
		id = newID()
	}
	_, err = s.db.Exec(`INSERT INTO books(id, content_hash, title, author, format, original_format, file_path, file_size,
		total_chapters, added_at, updated_at, last_read_at, remote_only, dirty, readable_format, source)
		VALUES(?,?,?,?,?,?,?,?,0,?,?,?,?,?,?, 'local')
		ON CONFLICT(id) DO UPDATE SET
			title=excluded.title,
			author=excluded.author,
			format=excluded.format,
			original_format=excluded.original_format,
			file_path=excluded.file_path,
			file_size=excluded.file_size,
			updated_at=excluded.updated_at,
			last_read_at=excluded.last_read_at,
			remote_only=0`,
		id, nullEmpty(hash), title, author, format, format, path, size, now, now, now, 0, 1, readableFormat(format))
	if err != nil {
		return fmt.Errorf("add book: %w", err)
	}
	return nil
}

// UpsertRemoteBook inserts or updates a server-side book row.
func (s *Store) UpsertRemoteBook(serverID, title, author, format, contentHash string, totalChapters int, deletedAt int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UnixMilli()
	id := serverID
	if id == "" {
		id = newID()
	}
	if format == "" {
		format = "epub"
	}
	_, err := s.db.Exec(`INSERT INTO books(id, server_id, content_hash, title, author, format, original_format, total_chapters,
		added_at, updated_at, deleted_at, remote_only, dirty, readable_format, source)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?, 'server')
		ON CONFLICT(id) DO UPDATE SET
			server_id=excluded.server_id,
			content_hash=COALESCE(excluded.content_hash, books.content_hash),
			title=excluded.title,
			author=excluded.author,
			format=excluded.format,
			original_format=excluded.original_format,
			total_chapters=excluded.total_chapters,
			updated_at=excluded.updated_at,
			deleted_at=excluded.deleted_at,
			remote_only=CASE WHEN books.file_path IS NULL OR books.file_path='' THEN 1 ELSE 0 END`,
		id, serverID, nullEmpty(contentHash), title, author, strings.ToLower(format), strings.ToLower(format), totalChapters, now, now, nullMillis(deletedAt), 1, 0, readableFormat(format))
	return err
}

// MarkDownloaded attaches a downloaded file to a remote book.
func (s *Store) MarkDownloaded(bookID, path, contentHash string, size int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	format := formatFromPath(path)
	if format == "" {
		format = "epub"
	}
	_, err := s.db.Exec(`UPDATE books SET file_path=?, content_hash=COALESCE(NULLIF(?,''), content_hash), file_size=?,
		format=CASE WHEN format='' THEN ? ELSE format END,
		readable_format=?, remote_only=0, updated_at=? WHERE id=? OR server_id=?`,
		path, contentHash, size, format, readableFormat(format), time.Now().UnixMilli(), bookID, bookID)
	return err
}

// BookStoragePath returns the target path for a downloaded book.
func (s *Store) BookStoragePath(contentHash, id, format string) string {
	if contentHash == "" {
		contentHash = id
	}
	ext := strings.ToLower(format)
	if ext == "" {
		ext = "epub"
	}
	if ext == "markdown" {
		ext = "md"
	}
	return filepath.Join(s.dataDir, "books", contentHash+"."+ext)
}

// RemoveBook removes a book from the local library.
func (s *Store) RemoveBook(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE books SET deleted_at=?, dirty=CASE WHEN server_id IS NULL OR server_id='' THEN 0 ELSE 1 END WHERE file_path=?`, time.Now().UnixMilli(), path)
	return err
}

// UpdateLastOpened updates the last opened time for a book.
func (s *Store) UpdateLastOpened(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE books SET last_read_at=?, updated_at=? WHERE file_path=?`, time.Now().UnixMilli(), time.Now().UnixMilli(), path)
	return err
}

// LoadProgress reads the reading progress for a book.
func (s *Store) LoadProgress(path string) (*epub.Progress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bookID, err := s.bookIDByPath(path)
	if err != nil {
		return nil, nil
	}
	var p epub.Progress
	var updatedAt int64
	var dirty int
	err = s.db.QueryRow(`SELECT section_index, line_pos, total_progression, updated_at, COALESCE(updated_by,''), dirty FROM progress WHERE book_id=?`, bookID).
		Scan(&p.SectionIndex, &p.LinePos, &p.Percent, &updatedAt, &p.UpdatedBy, &dirty)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.UpdatedAt = millisToTime(updatedAt)
	p.Dirty = dirty != 0
	return &p, nil
}

// SaveProgress persists reading progress for a book and marks it dirty.
func (s *Store) SaveProgress(path string, p epub.Progress) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	bookID, err := s.bookIDByPath(path)
	if err != nil {
		return nil
	}
	now := time.Now().UnixMilli()
	locator := locator(p.SectionIndex, p.LinePos)
	_, err = s.db.Exec(`INSERT INTO progress(book_id, section_index, line_pos, locator, total_progression, updated_at, updated_by, dirty)
		VALUES(?,?,?,?,?,?,?,1)
		ON CONFLICT(book_id) DO UPDATE SET
			section_index=excluded.section_index,
			line_pos=excluded.line_pos,
			locator=excluded.locator,
			total_progression=excluded.total_progression,
			updated_at=excluded.updated_at,
			updated_by=excluded.updated_by,
			dirty=1`,
		bookID, p.SectionIndex, p.LinePos, locator, p.Percent, now, s.deviceIDLocked())
	return err
}

// LoadBookmarks reads non-deleted bookmarks for a book.
func (s *Store) LoadBookmarks(path string) ([]epub.Bookmark, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bookID, err := s.bookIDByPath(path)
	if err != nil {
		return nil, nil
	}
	rows, err := s.db.Query(`SELECT id, section_index, line_pos, COALESCE(note,''), color, created_at, COALESCE(created_by,''), updated_at, COALESCE(deleted_at,0), dirty
		FROM bookmarks WHERE book_id=? AND deleted_at IS NULL ORDER BY created_at`, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []epub.Bookmark
	for rows.Next() {
		var bm epub.Bookmark
		var createdAt, updatedAt, deletedAt int64
		var dirty int
		if err := rows.Scan(&bm.ID, &bm.SectionIndex, &bm.LinePos, &bm.Note, &bm.Color, &createdAt, &bm.CreatedBy, &updatedAt, &deletedAt, &dirty); err != nil {
			return nil, err
		}
		bm.CreatedAt = millisToTime(createdAt)
		bm.UpdatedAt = millisToTime(updatedAt)
		bm.DeletedAt = millisToTime(deletedAt)
		bm.Dirty = dirty != 0
		out = append(out, bm)
	}
	return out, nil
}

// SaveBookmarks replaces visible bookmarks for a book and marks changed rows dirty.
func (s *Store) SaveBookmarks(path string, bm []epub.Bookmark) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	bookID, err := s.bookIDByPath(path)
	if err != nil {
		return nil
	}
	deviceID := s.deviceIDLocked()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	for _, b := range bm {
		if b.ID == "" {
			b.ID = newID()
		}
		if b.Color == "" {
			b.Color = "#FFC107"
		}
		createdAt := b.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now()
		}
		_, err = tx.Exec(`INSERT INTO bookmarks(id, book_id, section_index, line_pos, locator, note, color, created_at, created_by, updated_at, dirty)
			VALUES(?,?,?,?,?,?,?,?,?,?,1)
			ON CONFLICT(id) DO UPDATE SET
				section_index=excluded.section_index,
				line_pos=excluded.line_pos,
				locator=excluded.locator,
				note=excluded.note,
				color=excluded.color,
				updated_at=excluded.updated_at,
				deleted_at=NULL,
				dirty=1`,
			b.ID, bookID, b.SectionIndex, b.LinePos, locator(b.SectionIndex, b.LinePos), b.Note, b.Color, createdAt.UnixMilli(), deviceID, now)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DeleteBookmark tombstones a bookmark row.
func (s *Store) DeleteBookmark(path, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	bookID, err := s.bookIDByPath(path)
	if err != nil {
		return nil
	}
	_, err = s.db.Exec(`UPDATE bookmarks SET deleted_at=?, updated_at=?, dirty=1 WHERE book_id=? AND id=?`, time.Now().UnixMilli(), time.Now().UnixMilli(), bookID, id)
	return err
}

// AuthState loads persisted auth state.
func (s *Store) AuthState() (AuthState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var st AuthState
	rows, err := s.db.Query(`SELECT key, value FROM auth_state`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return st, err
		}
		switch key {
		case "server_url":
			st.ServerURL = value
		case "user_id":
			st.UserID = value
		case "device_id":
			st.DeviceID = value
		case "device_name":
			st.DeviceName = value
		case "platform":
			st.Platform = value
		case "access_token":
			st.AccessToken = value
		case "refresh_token":
			st.RefreshToken = value
		case "access_token_expires_at":
			fmt.Sscanf(value, "%d", &st.AccessTokenExpiresAt)
		}
	}
	if st.Platform == "" {
		st.Platform = "CLI"
	}
	if st.DeviceName == "" {
		host, _ := os.Hostname()
		if host == "" {
			host = "terminal"
		}
		st.DeviceName = host + " terminal"
	}
	return st, nil
}

// SaveAuthState persists auth state.
func (s *Store) SaveAuthState(st AuthState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveAuthStateLocked(st)
}

func (s *Store) saveAuthStateLocked(st AuthState) error {
	if st.DeviceID == "" {
		st.DeviceID = s.deviceIDLocked()
	}
	if st.Platform == "" {
		st.Platform = "CLI"
	}
	if st.DeviceName == "" {
		host, _ := os.Hostname()
		st.DeviceName = host + " terminal"
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	values := map[string]string{
		"server_url":              st.ServerURL,
		"user_id":                 st.UserID,
		"device_id":               st.DeviceID,
		"device_name":             st.DeviceName,
		"platform":                st.Platform,
		"access_token":            st.AccessToken,
		"refresh_token":           st.RefreshToken,
		"access_token_expires_at": fmt.Sprintf("%d", st.AccessTokenExpiresAt),
	}
	for k, v := range values {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO auth_state(key,value) VALUES(?,?)`, k, v); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ClearAuthTokens logs out locally while preserving the stable device id.
func (s *Store) ClearAuthTokens() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM auth_state WHERE key IN ('user_id','access_token','refresh_token','access_token_expires_at')`)
	return err
}

// EnsureDeviceID returns the stable local device id, generating one if needed.
func (s *Store) EnsureDeviceID() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.deviceIDLocked()
	if id != "" {
		host, _ := os.Hostname()
		if host == "" {
			host = "terminal"
		}
		_, _ = s.db.Exec(`INSERT OR IGNORE INTO auth_state(key,value) VALUES('platform','CLI')`)
		_, _ = s.db.Exec(`INSERT OR IGNORE INTO auth_state(key,value) VALUES('device_name',?)`, host+" terminal")
		return id, nil
	}
	id = newID()
	host, _ := os.Hostname()
	if host == "" {
		host = "terminal"
	}
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT OR REPLACE INTO auth_state(key,value) VALUES('device_id',?)`, id); err != nil {
		return "", err
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO auth_state(key,value) VALUES('platform','CLI')`); err != nil {
		return "", err
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO auth_state(key,value) VALUES('device_name',?)`, host+" terminal"); err != nil {
		return "", err
	}
	return id, tx.Commit()
}

func (s *Store) deviceIDLocked() string {
	var id string
	_ = s.db.QueryRow(`SELECT value FROM auth_state WHERE key='device_id'`).Scan(&id)
	return id
}

func (s *Store) bookIDByPath(path string) (string, error) {
	var id string
	err := s.db.QueryRow(`SELECT id FROM books WHERE file_path=? AND deleted_at IS NULL`, path).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		if abs, absErr := filepath.Abs(path); absErr == nil && abs != path {
			err = s.db.QueryRow(`SELECT id FROM books WHERE file_path=? AND deleted_at IS NULL`, abs).Scan(&id)
		}
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) localBookIDByServerID(serverBookID string) (string, error) {
	var id string
	err := s.db.QueryRow(`SELECT id FROM books WHERE deleted_at IS NULL AND (server_id=? OR id=?)`, serverBookID, serverBookID).Scan(&id)
	return id, err
}

func (s *Store) migrateLegacyJSON() error {
	var marker string
	_ = s.db.QueryRow(`SELECT value FROM metadata WHERE key='migrated_from_json_at'`).Scan(&marker)
	if marker != "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(s.oldDir, "library.json"))
	if err != nil {
		_, _ = s.db.Exec(`INSERT OR REPLACE INTO metadata(key,value) VALUES('migrated_from_json_at',?)`, time.Now().Format(time.RFC3339))
		return nil
	}
	var legacy []epub.LibraryEntry
	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}
	for _, e := range legacy {
		if e.Path == "" {
			continue
		}
		if err := s.AddBook(e.Path, e.Title, e.Author); err != nil {
			continue
		}
		hash := legacyBookHash(e.Path)
		if pdata, err := os.ReadFile(filepath.Join(s.oldDir, "progress", hash+".json")); err == nil {
			var p epub.Progress
			if json.Unmarshal(pdata, &p) == nil {
				_ = s.SaveProgress(e.Path, p)
			}
		}
		if bdata, err := os.ReadFile(filepath.Join(s.oldDir, "bookmarks", hash+".json")); err == nil {
			var bms []epub.Bookmark
			if json.Unmarshal(bdata, &bms) == nil {
				_ = s.SaveBookmarks(e.Path, bms)
			}
		}
	}
	_, err = s.db.Exec(`INSERT OR REPLACE INTO metadata(key,value) VALUES('migrated_from_json_at',?)`, time.Now().Format(time.RFC3339))
	return err
}

func formatFromPath(path string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	switch ext {
	case "markdown":
		return "md"
	case "epub", "txt", "md", "mobi", "azw3", "pdf":
		return ext
	default:
		return ext
	}
}

func readableFormat(format string) string {
	switch strings.ToLower(format) {
	case "mobi", "azw3":
		return "epub"
	case "pdf", "txt", "md", "markdown":
		return "txt"
	default:
		return "epub"
	}
}

func fileSHA256(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func locator(sectionIndex, linePos int) string {
	return fmt.Sprintf(`{"sectionIndex":%d,"linePos":%d}`, sectionIndex, linePos)
}

func parseLocator(s string) (int, int) {
	var v struct {
		SectionIndex int `json:"sectionIndex"`
		LinePos      int `json:"linePos"`
	}
	if json.Unmarshal([]byte(s), &v) == nil {
		return v.SectionIndex, v.LinePos
	}
	return 0, 0
}

func legacyBookHash(path string) string {
	h := sha256.Sum256([]byte(path))
	return hex.EncodeToString(h[:8])
}

func nullEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullMillis(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}

func millisToTime(v int64) time.Time {
	if v <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(v)
}

func writeJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
