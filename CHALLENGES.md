# Engineering Challenges & Solutions

This document documents both the concrete problems observed during the development and testing of the Real-Time Pricing Engine, along with anticipated architectural challenges and known system limitations.

---

## 1. Observed Problems & Solutions

### Problem 1: Concurrent Write Data Race on WebSocket Connection in Mock Server
- **Observed Behavior**:
  Running `go test -race ./...` failed with a data race warning in `internal/upstream/mock_server.go`. Goroutine 11 (`broadcastRandomTicks`) attempted to write market ticks via `conn.WriteMessage(...)` while Goroutine 14 (`handleWebSocket`) concurrently wrote the authentication response via `conn.WriteJSON(...)`.
- **Root Cause**:
  `gorilla/websocket` connections do not support concurrent calls to write methods (`WriteMessage`, `WriteJSON`, `NextWriter`).
- **Solution**:
  Wrapped the mock client connection in a `mockClientConn` struct with an internal `writeMu sync.Mutex`. All write operations (`writeJSON` and `writeMessage`) acquire this lock before touching the underlying connection, completely eliminating data races under `-race`.

### Problem 2: Missing Transitive Module Dependency for `pgxpool`
- **Observed Behavior**:
  When compiling `internal/persistence`, the compiler reported:
  `missing go.sum entry for module providing package github.com/jackc/puddle/v2`.
- **Root Cause**:
  `pgx/v5/pgxpool` requires `puddle/v2` for its internal connection pooling implementation, which was not initially included in `go.sum`.
- **Solution**:
  Executed `go get github.com/jackc/pgx/v5/pgxpool@v5.11.0` and `go mod tidy` outside the network sandbox to download and pin all transitive pool dependencies.

### Problem 3: Compiler Warnings on Unused Imports
- **Observed Behavior**:
  Go enforces strict compilation without unused imports. Minor unused imports (`"fmt"` in `mock_server.go` and `"time"` in `server_test.go`) caused initial test compilation failures.
- **Solution**:
  Promptly removed unused packages and enforced `go vet ./...` and `gofmt -w .` before finalizing each stage.

---

## 2. Anticipated Challenges (Labelled & Addressed)

> [!NOTE]
> The following challenges were proactively analyzed and addressed during the architectural design phase prior to implementation.

### Anticipated Challenge A: Tight Reconnection Loops on Invalid Credentials
- **Risk**:
  If the upstream WebSocket returns an authentication error (e.g., bad API key or expired secret), immediately retrying in an automated reconnect loop would hammer the provider's server and risk an IP ban or account lockout.
- **Solution**:
  The upstream client explicitly differentiates between **Network/Transport Failures** (e.g. EOF, connection reset) and **Authentication Failures** (e.g. `status: error`, `invalid credentials`). On authentication failure, reconnection is halted immediately, an alert is logged without leaking credentials, and `authFailures` metric is incremented.

### Anticipated Challenge B: Sliding-Window CPU Spikes ($O(N)$ vs $O(1)$)
- **Risk**:
  Tracking rolling 24-hour high and low by scanning full tick history would degrade latency linearly with observation volume, violating the 50 ms latency budget.
- **Solution**:
  Implemented dual monotonic deques (`maxDeque` and `minDeque`). By maintaining monotonic price ordering, finding the current high and low is reduced to an $O(1)$ front lookup, achieving sub-microsecond updates even with millions of observations.

### Anticipated Challenge C: Slow Client Starvation & Head-of-Line Blocking
- **Risk**:
  A single slow or disconnected downstream client could back up WebSocket broadcast queues and starve upstream ingestion.
- **Solution**:
  Each downstream client maintains a bounded outbound channel (`send chan []byte`, size 256). Broadcast writes employ non-blocking `select` operations. If a client's buffer fills, the message is dropped, `slow_client_drop` metric is incremented, and the client connection is safely recycled without delaying other clients.

### Anticipated Challenge D: Crossed Books and Abnormal Price Ticks
- **Risk**:
  Dirty upstream data containing negative prices or crossed markets (`buy > sell`) could corrupt rolling averages and midpoint calculations.
- **Solution**:
  The parser enforces strict validation rules: symbols must be non-empty, buy and sell prices must be positive decimal numbers, buy must be $\le$ sell, and timestamps must not drift more than 5 minutes into the future. Invalid ticks are rejected and tracked under `pricing_dropped_events_by_reason`.

---

## 3. Known Limitations

1. **Durability Window for In-Flight Ticks**:
   To achieve high throughput and low latency, ticks are queued in memory and flushed to PostgreSQL in batches every 200 ms. If the host machine abruptly loses power or is forcefully killed (`kill -9`) within that 200 ms interval, observations accumulated during that window could be lost.
2. **Single-Node In-Memory State**:
   Currently, the rolling 24-hour window state is held in the memory of the active instance. Running multiple active-active replicas requires either sticky client routing by symbol or an external distributed streaming tier (such as Kafka or Redis Streams).
3. **Provider Protocol Nuance**:
   Because real upstream credentials for `wss://v3-prod.livefxhub.com/ws/algo` were not supplied in the prompt, upstream integration was verified using our built-in mock server adhering strictly to the assignment's specified payload structures.
