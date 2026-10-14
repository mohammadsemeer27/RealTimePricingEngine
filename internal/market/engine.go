package market

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/metrics"
	"github.com/shopspring/decimal"
)

// PersistenceQueue defines the interface for passing ticks to background persistence.
type PersistenceQueue interface {
	QueueTick(tick Tick)
}

// BroadcastHub defines the interface for broadcasting snapshots to downstream clients.
type BroadcastHub interface {
	BroadcastSnapshot(snapshot MarketSnapshot)
}

// Engine manages real-time pricing and statistics across all symbols.
type Engine struct {
	mu             sync.RWMutex
	windows        map[string]*RollingWindow
	windowDuration time.Duration
	clock          Clock
	metrics        *metrics.Metrics
	persistence    PersistenceQueue
	broadcaster    BroadcastHub
	allowedSymbols map[string]struct{}
}

// NewEngine initializes a new market processing engine.
func NewEngine(windowDuration time.Duration, clk Clock, m *metrics.Metrics, symbols []string) *Engine {
	if clk == nil {
		clk = RealClock{}
	}
	allowed := make(map[string]struct{}, len(symbols))
	for _, s := range symbols {
		allowed[strings.ToUpper(strings.TrimSpace(s))] = struct{}{}
	}

	return &Engine{
		windows:        make(map[string]*RollingWindow),
		windowDuration: windowDuration,
		clock:          clk,
		metrics:        m,
		allowedSymbols: allowed,
	}
}

// SetPersistence sets the persistence queue handler.
func (e *Engine) SetPersistence(p PersistenceQueue) {
	e.persistence = p
}

// SetBroadcaster sets the downstream broadcast hub handler.
func (e *Engine) SetBroadcaster(b BroadcastHub) {
	e.broadcaster = b
}

// ProcessRawTick parses, validates, and updates the market state from a raw tick.
func (e *Engine) ProcessRawTick(ctx context.Context, raw RawTick) (MarketSnapshot, error) {
	start := time.Now()
	e.metrics.IncIncomingTicks()

	// 1. Symbol validation
	symbol := strings.ToUpper(strings.TrimSpace(raw.Symbol))
	if symbol == "" {
		e.metrics.IncDroppedEvents("empty_symbol")
		return MarketSnapshot{}, ErrEmptySymbol
	}
	if len(e.allowedSymbols) > 0 {
		if _, ok := e.allowedSymbols[symbol]; !ok {
			e.metrics.IncDroppedEvents("unrecognized_symbol")
			return MarketSnapshot{}, errors.New("unrecognized symbol: " + symbol)
		}
	}
	raw.Symbol = symbol

	// 2. Parse and validate fields
	tick, err := ParseRawTick(raw, e.clock)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidBuyPrice):
			e.metrics.IncDroppedEvents("invalid_buy_price")
		case errors.Is(err, ErrInvalidSellPrice):
			e.metrics.IncDroppedEvents("invalid_sell_price")
		case errors.Is(err, ErrCrossedMarket):
			e.metrics.IncDroppedEvents("crossed_market")
		case errors.Is(err, ErrInvalidTimestamp):
			e.metrics.IncDroppedEvents("invalid_timestamp")
		case errors.Is(err, ErrFutureTimestamp):
			e.metrics.IncDroppedEvents("future_timestamp")
		default:
			e.metrics.IncDroppedEvents("parse_error")
		}
		return MarketSnapshot{}, err
	}

	// 3. Obtain or create rolling window for symbol
	rw := e.getOrCreateWindow(tick.Symbol)

	// 4. Apply tick to rolling window
	snapshot, err := rw.ApplyTick(tick, e.clock)
	if err != nil {
		if errors.Is(err, ErrDuplicateTick) {
			e.metrics.IncDroppedEvents("duplicate_tick")
		} else if errors.Is(err, ErrOutdatedTick) {
			e.metrics.IncDroppedEvents("outdated_tick")
		} else {
			e.metrics.IncDroppedEvents("apply_error")
		}
		return MarketSnapshot{}, err
	}

	// 5. Asynchronously persist tick if persistence queue is attached
	if e.persistence != nil {
		e.persistence.QueueTick(tick)
	}

	// 6. Broadcast snapshot to downstream clients if broadcaster is attached
	if e.broadcaster != nil {
		e.broadcaster.BroadcastSnapshot(snapshot)
	}

	e.metrics.IncProcessedTicks()
	e.metrics.ObserveProcessingLatency(time.Since(start))

	return snapshot, nil
}

// GetSnapshot retrieves the latest snapshot for a symbol.
func (e *Engine) GetSnapshot(symbol string) (MarketSnapshot, bool) {
	e.mu.RLock()
	rw, exists := e.windows[strings.ToUpper(strings.TrimSpace(symbol))]
	e.mu.RUnlock()

	if !exists {
		return MarketSnapshot{}, false
	}
	return rw.GetSnapshot(e.clock)
}

// GetAllSnapshots returns snapshots for all active symbols, optionally filtered by symbol list.
func (e *Engine) GetAllSnapshots(filterSymbols []string) map[string]MarketSnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()

	filterMap := make(map[string]struct{}, len(filterSymbols))
	for _, s := range filterSymbols {
		filterMap[strings.ToUpper(strings.TrimSpace(s))] = struct{}{}
	}

	results := make(map[string]MarketSnapshot)
	for sym, rw := range e.windows {
		if len(filterMap) > 0 {
			if _, ok := filterMap[sym]; !ok {
				continue
			}
		}
		if snap, ok := rw.GetSnapshot(e.clock); ok {
			results[sym] = snap
		}
	}
	return results
}

// RestoreHistoricalObservations seeds historical ticks from persistence at startup.
func (e *Engine) RestoreHistoricalObservations(symbol string, observations []HistoricalObservation, cutoff time.Time) {
	rw := e.getOrCreateWindow(symbol)
	for _, obs := range observations {
		rw.SeedObservation(obs.Timestamp, obs.Buy, obs.Sell, obs.MidPrice, cutoff)
	}
}

// HistoricalObservation represents a persisted observation used to restore window statistics.
type HistoricalObservation struct {
	Timestamp time.Time
	Buy       decimal.Decimal
	Sell      decimal.Decimal
	MidPrice  decimal.Decimal
}

func (e *Engine) getOrCreateWindow(symbol string) *RollingWindow {
	e.mu.RLock()
	rw, exists := e.windows[symbol]
	e.mu.RUnlock()

	if exists {
		return rw
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	rw, exists = e.windows[symbol]
	if !exists {
		rw = NewRollingWindow(symbol, e.windowDuration)
		e.windows[symbol] = rw
	}
	return rw
}
