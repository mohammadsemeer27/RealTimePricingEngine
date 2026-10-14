package persistence

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/config"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/market"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/metrics"
	"github.com/shopspring/decimal"
)

//go:embed schema.sql
var DefaultSchemaSQL string

// Store defines the interface for database persistence and recovery.
type Store interface {
	QueueTick(tick market.Tick)
	RestoreRollingState(ctx context.Context, cutoff time.Time, engine *market.Engine) (int, error)
	Ping(ctx context.Context) error
	Close() error
}

// PostgresStore implements Store using PostgreSQL with connection pooling and batched writes.
type PostgresStore struct {
	pool          *pgxpool.Pool
	metrics       *metrics.Metrics
	logger        *slog.Logger
	batchSize     int
	flushInterval time.Duration

	tickQueue chan market.Tick
	stopCh    chan struct{}
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewPostgresStore establishes a connection pool to PostgreSQL and launches the batch worker.
func NewPostgresStore(ctx context.Context, cfg config.DatabaseConfig, m *metrics.Metrics, logger *slog.Logger) (*PostgresStore, error) {
	if logger == nil {
		logger = slog.Default()
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid database URL: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.MaxIdleConns)
	poolConfig.MaxConnLifetime = cfg.ConnMaxLifetime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connectivity
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	subCtx, cancel := context.WithCancel(context.Background())

	store := &PostgresStore{
		pool:          pool,
		metrics:       m,
		logger:        logger,
		batchSize:     cfg.BatchSize,
		flushInterval: cfg.FlushInterval,
		tickQueue:     make(chan market.Tick, 10000),
		stopCh:        make(chan struct{}),
		ctx:           subCtx,
		cancel:        cancel,
	}

	store.wg.Add(1)
	go store.batchFlushWorker()

	return store, nil
}

// AutoMigrate applies the embedded schema migrations.
func (ps *PostgresStore) AutoMigrate(ctx context.Context) error {
	return ps.RunMigrations(ctx, DefaultSchemaSQL)
}

// RunMigrations applies embedded SQL schema migrations with transaction isolation.
func (ps *PostgresStore) RunMigrations(ctx context.Context, migrationSQL string) error {
	tx, err := ps.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin migration transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, migrationSQL); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	// Record version 1
	_, err = tx.Exec(ctx, `
		INSERT INTO schema_migrations (version, applied_at) 
		VALUES (1, NOW()) 
		ON CONFLICT (version) DO NOTHING;
	`)
	if err != nil {
		return fmt.Errorf("failed to record migration version: %w", err)
	}

	return tx.Commit(ctx)
}

// QueueTick enqueues a tick for asynchronous batched persistence.
func (ps *PostgresStore) QueueTick(tick market.Tick) {
	select {
	case ps.tickQueue <- tick:
	default:
		// Buffer is full under sudden spike; increment metric and drop to prevent blocking engine
		if ps.metrics != nil {
			ps.metrics.IncDroppedEvents("persistence_buffer_full")
			ps.metrics.IncDatabaseErrors()
		}
		ps.logger.Warn("persistence queue buffer full, dropping tick", "symbol", tick.Symbol)
	}
}

// Ping checks database health.
func (ps *PostgresStore) Ping(ctx context.Context) error {
	return ps.pool.Ping(ctx)
}

// RestoreRollingState loads all observations since cutoff and rebuilds the market engine's rolling windows.
func (ps *PostgresStore) RestoreRollingState(ctx context.Context, cutoff time.Time, engine *market.Engine) (int, error) {
	ps.logger.Info("restoring historical observations from PostgreSQL", "cutoff", cutoff)

	query := `
		SELECT symbol, buy, sell, mid_price, event_time 
		FROM market_ticks 
		WHERE event_time >= $1 
		ORDER BY symbol, event_time ASC;
	`

	rows, err := ps.pool.Query(ctx, query, cutoff)
	if err != nil {
		if ps.metrics != nil {
			ps.metrics.IncDatabaseErrors()
		}
		return 0, fmt.Errorf("failed to query historical ticks: %w", err)
	}
	defer rows.Close()

	grouped := make(map[string][]market.HistoricalObservation)
	totalCount := 0

	for rows.Next() {
		var (
			symbol    string
			buy       decimal.Decimal
			sell      decimal.Decimal
			mid       decimal.Decimal
			eventTime time.Time
		)

		if err := rows.Scan(&symbol, &buy, &sell, &mid, &eventTime); err != nil {
			if ps.metrics != nil {
				ps.metrics.IncDatabaseErrors()
			}
			return totalCount, fmt.Errorf("failed to scan row: %w", err)
		}

		grouped[symbol] = append(grouped[symbol], market.HistoricalObservation{
			Timestamp: eventTime,
			Buy:       buy,
			Sell:      sell,
			MidPrice:  mid,
		})
		totalCount++
	}

	for sym, obs := range grouped {
		engine.RestoreHistoricalObservations(sym, obs, cutoff)
	}

	ps.logger.Info("restored historical state successfully",
		"total_ticks", totalCount,
		"symbols", len(grouped))

	return totalCount, nil
}

// Close flushes remaining ticks, terminates the batch worker, and closes the connection pool.
func (ps *PostgresStore) Close() error {
	ps.cancel()
	close(ps.stopCh)
	ps.wg.Wait()

	ps.pool.Close()
	return nil
}

func (ps *PostgresStore) batchFlushWorker() {
	defer ps.wg.Done()

	ticker := time.NewTicker(ps.flushInterval)
	defer ticker.Stop()

	batch := make([]market.Tick, 0, ps.batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := ps.persistBatch(batch); err != nil {
			ps.logger.Error("failed to flush batch to postgres", "error", err, "count", len(batch))
			if ps.metrics != nil {
				ps.metrics.IncDatabaseErrors()
			}
		} else {
			if ps.metrics != nil {
				ps.metrics.IncDatabaseWrites(uint64(len(batch)))
			}
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-ps.stopCh:
			// Drain remaining ticks in the queue before terminating
			for {
				select {
				case tick := <-ps.tickQueue:
					batch = append(batch, tick)
					if len(batch) >= ps.batchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}

		case tick := <-ps.tickQueue:
			batch = append(batch, tick)
			if len(batch) >= ps.batchSize {
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}

func (ps *PostgresStore) persistBatch(ticks []market.Tick) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows := make([][]interface{}, len(ticks))
	for i, t := range ticks {
		rows[i] = []interface{}{
			t.Symbol,
			t.Buy,
			t.Sell,
			t.MidPrice,
			t.EventTime,
		}
	}

	// Use pgx CopyFrom for fastest bulk insertion
	_, err := ps.pool.CopyFrom(
		ctx,
		pgx.Identifier{"market_ticks"},
		[]string{"symbol", "buy", "sell", "mid_price", "event_time"},
		pgx.CopyFromRows(rows),
	)

	return err
}
