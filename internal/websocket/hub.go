package websocket

import (
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/config"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/market"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/metrics"
)

var (
	ErrMaxConnectionsExceeded = errors.New("maximum downstream WebSocket connections exceeded")
)

// MarketReader allows querying current market snapshots.
type MarketReader interface {
	GetSnapshot(symbol string) (market.MarketSnapshot, bool)
}

// Hub manages active downstream client connections and targeted pub/sub delivery.
type Hub struct {
	mu                sync.RWMutex
	clients           map[*Client]struct{}
	symbolSubscribers map[string]map[*Client]struct{}

	engine         MarketReader
	metrics        *metrics.Metrics
	logger         *slog.Logger
	cfg            config.WebSocketConfig
	upgrader       websocket.Upgrader
	isShuttingDown bool
}

// NewHub creates a new downstream WebSocket hub.
func NewHub(cfg config.WebSocketConfig, engine MarketReader, m *metrics.Metrics, logger *slog.Logger) *Hub {
	if logger == nil {
		logger = slog.Default()
	}

	return &Hub{
		clients:           make(map[*Client]struct{}),
		symbolSubscribers: make(map[string]map[*Client]struct{}),
		engine:            engine,
		metrics:           m,
		logger:            logger,
		cfg:               cfg,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for API clients
			},
			EnableCompression: cfg.EnableCompression,
		},
	}
}

// Register adds a client if below connection limits.
func (h *Hub) Register(c *Client) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.isShuttingDown {
		return errors.New("hub is shutting down")
	}

	if len(h.clients) >= h.cfg.MaxConnections {
		if h.metrics != nil {
			h.metrics.IncDroppedEvents("max_connections_exceeded")
		}
		return ErrMaxConnectionsExceeded
	}

	h.clients[c] = struct{}{}
	if h.metrics != nil {
		h.metrics.IncDownstreamClients()
	}
	return nil
}

// Unregister cleans up a disconnected client.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.clients[c]; !exists {
		return
	}

	delete(h.clients, c)
	if h.metrics != nil {
		h.metrics.DecDownstreamClients()
	}

	// Remove from all symbol indices
	for _, sym := range c.GetSubscriptions() {
		if set, ok := h.symbolSubscribers[sym]; ok {
			delete(set, c)
			if len(set) == 0 {
				delete(h.symbolSubscribers, sym)
			}
		}
	}
}

// RegisterSubscriptions registers a client for specified symbols.
func (h *Hub) RegisterSubscriptions(c *Client, symbols []string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, sym := range symbols {
		if _, ok := h.symbolSubscribers[sym]; !ok {
			h.symbolSubscribers[sym] = make(map[*Client]struct{})
		}
		h.symbolSubscribers[sym][c] = struct{}{}
	}
}

// UnregisterSubscriptions unregisters a client from specified symbols.
func (h *Hub) UnregisterSubscriptions(c *Client, symbols []string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, sym := range symbols {
		if set, ok := h.symbolSubscribers[sym]; ok {
			delete(set, c)
			if len(set) == 0 {
				delete(h.symbolSubscribers, sym)
			}
		}
	}
}

// BroadcastSnapshot converts the snapshot to a compact delta and dispatches to subscribed clients.
// Does NOT block upstream processing.
func (h *Hub) BroadcastSnapshot(snapshot market.MarketSnapshot) {
	delta := snapshot.ToDelta()
	data, err := EncodeDelta(delta)
	if err != nil {
		h.logger.Error("failed to encode delta", "error", err)
		return
	}

	// Safety check: ensure payload size remains well below 10 KB
	if len(data) > 10*1024 {
		h.logger.Error("payload exceeded 10KB threshold", "size", len(data), "symbol", snapshot.Symbol)
		if h.metrics != nil {
			h.metrics.IncDroppedEvents("payload_exceeded_10kb")
		}
		return
	}

	h.mu.RLock()
	subscribers, ok := h.symbolSubscribers[snapshot.Symbol]
	if !ok || len(subscribers) == 0 {
		h.mu.RUnlock()
		return
	}

	// Make a quick slice copy to release lock immediately
	targetClients := make([]*Client, 0, len(subscribers))
	for c := range subscribers {
		targetClients = append(targetClients, c)
	}
	h.mu.RUnlock()

	// Non-blocking queue to each subscribed client
	for _, c := range targetClients {
		c.QueueMessage(data)
	}
}

// ServeHTTP handles incoming downstream WebSocket upgrade requests.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Warn("websocket upgrade error", "err", err)
		return
	}

	client := NewClient(h, conn, h.cfg, h.logger)
	if err := h.Register(client); err != nil {
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseTryAgainLater, err.Error()),
			time.Now().Add(time.Second))
		_ = conn.Close()
		return
	}

	go client.WritePump()
	go client.ReadPump()
}

// ClientCount returns the number of currently active downstream clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Close gracefully disconnects all downstream clients.
func (h *Hub) Close() {
	h.mu.Lock()
	h.isShuttingDown = true
	allClients := make([]*Client, 0, len(h.clients))
	for c := range h.clients {
		allClients = append(allClients, c)
	}
	h.mu.Unlock()

	for _, c := range allClients {
		c.Close()
	}
}
