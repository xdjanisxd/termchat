package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"termchat.local/termchat/internal/protocol"
)

func TestClientWebSocketRoundTrip(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/ws" {
			t.Fatalf("websocket path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer jwt-token" {
			t.Fatalf("Authorization = %q", got)
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Fatalf("websocket.Accept() error = %v", err)
		}
		defer conn.Close(websocket.StatusNormalClosure, "test complete")
		var request protocol.ClientEvent
		if err := wsjson.Read(r.Context(), conn, &request); err != nil {
			t.Fatalf("read client event: %v", err)
		}
		if request.Type != "ping" || request.RequestID != "request-1" {
			t.Fatalf("client event = %#v", request)
		}
		if err := wsjson.Write(r.Context(), conn, protocol.ServerEvent{Type: "pong", RequestID: request.RequestID}); err != nil {
			t.Fatalf("write server event: %v", err)
		}
	}))
	defer server.Close()

	api, err := New(strings.TrimSuffix(server.URL, "/"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	api.SetToken("jwt-token")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := api.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer api.Disconnect()
	if err := api.Send(ctx, protocol.ClientEvent{Type: "ping", RequestID: "request-1"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	event, err := api.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if event.Type != "pong" || event.RequestID != "request-1" {
		t.Fatalf("Receive() = %#v", event)
	}
}
