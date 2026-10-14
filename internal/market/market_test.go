package market

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/metrics"
	"github.com/shopspring/decimal"
)

func TestMidpointAndPercentageChange(t *testing.T) {
	baseTime := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	clock := NewMockClock(baseTime)
	rw := NewRollingWindow("EURUSD", 24*time.Hour)

	// Tick 1: Buy = 1.1000, Sell = 1.1020 => Mid = 1.1010
	tick1 := Tick{
		Symbol:        "EURUSD",
		Buy:           decimal.NewFromFloat(1.1000),
		Sell:          decimal.NewFromFloat(1.1020),
		MidPrice:      decimal.NewFromFloat(1.1010),
		EventTime:     baseTime,
		ProcessedTime: baseTime,
	}

	snap1, err := rw.ApplyTick(tick1, clock)
	if err != nil {
		t.Fatalf("unexpected error applying tick 1: %v", err)
	}

	if !snap1.MidPrice.Equal(decimal.NewFromFloat(1.1010)) {
		t.Errorf("expected mid 1.1010, got %s", snap1.MidPrice)
	}
	if !snap1.High24h.Equal(decimal.NewFromFloat(1.1010)) {
		t.Errorf("expected high 1.1010, got %s", snap1.High24h)
	}
	if !snap1.Low24h.Equal(decimal.NewFromFloat(1.1010)) {
		t.Errorf("expected low 1.1010, got %s", snap1.Low24h)
	}
	// On first tick, latestMid == referenceMid => change% should be 0
	if !snap1.Change24hPct.Equal(decimal.Zero) {
		t.Errorf("expected change 0, got %s", snap1.Change24hPct)
	}

	// Tick 2: 1 hour later, Mid = 1.2111 (10% higher)
	clock.Advance(1 * time.Hour)
	tick2 := Tick{
		Symbol:        "EURUSD",
		Buy:           decimal.NewFromFloat(1.2110),
		Sell:          decimal.NewFromFloat(1.2112),
		MidPrice:      decimal.NewFromFloat(1.2111),
		EventTime:     clock.Now(),
		ProcessedTime: clock.Now(),
	}

	snap2, err := rw.ApplyTick(tick2, clock)
	if err != nil {
		t.Fatalf("unexpected error applying tick 2: %v", err)
	}

	if !snap2.High24h.Equal(decimal.NewFromFloat(1.2111)) {
		t.Errorf("expected high 1.2111, got %s", snap2.High24h)
	}
	if !snap2.Low24h.Equal(decimal.NewFromFloat(1.1010)) {
		t.Errorf("expected low 1.1010, got %s", snap2.Low24h)
	}

	// Expected change = ((1.2111 - 1.1010) / 1.1010) * 100 = 10%
	expectedChange := decimal.NewFromFloat(1.2111).Sub(decimal.NewFromFloat(1.1010)).
		Div(decimal.NewFromFloat(1.1010)).Mul(decimal.NewFromInt(100)).Round(4)

	if !snap2.Change24hPct.Equal(expectedChange) {
		t.Errorf("expected change %s, got %s", expectedChange, snap2.Change24hPct)
	}
}

func TestRollingWindowExpiration(t *testing.T) {
	baseTime := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	clock := NewMockClock(baseTime)
	rw := NewRollingWindow("BTCUSD", 24*time.Hour)

	// Tick 1 at T=0h: Mid = 60,000 (becomes High and Reference)
	tick1 := Tick{
		Symbol:    "BTCUSD",
		Buy:       decimal.NewFromInt(59990),
		Sell:      decimal.NewFromInt(60010),
		MidPrice:  decimal.NewFromInt(60000),
		EventTime: baseTime,
	}
	rw.ApplyTick(tick1, clock)

	// Tick 2 at T=12h: Mid = 55,000 (Low)
	clock.Advance(12 * time.Hour)
	tick2 := Tick{
		Symbol:    "BTCUSD",
		Buy:       decimal.NewFromInt(54990),
		Sell:      decimal.NewFromInt(55010),
		MidPrice:  decimal.NewFromInt(55000),
		EventTime: clock.Now(),
	}
	rw.ApplyTick(tick2, clock)

	// Tick 3 at T=25h: Mid = 56,000.
	// Tick 1 (at T=0h) is now older than 24 hours (25h - 24h = 1h > 0h) and MUST expire!
	// New window contains Tick 2 (55,000) and Tick 3 (56,000).
	// Therefore:
	// New High: 56,000 (Tick 1's 60,000 expired)
	// New Low: 55,000
	// New Reference: 55,000 (Tick 2 is now earliest)
	clock.Advance(13 * time.Hour)
	tick3 := Tick{
		Symbol:    "BTCUSD",
		Buy:       decimal.NewFromInt(55990),
		Sell:      decimal.NewFromInt(56010),
		MidPrice:  decimal.NewFromInt(56000),
		EventTime: clock.Now(),
	}
	snap3, err := rw.ApplyTick(tick3, clock)
	if err != nil {
		t.Fatalf("unexpected error applying tick 3: %v", err)
	}

	if !snap3.High24h.Equal(decimal.NewFromInt(56000)) {
		t.Errorf("expected high to drop to 56000 after 60000 expired, got %s", snap3.High24h)
	}
	if !snap3.Low24h.Equal(decimal.NewFromInt(55000)) {
		t.Errorf("expected low 55000, got %s", snap3.Low24h)
	}
	if !snap3.ReferencePrice.Equal(decimal.NewFromInt(55000)) {
		t.Errorf("expected new reference price 55000, got %s", snap3.ReferencePrice)
	}

	// Change: ((56000 - 55000) / 55000) * 100 = 1.8182%
	expectedChange := decimal.NewFromInt(1000).Div(decimal.NewFromInt(55000)).Mul(decimal.NewFromInt(100)).Round(4)
	if !snap3.Change24hPct.Equal(expectedChange) {
		t.Errorf("expected change %s, got %s", expectedChange, snap3.Change24hPct)
	}
}

func TestDuplicateAndOutOfOrderHandling(t *testing.T) {
	baseTime := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	clock := NewMockClock(baseTime)
	rw := NewRollingWindow("EURUSD", 24*time.Hour)

	tick1 := Tick{
		Symbol:    "EURUSD",
		Buy:       decimal.NewFromFloat(1.1000),
		Sell:      decimal.NewFromFloat(1.1010),
		MidPrice:  decimal.NewFromFloat(1.1005),
		EventTime: baseTime,
	}

	_, err := rw.ApplyTick(tick1, clock)
	if err != nil {
		t.Fatalf("failed applying tick 1: %v", err)
	}

	// Identical tick (same timestamp, buy, sell) -> duplicate error
	_, err = rw.ApplyTick(tick1, clock)
	if err != ErrDuplicateTick {
		t.Errorf("expected ErrDuplicateTick, got: %v", err)
	}

	// Out of order tick (earlier timestamp)
	tickOut := Tick{
		Symbol:    "EURUSD",
		Buy:       decimal.NewFromFloat(1.1002),
		Sell:      decimal.NewFromFloat(1.1012),
		MidPrice:  decimal.NewFromFloat(1.1007),
		EventTime: baseTime.Add(-10 * time.Second),
	}
	_, err = rw.ApplyTick(tickOut, clock)
	if err == nil {
		t.Errorf("expected error for backwards out-of-order timestamp, got nil")
	}
}

func TestPriceValidationAndCrossedMarket(t *testing.T) {
	clock := NewMockClock(time.Now())

	// Crossed market (buy > sell)
	rawCrossed := RawTick{
		Symbol:    "EURUSD",
		Buy:       json.RawMessage(`"1.1050"`),
		Sell:      json.RawMessage(`"1.1020"`),
		Timestamp: json.RawMessage(`1700000000000`),
	}
	_, err := ParseRawTick(rawCrossed, clock)
	if err == nil {
		t.Errorf("expected error for crossed market (buy > sell), got nil")
	}

	// Non-positive price
	rawZero := RawTick{
		Symbol:    "EURUSD",
		Buy:       json.RawMessage(`"0.0"`),
		Sell:      json.RawMessage(`"1.1020"`),
		Timestamp: json.RawMessage(`1700000000000`),
	}
	_, err = ParseRawTick(rawZero, clock)
	if err == nil {
		t.Errorf("expected error for zero price, got nil")
	}

	// Empty symbol
	rawEmptySym := RawTick{
		Symbol:    "",
		Buy:       json.RawMessage(`"1.1000"`),
		Sell:      json.RawMessage(`"1.1020"`),
		Timestamp: json.RawMessage(`1700000000000`),
	}
	_, err = ParseRawTick(rawEmptySym, clock)
	if err == nil {
		t.Errorf("expected error for empty symbol, got nil")
	}
}

func TestEngineIntegration(t *testing.T) {
	m := metrics.NewMetrics()
	clock := NewMockClock(time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC))
	eng := NewEngine(24*time.Hour, clock, m, []string{"EURUSD", "GBPUSD"})

	raw := RawTick{
		Symbol:    "EURUSD",
		Buy:       json.RawMessage(`"1.0850"`),
		Sell:      json.RawMessage(`"1.0852"`),
		Timestamp: json.RawMessage(`"2026-10-14T10:00:00Z"`),
	}

	snap, err := eng.ProcessRawTick(context.Background(), raw)
	if err != nil {
		t.Fatalf("failed processing valid raw tick: %v", err)
	}

	if snap.Symbol != "EURUSD" {
		t.Errorf("expected EURUSD, got %s", snap.Symbol)
	}

	// Query via engine
	queried, ok := eng.GetSnapshot("EURUSD")
	if !ok {
		t.Fatalf("symbol EURUSD not found in engine")
	}
	if !queried.MidPrice.Equal(decimal.NewFromFloat(1.0851)) {
		t.Errorf("expected mid 1.0851, got %s", queried.MidPrice)
	}

	// Check metrics
	snapMetrics := m.GetSnapshot()
	if snapMetrics.IncomingTicks != 1 || snapMetrics.ProcessedTicks != 1 {
		t.Errorf("expected 1 incoming and 1 processed tick, got %d and %d", snapMetrics.IncomingTicks, snapMetrics.ProcessedTicks)
	}
}
