package upstream

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/market"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/metrics"
)

func TestClientConnectAndReceive(t *testing.T) {
	mockServer := NewMockServer(WithMockTickInterval(20 * time.Millisecond))
	defer mockServer.Close()

	m := metrics.NewMetrics()
	var receivedTicks atomic.Uint64
	var wg sync.WaitGroup
	wg.Add(3)

	client := NewClient(ClientConfig{
		URL:              mockServer.URL(),
		APIKey:           "test_key",
		SecretKey:        "test_secret",
		Symbols:          []string{"EURUSD", "BTCUSD"},
		ReconnectInitial: 50 * time.Millisecond,
		ReconnectMax:     500 * time.Millisecond,
		ReadTimeout:      2 * time.Second,
		Metrics:          m,
		Handler: func(ctx context.Context, raw market.RawTick) {
			if receivedTicks.Add(1) <= 3 {
				wg.Done()
			}
		},
	})

	client.Start()
	defer client.Close()

	// Wait for at least 3 ticks to be received
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success!
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for ticks, received %d", receivedTicks.Load())
	}

	if receivedTicks.Load() < 3 {
		t.Errorf("expected at least 3 ticks, got %d", receivedTicks.Load())
	}
}

func TestClientAuthFailureHandling(t *testing.T) {
	// Configure mock server to reject auth
	mockServer := NewMockServer(WithMockAuthFailure(true))
	defer mockServer.Close()

	m := metrics.NewMetrics()
	client := NewClient(ClientConfig{
		URL:              mockServer.URL(),
		APIKey:           "bad_key",
		SecretKey:        "bad_secret",
		Symbols:          []string{"EURUSD"},
		ReconnectInitial: 50 * time.Millisecond,
		Metrics:          m,
	})

	client.Start()
	defer client.Close()

	// Give client time to attempt auth
	time.Sleep(200 * time.Millisecond)

	snap := m.GetSnapshot()
	if snap.AuthFailures == 0 {
		t.Errorf("expected at least 1 auth failure recorded in metrics, got %d", snap.AuthFailures)
	}
}
