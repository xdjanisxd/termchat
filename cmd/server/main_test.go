package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

func TestRunRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := run(context.Background(), func(string) string { return "" }, logger)
	if err == nil {
		t.Fatal("run() accepted missing database URL and JWT secret")
	}
}
