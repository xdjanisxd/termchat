package config

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

type Server struct {
	ListenAddr        string
	DatabaseURL       string
	JWTSecret         []byte
	TokenTTL          time.Duration
	CleanupInterval   time.Duration
	TrustProxyHeaders bool
}

func LoadServer(getenv func(string) string) (Server, error) {
	config := Server{
		ListenAddr:      getenv("TERMCHAT_LISTEN_ADDR"),
		DatabaseURL:     getenv("TERMCHAT_DATABASE_URL"),
		JWTSecret:       []byte(getenv("TERMCHAT_JWT_SECRET")),
		TokenTTL:        time.Hour,
		CleanupInterval: time.Hour,
	}
	if config.ListenAddr == "" {
		config.ListenAddr = ":8080"
	}
	if config.DatabaseURL == "" {
		return Server{}, errors.New("TERMCHAT_DATABASE_URL is required")
	}
	if len(config.JWTSecret) < 32 {
		return Server{}, errors.New("TERMCHAT_JWT_SECRET must contain at least 32 bytes")
	}
	if value := getenv("TERMCHAT_TRUST_PROXY_HEADERS"); value != "" {
		trustProxyHeaders, err := strconv.ParseBool(value)
		if err != nil {
			return Server{}, fmt.Errorf("invalid TERMCHAT_TRUST_PROXY_HEADERS %q", value)
		}
		config.TrustProxyHeaders = trustProxyHeaders
	}
	if value := getenv("TERMCHAT_TOKEN_TTL"); value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 {
			return Server{}, fmt.Errorf("invalid TERMCHAT_TOKEN_TTL %q", value)
		}
		config.TokenTTL = duration
	}
	if value := getenv("TERMCHAT_CLEANUP_INTERVAL"); value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 {
			return Server{}, fmt.Errorf("invalid TERMCHAT_CLEANUP_INTERVAL %q", value)
		}
		config.CleanupInterval = duration
	}
	return config, nil
}
