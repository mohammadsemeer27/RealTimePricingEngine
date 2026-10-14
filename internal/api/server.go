package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/market"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/metrics"
)

// MarketSnapshotProvider defines engine snapshot retrieval methods.
type MarketSnapshotProvider interface {
	GetSnapshot(symbol string) (market.MarketSnapshot, bool)
	GetAllSnapshots(filterSymbols []string) map[string]market.MarketSnapshot
}

// ReadinessChecker checks if critical dependencies (e.g. database) are healthy.
type ReadinessChecker interface {
	Ping(ctx context.Context) error
}

// Server provides REST API and WebSocket endpoints.
type Server struct {
	engine    MarketSnapshotProvider
	metrics   *metrics.Metrics
	readiness ReadinessChecker
	wsHandler http.Handler
	startTime time.Time
}

// NewServer initializes the API server handlers.
func NewServer(engine MarketSnapshotProvider, m *metrics.Metrics, readiness ReadinessChecker, wsHandler http.Handler) *Server {
	return &Server{
		engine:    engine,
		metrics:   m,
		readiness: readiness,
		wsHandler: wsHandler,
		startTime: time.Now(),
	}
}

// Routes configures and returns the HTTP mux.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("GET /prices", s.handlePrices)
	mux.HandleFunc("GET /prices/{symbol}", s.handlePriceSymbol)
	mux.HandleFunc("GET /metrics", s.handleMetrics)

	if s.wsHandler != nil {
		mux.Handle("/ws", s.wsHandler)
	}

	return s.loggingMiddleware(mux)
}

// handleHealthz reports whether the application process is running.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":         "ok",
		"uptime_seconds": int64(time.Since(s.startTime).Seconds()),
	})
}

// handleReadyz verifies whether dependencies are ready to serve traffic.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if s.readiness != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := s.readiness.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
				"status": "not_ready",
				"error":  "database dependency unavailable: " + err.Error(),
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ready",
	})
}

// handlePrices returns market snapshots, optionally filtered by ?symbols=EURUSD,BTCUSD.
func (s *Server) handlePrices(w http.ResponseWriter, r *http.Request) {
	var symbols []string
	querySymbols := r.URL.Query().Get("symbols")
	if querySymbols != "" {
		for _, sym := range strings.Split(querySymbols, ",") {
			trimmed := strings.ToUpper(strings.TrimSpace(sym))
			if trimmed != "" {
				symbols = append(symbols, trimmed)
			}
		}
	}

	snapshots := s.engine.GetAllSnapshots(symbols)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"count": len(snapshots),
		"data":  snapshots,
	})
}

// handlePriceSymbol returns current snapshot for a single symbol.
func (s *Server) handlePriceSymbol(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(strings.TrimSpace(r.PathValue("symbol")))
	if symbol == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "symbol parameter is required"})
		return
	}

	snapshot, exists := s.engine.GetSnapshot(symbol)
	if !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "symbol not found: " + symbol})
		return
	}

	writeJSON(w, http.StatusOK, snapshot)
}

// handleMetrics exports operational metrics.
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		writeJSON(w, http.StatusOK, s.metrics.GetSnapshot())
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(s.metrics.ExportPrometheus()))
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Enforce safety headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
