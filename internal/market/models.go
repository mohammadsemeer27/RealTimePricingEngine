package market

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

// RawTick represents the raw payload structure received from upstream WebSocket.
type RawTick struct {
	Symbol    string          `json:"symbol"`
	Buy       json.RawMessage `json:"buy"`
	Sell      json.RawMessage `json:"sell"`
	Low24h    json.RawMessage `json:"low_24h,omitempty"`    // Ignored as per assignment specs
	High24h   json.RawMessage `json:"high_24h,omitempty"`   // Ignored as per assignment specs
	Change24h json.RawMessage `json:"change_24h,omitempty"` // Ignored as per assignment specs
	Timestamp json.RawMessage `json:"timestamp"`
}

// Tick represents a validated, parsed market observation.
type Tick struct {
	Symbol        string          `json:"symbol"`
	Buy           decimal.Decimal `json:"buy"`
	Sell          decimal.Decimal `json:"sell"`
	MidPrice      decimal.Decimal `json:"mid_price"`
	EventTime     time.Time       `json:"event_time"`
	ProcessedTime time.Time       `json:"processed_time"`
}

// MarketSnapshot represents the full computed market state for a symbol.
type MarketSnapshot struct {
	Symbol         string          `json:"symbol"`
	Buy            decimal.Decimal `json:"buy"`
	Sell           decimal.Decimal `json:"sell"`
	MidPrice       decimal.Decimal `json:"mid_price"`
	High24h        decimal.Decimal `json:"high_24h"`
	Low24h         decimal.Decimal `json:"low_24h"`
	Change24hPct   decimal.Decimal `json:"change_24h_pct"`
	ReferencePrice decimal.Decimal `json:"reference_price"`
	EventTime      time.Time       `json:"event_time"`
	ProcessedTime  time.Time       `json:"processed_time"`
	Sequence       uint64          `json:"sequence"`
	FreshnessMs    int64           `json:"freshness_ms"`
	IsStale        bool            `json:"is_stale"`
}

// MarketDelta represents an optimized incremental price update.
type MarketDelta struct {
	Type          string          `json:"t"` // "d" for delta, "s" for snapshot
	Symbol        string          `json:"s"`
	Buy           decimal.Decimal `json:"b"`
	Sell          decimal.Decimal `json:"a"` // ask / sell
	MidPrice      decimal.Decimal `json:"m"`
	High24h       decimal.Decimal `json:"h"`
	Low24h        decimal.Decimal `json:"l"`
	Change24hPct  decimal.Decimal `json:"c"`
	EventTimeUnix int64           `json:"e"` // unix milliseconds
	Sequence      uint64          `json:"seq"`
}

// ToDelta converts a snapshot to a compact delta update.
func (s *MarketSnapshot) ToDelta() MarketDelta {
	return MarketDelta{
		Type:          "d",
		Symbol:        s.Symbol,
		Buy:           s.Buy,
		Sell:          s.Sell,
		MidPrice:      s.MidPrice,
		High24h:       s.High24h,
		Low24h:        s.Low24h,
		Change24hPct:  s.Change24hPct,
		EventTimeUnix: s.EventTime.UnixMilli(),
		Sequence:      s.Sequence,
	}
}
