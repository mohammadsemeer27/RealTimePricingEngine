CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS market_ticks (
    id BIGSERIAL PRIMARY KEY,
    symbol VARCHAR(16) NOT NULL,
    buy NUMERIC(20, 8) NOT NULL,
    sell NUMERIC(20, 8) NOT NULL,
    mid_price NUMERIC(20, 8) NOT NULL,
    event_time TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_market_ticks_symbol_event_time 
    ON market_ticks (symbol, event_time ASC);

CREATE INDEX IF NOT EXISTS idx_market_ticks_event_time 
    ON market_ticks (event_time);
