package market

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrEmptySymbol      = errors.New("empty symbol")
	ErrInvalidBuyPrice  = errors.New("invalid or non-positive buy price")
	ErrInvalidSellPrice = errors.New("invalid or non-positive sell price")
	ErrCrossedMarket    = errors.New("crossed market: buy price exceeds sell price")
	ErrInvalidTimestamp = errors.New("invalid event timestamp")
	ErrFutureTimestamp  = errors.New("timestamp is too far in future")
	ErrDuplicateTick    = errors.New("duplicate tick")
	ErrOutdatedTick     = errors.New("outdated or stale tick")
)

// Clock defines a time provider interface for deterministic testing.
type Clock interface {
	Now() time.Time
}

// RealClock implements Clock using time.Now().
type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now()
}

// MockClock provides a controllable clock for testing.
type MockClock struct {
	currentTime time.Time
}

// NewMockClock creates a mock clock initialized to the given time.
func NewMockClock(t time.Time) *MockClock {
	return &MockClock{currentTime: t}
}

func (m *MockClock) Now() time.Time {
	return m.currentTime
}

// Set updates the mock clock's current time.
func (m *MockClock) Set(t time.Time) {
	m.currentTime = t
}

// Advance advances the mock clock by a duration.
func (m *MockClock) Advance(d time.Duration) {
	m.currentTime = m.currentTime.Add(d)
}

// ParseRawTick parses a RawTick into a validated domain Tick.
func ParseRawTick(raw RawTick, clock Clock) (Tick, error) {
	symbol := strings.TrimSpace(raw.Symbol)
	if symbol == "" {
		return Tick{}, ErrEmptySymbol
	}

	buy, err := parseDecimal(raw.Buy)
	if err != nil || !buy.IsPositive() {
		return Tick{}, fmt.Errorf("%w: %v", ErrInvalidBuyPrice, err)
	}

	sell, err := parseDecimal(raw.Sell)
	if err != nil || !sell.IsPositive() {
		return Tick{}, fmt.Errorf("%w: %v", ErrInvalidSellPrice, err)
	}

	// Sanity check: in standard liquid markets, buy (bid) must be <= sell (ask)
	if buy.GreaterThan(sell) {
		return Tick{}, fmt.Errorf("%w: buy %s > sell %s", ErrCrossedMarket, buy.String(), sell.String())
	}

	eventTime, err := parseTimestamp(raw.Timestamp)
	if err != nil {
		return Tick{}, fmt.Errorf("%w: %v", ErrInvalidTimestamp, err)
	}

	// Check if timestamp is unreasonably far in the future (> 5 minutes clock drift)
	now := clock.Now()
	if eventTime.After(now.Add(5 * time.Minute)) {
		return Tick{}, fmt.Errorf("%w: event time %v is ahead of system time %v", ErrFutureTimestamp, eventTime, now)
	}

	// Calculate midpoint: (buy + sell) / 2
	midPrice := buy.Add(sell).Div(decimal.NewFromInt(2))

	return Tick{
		Symbol:        symbol,
		Buy:           buy,
		Sell:          sell,
		MidPrice:      midPrice,
		EventTime:     eventTime,
		ProcessedTime: now,
	}, nil
}

func parseDecimal(raw json.RawMessage) (decimal.Decimal, error) {
	if len(raw) == 0 {
		return decimal.Zero, errors.New("empty price field")
	}
	s := strings.Trim(string(raw), `"`)
	if s == "" || s == "null" {
		return decimal.Zero, errors.New("null or empty price")
	}
	return decimal.NewFromString(s)
}

func parseTimestamp(raw json.RawMessage) (time.Time, error) {
	if len(raw) == 0 {
		return time.Time{}, errors.New("empty timestamp field")
	}
	s := strings.Trim(string(raw), `"`)
	if s == "" || s == "null" {
		return time.Time{}, errors.New("null timestamp")
	}

	// Try numeric timestamp (seconds, milliseconds, or microseconds)
	if num, err := strconv.ParseInt(s, 10, 64); err == nil {
		switch {
		case num > 1e15: // Microseconds
			return time.UnixMicro(num).UTC(), nil
		case num > 1e11: // Milliseconds
			return time.UnixMilli(num).UTC(), nil
		default: // Seconds
			return time.Unix(num, 0).UTC(), nil
		}
	}

	// Try floating point timestamp (e.g. seconds with fractional parts)
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		sec := int64(f)
		nsec := int64((f - float64(sec)) * 1e9)
		return time.Unix(sec, nsec).UTC(), nil
	}

	// Try standard ISO-8601 / RFC3339 formats
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized timestamp format: %s", s)
}
