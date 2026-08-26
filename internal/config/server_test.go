package config

import (
	"testing"
	"time"
)

func TestLoadServerConfig(t *testing.T) {
	t.Parallel()

	environment := map[string]string{
		"TERMCHAT_LISTEN_ADDR":      ":9090",
		"TERMCHAT_DATABASE_URL":     "postgres://localhost/termchat",
		"TERMCHAT_JWT_SECRET":       "01234567890123456789012345678901",
		"TERMCHAT_TOKEN_TTL":        "12h",
		"TERMCHAT_CLEANUP_INTERVAL": "30m",
	}
	config, err := LoadServer(func(key string) string { return environment[key] })
	if err != nil {
		t.Fatalf("LoadServer() error = %v", err)
	}
	if config.ListenAddr != ":9090" || config.TokenTTL != 12*time.Hour || config.CleanupInterval != 30*time.Minute {
		t.Fatalf("LoadServer() = %#v", config)
	}
}

func TestLoadServerConfigRequiresSecrets(t *testing.T) {
	t.Parallel()

	for _, environment := range []map[string]string{
		{"TERMCHAT_JWT_SECRET": "01234567890123456789012345678901"},
		{"TERMCHAT_DATABASE_URL": "postgres://localhost/termchat", "TERMCHAT_JWT_SECRET": "too-short"},
	} {
		if _, err := LoadServer(func(key string) string { return environment[key] }); err == nil {
			t.Fatalf("LoadServer() accepted environment %#v", environment)
		}
	}
}
