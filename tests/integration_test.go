package tests

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/api"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/config"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/market"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/metrics"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/persistence"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/upstream"
	internalws "github.com/mohammadsemeer27/RealTimePricingEngine/internal/websocket"
)

func TestEndToEndPipeline(t *testing.T) {
	// 1. Mock upstream streaming every 15ms
	mockUpstream := upstream.NewMockServer(upstream.WithMockTickInterval(15 * time.Millisecond))
	defer mockUpstream.Close()

	// 2. Metrics & Engine setup
	m := metrics.NewMetrics()
	clock := market.RealClock{}
	engine := market.NewEngine(24*time.Hour, clock, m, upstream.DefaultSymbols)

	// 3. Persistence store
	store := persistence.NewMemoryStore()
	engine.SetPersistence(store)

	// 4. Downstream WebSocket Hub
	wsCfg := config.WebSocketConfig{
		MaxConnections:    100,
		ClientQueueSize:   128,
		WriteTimeout:      2 * time.Second,
		PingInterval:      10 * time.Second,
		PongWait:          15 * time.Second,
		EnableCompression: true,
	}
	hub := internalws.NewHub(wsCfg, engine, m, nil)
	engine.SetBroadcaster(hub)

	// 5. API Server & HTTP Test Server
	apiServer := api.NewServer(engine, m, store, hub)
	testServer := httptest.NewServer(apiServer.Routes())
	defer testServer.Close()
	defer hub.Close()

	// 6. Connect upstream client
	upstreamClient := upstream.NewClient(upstream.ClientConfig{
		URL:              mockUpstream.URL(),
		APIKey:           "valid_key",
		SecretKey:        "valid_secret",
		Symbols:          upstream.DefaultSymbols,
		ReconnectInitial: 50 * time.Millisecond,
		Metrics:          m,
		Handler: func(ctx context.Context, raw market.RawTick) {
			_, _ = engine.ProcessRawTick(ctx, raw)
		},
	})
	upstreamClient.Start()
	defer upstreamClient.Close()

	// 7. Connect 3 downstream clients subscribing to different pairs
	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/ws"

	var wg sync.WaitGroup
	var client1Received, client2Received atomic.Uint64
	var maxObservedLatencyMs atomic.Int64

	// Client 1: Subscribes to EURUSD
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Errorf("client 1 dial failed: %v", err)
			return
		}
		defer conn.Close()

		// Subscribe
		_ = conn.WriteJSON(internalws.ClientMessage{
			Action:  internalws.ActionSubscribe,
			Symbols: []string{"EURUSD"},
		})

		// Read messages
		for client1Received.Load() < 3 {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var delta market.MarketDelta
			if err := json.Unmarshal(msg, &delta); err == nil && delta.Symbol == "EURUSD" {
				client1Received.Add(1)
				if delta.EventTimeUnix > 0 {
					latency := time.Now().UnixMilli() - delta.EventTimeUnix
					if latency > maxObservedLatencyMs.Load() {
						maxObservedLatencyMs.Store(latency)
					}
				}
			}
		}
	}()

	// Client 2: Subscribes to BTCUSD
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Errorf("client 2 dial failed: %v", err)
			return
		}
		defer conn.Close()

		_ = conn.WriteJSON(internalws.ClientMessage{
			Action:  internalws.ActionSubscribe,
			Symbols: []string{"BTCUSD"},
		})

		for client2Received.Load() < 3 {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var delta market.MarketDelta
			if err := json.Unmarshal(msg, &delta); err == nil && delta.Symbol == "BTCUSD" {
				client2Received.Add(1)
			}
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Succeeded!
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for downstream messages. Client 1 received %d, Client 2 received %d",
			client1Received.Load(), client2Received.Load())
	}

	t.Logf("End-to-end pipeline test passed. Max observed distribution latency: %d ms", maxObservedLatencyMs.Load())
}
