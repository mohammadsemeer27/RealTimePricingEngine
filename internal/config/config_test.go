package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestConfigLoadDefaults(t *testing.T) {
	os.Unsetenv("HTTP_PORT")
	os.Unsetenv("UPSTREAM_WS_URL")
	os.Unsetenv("UPSTREAM_API_KEY")
	os.Unsetenv("UPSTREAM_SECRET_KEY")
	os.Unsetenv("UPSTREAM_USE_MOCK")

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
	if !cfg.Upstream.UseMock {
		t.Errorf("expected UseMock to default to true when no API keys are present")
	}
}

func TestConfigValidationRealModeMissingCredentials(t *testing.T) {
	cfg := &Config{
		HTTPPort: 8080,
		Upstream: UpstreamConfig{
			URL:       "wss://v3-prod.livefxhub.com/ws/algo",
			UseMock:   false, // Real mode
			APIKey:    "",
			SecretKey: "",
		},
		WebSocket: WebSocketConfig{
			MaxConnections:  100,
			ClientQueueSize: 10,
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected error when UPSTREAM_USE_MOCK=false without credentials, got nil")
	}
	if !strings.Contains(err.Error(), "UPSTREAM_API_KEY and UPSTREAM_SECRET_KEY must be provided") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestConfigValidationRealModeWithCredentials(t *testing.T) {
	cfg := &Config{
		HTTPPort: 8080,
		Upstream: UpstreamConfig{
			URL:       "wss://v3-prod.livefxhub.com/ws/algo",
			UseMock:   false,
			APIKey:    "test_key",
			SecretKey: "test_secret",
		},
		WebSocket: WebSocketConfig{
			MaxConnections:  100,
			ClientQueueSize: 10,
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid configuration with credentials, got: %v", err)
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
	if strings.Contains(redacted, "super_secret_api_key") || strings.Contains(redacted, "super_secret_private_key") {
		t.Fatalf("redacted string leaked credentials: %s", redacted)
	}
	if !strings.Contains(redacted, "[REDACTED]") {
		t.Fatalf("redacted string missing [REDACTED] placeholder: %s", redacted)
	}
}
