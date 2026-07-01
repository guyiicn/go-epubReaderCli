package server

import (
	"context"
	"fmt"
	"strings"

	"epub-reader/store"
)

type Engine struct {
	store  *store.Store
	client *Client
}

func NewEngine(st *store.Store, client *Client) *Engine {
	return &Engine{store: st, client: client}
}

func (e *Engine) PullBooks(ctx context.Context) (int, error) {
	cursor, err := e.store.Cursor("books")
	if err != nil {
		return 0, err
	}
	res, err := e.client.ListBooks(ctx, cursor, 500)
	if err != nil {
		return 0, err
	}
	for _, b := range res.Items {
		var deletedAt int64
		if b.DeletedAt != nil {
			deletedAt = *b.DeletedAt
		}
		if err := e.store.UpsertRemoteBook(b.ID, b.Title, b.Author, b.Format, b.ContentHash, b.TotalChapters, deletedAt); err != nil {
			return 0, err
		}
	}
	if res.Cursor > cursor {
		if err := e.store.SaveCursor("books", res.Cursor); err != nil {
			return 0, err
		}
	}
	return len(res.Items), nil
}

func (e *Engine) PullProgress(ctx context.Context) (int, error) {
	cursor, err := e.store.Cursor("progress")
	if err != nil {
		return 0, err
	}
	res, err := e.client.ListProgress(ctx, cursor, 500)
	if err != nil {
		return 0, err
	}
	for _, p := range res.Items {
		updatedBy := p.UpdatedBy
		if updatedBy == "" {
			updatedBy = p.DeviceID
		}
		if err := e.store.ApplyRemoteProgress(p.BookID, p.Locator, p.TotalProgression, p.UpdatedAt, updatedBy); err != nil {
			return 0, err
		}
	}
	if res.Cursor > cursor {
		if err := e.store.SaveCursor("progress", res.Cursor); err != nil {
			return 0, err
		}
	}
	return len(res.Items), nil
}

func (e *Engine) PullBookmarks(ctx context.Context) (int, error) {
	cursor, err := e.store.Cursor("bookmarks")
	if err != nil {
		return 0, err
	}
	res, err := e.client.ListBookmarks(ctx, cursor, 500)
	if err != nil {
		return 0, err
	}
	for _, bm := range res.Items {
		var deletedAt int64
		if bm.DeletedAt != nil {
			deletedAt = *bm.DeletedAt
		}
		rec := store.BookmarkRecord{
			ID:        bm.ID,
			Locator:   bm.Locator,
			Note:      bm.Note,
			Color:     bm.Color,
			CreatedAt: bm.CreatedAt,
			CreatedBy: bm.CreatedBy,
			UpdatedAt: bm.UpdatedAt,
			DeletedAt: deletedAt,
		}
		if err := e.store.ApplyRemoteBookmark(bm.BookID, rec); err != nil {
			return 0, err
		}
	}
	if res.Cursor > cursor {
		if err := e.store.SaveCursor("bookmarks", res.Cursor); err != nil {
			return 0, err
		}
	}
	return len(res.Items), nil
}

func (e *Engine) PullAnnotations(ctx context.Context) (int, error) {
	cursor, err := e.store.Cursor("annotations")
	if err != nil {
		return 0, err
	}
	res, err := e.client.ListAnnotations(ctx, cursor, 500)
	if err != nil {
		return 0, err
	}
	for _, an := range res.Items {
		var deletedAt int64
		if an.DeletedAt != nil {
			deletedAt = *an.DeletedAt
		}
		rec := store.AnnotationRecord{
			ID:           an.ID,
			Locator:      an.Locator,
			SelectedText: an.SelectedText,
			Note:         an.Note,
			Color:        an.Color,
			CreatedAt:    an.CreatedAt,
			CreatedBy:    an.CreatedBy,
			UpdatedAt:    an.UpdatedAt,
			DeletedAt:    deletedAt,
		}
		if err := e.store.ApplyRemoteAnnotation(an.BookID, rec); err != nil {
			return 0, err
		}
	}
	if res.Cursor > cursor {
		if err := e.store.SaveCursor("annotations", res.Cursor); err != nil {
			return 0, err
		}
	}
	return len(res.Items), nil
}

func (e *Engine) PushProgress(ctx context.Context) (int, error) {
	auth, _ := e.store.AuthState()
	rows, err := e.store.DirtyProgress()
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		req := ProgressUpsert{
			Locator:          row.Locator,
			TotalProgression: row.TotalProgression,
			UpdatedAt:        row.UpdatedAt,
			DeviceID:         auth.DeviceID,
		}
		resp, err := e.client.PutProgress(ctx, row.ServerBookID, req)
		if err != nil {
			return 0, err
		}
		updatedAt := resp.UpdatedAt
		if updatedAt == 0 {
			updatedAt = row.UpdatedAt
		}
		if err := e.store.MarkProgressSynced(row.BookID, updatedAt); err != nil {
			return 0, err
		}
	}
	return len(rows), nil
}

func (e *Engine) PushBooks(ctx context.Context) (int, error) {
	rows, err := e.store.DirtyBooksForUpload()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, row := range rows {
		format := strings.ToUpper(row.Format)
		switch format {
		case "EPUB", "TXT", "MD", "MARKDOWN", "MOBI":
		default:
			continue
		}
		if format == "MD" {
			format = "MARKDOWN"
		}
		book, err := e.client.UploadBook(ctx, row.Path, BookUploadMetadata{
			Title:         row.Title,
			Author:        row.Author,
			Format:        format,
			TotalChapters: row.TotalChapters,
			AddedAt:       row.AddedAt.UnixMilli(),
		})
		if err != nil {
			return count, err
		}
		if err := e.store.MarkBookSynced(row.ID, book.ID, book.ContentHash); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (e *Engine) PushBookmarks(ctx context.Context) (int, error) {
	auth, _ := e.store.AuthState()
	rows, err := e.store.DirtyBookmarks()
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		if row.DeletedAt > 0 {
			if err := e.client.DeleteBookmark(ctx, row.ID); err != nil {
				return 0, err
			}
			if err := e.store.MarkBookmarkSynced(row.ID); err != nil {
				return 0, err
			}
			continue
		}
		color := row.Color
		if color == "" {
			color = "#FFC107"
		}
		_, err := e.client.PostBookmark(ctx, BookmarkUpsert{
			ID:        row.ID,
			BookID:    row.ServerBookID,
			Locator:   row.Locator,
			Note:      row.Note,
			Color:     color,
			CreatedAt: row.CreatedAt,
			DeviceID:  auth.DeviceID,
		})
		if err != nil {
			return 0, err
		}
		if err := e.store.MarkBookmarkSynced(row.ID); err != nil {
			return 0, err
		}
	}
	return len(rows), nil
}

func (e *Engine) PushAnnotations(ctx context.Context) (int, error) {
	auth, _ := e.store.AuthState()
	rows, err := e.store.DirtyAnnotations()
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		if row.DeletedAt > 0 {
			if err := e.client.DeleteAnnotation(ctx, row.ID); err != nil {
				return 0, err
			}
			if err := e.store.MarkAnnotationSynced(row.ID); err != nil {
				return 0, err
			}
			continue
		}
		color := row.Color
		if color == "" {
			color = "#FFC107"
		}
		_, err := e.client.PostAnnotation(ctx, AnnotationUpsert{
			ID:           row.ID,
			BookID:       row.ServerBookID,
			Locator:      row.Locator,
			SelectedText: row.SelectedText,
			Note:         row.Note,
			Color:        color,
			CreatedAt:    row.CreatedAt,
			DeviceID:     auth.DeviceID,
		})
		if err != nil {
			return 0, err
		}
		if err := e.store.MarkAnnotationSynced(row.ID); err != nil {
			return 0, err
		}
	}
	return len(rows), nil
}

func (e *Engine) Sync(ctx context.Context) error {
	if _, err := e.PullBooks(ctx); err != nil {
		return err
	}
	if _, err := e.PullProgress(ctx); err != nil {
		return err
	}
	if _, err := e.PullBookmarks(ctx); err != nil {
		return err
	}
	if _, err := e.PullAnnotations(ctx); err != nil {
		return err
	}
	if _, err := e.PushBooks(ctx); err != nil {
		return err
	}
	if _, err := e.PushProgress(ctx); err != nil {
		return err
	}
	if _, err := e.PushBookmarks(ctx); err != nil {
		return err
	}
	if _, err := e.PushAnnotations(ctx); err != nil {
		return err
	}
	if _, err := e.PullBooks(ctx); err != nil {
		return err
	}
	if _, err := e.PullProgress(ctx); err != nil {
		return err
	}
	if _, err := e.PullBookmarks(ctx); err != nil {
		return err
	}
	if _, err := e.PullAnnotations(ctx); err != nil {
		return err
	}
	return nil
}

func LoginAndRegister(ctx context.Context, st *store.Store, serverURL, username, password string) (store.AuthState, error) {
	client, err := NewClient(serverURL, st)
	if err != nil {
		return store.AuthState{}, err
	}
	login, err := client.Login(ctx, username, password)
	if err != nil {
		return store.AuthState{}, err
	}
	deviceID, err := st.EnsureDeviceID()
	if err != nil {
		return store.AuthState{}, err
	}
	auth := store.AuthState{
		ServerURL:            serverURL,
		UserID:               login.UserID,
		DeviceID:             deviceID,
		Platform:             "CLI",
		AccessToken:          login.AccessToken,
		RefreshToken:         login.RefreshToken,
		AccessTokenExpiresAt: login.AccessTokenExpiresAt,
	}
	if err := st.SaveAuthState(auth); err != nil {
		return store.AuthState{}, err
	}
	auth, _ = st.AuthState()
	client, err = NewClient(serverURL, st)
	if err != nil {
		return store.AuthState{}, err
	}
	if _, err := client.RegisterDevice(ctx, DeviceRegistration{DeviceID: auth.DeviceID, Name: auth.DeviceName, Platform: "CLI"}); err != nil {
		return auth, fmt.Errorf("register device: %w", err)
	}
	return auth, nil
}
