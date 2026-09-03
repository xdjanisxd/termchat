package observability

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestRedactingHandlerPreservesSafeMetadataAndRedactsSensitiveAttributes(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewTextHandler(&output, nil)))
	logger.LogAttrs(context.Background(), slog.LevelInfo, "request completed",
		slog.String("request_id", "request-1"),
		slog.String("username", "alice"),
		slog.String("token", "jwt-secret-value"),
		slog.String("password", "correct-horse-battery-staple"),
		slog.String("content", "private chat content"),
		slog.Any("error", errors.New("postgres://termchat:db-secret@db:5432/termchat")),
	)

	logged := output.String()
	for _, safe := range []string{"request completed", "request-1", "alice"} {
		if !strings.Contains(logged, safe) {
			t.Fatalf("log omitted safe metadata %q: %s", safe, logged)
		}
	}
	for _, secret := range []string{"jwt-secret-value", "correct-horse-battery-staple", "private chat content", "db-secret"} {
		if strings.Contains(logged, secret) {
			t.Fatalf("log leaked %q: %s", secret, logged)
		}
	}
	if got := strings.Count(logged, "[REDACTED]"); got != 4 {
		t.Fatalf("redacted values = %d, want 4: %s", got, logged)
	}
}

func TestRedactingHandlerRedactsSensitiveNestedGroupAttributes(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewTextHandler(&output, nil)))
	logger.LogAttrs(context.Background(), slog.LevelInfo, "websocket event",
		slog.Group("event", slog.String("type", "send_message"), slog.String("content", "ephemeral secret")),
	)

	logged := output.String()
	if strings.Contains(logged, "ephemeral secret") || !strings.Contains(logged, "[REDACTED]") {
		t.Fatalf("nested sensitive attribute was not redacted: %s", logged)
	}
	if !strings.Contains(logged, "send_message") {
		t.Fatalf("nested safe attribute was removed: %s", logged)
	}
}
