package server

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"epub-reader/store"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type InvalidateMessage struct {
	Type           string `json:"type"`
	Table          string `json:"table"`
	Cursor         int64  `json:"cursor"`
	OriginDeviceID string `json:"originDeviceId"`
	TargetID       string `json:"targetId"`
	TS             int64  `json:"ts"`
}

type WSClient struct {
	baseURL string
	store   *store.Store
}

func NewWSClient(baseURL string, st *store.Store) (*WSClient, error) {
	client, err := NewClient(baseURL, st)
	if err != nil {
		return nil, err
	}
	return &WSClient{baseURL: client.baseURL, store: st}, nil
}

func (c *WSClient) Listen(ctx context.Context, out chan<- InvalidateMessage) error {
	auth, _ := c.store.AuthState()
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/v1/sync"
	q := u.Query()
	q.Set("token", auth.AccessToken)
	u.RawQuery = q.Encode()

	header := http.Header{}
	header.Set("X-Device-Id", auth.DeviceID)
	conn, _, err := websocket.Dial(ctx, u.String(), &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		return err
	}
	defer conn.CloseNow()

	for {
		var msg InvalidateMessage
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			return err
		}
		if msg.Type == "hello" {
			continue
		}
		if msg.Type != "invalidate" {
			continue
		}
		select {
		case out <- msg:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
