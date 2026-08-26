package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"termchat.local/termchat/internal/client"
	"termchat.local/termchat/internal/tui"
)

const defaultServerURL = "http://localhost:8080"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Getenv); err != nil && !errors.Is(err, flag.ErrHelp) {
		_, _ = fmt.Fprintln(os.Stderr, "termchat:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string) error {
	serverURL, err := resolveServerURL(args, getenv)
	if err != nil {
		return err
	}
	api, err := client.New(serverURL)
	if err != nil {
		return fmt.Errorf("configure server: %w", err)
	}
	defer api.Disconnect()
	sessions, err := client.DefaultSessionStore()
	if err != nil {
		return fmt.Errorf("configure session storage: %w", err)
	}
	model := tui.NewModel(api, sessions)
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(ctx))
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("run terminal UI: %w", err)
	}
	return nil
}

func resolveServerURL(args []string, getenv func(string) string) (string, error) {
	flags := flag.NewFlagSet("termchat", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	server := flags.String("server", "", "TermChat server URL")
	if err := flags.Parse(args); err != nil {
		return "", err
	}
	if flags.NArg() != 0 {
		return "", fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if value := strings.TrimSpace(*server); value != "" {
		return value, nil
	}
	if value := strings.TrimSpace(getenv("TERMCHAT_SERVER_URL")); value != "" {
		return value, nil
	}
	return defaultServerURL, nil
}
