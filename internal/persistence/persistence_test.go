package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/market"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/metrics"
	"github.com/shopspring/decimal"
)

func TestMemoryStoreRecovery(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	cutoff := now.Add(-24 * time.Hour)

	store := NewMemoryStore()

	// 1. Tick outside 24h window (30h ago) -> should be excluded
	store.QueueTick(market.Tick{
		Symbol:    "EURUSD",
		Buy:       decimal.NewFromFloat(1.0000),
		Sell:      decimal.NewFromFloat(1.0010),
		MidPrice:  decimal.NewFromFloat(1.0005),
		EventTime: now.Add(-30 * time.Hour),
	})

	// 2. Tick inside 24h window (10h ago) -> High = 1.2000
	store.QueueTick(market.Tick{
		Symbol:    "EURUSD",
		Buy:       decimal.NewFromFloat(1.1990),
		Sell:      decimal.NewFromFloat(1.2010),
		MidPrice:  decimal.NewFromFloat(1.2000),
		EventTime: now.Add(-10 * time.Hour),
	})

	// 3. Tick inside 24h window (2h ago) -> Low = 1.1500
	store.QueueTick(market.Tick{
		Symbol:    "EURUSD",
		Buy:       decimal.NewFromFloat(1.1490),
		Sell:      decimal.NewFromFloat(1.1510),
		MidPrice:  decimal.NewFromFloat(1.1500),
		EventTime: now.Add(-2 * time.Hour),
	})

	// Create a new fresh Engine simulating application restart
	m := metrics.NewMetrics()
	clock := market.NewMockClock(now)
	engine := market.NewEngine(24*time.Hour, clock, m, []string{"EURUSD"})

	// Restore state
	restoredCount, err := store.RestoreRollingState(context.Background(), cutoff, engine)
	if err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	if restoredCount != 2 {
		t.Errorf("expected 2 restored ticks (1 excluded), got %d", restoredCount)
	}

	// Verify restored statistics in engine
	snap, ok := engine.GetSnapshot("EURUSD")
	if !ok {
		t.Fatalf("snapshot for EURUSD not found after recovery")
	}

	if !snap.High24h.Equal(decimal.NewFromFloat(1.2000)) {
		t.Errorf("expected recovered high 1.2000, got %s", snap.High24h)
	}
	if !snap.Low24h.Equal(decimal.NewFromFloat(1.1500)) {
		t.Errorf("expected recovered low 1.1500, got %s", snap.Low24h)
	}
	if !snap.ReferencePrice.Equal(decimal.NewFromFloat(1.2000)) {
		t.Errorf("expected reference price 1.2000, got %s", snap.ReferencePrice)
	}
}
