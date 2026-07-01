package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"epub-reader/store"
)

type Client struct {
	baseURL string
	store   *store.Store
	http    *http.Client
	mu      sync.Mutex
}

func NewClient(baseURL string, st *store.Store) (*Client, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		auth, _ := st.AuthState()
		baseURL = auth.ServerURL
	}
	if baseURL == "" {
		return nil, fmt.Errorf("server URL is not configured; run login --server first")
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid server URL %q", baseURL)
	}
	return &Client{
		baseURL: baseURL,
		store:   st,
		http:    &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, in any, out any, auth bool) (*http.Response, error) {
	return c.doJSONAttempt(ctx, method, path, in, out, auth, true)
}

func (c *Client) doJSONAttempt(ctx context.Context, method, path string, in any, out any, auth bool, allowRefresh bool) (*http.Response, error) {
	var data []byte
	var body io.Reader
	if in != nil {
		var err error
		data, err = json.Marshal(in)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+"/api/v1"+path, body)
	if err != nil {
		return nil, err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth {
		st, _ := c.store.AuthState()
		if st.AccessToken != "" {
			req.Header.Set("Authorization", "Bearer "+st.AccessToken)
		}
		if st.DeviceID != "" {
			req.Header.Set("X-Device-Id", st.DeviceID)
		}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if auth && allowRefresh && resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		if err := c.refresh(ctx); err != nil {
			return nil, err
		}
		return c.doJSONAttempt(ctx, method, path, in, out, auth, false)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return resp, decodeAPIError(resp)
	}
	if out != nil {
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp, err
		}
	}
	return resp, nil
}

func (c *Client) refresh(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	st, err := c.store.AuthState()
	if err != nil {
		return err
	}
	if st.RefreshToken == "" {
		return APIError{Status: http.StatusUnauthorized, Title: "needs_login", Detail: "missing refresh token"}
	}
	payload, err := json.Marshal(RefreshRequest{RefreshToken: st.RefreshToken})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/auth/refresh", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = c.store.ClearAuthTokens()
		return decodeAPIError(resp)
	}
	var login LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&login); err != nil {
		return err
	}
	st.UserID = login.UserID
	st.AccessToken = login.AccessToken
	st.RefreshToken = login.RefreshToken
	st.AccessTokenExpiresAt = login.AccessTokenExpiresAt
	return c.store.SaveAuthState(st)
}

func decodeAPIError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var p struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
		Type   string `json:"type"`
		Error  string `json:"error"`
	}
	_ = json.Unmarshal(data, &p)
	if p.Detail == "" {
		p.Detail = p.Error
	}
	return APIError{Status: resp.StatusCode, Title: p.Title, Detail: p.Detail, Type: p.Type}
}
