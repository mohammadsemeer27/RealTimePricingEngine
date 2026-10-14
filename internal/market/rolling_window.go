package market

import (
	"fmt"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// windowEntry holds an observation inside the rolling window.
type windowEntry struct {
	id        uint64
	timestamp time.Time
	midPrice  decimal.Decimal
}

// RollingWindow maintains the independent rolling 24-hour statistics for a single symbol.
type RollingWindow struct {
	mu sync.RWMutex

	symbol         string
	windowDuration time.Duration

	// Latest tick state
	latestBuy       decimal.Decimal
	latestSell      decimal.Decimal
	latestMid       decimal.Decimal
	latestEventTime time.Time
	latestProcTime  time.Time
	sequence        uint64
	entryCounter    uint64

	// Sliding window data structures
	// entries maintains chronological order of observations
	entries []windowEntry

	// maxDeque stores entries in strictly decreasing midPrice order (front is maximum)
	maxDeque []windowEntry

	// minDeque stores entries in strictly increasing midPrice order (front is minimum)
	minDeque []windowEntry
}

// NewRollingWindow initializes a rolling window for a symbol with the specified duration.
func NewRollingWindow(symbol string, duration time.Duration) *RollingWindow {
	if duration <= 0 {
		duration = 24 * time.Hour
	}
	return &RollingWindow{
		symbol:         symbol,
		windowDuration: duration,
		entries:        make([]windowEntry, 0, 512),
		maxDeque:       make([]windowEntry, 0, 128),
		minDeque:       make([]windowEntry, 0, 128),
	}
}

// ApplyTick updates the rolling window with a validated tick.
// Returns the resulting MarketSnapshot or an error if the tick is duplicate or out-of-order.
func (rw *RollingWindow) ApplyTick(tick Tick, clock Clock) (MarketSnapshot, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	// 1. Duplicate detection: same timestamp and same buy/sell as the latest tick
	if !rw.latestEventTime.IsZero() && tick.EventTime.Equal(rw.latestEventTime) {
		if tick.Buy.Equal(rw.latestBuy) && tick.Sell.Equal(rw.latestSell) {
			return MarketSnapshot{}, ErrDuplicateTick
		}
	}

	// 2. Out-of-order detection: reject ticks whose timestamp is backwards relative to the latest
	// processed tick for this symbol to maintain strictly causal rolling stats.
	if !rw.latestEventTime.IsZero() && tick.EventTime.Before(rw.latestEventTime) {
		return MarketSnapshot{}, fmt.Errorf("%w: tick time %v < latest time %v", ErrOutdatedTick, tick.EventTime, rw.latestEventTime)
	}

	// 3. Increment counters
	rw.sequence++
	rw.entryCounter++
	entryID := rw.entryCounter

	newEntry := windowEntry{
		id:        entryID,
		timestamp: tick.EventTime,
		midPrice:  tick.MidPrice,
	}

	// 4. Update latest values
	rw.latestBuy = tick.Buy
	rw.latestSell = tick.Sell
	rw.latestMid = tick.MidPrice
	rw.latestEventTime = tick.EventTime
	rw.latestProcTime = tick.ProcessedTime

	// 5. Expire observations older than (tick.EventTime - windowDuration)
	cutoff := tick.EventTime.Add(-rw.windowDuration)
	rw.expireOldEntries(cutoff)

	// 6. Append to chronological entries
	rw.entries = append(rw.entries, newEntry)

	// 7. Maintain maxDeque (monotonic decreasing)
	for len(rw.maxDeque) > 0 && rw.maxDeque[len(rw.maxDeque)-1].midPrice.LessThanOrEqual(newEntry.midPrice) {
		rw.maxDeque = rw.maxDeque[:len(rw.maxDeque)-1]
	}
	rw.maxDeque = append(rw.maxDeque, newEntry)

	// 8. Maintain minDeque (monotonic increasing)
	for len(rw.minDeque) > 0 && rw.minDeque[len(rw.minDeque)-1].midPrice.GreaterThanOrEqual(newEntry.midPrice) {
		rw.minDeque = rw.minDeque[:len(rw.minDeque)-1]
	}
	rw.minDeque = append(rw.minDeque, newEntry)

	// 9. Compute rolling statistics
	high24h := rw.maxDeque[0].midPrice
	low24h := rw.minDeque[0].midPrice

	// Reference price is the earliest valid midpoint in the rolling 24-hour window
	referencePrice := rw.entries[0].midPrice

	// Calculate percentage change: ((latestMidPrice - referenceMidPrice) / referenceMidPrice) * 100
	var change24hPct decimal.Decimal
	if referencePrice.IsPositive() {
		change24hPct = rw.latestMid.Sub(referencePrice).Div(referencePrice).Mul(decimal.NewFromInt(100)).Round(4)
	} else {
		change24hPct = decimal.Zero
	}

	now := clock.Now()
	freshnessMs := now.Sub(tick.EventTime).Milliseconds()
	if freshnessMs < 0 {
		freshnessMs = 0
	}
	isStale := freshnessMs > (60 * 1000) // Stale if older than 60 seconds

	snapshot := MarketSnapshot{
		Symbol:         rw.symbol,
		Buy:            rw.latestBuy,
		Sell:           rw.latestSell,
		MidPrice:       rw.latestMid,
		High24h:        high24h,
		Low24h:         low24h,
		Change24hPct:   change24hPct,
		ReferencePrice: referencePrice,
		EventTime:      rw.latestEventTime,
		ProcessedTime:  rw.latestProcTime,
		Sequence:       rw.sequence,
		FreshnessMs:    freshnessMs,
		IsStale:        isStale,
	}

	return snapshot, nil
}

// SeedObservation inserts a historical observation into the window during startup recovery.
// Assumes observations are provided in chronological order.
func (rw *RollingWindow) SeedObservation(timestamp time.Time, buy, sell, mid decimal.Decimal, cutoff time.Time) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if timestamp.Before(cutoff) {
		return // Outside window, discard
	}

	rw.entryCounter++
	entryID := rw.entryCounter
	entry := windowEntry{
		id:        entryID,
		timestamp: timestamp,
		midPrice:  mid,
	}

	rw.entries = append(rw.entries, entry)
	rw.latestBuy = buy
	rw.latestSell = sell
	rw.latestMid = mid
	rw.latestEventTime = timestamp
	rw.latestProcTime = timestamp

	// Maintain maxDeque
	for len(rw.maxDeque) > 0 && rw.maxDeque[len(rw.maxDeque)-1].midPrice.LessThanOrEqual(entry.midPrice) {
		rw.maxDeque = rw.maxDeque[:len(rw.maxDeque)-1]
	}
	rw.maxDeque = append(rw.maxDeque, entry)

	// Maintain minDeque
	for len(rw.minDeque) > 0 && rw.minDeque[len(rw.minDeque)-1].midPrice.GreaterThanOrEqual(entry.midPrice) {
		rw.minDeque = rw.minDeque[:len(rw.minDeque)-1]
	}
	rw.minDeque = append(rw.minDeque, entry)
}

// GetSnapshot returns the current state of the symbol without modifying it.
func (rw *RollingWindow) GetSnapshot(clock Clock) (MarketSnapshot, bool) {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	if len(rw.entries) == 0 {
		return MarketSnapshot{}, false
	}

	high24h := rw.maxDeque[0].midPrice
	low24h := rw.minDeque[0].midPrice
	referencePrice := rw.entries[0].midPrice

	var change24hPct decimal.Decimal
	if referencePrice.IsPositive() {
		change24hPct = rw.latestMid.Sub(referencePrice).Div(referencePrice).Mul(decimal.NewFromInt(100)).Round(4)
	}

	now := clock.Now()
	freshnessMs := now.Sub(rw.latestEventTime).Milliseconds()
	if freshnessMs < 0 {
		freshnessMs = 0
	}

	return MarketSnapshot{
		Symbol:         rw.symbol,
		Buy:            rw.latestBuy,
		Sell:           rw.latestSell,
		MidPrice:       rw.latestMid,
		High24h:        high24h,
		Low24h:         low24h,
		Change24hPct:   change24hPct,
		ReferencePrice: referencePrice,
		EventTime:      rw.latestEventTime,
		ProcessedTime:  rw.latestProcTime,
		Sequence:       rw.sequence,
		FreshnessMs:    freshnessMs,
		IsStale:        freshnessMs > (60 * 1000),
	}, true
}

// expireOldEntries removes entries older than cutoff from entries, maxDeque, and minDeque.
func (rw *RollingWindow) expireOldEntries(cutoff time.Time) {
	// 1. Identify how many entries to expire
	expireCount := 0
	for expireCount < len(rw.entries) && rw.entries[expireCount].timestamp.Before(cutoff) {
		expireCount++
	}

	if expireCount == 0 {
		return
	}

	// 2. Remove expired items from deques
	for i := 0; i < expireCount; i++ {
		expiredID := rw.entries[i].id
		if len(rw.maxDeque) > 0 && rw.maxDeque[0].id == expiredID {
			rw.maxDeque = rw.maxDeque[1:]
		}
		if len(rw.minDeque) > 0 && rw.minDeque[0].id == expiredID {
			rw.minDeque = rw.minDeque[1:]
		}
	}

	// 3. Slice off expired entries, and periodically re-allocate if slice grows large
	rw.entries = rw.entries[expireCount:]
	if cap(rw.entries) > 2048 && len(rw.entries) < cap(rw.entries)/4 {
		newEntries := make([]windowEntry, len(rw.entries), len(rw.entries)*2)
		copy(newEntries, rw.entries)
		rw.entries = newEntries
	}
}
