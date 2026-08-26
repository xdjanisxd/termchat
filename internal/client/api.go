package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"termchat.local/termchat/internal/domain"
)

const maxResponseBody = 1 << 20

type Session struct {
	User  domain.PublicUser `json:"user"`
	Token string            `json:"token"`
}

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("server returned HTTP %d", e.Status)
}

type Client struct {
	baseURL    string
	httpClient *http.Client

	mu    sync.RWMutex
	token string

	connMu  sync.RWMutex
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func New(serverURL string) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(serverURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, errors.New("server URL must be an absolute http or https URL")
	}
	return &Client{
		baseURL: strings.TrimRight(parsed.String(), "/"),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}, nil
}

func (c *Client) Register(ctx context.Context, username, password string) (Session, error) {
	return c.authenticate(ctx, "/v1/auth/register", username, password)
}

func (c *Client) Login(ctx context.Context, username, password string) (Session, error) {
	return c.authenticate(ctx, "/v1/auth/login", username, password)
}

func (c *Client) Token() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token
}

func (c *Client) SetToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = token
}

func (c *Client) authenticate(ctx context.Context, path, username, password string) (Session, error) {
	payload, err := json.Marshal(map[string]string{"username": username, "password": password})
	if err != nil {
		return Session{}, fmt.Errorf("encode credentials: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return Session{}, fmt.Errorf("create authentication request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return Session{}, fmt.Errorf("contact server: %w", err)
	}
	defer response.Body.Close()
	body := io.LimitReader(response.Body, maxResponseBody)

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var envelope struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.NewDecoder(body).Decode(&envelope); err != nil {
			return Session{}, &APIError{Status: response.StatusCode, Code: "HTTP_ERROR"}
		}
		return Session{}, &APIError{
			Status: response.StatusCode, Code: envelope.Error.Code, Message: envelope.Error.Message,
		}
	}

	var session Session
	if err := json.NewDecoder(body).Decode(&session); err != nil {
		return Session{}, fmt.Errorf("decode authentication response: %w", err)
	}
	if session.Token == "" || session.User.ID == "" || session.User.Username == "" {
		return Session{}, errors.New("server returned an incomplete authentication session")
	}
	c.SetToken(session.Token)
	return session, nil
}
