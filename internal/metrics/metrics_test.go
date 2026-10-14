package metrics

import (
	"strings"
	"testing"
	"time"
)

func TestMetricsOperations(t *testing.T) {
	m := NewMetrics()

	m.IncIncomingTicks()
	m.IncProcessedTicks()
	m.IncDroppedEvents("invalid_price")
	m.IncUpstreamReconnects()
	m.IncAuthFailures()
	m.IncDownstreamClients()
	m.IncDownstreamClients()
	m.DecDownstreamClients()
	m.IncDatabaseErrors()
	m.IncDatabaseWrites(5)
	m.IncOutboundMessages()
	m.ObserveProcessingLatency(250 * time.Microsecond)
	m.ObservePayloadSize(1024)

	snap := m.GetSnapshot()

	if snap.IncomingTicks != 1 {
		t.Errorf("expected 1 incoming tick, got %d", snap.IncomingTicks)
	}
	if snap.ProcessedTicks != 1 {
		t.Errorf("expected 1 processed tick, got %d", snap.ProcessedTicks)
	}
	if snap.DroppedEvents != 1 {
		t.Errorf("expected 1 dropped event, got %d", snap.DroppedEvents)
	}
	if snap.DroppedByReason["invalid_price"] != 1 {
		t.Errorf("expected 1 invalid_price drop, got %d", snap.DroppedByReason["invalid_price"])
	}
	if snap.DownstreamClients != 1 {
		t.Errorf("expected 1 downstream client, got %d", snap.DownstreamClients)
	}
	if snap.DatabaseWrites != 5 {
		t.Errorf("expected 5 db writes, got %d", snap.DatabaseWrites)
	}
	if snap.MaxPayloadBytes != 1024 {
		t.Errorf("expected max payload 1024, got %d", snap.MaxPayloadBytes)
	}

	prom := m.ExportPrometheus()
	if !strings.Contains(prom, "pricing_incoming_ticks_total 1") {
		t.Errorf("prometheus export missing incoming ticks counter")
	}
	if !strings.Contains(prom, "pricing_downstream_clients 1") {
		t.Errorf("prometheus export missing downstream clients gauge")
	}
}
