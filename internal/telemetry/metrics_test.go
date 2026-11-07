package telemetry

import (
	"testing"
)

func TestMetricsAndRuntimeStats(t *testing.T) {
	m := NewMetrics(nil)
	if m.ActiveConnections == nil {
		t.Fatalf("expected ActiveConnections metric")
	}

	m.ActiveConnections.Inc()
	m.ActiveConnections.Dec()

	stats := GetRuntimeStats()
	if stats.Goroutines <= 0 {
		t.Errorf("expected positive goroutine count, got %d", stats.Goroutines)
	}
	if stats.AllocBytes == 0 {
		t.Errorf("expected non-zero memory allocation")
	}
	if stats.AllocMB == "" {
		t.Errorf("expected AllocMB string representation")
	}
}
