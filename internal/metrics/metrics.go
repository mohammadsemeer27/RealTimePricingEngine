package metrics

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds the operational metrics for the Real-Time Pricing Engine.
type Metrics struct {
	mu sync.RWMutex

	incomingTicks      atomic.Uint64
	processedTicks     atomic.Uint64
	droppedEvents      atomic.Uint64
	upstreamReconnects atomic.Uint64
	authFailures       atomic.Uint64
	downstreamClients  atomic.Int64
	databaseErrors     atomic.Uint64
	databaseWrites     atomic.Uint64
	outboundMessages   atomic.Uint64

	// Dropped events breakdown by reason
	droppedByReason map[string]*atomic.Uint64

	// Processing latency tracking in microseconds
	latencyCount     atomic.Uint64
	latencySumMicros atomic.Uint64
	latencyBuckets   map[int64]*atomic.Uint64 // bucket ceiling in micros -> count

	// Payload sizes
	payloadCount    atomic.Uint64
	payloadSumBytes atomic.Uint64
	maxPayloadBytes atomic.Uint64
}

// NewMetrics initializes an operational metrics collector.
func NewMetrics() *Metrics {
	m := &Metrics{
		droppedByReason: make(map[string]*atomic.Uint64),
		latencyBuckets:  make(map[int64]*atomic.Uint64),
	}

	// Microsecond latency buckets: 100µs, 500µs, 1ms, 5ms, 10ms, 25ms, 50ms, 100ms
	buckets := []int64{100, 500, 1000, 5000, 10000, 25000, 50000, 100000}
	for _, b := range buckets {
		m.latencyBuckets[b] = &atomic.Uint64{}
	}

	return m
}

// IncIncomingTicks increments the count of ticks received from upstream.
func (m *Metrics) IncIncomingTicks() {
	m.incomingTicks.Add(1)
}

// IncProcessedTicks increments successfully processed and broadcast ticks.
func (m *Metrics) IncProcessedTicks() {
	m.processedTicks.Add(1)
}

// IncDroppedEvents increments dropped events with a specific reason.
func (m *Metrics) IncDroppedEvents(reason string) {
	m.droppedEvents.Add(1)

	m.mu.Lock()
	counter, exists := m.droppedByReason[reason]
	if !exists {
		counter = &atomic.Uint64{}
		m.droppedByReason[reason] = counter
	}
	m.mu.Unlock()

	counter.Add(1)
}

// IncUpstreamReconnects records an upstream reconnection attempt.
func (m *Metrics) IncUpstreamReconnects() {
	m.upstreamReconnects.Add(1)
}

// IncAuthFailures records an authentication failure.
func (m *Metrics) IncAuthFailures() {
	m.authFailures.Add(1)
}

// IncDownstreamClients increments active downstream client connections.
func (m *Metrics) IncDownstreamClients() {
	m.downstreamClients.Add(1)
}

// DecDownstreamClients decrements active downstream client connections.
func (m *Metrics) DecDownstreamClients() {
	m.downstreamClients.Add(-1)
}

// GetDownstreamClients returns current active downstream connections.
func (m *Metrics) GetDownstreamClients() int64 {
	return m.downstreamClients.Load()
}

// IncDatabaseErrors increments database write or read errors.
func (m *Metrics) IncDatabaseErrors() {
	m.databaseErrors.Add(1)
}

// IncDatabaseWrites increments successful persisted records count.
func (m *Metrics) IncDatabaseWrites(count uint64) {
	m.databaseWrites.Add(count)
}

// IncOutboundMessages records an outbound message pushed to downstream clients.
func (m *Metrics) IncOutboundMessages() {
	m.outboundMessages.Add(1)
}

// ObserveProcessingLatency records processing duration.
func (m *Metrics) ObserveProcessingLatency(d time.Duration) {
	micros := d.Microseconds()
	if micros < 0 {
		micros = 0
	}
	m.latencyCount.Add(1)
	m.latencySumMicros.Add(uint64(micros))

	m.mu.RLock()
	defer m.mu.RUnlock()
	for bucket, counter := range m.latencyBuckets {
		if micros <= bucket {
			counter.Add(1)
		}
	}
}

// ObservePayloadSize records outbound or inbound payload size.
func (m *Metrics) ObservePayloadSize(bytes int) {
	if bytes <= 0 {
		return
	}
	ubytes := uint64(bytes)
	m.payloadCount.Add(1)
	m.payloadSumBytes.Add(ubytes)

	for {
		currentMax := m.maxPayloadBytes.Load()
		if ubytes <= currentMax {
			break
		}
		if m.maxPayloadBytes.CompareAndSwap(currentMax, ubytes) {
			break
		}
	}
}

// Snapshot returns a copy of key metrics for API inspection.
type Snapshot struct {
	IncomingTicks      uint64            `json:"incoming_ticks"`
	ProcessedTicks     uint64            `json:"processed_ticks"`
	DroppedEvents      uint64            `json:"dropped_events"`
	DroppedByReason    map[string]uint64 `json:"dropped_by_reason"`
	UpstreamReconnects uint64            `json:"upstream_reconnects"`
	AuthFailures       uint64            `json:"auth_failures"`
	DownstreamClients  int64             `json:"downstream_clients"`
	DatabaseErrors     uint64            `json:"database_errors"`
	DatabaseWrites     uint64            `json:"database_writes"`
	OutboundMessages   uint64            `json:"outbound_messages"`
	AvgLatencyMicros   float64           `json:"avg_latency_micros"`
	AvgPayloadBytes    float64           `json:"avg_payload_bytes"`
	MaxPayloadBytes    uint64            `json:"max_payload_bytes"`
}

// GetSnapshot returns current metrics.
func (m *Metrics) GetSnapshot() Snapshot {
	m.mu.RLock()
	droppedCopy := make(map[string]uint64, len(m.droppedByReason))
	for k, v := range m.droppedByReason {
		droppedCopy[k] = v.Load()
	}
	m.mu.RUnlock()

	lCount := m.latencyCount.Load()
	var avgLatency float64
	if lCount > 0 {
		avgLatency = float64(m.latencySumMicros.Load()) / float64(lCount)
	}

	pCount := m.payloadCount.Load()
	var avgPayload float64
	if pCount > 0 {
		avgPayload = float64(m.payloadSumBytes.Load()) / float64(pCount)
	}

	return Snapshot{
		IncomingTicks:      m.incomingTicks.Load(),
		ProcessedTicks:     m.processedTicks.Load(),
		DroppedEvents:      m.droppedEvents.Load(),
		DroppedByReason:    droppedCopy,
		UpstreamReconnects: m.upstreamReconnects.Load(),
		AuthFailures:       m.authFailures.Load(),
		DownstreamClients:  m.downstreamClients.Load(),
		DatabaseErrors:     m.databaseErrors.Load(),
		DatabaseWrites:     m.databaseWrites.Load(),
		OutboundMessages:   m.outboundMessages.Load(),
		AvgLatencyMicros:   avgLatency,
		AvgPayloadBytes:    avgPayload,
		MaxPayloadBytes:    m.maxPayloadBytes.Load(),
	}
}

// ExportPrometheus formats metrics in standard Prometheus exposition format.
func (m *Metrics) ExportPrometheus() string {
	snap := m.GetSnapshot()
	var sb strings.Builder

	sb.WriteString("# HELP pricing_incoming_ticks_total Total number of ticks received from upstream\n")
	sb.WriteString("# TYPE pricing_incoming_ticks_total counter\n")
	sb.WriteString(fmt.Sprintf("pricing_incoming_ticks_total %d\n\n", snap.IncomingTicks))

	sb.WriteString("# HELP pricing_processed_ticks_total Total number of ticks processed and stored\n")
	sb.WriteString("# TYPE pricing_processed_ticks_total counter\n")
	sb.WriteString(fmt.Sprintf("pricing_processed_ticks_total %d\n\n", snap.ProcessedTicks))

	sb.WriteString("# HELP pricing_dropped_events_total Total dropped or rejected events\n")
	sb.WriteString("# TYPE pricing_dropped_events_total counter\n")
	sb.WriteString(fmt.Sprintf("pricing_dropped_events_total %d\n\n", snap.DroppedEvents))

	for reason, count := range snap.DroppedByReason {
		sb.WriteString(fmt.Sprintf("pricing_dropped_events_by_reason{reason=\"%s\"} %d\n", reason, count))
	}
	if len(snap.DroppedByReason) > 0 {
		sb.WriteString("\n")
	}

	sb.WriteString("# HELP pricing_upstream_reconnects_total Total reconnect attempts to upstream\n")
	sb.WriteString("# TYPE pricing_upstream_reconnects_total counter\n")
	sb.WriteString(fmt.Sprintf("pricing_upstream_reconnects_total %d\n\n", snap.UpstreamReconnects))

	sb.WriteString("# HELP pricing_auth_failures_total Total authentication failures with upstream\n")
	sb.WriteString("# TYPE pricing_auth_failures_total counter\n")
	sb.WriteString(fmt.Sprintf("pricing_auth_failures_total %d\n\n", snap.AuthFailures))

	sb.WriteString("# HELP pricing_downstream_clients Active connected downstream WebSocket clients\n")
	sb.WriteString("# TYPE pricing_downstream_clients gauge\n")
	sb.WriteString(fmt.Sprintf("pricing_downstream_clients %d\n\n", snap.DownstreamClients))

	sb.WriteString("# HELP pricing_database_errors_total Total errors encountered in persistence\n")
	sb.WriteString("# TYPE pricing_database_errors_total counter\n")
	sb.WriteString(fmt.Sprintf("pricing_database_errors_total %d\n\n", snap.DatabaseErrors))

	sb.WriteString("# HELP pricing_database_writes_total Total ticks persisted to PostgreSQL\n")
	sb.WriteString("# TYPE pricing_database_writes_total counter\n")
	sb.WriteString(fmt.Sprintf("pricing_database_writes_total %d\n\n", snap.DatabaseWrites))

	sb.WriteString("# HELP pricing_outbound_messages_total Total messages emitted to downstream clients\n")
	sb.WriteString("# TYPE pricing_outbound_messages_total counter\n")
	sb.WriteString(fmt.Sprintf("pricing_outbound_messages_total %d\n\n", snap.OutboundMessages))

	sb.WriteString("# HELP pricing_avg_latency_microseconds Average processing latency in microseconds\n")
	sb.WriteString("# TYPE pricing_avg_latency_microseconds gauge\n")
	sb.WriteString(fmt.Sprintf("pricing_avg_latency_microseconds %.2f\n\n", snap.AvgLatencyMicros))

	sb.WriteString("# HELP pricing_max_payload_bytes Maximum payload size observed in bytes\n")
	sb.WriteString("# TYPE pricing_max_payload_bytes gauge\n")
	sb.WriteString(fmt.Sprintf("pricing_max_payload_bytes %d\n", snap.MaxPayloadBytes))

	return sb.String()
}
