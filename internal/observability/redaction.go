package observability

import (
	"context"
	"log/slog"
	"strings"
)

const redactedValue = "[REDACTED]"

type redactingHandler struct {
	next slog.Handler
}

func NewRedactingHandler(next slog.Handler) slog.Handler {
	return &redactingHandler{next: next}
}

func (h *redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *redactingHandler) Handle(ctx context.Context, record slog.Record) error {
	redacted := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		redacted.AddAttrs(redactAttr(attr))
		return true
	})
	return h.next.Handle(ctx, redacted)
}

func (h *redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		redacted = append(redacted, redactAttr(attr))
	}
	return &redactingHandler{next: h.next.WithAttrs(redacted)}
}

func (h *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{next: h.next.WithGroup(name)}
}

func redactAttr(attr slog.Attr) slog.Attr {
	if isSensitiveKey(attr.Key) {
		return slog.String(attr.Key, redactedValue)
	}
	if attr.Value.Kind() != slog.KindGroup {
		return attr
	}
	group := attr.Value.Group()
	redacted := make([]slog.Attr, 0, len(group))
	for _, child := range group {
		redacted = append(redacted, redactAttr(child))
	}
	return slog.Attr{Key: attr.Key, Value: slog.GroupValue(redacted...)}
}

func isSensitiveKey(key string) bool {
	key = strings.ToLower(key)
	for _, marker := range []string{"password", "secret", "token", "authorization", "content", "message", "body", "error", "database_url"} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}
