package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/api"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/config"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/market"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/metrics"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/persistence"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/upstream"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/websocket"
)

func main() {
	// 1. Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting Real-Time Pricing Engine...")

	// 2. Load and validate configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}
	logger.Info("configuration loaded successfully", "config", cfg.RedactedString())

	// 3. Initialize metrics collector
	metricsCollector := metrics.NewMetrics()

	// 4. Initialize market engine
	clock := market.RealClock{}
	marketEngine := market.NewEngine(cfg.Market.RollingWindowDuration, clock, metricsCollector, upstream.DefaultSymbols)

	// 5. Connect to PostgreSQL and restore state
	var store persistence.Store
	var readinessChecker api.ReadinessChecker

	if cfg.Database.EnablePersistence {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		pgStore, err := persistence.NewPostgresStore(ctx, cfg.Database, metricsCollector, logger)
		cancel()

		if err != nil {
			logger.Warn("could not connect to PostgreSQL, falling back to in-memory store", "error", err)
			store = persistence.NewMemoryStore()
		} else {
			store = pgStore
			readinessChecker = pgStore

			// Apply schema migrations
			migCtx, migCancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := pgStore.AutoMigrate(migCtx); err != nil {
				logger.Error("failed to apply database migrations", "error", err)
			} else {
				logger.Info("database migrations applied successfully")
			}
			migCancel()

			// Restore persisted state within rolling 24-hour cutoff
			cutoff := time.Now().UTC().Add(-cfg.Market.RollingWindowDuration)
			resCtx, resCancel := context.WithTimeout(context.Background(), 30*time.Second)
			restoredCount, err := pgStore.RestoreRollingState(resCtx, cutoff, marketEngine)
			if err != nil {
				logger.Error("failed to restore historical observations", "error", err)
			} else {
				logger.Info("historical state restored", "ticks", restoredCount)
			}
			resCancel()
		}
	} else {
		logger.Info("database persistence disabled by configuration, using in-memory store")
		store = persistence.NewMemoryStore()
	}

	marketEngine.SetPersistence(store)

	// 6. Initialize downstream WebSocket hub
	wsHub := websocket.NewHub(cfg.WebSocket, marketEngine, metricsCollector, logger)
	marketEngine.SetBroadcaster(wsHub)

	// 7. Start upstream ingestion
	var mockServer *upstream.MockServer
	upstreamURL := cfg.Upstream.URL

	if cfg.Upstream.UseMock || cfg.Upstream.APIKey == "" {
		logger.Info("running with built-in mock upstream feed")
		mockServer = upstream.NewMockServer(upstream.WithMockTickInterval(100 * time.Millisecond))
		upstreamURL = mockServer.URL()
	}

	upstreamClient := upstream.NewClient(upstream.ClientConfig{
		URL:              upstreamURL,
		APIKey:           cfg.Upstream.APIKey,
		SecretKey:        cfg.Upstream.SecretKey,
		Symbols:          upstream.DefaultSymbols,
		ReconnectInitial: cfg.Upstream.ReconnectInitial,
		ReconnectMax:     cfg.Upstream.ReconnectMax,
		ReadTimeout:      cfg.Upstream.ReadTimeout,
		Metrics:          metricsCollector,
		Logger:           logger,
		Handler: func(ctx context.Context, raw market.RawTick) {
			_, _ = marketEngine.ProcessRawTick(ctx, raw)
		},
	})
	upstreamClient.Start()

	// 8. Start HTTP REST & WebSocket server
	apiServer := api.NewServer(marketEngine, metricsCollector, readinessChecker, wsHub)
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      apiServer.Routes(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("HTTP and WebSocket server listening", "port", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	// 9. Graceful shutdown handler
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)
	sig := <-shutdownCh
	logger.Info("received termination signal, initiating graceful shutdown...", "signal", sig.String())

	// Shutdown HTTP server (stops receiving new HTTP requests)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("error during HTTP server shutdown", "error", err)
	}

	// Close downstream WebSocket clients
	wsHub.Close()

	// Stop upstream client and mock
	_ = upstreamClient.Close()
	if mockServer != nil {
		mockServer.Close()
	}

	// Flush and close persistence store
	_ = store.Close()

	logger.Info("server shutdown completed successfully")
}
