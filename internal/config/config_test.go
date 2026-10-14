package config

import (
	"os"
	"testing"
	"time"
)

func TestConfigLoadDefaults(t *testing.T) {
	// Clear any env variables that might affect the test
	os.Unsetenv("HTTP_PORT")
	os.Unsetenv("UPSTREAM_WS_URL")
	os.Unsetenv("UPSTREAM_API_KEY")
	os.Unsetenv("UPSTREAM_SECRET_KEY")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected successful load, got: %v", err)
	}

	if cfg.HTTPPort != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.HTTPPort)
	}
	if cfg.Market.RollingWindowDuration != 24*time.Hour {
		t.Errorf("expected 24h rolling window, got %v", cfg.Market.RollingWindowDuration)
	}
	if cfg.WebSocket.MaxConnections != 1000 {
		t.Errorf("expected max connections 1000, got %d", cfg.WebSocket.MaxConnections)
	}
}

func TestConfigValidation(t *testing.T) {
	cfg := &Config{
		HTTPPort: 99999, // Invalid port
		WebSocket: WebSocketConfig{
			MaxConnections:  100,
			ClientQueueSize: 10,
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Errorf("expected validation error for invalid port, got nil")
	}
}

func TestConfigRedactedString(t *testing.T) {
	cfg := &Config{
		HTTPPort: 8080,
		Upstream: UpstreamConfig{
			URL:       "wss://example.com",
			APIKey:    "super_secret_api_key",
			SecretKey: "super_secret_private_key",
		},
		Database: DatabaseConfig{
			EnablePersistence: true,
		},
		WebSocket: WebSocketConfig{
			MaxConnections: 500,
		},
	}

	redacted := cfg.RedactedString()
	if contains(redacted, "super_secret_api_key") || contains(redacted, "super_secret_private_key") {
		t.Fatalf("redacted string leaked credentials: %s", redacted)
	}
	if !contains(redacted, "[REDACTED]") {
		t.Fatalf("redacted string missing [REDACTED] placeholder: %s", redacted)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && containsSubstr(s, substr)))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
