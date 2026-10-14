package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/market"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/metrics"
)

var (
	ErrAuthFailed       = errors.New("upstream authentication failed")
	ErrConnectionClosed = errors.New("upstream connection closed")
	ErrClientShutdown   = errors.New("upstream client is shutting down")
)

// RawTickHandler callback type for handling parsed incoming ticks.
type RawTickHandler func(ctx context.Context, raw market.RawTick)

// Client handles connecting, authenticating, and receiving messages from the upstream WebSocket.
type Client struct {
	url       string
	apiKey    string
	secretKey string
	symbols   []string
	handler   RawTickHandler
	metrics   *metrics.Metrics
	logger    *slog.Logger

	reconnectInitial time.Duration
	reconnectMax     time.Duration
	readTimeout      time.Duration

	mu             sync.Mutex
	conn           *websocket.Conn
	isReconnecting atomic.Bool
	stopCh         chan struct{}
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
}

// ClientConfig holds configuration for the upstream client.
type ClientConfig struct {
	URL              string
	APIKey           string
	SecretKey        string
	Symbols          []string
	ReconnectInitial time.Duration
	ReconnectMax     time.Duration
	ReadTimeout      time.Duration
	Handler          RawTickHandler
	Metrics          *metrics.Metrics
	Logger           *slog.Logger
}

// NewClient initializes a new Upstream Client.
func NewClient(cfg ClientConfig) *Client {
	if cfg.ReconnectInitial <= 0 {
		cfg.ReconnectInitial = 500 * time.Millisecond
	}
	if cfg.ReconnectMax <= 0 {
		cfg.ReconnectMax = 30 * time.Second
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = 15 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if len(cfg.Symbols) == 0 {
		cfg.Symbols = DefaultSymbols
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		url:              cfg.URL,
		apiKey:           cfg.APIKey,
		secretKey:        cfg.SecretKey,
		symbols:          cfg.Symbols,
		handler:          cfg.Handler,
		metrics:          cfg.Metrics,
		logger:           cfg.Logger,
		reconnectInitial: cfg.ReconnectInitial,
		reconnectMax:     cfg.ReconnectMax,
		readTimeout:      cfg.ReadTimeout,
		stopCh:           make(chan struct{}),
		ctx:              ctx,
		cancel:           cancel,
	}
}

// Start initiates the upstream connection loop.
func (c *Client) Start() {
	c.wg.Add(1)
	go c.runLoop()
}

// Close gracefully terminates the connection and background routines.
func (c *Client) Close() error {
	c.cancel()
	select {
	case <-c.stopCh:
		// already closed
	default:
		close(c.stopCh)
	}

	c.mu.Lock()
	if c.conn != nil {
		_ = c.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "shutdown"),
			time.Now().Add(time.Second))
		_ = c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()

	c.wg.Wait()
	return nil
}

// runLoop manages connection, exponential backoff with jitter, and reconnection.
func (c *Client) runLoop() {
	defer c.wg.Done()

	backoff := c.reconnectInitial

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Info("upstream client connection loop stopped")
			return
		default:
		}

		c.logger.Info("connecting to upstream feed", "url", c.url)
		err := c.connectAndStream()
		if err == nil || errors.Is(err, ErrClientShutdown) {
			return
		}

		// Check if it's an authentication error
		if errors.Is(err, ErrAuthFailed) {
			if c.metrics != nil {
				c.metrics.IncAuthFailures()
			}
			c.logger.Error("upstream authentication failed, halting reconnect loop to prevent lockouts", "error", err)
			return
		}

		// Network or protocol failure - perform exponential backoff with jitter
		if c.metrics != nil {
			c.metrics.IncUpstreamReconnects()
		}

		// Calculate jitter (+- 20%)
		jitter := time.Duration(float64(backoff) * (0.8 + 0.4*rand.Float64()))
		c.logger.Warn("upstream connection failed, retrying after backoff",
			"error", err,
			"backoff_ms", jitter.Milliseconds())

		select {
		case <-c.ctx.Done():
			return
		case <-time.After(jitter):
		}

		backoff *= 2
		if backoff > c.reconnectMax {
			backoff = c.reconnectMax
		}
	}
}

// connectAndStream handles single session: connect, authenticate, subscribe, and read.
func (c *Client) connectAndStream() error {
	// Prevent duplicate reconnect loops
	if !c.isReconnecting.CompareAndSwap(false, true) {
		return errors.New("reconnection already in progress")
	}
	defer c.isReconnecting.Store(false)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		Proxy:            http.ProxyFromEnvironment,
	}

	conn, _, err := dialer.DialContext(c.ctx, c.url, nil)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		if c.conn != nil {
			_ = c.conn.Close()
			c.conn = nil
		}
		c.mu.Unlock()
	}()

	// 1. Authenticate if credentials are provided
	if c.apiKey != "" && c.secretKey != "" {
		if err := c.authenticate(conn); err != nil {
			return err
		}
	} else {
		c.logger.Info("no upstream credentials configured, proceeding without auth")
	}

	// 2. Subscribe to symbols
	if err := c.subscribe(conn); err != nil {
		return fmt.Errorf("subscription failed: %w", err)
	}

	// 3. Read loop with deadline
	return c.readLoop(conn)
}

// authenticate sends credentials and awaits auth response.
func (c *Client) authenticate(conn *websocket.Conn) error {
	authMsg := map[string]interface{}{
		"action":  "auth",
		"api_key": c.apiKey,
		"secret":  c.secretKey,
	}

	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteJSON(authMsg); err != nil {
		return fmt.Errorf("failed to send auth payload: %w", err)
	}

	// Read auth response
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read auth response: %w", err)
	}

	// Log response message without leaking credentials
	c.logger.Info("received auth response from upstream", "bytes", len(msg))

	// Check if auth response contains failure indicators
	var resp map[string]interface{}
	if err := json.Unmarshal(msg, &resp); err == nil {
		if status, ok := resp["status"].(string); ok {
			statusLower := strings.ToLower(status)
			if statusLower == "error" || statusLower == "failed" || statusLower == "unauthorized" {
				return fmt.Errorf("%w: response status '%s'", ErrAuthFailed, status)
			}
		}
		if errMsg, ok := resp["error"].(string); ok && errMsg != "" {
			return fmt.Errorf("%w: %s", ErrAuthFailed, errMsg)
		}
	}

	c.logger.Info("upstream authentication successful")
	return nil
}

// subscribe sends subscription message for all symbols.
func (c *Client) subscribe(conn *websocket.Conn) error {
	subMsg := map[string]interface{}{
		"action":  "subscribe",
		"symbols": c.symbols,
	}

	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteJSON(subMsg); err != nil {
		return fmt.Errorf("write subscribe error: %w", err)
	}

	c.logger.Info("subscribed to upstream symbols", "symbol_count", len(c.symbols))
	return nil
}

// readLoop reads incoming frames, resets deadlines, and forwards ticks to handler.
func (c *Client) readLoop(conn *websocket.Conn) error {
	for {
		select {
		case <-c.ctx.Done():
			return ErrClientShutdown
		default:
		}

		_ = conn.SetReadDeadline(time.Now().Add(c.readTimeout))
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}

		if messageType != websocket.TextMessage {
			continue
		}

		// Inspect message structure
		// Could be a single RawTick or an array of RawTicks
		var single market.RawTick
		if err := json.Unmarshal(message, &single); err == nil && single.Symbol != "" {
			if c.handler != nil {
				c.handler(c.ctx, single)
			}
			continue
		}

		var batch []market.RawTick
		if err := json.Unmarshal(message, &batch); err == nil && len(batch) > 0 {
			if c.handler != nil {
				for _, tick := range batch {
					c.handler(c.ctx, tick)
				}
			}
			continue
		}
	}
}
