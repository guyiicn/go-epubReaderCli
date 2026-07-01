package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
)

func (c *Client) Login(ctx context.Context, username, password string) (LoginResponse, error) {
	var out LoginResponse
	_, err := c.doJSON(ctx, http.MethodPost, "/auth/login", LoginRequest{Username: username, Password: password}, &out, false)
	return out, err
}

func (c *Client) RegisterDevice(ctx context.Context, req DeviceRegistration) (Device, error) {
	var out Device
	_, err := c.doJSON(ctx, http.MethodPost, "/auth/devices", req, &out, true)
	return out, err
}

func (c *Client) ListBooks(ctx context.Context, since int64, limit int) (ListResult[SyncBook], error) {
	q := url.Values{}
	q.Set("since", strconv.FormatInt(since, 10))
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var raw json.RawMessage
	resp, err := c.doJSON(ctx, http.MethodGet, "/books?"+q.Encode(), nil, &raw, true)
	if err != nil {
		return ListResult[SyncBook]{}, err
	}
	return decodeList[SyncBook](raw, resp.Header.Get("X-Sync-Cursor"))
}

func (c *Client) ListProgress(ctx context.Context, since int64, limit int) (ListResult[SyncProgress], error) {
	q := url.Values{}
	q.Set("since", strconv.FormatInt(since, 10))
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var raw json.RawMessage
	resp, err := c.doJSON(ctx, http.MethodGet, "/progress?"+q.Encode(), nil, &raw, true)
	if err != nil {
		return ListResult[SyncProgress]{}, err
	}
	return decodeList[SyncProgress](raw, resp.Header.Get("X-Sync-Cursor"))
}

func (c *Client) PutProgress(ctx context.Context, serverBookID string, p ProgressUpsert) (SyncProgress, error) {
	var out SyncProgress
	_, err := c.doJSON(ctx, http.MethodPut, "/progress/"+url.PathEscape(serverBookID), p, &out, true)
	return out, err
}

func (c *Client) ListBookmarks(ctx context.Context, since int64, limit int) (ListResult[SyncBookmark], error) {
	q := url.Values{}
	q.Set("since", strconv.FormatInt(since, 10))
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var raw json.RawMessage
	resp, err := c.doJSON(ctx, http.MethodGet, "/bookmarks?"+q.Encode(), nil, &raw, true)
	if err != nil {
		return ListResult[SyncBookmark]{}, err
	}
	return decodeList[SyncBookmark](raw, resp.Header.Get("X-Sync-Cursor"))
}

func (c *Client) PostBookmark(ctx context.Context, b BookmarkUpsert) (SyncBookmark, error) {
	var out SyncBookmark
	_, err := c.doJSON(ctx, http.MethodPost, "/bookmarks", b, &out, true)
	return out, err
}

func (c *Client) DeleteBookmark(ctx context.Context, id string) error {
	_, err := c.doJSON(ctx, http.MethodDelete, "/bookmarks/"+url.PathEscape(id), nil, nil, true)
	return err
}

func (c *Client) ListAnnotations(ctx context.Context, since int64, limit int) (ListResult[SyncAnnotation], error) {
	q := url.Values{}
	q.Set("since", strconv.FormatInt(since, 10))
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var raw json.RawMessage
	resp, err := c.doJSON(ctx, http.MethodGet, "/annotations?"+q.Encode(), nil, &raw, true)
	if err != nil {
		return ListResult[SyncAnnotation]{}, err
	}
	return decodeList[SyncAnnotation](raw, resp.Header.Get("X-Sync-Cursor"))
}

func (c *Client) PostAnnotation(ctx context.Context, a AnnotationUpsert) (SyncAnnotation, error) {
	var out SyncAnnotation
	_, err := c.doJSON(ctx, http.MethodPost, "/annotations", a, &out, true)
	return out, err
}

func (c *Client) DeleteAnnotation(ctx context.Context, id string) error {
	_, err := c.doJSON(ctx, http.MethodDelete, "/annotations/"+url.PathEscape(id), nil, nil, true)
	return err
}

func (c *Client) Search(ctx context.Context, query string, page int, preference string) (SearchResult, error) {
	q := url.Values{}
	q.Set("q", query)
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if preference != "" {
		q.Set("preference", preference)
	}
	var out SearchResult
	_, err := c.doJSON(ctx, http.MethodGet, "/search?"+q.Encode(), nil, &out, true)
	return out, err
}

func (c *Client) SearchDownload(ctx context.Context, req SearchDownloadRequest) (SyncBook, error) {
	var out SyncBook
	_, err := c.doJSON(ctx, http.MethodPost, "/search/download", req, &out, true)
	return out, err
}

func (c *Client) DownloadBookFile(ctx context.Context, serverBookID, dst string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/books/"+url.PathEscape(serverBookID)+"/file", nil)
	if err != nil {
		return 0, err
	}
	st, _ := c.store.AuthState()
	if st.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+st.AccessToken)
	}
	if st.DeviceID != "" {
		req.Header.Set("X-Device-Id", st.DeviceID)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, decodeAPIError(resp)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return 0, err
	}
	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return 0, err
	}
	n, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return n, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return n, closeErr
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return n, err
	}
	return n, nil
}

func (c *Client) UploadBook(ctx context.Context, path string, meta BookUploadMetadata) (SyncBook, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	metaData, err := json.Marshal(meta)
	if err != nil {
		return SyncBook{}, err
	}
	if err := w.WriteField("metadata", string(metaData)); err != nil {
		return SyncBook{}, err
	}
	fw, err := w.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return SyncBook{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return SyncBook{}, err
	}
	if _, err := io.Copy(fw, f); err != nil {
		f.Close()
		return SyncBook{}, err
	}
	if err := f.Close(); err != nil {
		return SyncBook{}, err
	}
	if err := w.Close(); err != nil {
		return SyncBook{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/books", &body)
	if err != nil {
		return SyncBook{}, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	st, _ := c.store.AuthState()
	if st.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+st.AccessToken)
	}
	if st.DeviceID != "" {
		req.Header.Set("X-Device-Id", st.DeviceID)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return SyncBook{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SyncBook{}, decodeAPIError(resp)
	}
	var out SyncBook
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return SyncBook{}, err
	}
	return out, nil
}

func decodeList[T any](raw json.RawMessage, headerCursor string) (ListResult[T], error) {
	var result ListResult[T]
	if headerCursor != "" {
		result.Cursor, _ = strconv.ParseInt(headerCursor, 10, 64)
	}
	var arr []T
	if err := json.Unmarshal(raw, &arr); err == nil {
		result.Items = arr
		return result, nil
	}
	var obj struct {
		Items  []T   `json:"items"`
		Books  []T   `json:"books"`
		Data   []T   `json:"data"`
		Cursor int64 `json:"cursor"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return result, fmt.Errorf("decode list: %w", err)
	}
	switch {
	case obj.Items != nil:
		result.Items = obj.Items
	case obj.Books != nil:
		result.Items = obj.Books
	case obj.Data != nil:
		result.Items = obj.Data
	}
	if result.Cursor == 0 {
		result.Cursor = obj.Cursor
	}
	return result, nil
}
