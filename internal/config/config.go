package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration parameters for the Real-Time Pricing Engine.
type Config struct {
	HTTPPort  int
	LogLevel  string
	Upstream  UpstreamConfig
	Market    MarketConfig
	WebSocket WebSocketConfig
	Database  DatabaseConfig
}

// UpstreamConfig contains parameters for connecting to the upstream feed.
type UpstreamConfig struct {
	URL              string
	APIKey           string
	SecretKey        string
	UseMock          bool
	ReconnectInitial time.Duration
	ReconnectMax     time.Duration
	ReadTimeout      time.Duration
}

// MarketConfig holds pricing calculation parameters.
type MarketConfig struct {
	RollingWindowDuration time.Duration
}

// WebSocketConfig holds downstream WebSocket server parameters.
type WebSocketConfig struct {
	MaxConnections    int
	ClientQueueSize   int
	WriteTimeout      time.Duration
	PingInterval      time.Duration
	PongWait          time.Duration
	EnableCompression bool
}

// DatabaseConfig holds PostgreSQL persistence configuration.
type DatabaseConfig struct {
	URL               string
	MaxOpenConns      int
	MaxIdleConns      int
	ConnMaxLifetime   time.Duration
	BatchSize         int
	FlushInterval     time.Duration
	EnablePersistence bool
}

// Load reads configuration from the environment and optional .env file.
func Load() (*Config, error) {
	// Attempt to load .env from current directory or parent directory
	loadDotEnv(".env")
	loadDotEnv("../.env")

	// Determine if mock is explicitly set
	useMockVal, mockSpecified := os.LookupEnv("UPSTREAM_USE_MOCK")
	var useMock bool
	if mockSpecified {
		parsed, err := strconv.ParseBool(strings.TrimSpace(useMockVal))
		if err == nil {
			useMock = parsed
		} else {
			useMock = false
		}
	} else {
		// If not explicitly set, default to mock only if API key is absent
		useMock = (getEnv("UPSTREAM_API_KEY", "") == "")
	}

	cfg := &Config{
		HTTPPort: getEnvInt("HTTP_PORT", 8080),
		LogLevel: getEnv("LOG_LEVEL", "info"),
		Upstream: UpstreamConfig{
			URL:              getEnv("UPSTREAM_WS_URL", "wss://v3-prod.livefxhub.com/ws/algo"),
			APIKey:           getEnv("UPSTREAM_API_KEY", ""),
			SecretKey:        getEnv("UPSTREAM_SECRET_KEY", ""),
			UseMock:          useMock,
			ReconnectInitial: time.Duration(getEnvInt("UPSTREAM_RECONNECT_INITIAL_MS", 500)) * time.Millisecond,
			ReconnectMax:     time.Duration(getEnvInt("UPSTREAM_RECONNECT_MAX_MS", 30000)) * time.Millisecond,
			ReadTimeout:      time.Duration(getEnvInt("UPSTREAM_READ_TIMEOUT_MS", 15000)) * time.Millisecond,
		},
		Market: MarketConfig{
			RollingWindowDuration: time.Duration(getEnvInt("ROLLING_WINDOW_HOURS", 24)) * time.Hour,
		},
		WebSocket: WebSocketConfig{
			MaxConnections:    getEnvInt("WS_MAX_CONNECTIONS", 1000),
			ClientQueueSize:   getEnvInt("WS_CLIENT_QUEUE_SIZE", 256),
			WriteTimeout:      time.Duration(getEnvInt("WS_WRITE_TIMEOUT_MS", 5000)) * time.Millisecond,
			PingInterval:      time.Duration(getEnvInt("WS_PING_INTERVAL_MS", 30000)) * time.Millisecond,
			PongWait:          time.Duration(getEnvInt("WS_PONG_WAIT_MS", 35000)) * time.Millisecond,
			EnableCompression: getEnvBool("WS_ENABLE_COMPRESSION", true),
		},
		Database: DatabaseConfig{
			URL:               getEnv("DATABASE_URL", "postgres://pricing_user:pricing_pass@localhost:5432/pricing_db?sslmode=disable"),
			MaxOpenConns:      getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:      getEnvInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime:   time.Duration(getEnvInt("DB_CONN_MAX_LIFETIME_SEC", 300)) * time.Second,
			BatchSize:         getEnvInt("DB_BATCH_SIZE", 100),
			FlushInterval:     time.Duration(getEnvInt("DB_FLUSH_INTERVAL_MS", 200)) * time.Millisecond,
			EnablePersistence: getEnvBool("DB_ENABLE_PERSISTENCE", true),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// Validate checks that required fields and ranges are valid.
func (c *Config) Validate() error {
	if c.HTTPPort <= 0 || c.HTTPPort > 65535 {
		return fmt.Errorf("HTTP_PORT must be between 1 and 65535, got %d", c.HTTPPort)
	}
	if !c.Upstream.UseMock {
		if c.Upstream.URL == "" {
			return fmt.Errorf("UPSTREAM_WS_URL cannot be empty when mock is disabled")
		}
		if c.Upstream.APIKey == "" || c.Upstream.SecretKey == "" {
			return fmt.Errorf("UPSTREAM_API_KEY and UPSTREAM_SECRET_KEY must be provided when UPSTREAM_USE_MOCK=false")
		}
	}
	if c.WebSocket.MaxConnections <= 0 {
		return fmt.Errorf("WS_MAX_CONNECTIONS must be > 0, got %d", c.WebSocket.MaxConnections)
	}
	if c.WebSocket.ClientQueueSize <= 0 {
		return fmt.Errorf("WS_CLIENT_QUEUE_SIZE must be > 0, got %d", c.WebSocket.ClientQueueSize)
	}
	return nil
}

// RedactedString returns a string representation with secrets masked.
func (c *Config) RedactedString() string {
	redactedKey := "[NOT SET]"
	if c.Upstream.APIKey != "" {
		redactedKey = "[REDACTED]"
	}
	redactedSecret := "[NOT SET]"
	if c.Upstream.SecretKey != "" {
		redactedSecret = "[REDACTED]"
	}

	return fmt.Sprintf("HTTPPort=%d UpstreamURL=%s UpstreamUseMock=%t APIKey=%s SecretKey=%s DBEnabled=%t WSMaxConns=%d",
		c.HTTPPort, c.Upstream.URL, c.Upstream.UseMock, redactedKey, redactedSecret, c.Database.EnablePersistence, c.WebSocket.MaxConnections)
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			return parsed
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.ParseBool(strings.TrimSpace(val)); err == nil {
			return parsed
		}
	}
	return defaultVal
}

// loadDotEnv parses a simple KEY=VALUE format file without external libraries.
func loadDotEnv(filepath string) {
	file, err := os.Open(filepath)
	if err != nil {
		return // File doesn't exist, which is completely fine
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			v = strings.Trim(v, `"'`)
			// Set if not set or currently empty
			existing, exists := os.LookupEnv(k)
			if !exists || strings.TrimSpace(existing) == "" {
				_ = os.Setenv(k, v)
			}
		}
	}
}
