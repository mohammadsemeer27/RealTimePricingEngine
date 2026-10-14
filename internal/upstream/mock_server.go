package upstream

import (
	"encoding/json"
	"log/slog"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
)

type mockClientConn struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func (mc *mockClientConn) writeJSON(v interface{}) error {
	mc.writeMu.Lock()
	defer mc.writeMu.Unlock()
	_ = mc.conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
	return mc.conn.WriteJSON(v)
}

func (mc *mockClientConn) writeMessage(messageType int, data []byte) error {
	mc.writeMu.Lock()
	defer mc.writeMu.Unlock()
	_ = mc.conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
	return mc.conn.WriteMessage(messageType, data)
}

// MockServer provides a simulated upstream WebSocket feed for testing and local development.
type MockServer struct {
	server       *httptest.Server
	upgrader     websocket.Upgrader
	logger       *slog.Logger
	symbols      []string
	basePrices   map[string]decimal.Decimal
	tickInterval time.Duration
	failAuth     bool

	mu      sync.Mutex
	conns   map[*mockClientConn]struct{}
	stopCh  chan struct{}
	running bool
}

// MockServerOption configures the MockServer.
type MockServerOption func(*MockServer)

// WithMockTickInterval sets how frequently the mock streams ticks.
func WithMockTickInterval(interval time.Duration) MockServerOption {
	return func(s *MockServer) {
		s.tickInterval = interval
	}
}

// WithMockAuthFailure configures the mock to reject authentication attempts.
func WithMockAuthFailure(fail bool) MockServerOption {
	return func(s *MockServer) {
		s.failAuth = fail
	}
}

// NewMockServer creates and starts a new Mock WebSocket Server.
func NewMockServer(opts ...MockServerOption) *MockServer {
	ms := &MockServer{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		logger:       slog.Default(),
		symbols:      DefaultSymbols,
		basePrices:   initBasePrices(),
		tickInterval: 100 * time.Millisecond,
		conns:        make(map[*mockClientConn]struct{}),
		stopCh:       make(chan struct{}),
		running:      true,
	}

	for _, opt := range opts {
		opt(ms)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/algo", ms.handleWebSocket)
	mux.HandleFunc("/", ms.handleWebSocket)

	ms.server = httptest.NewServer(mux)
	go ms.generatorLoop()

	return ms
}

// URL returns the ws:// endpoint URL for this mock server.
func (ms *MockServer) URL() string {
	return "ws" + strings.TrimPrefix(ms.server.URL, "http") + "/ws/algo"
}

// Close shuts down connections and the HTTP test server.
func (ms *MockServer) Close() {
	ms.mu.Lock()
	if !ms.running {
		ms.mu.Unlock()
		return
	}
	ms.running = false
	close(ms.stopCh)

	for mc := range ms.conns {
		_ = mc.conn.Close()
	}
	ms.mu.Unlock()

	ms.server.Close()
}

func (ms *MockServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := ms.upgrader.Upgrade(w, r, nil)
	if err != nil {
		ms.logger.Error("mock server upgrade error", "err", err)
		return
	}

	mc := &mockClientConn{conn: conn}

	ms.mu.Lock()
	ms.conns[mc] = struct{}{}
	ms.mu.Unlock()

	defer func() {
		ms.mu.Lock()
		delete(ms.conns, mc)
		ms.mu.Unlock()
		_ = conn.Close()
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var req map[string]interface{}
		if err := json.Unmarshal(message, &req); err != nil {
			continue
		}

		action, _ := req["action"].(string)
		switch action {
		case "auth":
			if ms.failAuth {
				_ = mc.writeJSON(map[string]interface{}{
					"action": "auth",
					"status": "error",
					"error":  "invalid credentials",
				})
			} else {
				_ = mc.writeJSON(map[string]interface{}{
					"action": "auth",
					"status": "success",
				})
			}
		case "subscribe":
			_ = mc.writeJSON(map[string]interface{}{
				"action": "subscribe",
				"status": "ok",
			})
		}
	}
}

func (ms *MockServer) generatorLoop() {
	ticker := time.NewTicker(ms.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ms.stopCh:
			return
		case <-ticker.C:
			ms.broadcastRandomTicks()
		}
	}
}

func (ms *MockServer) broadcastRandomTicks() {
	ms.mu.Lock()
	if len(ms.conns) == 0 {
		ms.mu.Unlock()
		return
	}

	// Pick 3-5 random symbols to update in each tick cycle
	numSymbols := 3 + rand.Intn(4)
	now := time.Now().UTC()

	// Snapshot list of active connections to minimize lock contention
	clientList := make([]*mockClientConn, 0, len(ms.conns))
	for mc := range ms.conns {
		clientList = append(clientList, mc)
	}
	ms.mu.Unlock()

	for i := 0; i < numSymbols; i++ {
		sym := ms.symbols[rand.Intn(len(ms.symbols))]

		ms.mu.Lock()
		base := ms.basePrices[sym]
		deltaPct := (rand.Float64() - 0.5) * 0.001
		newMid := base.Mul(decimal.NewFromFloat(1.0 + deltaPct))
		ms.basePrices[sym] = newMid
		ms.mu.Unlock()

		spread := newMid.Mul(decimal.NewFromFloat(0.0001))
		buy := newMid.Sub(spread).Round(5)
		sell := newMid.Add(spread).Round(5)

		payload := map[string]interface{}{
			"symbol":     sym,
			"buy":        buy.String(),
			"sell":       sell.String(),
			"low_24h":    "9999.99",
			"high_24h":   "0.00",
			"change_24h": "100.00",
			"timestamp":  now.UnixMilli(),
		}

		data, err := json.Marshal(payload)
		if err != nil {
			continue
		}

		for _, mc := range clientList {
			_ = mc.writeMessage(websocket.TextMessage, data)
		}
	}
}

func initBasePrices() map[string]decimal.Decimal {
	p := make(map[string]decimal.Decimal)
	for _, sym := range DefaultSymbols {
		switch {
		case strings.HasPrefix(sym, "BTC"):
			p[sym] = decimal.NewFromInt(65000)
		case strings.HasPrefix(sym, "ETH"):
			p[sym] = decimal.NewFromInt(3500)
		case strings.HasPrefix(sym, "SOL"):
			p[sym] = decimal.NewFromInt(140)
		case strings.HasPrefix(sym, "BNB"):
			p[sym] = decimal.NewFromInt(580)
		case strings.HasPrefix(sym, "XAU"):
			p[sym] = decimal.NewFromInt(2350)
		case strings.HasPrefix(sym, "XAG"):
			p[sym] = decimal.NewFromFloat(28.5)
		case strings.Contains(sym, "JPY"):
			p[sym] = decimal.NewFromFloat(155.50)
		default:
			p[sym] = decimal.NewFromFloat(1.1000 + rand.Float64()*0.5)
		}
	}
	return p
}
