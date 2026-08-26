package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"termchat.local/termchat/internal/protocol"
)

var (
	ErrNotAuthenticated = errors.New("authenticate before connecting to chat")
	ErrNotConnected     = errors.New("websocket is not connected")
	ErrAlreadyConnected = errors.New("websocket is already connected")
)

func (c *Client) Connect(ctx context.Context) error {
	token := c.Token()
	if token == "" {
		return ErrNotAuthenticated
	}

	c.connMu.RLock()
	alreadyConnected := c.conn != nil
	c.connMu.RUnlock()
	if alreadyConnected {
		return ErrAlreadyConnected
	}

	websocketURL, err := c.websocketURL()
	if err != nil {
		return err
	}
	conn, response, err := websocket.Dial(ctx, websocketURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": []string{"Bearer " + token}},
	})
	if err != nil {
		if response != nil {
			defer response.Body.Close()
			return &APIError{Status: response.StatusCode, Code: "WEBSOCKET_HANDSHAKE_FAILED", Message: "Chat connection was rejected."}
		}
		return fmt.Errorf("connect websocket: %w", err)
	}
	conn.SetReadLimit(16 * 1024)

	c.connMu.Lock()
	if c.conn != nil {
		c.connMu.Unlock()
		_ = conn.Close(websocket.StatusNormalClosure, "duplicate connection")
		return ErrAlreadyConnected
	}
	c.conn = conn
	c.connMu.Unlock()
	return nil
}

func (c *Client) Send(ctx context.Context, event protocol.ClientEvent) error {
	conn, err := c.connection()
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := wsjson.Write(ctx, conn, event); err != nil {
		return fmt.Errorf("send websocket event: %w", err)
	}
	return nil
}

func (c *Client) Receive(ctx context.Context) (protocol.ServerEvent, error) {
	conn, err := c.connection()
	if err != nil {
		return protocol.ServerEvent{}, err
	}
	var event protocol.ServerEvent
	if err := wsjson.Read(ctx, conn, &event); err != nil {
		return protocol.ServerEvent{}, fmt.Errorf("receive websocket event: %w", err)
	}
	return event, nil
}

func (c *Client) Disconnect() error {
	c.connMu.Lock()
	conn := c.conn
	c.conn = nil
	c.connMu.Unlock()
	if conn == nil {
		return nil
	}
	return conn.Close(websocket.StatusNormalClosure, "client disconnected")
}

func (c *Client) connection() (*websocket.Conn, error) {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	if c.conn == nil {
		return nil, ErrNotConnected
	}
	return c.conn, nil
}

func (c *Client) websocketURL() (string, error) {
	parsed, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse server URL: %w", err)
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	default:
		return "", errors.New("server URL must use http or https")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/v1/ws"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}
