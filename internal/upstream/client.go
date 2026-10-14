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
	maxAuthRetries   int
	authAttempts     int

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
	MaxAuthRetries   int
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
		maxAuthRetries:   cfg.MaxAuthRetries,
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

		// Check if it's an authentication error or close code 4003 (invalid credentials)
		if errors.Is(err, ErrAuthFailed) || isAuthCloseError(err) {
			if c.metrics != nil {
				c.metrics.IncAuthFailures()
			}
			c.authAttempts++
			if c.authAttempts <= c.maxAuthRetries {
				c.logger.Warn("upstream authentication failed, retrying per retry policy",
					"attempt", c.authAttempts,
					"max_retries", c.maxAuthRetries,
					"error", err)
				select {
				case <-c.ctx.Done():
					return
				case <-time.After(c.reconnectInitial):
					continue
				}
			}

			c.logger.Error("upstream authentication failed: invalid credentials or authorization failure (close code 4003). Please verify UPSTREAM_API_KEY and UPSTREAM_SECRET_KEY in your configuration. Halting reconnect loop to prevent account lockout.",
				"error", err,
				"attempts", c.authAttempts)
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
		if isAuthCloseError(err) {
			return fmt.Errorf("%w: dial failed with auth error: %v", ErrAuthFailed, err)
		}
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

	// 2. Subscribe to symbols ONLY after validated authentication
	if err := c.subscribe(conn); err != nil {
		if isAuthCloseError(err) {
			return fmt.Errorf("%w: subscription failed with auth close: %v", ErrAuthFailed, err)
		}
		return fmt.Errorf("subscription failed: %w", err)
	}

	// 3. Read loop with deadline
	return c.readLoop(conn)
}

// authenticate sends credentials and awaits auth response according to the provider protocol.
func (c *Client) authenticate(conn *websocket.Conn) error {
	authMsg := map[string]interface{}{
		"action":  "auth",
		"api_key": c.apiKey,
		"secret":  c.secretKey,
	}

	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteJSON(authMsg); err != nil {
		if isAuthCloseError(err) {
			return fmt.Errorf("%w: failed to send auth payload due to close code 4003: %v", ErrAuthFailed, err)
		}
		return fmt.Errorf("failed to send auth payload: %w", err)
	}

	// Read auth response
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		if isAuthCloseError(err) {
			return fmt.Errorf("%w: auth response read failed with close code 4003: %v", ErrAuthFailed, err)
		}
		return fmt.Errorf("failed to read auth response: %w", err)
	}

	c.logger.Info("received auth response from upstream", "bytes", len(msg))

	if err := validateAuthResponse(msg); err != nil {
		return err
	}

	c.logger.Info("upstream authentication verified successfully")
	return nil
}

// validateAuthResponse parses and verifies the upstream authentication response JSON.
// Never assumes success based only on receiving a response.
func validateAuthResponse(msg []byte) error {
	var resp map[string]interface{}
	if err := json.Unmarshal(msg, &resp); err != nil {
		return fmt.Errorf("%w: invalid json in auth response: %v", ErrAuthFailed, err)
	}

	// 1. Check explicit failure indicators
	// Check "event" field errors (e.g. {"event":"error","message":"invalid credentials"})
	if event, ok := resp["event"].(string); ok {
		eventLower := strings.ToLower(event)
		if eventLower == "error" || eventLower == "failed" || eventLower == "rejected" {
			errMsg := "authentication rejected"
			if m, ok := resp["message"].(string); ok && m != "" {
				errMsg = m
			} else if m, ok := resp["error"].(string); ok && m != "" {
				errMsg = m
			}
			return fmt.Errorf("%w: %s (event: %s)", ErrAuthFailed, errMsg, event)
		}
	}

	// Check "status" field errors
	if status, ok := resp["status"].(string); ok {
		statusLower := strings.ToLower(status)
		if statusLower == "error" || statusLower == "failed" || statusLower == "unauthorized" || statusLower == "rejected" {
			errMsg := "authentication rejected"
			if m, ok := resp["error"].(string); ok && m != "" {
				errMsg = m
			} else if m, ok := resp["message"].(string); ok && m != "" {
				errMsg = m
			}
			return fmt.Errorf("%w: %s (status: %s)", ErrAuthFailed, errMsg, status)
		}
	}

	// Check top-level "error" field
	if errMsg, ok := resp["error"].(string); ok && errMsg != "" {
		return fmt.Errorf("%w: %s", ErrAuthFailed, errMsg)
	}

	// Check status code
	if code, ok := resp["code"].(float64); ok && code >= 400 {
		return fmt.Errorf("%w: status code %d", ErrAuthFailed, int(code))
	}

	// Check explicit boolean flags
	if auth, ok := resp["authenticated"].(bool); ok && !auth {
		return fmt.Errorf("%w: authenticated flag is false", ErrAuthFailed)
	}
	if succ, ok := resp["success"].(bool); ok && !succ {
		return fmt.Errorf("%w: success flag is false", ErrAuthFailed)
	}

	// 2. Validate confirmation of success (never assume success blindly)
	isSuccess := false
	if status, ok := resp["status"].(string); ok {
		statusLower := strings.ToLower(status)
		if statusLower == "success" || statusLower == "ok" || statusLower == "authenticated" || statusLower == "connected" {
			isSuccess = true
		}
	}
	if event, ok := resp["event"].(string); ok {
		eventLower := strings.ToLower(event)
		if eventLower == "authenticated" || eventLower == "success" || eventLower == "ok" {
			isSuccess = true
		}
	}
	if auth, ok := resp["authenticated"].(bool); ok && auth {
		isSuccess = true
	}
	if succ, ok := resp["success"].(bool); ok && succ {
		isSuccess = true
	}
	if action, ok := resp["action"].(string); ok && strings.EqualFold(action, "auth") {
		if st, ok := resp["status"].(string); ok {
			stLower := strings.ToLower(st)
			if stLower == "success" || stLower == "ok" {
				isSuccess = true
			}
		}
	}

	if !isSuccess {
		keys := make([]string, 0, len(resp))
		for k := range resp {
			keys = append(keys, k)
		}
		return fmt.Errorf("%w: response does not confirm authentication success (received keys: %v)", ErrAuthFailed, keys)
	}

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
		if isAuthCloseError(err) {
			return fmt.Errorf("%w: subscription write failed with close code 4003: %v", ErrAuthFailed, err)
		}
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
			if isAuthCloseError(err) {
				return fmt.Errorf("%w: upstream connection closed with code 4003 (invalid credentials): %v", ErrAuthFailed, err)
			}
			return fmt.Errorf("read error: %w", err)
		}

		if messageType != websocket.TextMessage {
			continue
		}

		// 1. Inspect control messages / error events from upstream
		var ctrl map[string]interface{}
		if err := json.Unmarshal(message, &ctrl); err == nil {
			if ev, ok := ctrl["event"].(string); ok && strings.EqualFold(ev, "error") {
				msgText := "upstream error"
				if m, ok := ctrl["message"].(string); ok {
					msgText = m
				}
				if strings.Contains(strings.ToLower(msgText), "credentials") || strings.Contains(strings.ToLower(msgText), "authenticate") {
					return fmt.Errorf("%w: upstream stream rejected with auth error: %s", ErrAuthFailed, msgText)
				}
				c.logger.Warn("upstream reported error event", "message", msgText)
				continue
			}
			if act, ok := ctrl["action"].(string); ok && (act == "subscribe" || act == "pong" || act == "heartbeat") {
				continue
			}
		}

		// 2. Try single RawTick
		var single market.RawTick
		if err := json.Unmarshal(message, &single); err == nil && single.Symbol != "" && len(single.Buy) > 0 {
			if c.handler != nil {
				c.handler(c.ctx, single)
			}
			continue
		}

		// 3. Try array of RawTicks
		var batch []market.RawTick
		if err := json.Unmarshal(message, &batch); err == nil && len(batch) > 0 && batch[0].Symbol != "" {
			if c.handler != nil {
				for _, tick := range batch {
					c.handler(c.ctx, tick)
				}
			}
			continue
		}

		// 4. Try envelope wrappers (e.g., {"data": ...} or {"ticks": ...})
		var envelope struct {
			Data   json.RawMessage `json:"data"`
			Ticks  json.RawMessage `json:"ticks"`
			Prices json.RawMessage `json:"prices"`
		}
		if err := json.Unmarshal(message, &envelope); err == nil {
			payload := envelope.Data
			if len(payload) == 0 {
				payload = envelope.Ticks
			}
			if len(payload) == 0 {
				payload = envelope.Prices
			}

			if len(payload) > 0 {
				var envSingle market.RawTick
				if err := json.Unmarshal(payload, &envSingle); err == nil && envSingle.Symbol != "" {
					if c.handler != nil {
						c.handler(c.ctx, envSingle)
					}
					continue
				}

				var envBatch []market.RawTick
				if err := json.Unmarshal(payload, &envBatch); err == nil && len(envBatch) > 0 {
					if c.handler != nil {
						for _, tick := range envBatch {
							c.handler(c.ctx, tick)
						}
					}
					continue
				}
			}
		}

		c.logger.Debug("unhandled upstream message format", "bytes", len(message))
	}
}

// isAuthCloseError checks if an error represents close code 4003, 4000, or credential failure.
func isAuthCloseError(err error) bool {
	if err == nil {
		return false
	}
	var closeErr *websocket.CloseError
	if errors.As(err, &closeErr) {
		if closeErr.Code == 4003 || closeErr.Code == 4000 {
			return true
		}
	}
	if websocket.IsCloseError(err, 4003, 4000) {
		return true
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "4003") || strings.Contains(errStr, "invalid credentials")
}
