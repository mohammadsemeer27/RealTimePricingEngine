package websocket

import (
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/config"
)

// Client represents an active downstream WebSocket client connection.
type Client struct {
	hub           *Hub
	conn          *websocket.Conn
	send          chan []byte
	subscriptions map[string]struct{}
	subMu         sync.RWMutex
	cfg           config.WebSocketConfig
	logger        *slog.Logger
	isClosed      bool
	closeMu       sync.Mutex
}

// NewClient creates a new downstream client wrapper.
func NewClient(hub *Hub, conn *websocket.Conn, cfg config.WebSocketConfig, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}

	if cfg.EnableCompression {
		conn.EnableWriteCompression(true)
		_ = conn.SetCompressionLevel(6)
	}

	return &Client{
		hub:           hub,
		conn:          conn,
		send:          make(chan []byte, cfg.ClientQueueSize),
		subscriptions: make(map[string]struct{}),
		cfg:           cfg,
		logger:        logger,
	}
}

// IsSubscribed checks if the client is subscribed to a symbol.
func (c *Client) IsSubscribed(symbol string) bool {
	c.subMu.RLock()
	defer c.subMu.RUnlock()
	_, ok := c.subscriptions[symbol]
	return ok
}

// GetSubscriptions returns a copy of current subscribed symbols.
func (c *Client) GetSubscriptions() []string {
	c.subMu.RLock()
	defer c.subMu.RUnlock()
	symbols := make([]string, 0, len(c.subscriptions))
	for s := range c.subscriptions {
		symbols = append(symbols, s)
	}
	return symbols
}

// Subscribe adds symbols to client subscriptions.
func (c *Client) Subscribe(symbols []string) []string {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	added := make([]string, 0, len(symbols))
	for _, s := range symbols {
		norm := strings.ToUpper(strings.TrimSpace(s))
		if norm != "" {
			if _, exists := c.subscriptions[norm]; !exists {
				c.subscriptions[norm] = struct{}{}
				added = append(added, norm)
			}
		}
	}
	return added
}

// Unsubscribe removes symbols from client subscriptions.
func (c *Client) Unsubscribe(symbols []string) []string {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	removed := make([]string, 0, len(symbols))
	for _, s := range symbols {
		norm := strings.ToUpper(strings.TrimSpace(s))
		if _, exists := c.subscriptions[norm]; exists {
			delete(c.subscriptions, norm)
			removed = append(removed, norm)
		}
	}
	return removed
}

// QueueMessage attempts to queue a message without blocking the caller.
// If queue is full, the message is dropped or connection terminated (slow client policy).
func (c *Client) QueueMessage(msg []byte) bool {
	select {
	case c.send <- msg:
		return true
	default:
		// Queue full: slow client detected
		c.logger.Warn("slow client outbound buffer full, dropping frame")
		if c.hub.metrics != nil {
			c.hub.metrics.IncDroppedEvents("slow_client_drop")
		}
		return false
	}
}

// Close gracefully closes the client's send channel and connection.
func (c *Client) Close() {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	if c.isClosed {
		return
	}
	c.isClosed = true
	close(c.send)
	_ = c.conn.Close()
}

// ReadPump listens for incoming messages from the WebSocket client.
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.Close()
	}()

	c.conn.SetReadLimit(64 * 1024) // 64 KB max client request size
	_ = c.conn.SetReadDeadline(time.Now().Add(c.cfg.PongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(c.cfg.PongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var clientMsg ClientMessage
		if err := json.Unmarshal(message, &clientMsg); err != nil {
			errMsg, _ := EncodeError("invalid json message format")
			c.QueueMessage(errMsg)
			continue
		}

		c.handleClientMessage(clientMsg)
	}
}

// handleClientMessage handles subscribe, unsubscribe, snapshot, and ping commands.
func (c *Client) handleClientMessage(msg ClientMessage) {
	switch msg.Action {
	case ActionSubscribe:
		if len(msg.Symbols) == 0 {
			errMsg, _ := EncodeError("no symbols specified for subscribe")
			c.QueueMessage(errMsg)
			return
		}
		added := c.Subscribe(msg.Symbols)
		c.hub.RegisterSubscriptions(c, added)

		ack, _ := EncodeAck("subscribe", added)
		c.QueueMessage(ack)

		// Immediately push initial snapshots for each newly subscribed symbol
		for _, sym := range added {
			if snap, ok := c.hub.engine.GetSnapshot(sym); ok {
				snapData, err := EncodeSnapshot(snap)
				if err == nil {
					c.QueueMessage(snapData)
				}
			}
		}

	case ActionUnsubscribe:
		removed := c.Unsubscribe(msg.Symbols)
		c.hub.UnregisterSubscriptions(c, removed)
		ack, _ := EncodeAck("unsubscribe", removed)
		c.QueueMessage(ack)

	case ActionSnapshot:
		// Client requested fresh snapshots (e.g. after gap recovery)
		for _, sym := range msg.Symbols {
			norm := strings.ToUpper(strings.TrimSpace(sym))
			if snap, ok := c.hub.engine.GetSnapshot(norm); ok {
				snapData, err := EncodeSnapshot(snap)
				if err == nil {
					c.QueueMessage(snapData)
				}
			}
		}

	case ActionPing:
		ack, _ := EncodeAck("pong", nil)
		c.QueueMessage(ack)

	default:
		errMsg, _ := EncodeError("unsupported action: " + string(msg.Action))
		c.QueueMessage(errMsg)
	}
}

// WritePump manages the SINGLE WRITER routine per connection.
func (c *Client) WritePump() {
	ticker := time.NewTicker(c.cfg.PingInterval)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteTimeout))
			if !ok {
				// The hub closed the channel
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(msg)

			// Coalesce queued messages into the same frame if available
			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				_, _ = w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

			if c.hub.metrics != nil {
				c.hub.metrics.IncOutboundMessages()
				c.hub.metrics.ObservePayloadSize(len(msg))
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
