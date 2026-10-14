package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/market"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/metrics"
	"github.com/shopspring/decimal"
)

type mockSnapshotProvider struct {
	snapshots map[string]market.MarketSnapshot
}

func (m *mockSnapshotProvider) GetSnapshot(symbol string) (market.MarketSnapshot, bool) {
	snap, ok := m.snapshots[symbol]
	return snap, ok
}

func (m *mockSnapshotProvider) GetAllSnapshots(filter []string) map[string]market.MarketSnapshot {
	if len(filter) == 0 {
		return m.snapshots
	}
	res := make(map[string]market.MarketSnapshot)
	for _, f := range filter {
		if snap, ok := m.snapshots[f]; ok {
			res[f] = snap
		}
	}
	return res
}

type mockReadiness struct {
	err error
}

func (m *mockReadiness) Ping(ctx context.Context) error {
	return m.err
}

func TestAPIEndpoints(t *testing.T) {
	m := metrics.NewMetrics()
	engine := &mockSnapshotProvider{
		snapshots: map[string]market.MarketSnapshot{
			"EURUSD": {
				Symbol:   "EURUSD",
				MidPrice: decimal.NewFromFloat(1.0850),
			},
			"BTCUSD": {
				Symbol:   "BTCUSD",
				MidPrice: decimal.NewFromInt(65000),
			},
		},
	}

	readiness := &mockReadiness{}
	server := NewServer(engine, m, readiness, nil)
	handler := server.Routes()

	// 1. Test /healthz
	reqHealth := httptest.NewRequest("GET", "/healthz", nil)
	recHealth := httptest.NewRecorder()
	handler.ServeHTTP(recHealth, reqHealth)
	if recHealth.Code != http.StatusOK {
		t.Errorf("healthz expected 200, got %d", recHealth.Code)
	}

	// 2. Test /readyz (healthy)
	reqReady := httptest.NewRequest("GET", "/readyz", nil)
	recReady := httptest.NewRecorder()
	handler.ServeHTTP(recReady, reqReady)
	if recReady.Code != http.StatusOK {
		t.Errorf("readyz expected 200, got %d", recReady.Code)
	}

	// 3. Test /readyz (unhealthy)
	readiness.err = errors.New("connection refused")
	recReadyFail := httptest.NewRecorder()
	handler.ServeHTTP(recReadyFail, reqReady)
	if recReadyFail.Code != http.StatusServiceUnavailable {
		t.Errorf("readyz expected 503, got %d", recReadyFail.Code)
	}
	readiness.err = nil // reset

	// 4. Test /prices (all)
	reqPrices := httptest.NewRequest("GET", "/prices", nil)
	recPrices := httptest.NewRecorder()
	handler.ServeHTTP(recPrices, reqPrices)
	if recPrices.Code != http.StatusOK {
		t.Errorf("prices expected 200, got %d", recPrices.Code)
	}
	var pricesResp struct {
		Count int                              `json:"count"`
		Data  map[string]market.MarketSnapshot `json:"data"`
	}
	if err := json.Unmarshal(recPrices.Body.Bytes(), &pricesResp); err != nil {
		t.Fatalf("failed unmarshaling prices response: %v", err)
	}
	if pricesResp.Count != 2 {
		t.Errorf("expected 2 symbols, got %d", pricesResp.Count)
	}

	// 5. Test /prices with filter: ?symbols=EURUSD
	reqPricesFiltered := httptest.NewRequest("GET", "/prices?symbols=EURUSD", nil)
	recPricesFiltered := httptest.NewRecorder()
	handler.ServeHTTP(recPricesFiltered, reqPricesFiltered)
	var filteredResp struct {
		Count int                              `json:"count"`
		Data  map[string]market.MarketSnapshot `json:"data"`
	}
	_ = json.Unmarshal(recPricesFiltered.Body.Bytes(), &filteredResp)
	if filteredResp.Count != 1 || filteredResp.Data["EURUSD"].Symbol != "EURUSD" {
		t.Errorf("expected 1 filtered symbol EURUSD, got count %d", filteredResp.Count)
	}

	// 6. Test /prices/{symbol} (found)
	reqSingle := httptest.NewRequest("GET", "/prices/BTCUSD", nil)
	recSingle := httptest.NewRecorder()
	handler.ServeHTTP(recSingle, reqSingle)
	if recSingle.Code != http.StatusOK {
		t.Errorf("expected 200 for BTCUSD, got %d", recSingle.Code)
	}

	// 7. Test /prices/{symbol} (not found)
	reqNotFound := httptest.NewRequest("GET", "/prices/UNKNOWN", nil)
	recNotFound := httptest.NewRecorder()
	handler.ServeHTTP(recNotFound, reqNotFound)
	if recNotFound.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown symbol, got %d", recNotFound.Code)
	}

	// 8. Test /metrics (Prometheus format)
	m.IncIncomingTicks()
	reqMetrics := httptest.NewRequest("GET", "/metrics", nil)
	recMetrics := httptest.NewRecorder()
	handler.ServeHTTP(recMetrics, reqMetrics)
	if recMetrics.Code != http.StatusOK {
		t.Errorf("expected 200 for metrics, got %d", recMetrics.Code)
	}
	if !strings.Contains(recMetrics.Body.String(), "pricing_incoming_ticks_total 1") {
		t.Errorf("metrics missing pricing_incoming_ticks_total")
	}

	// 9. Test /metrics (JSON format)
	reqMetricsJSON := httptest.NewRequest("GET", "/metrics", nil)
	reqMetricsJSON.Header.Set("Accept", "application/json")
	recMetricsJSON := httptest.NewRecorder()
	handler.ServeHTTP(recMetricsJSON, reqMetricsJSON)
	var metricsSnap metrics.Snapshot
	if err := json.Unmarshal(recMetricsJSON.Body.Bytes(), &metricsSnap); err != nil {
		t.Fatalf("failed to unmarshal JSON metrics: %v", err)
	}
	if metricsSnap.IncomingTicks != 1 {
		t.Errorf("expected 1 incoming tick in JSON metrics, got %d", metricsSnap.IncomingTicks)
	}
}
