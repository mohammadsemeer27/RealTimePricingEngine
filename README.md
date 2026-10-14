# Real-Time Pricing Engine

A high-performance, low-latency financial pricing engine written in **Go**, designed to ingest real-time market data from an upstream WebSocket feed, compute independent 24-hour rolling statistics (High, Low, Midpoint, and Percentage Change) in $O(1)$ amortized time, persist state into **PostgreSQL**, and distribute compact price updates to downstream clients via **WebSockets** and **REST APIs**.

---

## Architecture Overview

```
Upstream Feed (wss://v3-prod.livefxhub.com/ws/algo or Built-in Mock)
       │
       ▼
[Upstream Client] ──> [Parser & Validator] ──> [Rolling 24h Window (O(1) Deques)]
                                                           │
                      ┌────────────────────────────────────┴─────────────────────────────┐
                      ▼                                                                  ▼
             [WebSocket Hub]                                                    [PostgreSQL Store]
                    │                                                                    │
           [Downstream Clients]                                                 (Asynchronous Batched
           (/ws delta < 150 B)                                                  Copy & State Recovery)
```

---

## Key Features

- **Sub-Millisecond Processing**: $p50$ latency of **3.9 µs** and $p99$ of **30.6 µs**, well below the 50 ms assignment requirement.
- **Independent 24h Rolling Window**: Calculates true midpoint, 24h high, 24h low, and 24h percentage change from the earliest midpoint in the 24h window using monotonic deques ($O(1)$ amortized complexity).
- **PostgreSQL Persistence & Recovery**: Uses `pgxpool` with binary `CopyFrom` bulk inserts. Restores 24-hour historical observations on boot and automatically prunes expired ticks.
- **Payload Optimization**: Outbound deltas are compact JSON (~122 bytes) and snapshots (~318 bytes), well below the 10 KB threshold.
- **Resilient Upstream Streaming**: Exponential backoff with randomized jitter, single-flight reconnect guards, and distinct authentication failure handling.
- **Slow-Client Isolation**: Downstream clients use dedicated single-writer routines and bounded queues. Slow consumers are dropped without impacting upstream ingestion.
- **Observability**: Prometheus metrics export (`GET /metrics`), liveness (`/healthz`), readiness (`/readyz`), and snapshot endpoints.

---

## Prerequisites & Installation on Pop!_OS

The engine is developed and tested on **Pop!_OS 22.04 LTS**.

### 1. Install System Dependencies
```bash
sudo apt update
sudo apt install -y golang git docker.io docker-compose-plugin curl
```

Verify your Go version (requires Go 1.22+):
```bash
go version
```

### 2. Clone the Repository
```bash
git clone https://github.com/mohammadsemeer27/RealTimePricingEngine.git
cd RealTimePricingEngine
```

---

## Environment Configuration

Copy the example environment file:
```bash
cp .env.example .env
```

| Variable | Default Value | Description |
| :--- | :--- | :--- |
| `HTTP_PORT` | `8080` | Port for REST API and WebSocket server |
| `LOG_LEVEL` | `info` | Structured logging verbosity |
| `UPSTREAM_WS_URL` | `wss://v3-prod.livefxhub.com/ws/algo` | LiveFXHub WebSocket URL |
| `UPSTREAM_API_KEY` | `""` | Provider public API key |
| `UPSTREAM_SECRET_KEY`| `""` | Provider private secret key |
| `UPSTREAM_USE_MOCK` | `false` | When true or credentials empty, runs built-in mock feed |
| `ROLLING_WINDOW_HOURS`| `24` | Duration for rolling statistics |
| `DATABASE_URL` | `postgres://pricing_user:pricing_pass@localhost:5432/pricing_db?sslmode=disable` | PostgreSQL connection string |
| `DB_ENABLE_PERSISTENCE`| `true` | Enables PostgreSQL storage |
| `DB_BATCH_SIZE` | `100` | Asynchronous batch write threshold |
| `DB_FLUSH_INTERVAL_MS` | `200` | Flush interval for write batches |
| `WS_MAX_CONNECTIONS` | `1000` | Maximum downstream WebSocket connections |

> [!NOTE]
> When `UPSTREAM_API_KEY` is not provided, the application automatically launches a realistic built-in mock stream generating ticks for all 60 symbols.

---

## Running with Docker Compose

Start both PostgreSQL and the Pricing Engine with persistent storage and health checks:

```bash
docker compose up --build
```

To run in detached mode:
```bash
docker compose up -d --build
```

> [!NOTE]
> **Host Port Mappings:**
> - Pricing Engine API & WebSocket: `http://localhost:8082` (container port `8080`)
> - PostgreSQL: `localhost:5433` (container port `5432`)
> This avoids conflicts with existing services running on host ports 8080 and 5432.

Check logs:
```bash
docker compose logs -f pricing-engine
```

Stop services cleanly:
```bash
docker compose down
```

---

## Running Locally

If you have PostgreSQL running locally:
```bash
# 1. Start application
go run ./cmd/server
```

---

## REST API Examples

### 1. Liveness Check
```bash
curl -s http://localhost:8080/healthz | jq .
```
Response:
```json
{
  "status": "ok",
  "uptime_seconds": 42
}
```

### 2. Readiness Check
```bash
curl -s http://localhost:8080/readyz | jq .
```
Response:
```json
{
  "status": "ready"
}
```

### 3. Query All Market Snapshots
```bash
curl -s http://localhost:8080/prices | jq .
```

### 4. Query Filtered Market Snapshots
```bash
curl -s "http://localhost:8080/prices?symbols=EURUSD,BTCUSD" | jq .
```

### 5. Query Single Symbol
```bash
curl -s http://localhost:8080/prices/EURUSD | jq .
```
Response:
```json
{
  "symbol": "EURUSD",
  "buy": "1.0850",
  "sell": "1.0852",
  "mid_price": "1.0851",
  "high_24h": "1.0910",
  "low_24h": "1.0820",
  "change_24h_pct": "0.1500",
  "reference_price": "1.0835",
  "event_time": "2026-10-14T16:00:00Z",
  "processed_time": "2026-10-14T16:00:00.000003Z",
  "sequence": 105,
  "freshness_ms": 12,
  "is_stale": false
}
```

### 6. Prometheus Operational Metrics
```bash
curl -s http://localhost:8080/metrics
```

Or JSON metrics snapshot:
```bash
curl -s -H "Accept: application/json" http://localhost:8080/metrics | jq .
```

---

## WebSocket Client Usage

Connect to:
`ws://localhost:8080/ws`

### Subscribe to Symbols
Send:
```json
{
  "action": "subscribe",
  "symbols": ["EURUSD", "BTCUSD", "XAUUSD"]
}
```

Server replies with an acknowledgment and the full initial snapshot:
```json
{
  "type": "ack",
  "action": "subscribe",
  "status": "ok",
  "symbols": ["EURUSD", "BTCUSD", "XAUUSD"]
}
```

### Compact Delta Stream (<150 Bytes)
Subsequent price ticks are delivered as compact deltas:
```json
{
  "t": "d",
  "s": "EURUSD",
  "b": "1.0850",
  "a": "1.0852",
  "m": "1.0851",
  "h": "1.0910",
  "l": "1.0820",
  "c": "0.1500",
  "e": 1700000000000,
  "seq": 106
}
```

### Unsubscribe
```json
{
  "action": "unsubscribe",
  "symbols": ["XAUUSD"]
}
```

### Sequence Gap Recovery
If a client observes a gap in sequence numbers (`seq > last_seq + 1`), it can request a fresh snapshot:
```json
{
  "action": "snapshot",
  "symbols": ["EURUSD"]
}
```

---

## Running Tests & Benchmarks

Run standard tests:
```bash
go test -v ./...
```

Run race detector:
```bash
go test -race ./...
```

Run benchmarks with memory allocation metrics:
```bash
go test -bench=. -benchmem ./tests/...
```

Run linter / static analysis:
```bash
gofmt -w .
go vet ./...
```

---

## Troubleshooting

- **PostgreSQL Connection Refused**:
  Ensure PostgreSQL is running (`sudo systemctl status postgresql` or `docker compose ps`). Verify credentials in `DATABASE_URL`.
- **Port 8080 Already in Use**:
  Change `HTTP_PORT` in `.env` or set `HTTP_PORT=8081 go run ./cmd/server`.
- **Upstream Connection Error**:
  Verify network connectivity and proxy settings. Set `UPSTREAM_USE_MOCK=true` to test offline.
