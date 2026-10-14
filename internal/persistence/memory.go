package persistence

import (
	"context"
	"sync"
	"time"

	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/market"
)

// MemoryStore provides an in-memory implementation of Store for testing and development.
type MemoryStore struct {
	mu    sync.RWMutex
	ticks []market.Tick
}

// NewMemoryStore initializes an in-memory persistence store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		ticks: make([]market.Tick, 0, 1024),
	}
}

// QueueTick stores the tick in memory.
func (m *MemoryStore) QueueTick(tick market.Tick) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ticks = append(m.ticks, tick)
}

// RestoreRollingState replays historical ticks within cutoff into the engine.
func (m *MemoryStore) RestoreRollingState(ctx context.Context, cutoff time.Time, engine *market.Engine) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	grouped := make(map[string][]market.HistoricalObservation)
	count := 0

	for _, t := range m.ticks {
		if t.EventTime.Before(cutoff) {
			continue
		}
		grouped[t.Symbol] = append(grouped[t.Symbol], market.HistoricalObservation{
			Timestamp: t.EventTime,
			Buy:       t.Buy,
			Sell:      t.Sell,
			MidPrice:  t.MidPrice,
		})
		count++
	}

	for sym, obs := range grouped {
		engine.RestoreHistoricalObservations(sym, obs, cutoff)
	}

	return count, nil
}

// Ping always succeeds for in-memory store.
func (m *MemoryStore) Ping(ctx context.Context) error {
	return nil
}

// Close is a no-op for in-memory store.
func (m *MemoryStore) Close() error {
	return nil
}
