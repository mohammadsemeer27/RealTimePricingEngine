package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/market"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/metrics"
	"github.com/shopspring/decimal"
)

// BenchmarkEngineProcessTick benchmarks parsing, validating, and updating the rolling window.
func BenchmarkEngineProcessTick(b *testing.B) {
	m := metrics.NewMetrics()
	clock := market.RealClock{}
	engine := market.NewEngine(24*time.Hour, clock, m, upstreamSymbols())

	rawTicks := make([]market.RawTick, 1000)
	baseTime := time.Now().UTC()
	for i := 0; i < 1000; i++ {
		buyStr := fmt.Sprintf("%.5f", 1.0850+float64(i)*0.00001)
		sellStr := fmt.Sprintf("%.5f", 1.0852+float64(i)*0.00001)
		tMillis := baseTime.Add(time.Duration(i) * time.Millisecond).UnixMilli()

		rawTicks[i] = market.RawTick{
			Symbol:    "EURUSD",
			Buy:       json.RawMessage(`"` + buyStr + `"`),
			Sell:      json.RawMessage(`"` + sellStr + `"`),
			Timestamp: json.RawMessage(fmt.Sprintf("%d", tMillis)),
		}
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		raw := rawTicks[i%1000]
		// Slightly offset timestamp to maintain strictly increasing time
		raw.Timestamp = json.RawMessage(fmt.Sprintf("%d", baseTime.UnixMilli()+int64(i)))
		_, _ = engine.ProcessRawTick(ctx, raw)
	}
}

// BenchmarkRollingWindowApply benchmarks the O(1) monotonic deque updates.
func BenchmarkRollingWindowApply(b *testing.B) {
	rw := market.NewRollingWindow("EURUSD", 24*time.Hour)
	clock := market.RealClock{}
	baseTime := time.Now().UTC()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		t := baseTime.Add(time.Duration(i) * time.Millisecond)
		tick := market.Tick{
			Symbol:        "EURUSD",
			Buy:           decimal.NewFromFloat(1.0850),
			Sell:          decimal.NewFromFloat(1.0852),
			MidPrice:      decimal.NewFromFloat(1.0851),
			EventTime:     t,
			ProcessedTime: t,
		}
		_, _ = rw.ApplyTick(tick, clock)
	}
}

// TestMeasureLatencyPercentiles executes 5,000 realistic ticks and calculates actual p50, p95, p99 latency.
func TestMeasureLatencyPercentiles(t *testing.T) {
	m := metrics.NewMetrics()
	clock := market.RealClock{}
	engine := market.NewEngine(24*time.Hour, clock, m, upstreamSymbols())

	numTicks := 5000
	latencies := make([]time.Duration, numTicks)
	baseTime := time.Now().UTC()
	ctx := context.Background()

	for i := 0; i < numTicks; i++ {
		raw := market.RawTick{
			Symbol:    "EURUSD",
			Buy:       json.RawMessage(`"1.0850"`),
			Sell:      json.RawMessage(`"1.0852"`),
			Timestamp: json.RawMessage(fmt.Sprintf("%d", baseTime.UnixMilli()+int64(i))),
		}

		start := time.Now()
		_, err := engine.ProcessRawTick(ctx, raw)
		latencies[i] = time.Since(start)

		if err != nil {
			t.Fatalf("unexpected error at tick %d: %v", i, err)
		}
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	p50 := latencies[int(float64(numTicks)*0.50)]
	p95 := latencies[int(float64(numTicks)*0.95)]
	p99 := latencies[int(float64(numTicks)*0.99)]

	t.Logf("--- LATENCY BENCHMARK RESULTS (5,000 ticks) ---")
	t.Logf("p50 latency: %v (%.3f µs)", p50, float64(p50.Nanoseconds())/1000.0)
	t.Logf("p95 latency: %v (%.3f µs)", p95, float64(p95.Nanoseconds())/1000.0)
	t.Logf("p99 latency: %v (%.3f µs)", p99, float64(p99.Nanoseconds())/1000.0)

	// Target requirement: end-to-end latency <= 50 milliseconds
	if p99 > 50*time.Millisecond {
		t.Errorf("p99 latency %v exceeded 50ms requirement", p99)
	}
}

func upstreamSymbols() []string {
	return []string{
		"AUDUSD", "AUDCAD", "AUDCHF", "AUDJPY", "AUDNZD",
		"AUDSGD", "CADCHF", "CADJPY", "CHFJPY", "CHFSGD",
		"EURAUD", "EURCAD", "EURCHF", "EURGBP", "EURJPY",
		"EURNZD", "EURUSD", "GBPAUD", "GBPCAD", "GBPCHF",
		"GBPJPY", "GBPNZD", "GBPUSD", "NZDCAD", "NZDCHF",
		"NZDJPY", "NZDUSD", "USDCAD", "USDCHF", "USDCNH",
		"USDJPY", "USDMXN", "USDNOK", "USDPLN", "USDSEK",
		"USDSGD", "USDTRY", "USDZAR", "EURTRY", "GBPTRY",
		"NOKJPY", "SEKJPY", "SGDJPY", "ZARJPY", "XAUUSD",
		"XAGUSD", "BTCUSD", "ETHUSD", "BNBUSD", "SOLUSD",
		"XRPUSD", "ADAUSD", "DOGUSD", "DOTUSD", "LTCUSD",
		"BCHUSD", "XLMUSD", "TRXUSD", "UNIUSD", "FILUSD",
		"AVXUSD",
	}
}
