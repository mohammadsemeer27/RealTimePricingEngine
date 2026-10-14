package websocket

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/config"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/market"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/metrics"
	"github.com/shopspring/decimal"
)

type mockMarketReader struct {
	snapshots map[string]market.MarketSnapshot
}

func (m *mockMarketReader) GetSnapshot(symbol string) (market.MarketSnapshot, bool) {
	snap, ok := m.snapshots[symbol]
	return snap, ok
}

func TestDownstreamWebSocketFlow(t *testing.T) {
	m := metrics.NewMetrics()
	engine := &mockMarketReader{
		snapshots: map[string]market.MarketSnapshot{
			"EURUSD": {
				Symbol:         "EURUSD",
				Buy:            decimal.NewFromFloat(1.0850),
				Sell:           decimal.NewFromFloat(1.0852),
				MidPrice:       decimal.NewFromFloat(1.0851),
				High24h:        decimal.NewFromFloat(1.0900),
				Low24h:         decimal.NewFromFloat(1.0800),
				Change24hPct:   decimal.NewFromFloat(0.15),
				ReferencePrice: decimal.NewFromFloat(1.0835),
				EventTime:      time.Now().UTC(),
				Sequence:       1,
			},
		},
	}

	cfg := config.WebSocketConfig{
		MaxConnections:    10,
		ClientQueueSize:   32,
		WriteTimeout:      2 * time.Second,
		PingInterval:      10 * time.Second,
		PongWait:          15 * time.Second,
		EnableCompression: true,
	}

	hub := NewHub(cfg, engine, m, nil)
	server := httptest.NewServer(hub)
	defer server.Close()
	defer hub.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Client 1 connects
	dialer := websocket.Dialer{}
	conn1, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial downstream ws: %v", err)
	}
	defer conn1.Close()

	// Subscribe to EURUSD
	subMsg := ClientMessage{
		Action:  ActionSubscribe,
		Symbols: []string{"EURUSD"},
	}
	if err := conn1.WriteJSON(subMsg); err != nil {
		t.Fatalf("failed to write subscribe msg: %v", err)
	}

	// 1. Read ACK
	_, msgAck, err := conn1.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read ack: %v", err)
	}
	var resp ServerResponse
	if err := json.Unmarshal(msgAck, &resp); err != nil || resp.Type != "ack" {
		t.Errorf("expected ack response, got: %s", string(msgAck))
	}

	// 2. Read initial snapshot
	_, msgSnap, err := conn1.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read snapshot: %v", err)
	}
	var snapMsg struct {
		Type string                `json:"type"`
		Data market.MarketSnapshot `json:"data"`
	}
	if err := json.Unmarshal(msgSnap, &snapMsg); err != nil || snapMsg.Data.Symbol != "EURUSD" {
		t.Errorf("expected EURUSD snapshot, got: %s", string(msgSnap))
	}

	// 3. Hub broadcasts delta
	testSnapshot := market.MarketSnapshot{
		Symbol:       "EURUSD",
		Buy:          decimal.NewFromFloat(1.0855),
		Sell:         decimal.NewFromFloat(1.0857),
		MidPrice:     decimal.NewFromFloat(1.0856),
		High24h:      decimal.NewFromFloat(1.0900),
		Low24h:       decimal.NewFromFloat(1.0800),
		Change24hPct: decimal.NewFromFloat(0.19),
		EventTime:    time.Now().UTC(),
		Sequence:     2,
	}

	hub.BroadcastSnapshot(testSnapshot)

	// Client 1 reads broadcast delta
	_, msgDelta, err := conn1.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read delta: %v", err)
	}

	var delta market.MarketDelta
	if err := json.Unmarshal(msgDelta, &delta); err != nil {
		t.Fatalf("failed to unmarshal delta: %v", err)
	}
	if delta.Symbol != "EURUSD" || !delta.MidPrice.Equal(decimal.NewFromFloat(1.0856)) {
		t.Errorf("unexpected delta content: %+v", delta)
	}

	// Verify payload size is well below 10 KB
	if len(msgDelta) > 10*1024 {
		t.Errorf("payload size exceeded 10KB: %d bytes", len(msgDelta))
	}
}

func TestPayloadSizeUnder10KB(t *testing.T) {
	snap := market.MarketSnapshot{
		Symbol:         "BTCUSD",
		Buy:            decimal.NewFromInt(65432),
		Sell:           decimal.NewFromInt(65434),
		MidPrice:       decimal.NewFromInt(65433),
		High24h:        decimal.NewFromInt(67000),
		Low24h:         decimal.NewFromInt(64000),
		Change24hPct:   decimal.NewFromFloat(2.25),
		ReferencePrice: decimal.NewFromInt(64000),
		EventTime:      time.Now().UTC(),
		Sequence:       1024,
	}

	deltaData, err := EncodeDelta(snap.ToDelta())
	if err != nil {
		t.Fatalf("failed to encode delta: %v", err)
	}

	snapData, err := EncodeSnapshot(snap)
	if err != nil {
		t.Fatalf("failed to encode snapshot: %v", err)
	}

	t.Logf("Delta payload size: %d bytes", len(deltaData))
	t.Logf("Snapshot payload size: %d bytes", len(snapData))

	if len(deltaData) >= 10*1024 {
		t.Errorf("delta payload must be < 10KB, got %d", len(deltaData))
	}
	if len(snapData) >= 10*1024 {
		t.Errorf("snapshot payload must be < 10KB, got %d", len(snapData))
	}
}

func TestSlowClientNonBlocking(t *testing.T) {
	m := metrics.NewMetrics()
	engine := &mockMarketReader{snapshots: map[string]market.MarketSnapshot{}}

	cfg := config.WebSocketConfig{
		MaxConnections:  10,
		ClientQueueSize: 2, // very small queue to test overflow
		WriteTimeout:    1 * time.Second,
		PingInterval:    10 * time.Second,
		PongWait:        15 * time.Second,
	}

	hub := NewHub(cfg, engine, m, nil)

	// Simulate slow client with tiny send buffer
	fakeClient := &Client{
		hub:           hub,
		send:          make(chan []byte, 1),
		subscriptions: map[string]struct{}{"EURUSD": {}},
		cfg:           cfg,
	}

	// Fill client queue
	fakeClient.send <- []byte("busy")

	var wg sync.WaitGroup
	wg.Add(1)

	// Broadcast should not block
	go func() {
		defer wg.Done()
		hub.BroadcastSnapshot(market.MarketSnapshot{
			Symbol:   "EURUSD",
			MidPrice: decimal.NewFromInt(1),
		})
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Broadcast completed immediately without hanging on slow client!
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("broadcast blocked on slow client!")
	}
}
