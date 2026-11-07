package telemetry

import (
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds the registered Prometheus metric collectors.
type Metrics struct {
	Registry              *prometheus.Registry
	ActiveConnections     prometheus.Gauge
	MessagesPublished     *prometheus.CounterVec
	MessagesDelivered     *prometheus.CounterVec
	MessagesDropped       *prometheus.CounterVec
	FanOutDurationSeconds prometheus.Histogram
	PublishDuration       prometheus.Histogram
}

// RuntimeStats captures current Go runtime memory and goroutine stats.
type RuntimeStats struct {
	Goroutines   int    `json:"goroutines"`
	AllocBytes   uint64 `json:"alloc_bytes"`
	TotalAlloc   uint64 `json:"total_alloc_bytes"`
	SysBytes     uint64 `json:"sys_bytes"`
	NumGC        uint32 `json:"num_gc"`
	HeapObjects  uint64 `json:"heap_objects"`
	AllocMB      string `json:"alloc_mb"`
	TimestampSec int64  `json:"timestamp"`
}

// NewMetrics initializes and registers Prometheus metrics with a dedicated registry.
func NewMetrics(reg *prometheus.Registry) *Metrics {
	if reg == nil {
		reg = prometheus.NewRegistry()
	}
	factory := promauto.With(reg)

	return &Metrics{
		Registry: reg,
		ActiveConnections: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: "streamgate",
			Name:      "active_connections",
			Help:      "Current number of active Server-Sent Events subscriber connections.",
		}),
		MessagesPublished: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: "streamgate",
			Name:      "messages_published_total",
			Help:      "Total number of messages published through the gateway.",
		}, []string{"channel"}),
		MessagesDelivered: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: "streamgate",
			Name:      "messages_delivered_total",
			Help:      "Total number of messages successfully delivered to client streams.",
		}, []string{"channel"}),
		MessagesDropped: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: "streamgate",
			Name:      "messages_dropped_total",
			Help:      "Total number of messages dropped due to subscriber buffer backpressure.",
		}, []string{"policy"}),
		FanOutDurationSeconds: factory.NewHistogram(prometheus.HistogramOpts{
			Namespace: "streamgate",
			Name:      "fanout_duration_seconds",
			Help:      "Time taken to fan out an event across all channel subscribers.",
			Buckets:   []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1},
		}),
		PublishDuration: factory.NewHistogram(prometheus.HistogramOpts{
			Namespace: "streamgate",
			Name:      "publish_request_duration_seconds",
			Help:      "Duration of HTTP publish ingestion requests.",
			Buckets:   []float64{0.0005, 0.001, 0.002, 0.005, 0.01, 0.05, 0.1},
		}),
	}
}

// GetRuntimeStats samples the Go runtime memory and goroutine counters.
func GetRuntimeStats() RuntimeStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	allocMB := float64(m.Alloc) / (1024 * 1024)

	return RuntimeStats{
		Goroutines:   runtime.NumGoroutine(),
		AllocBytes:   m.Alloc,
		TotalAlloc:   m.TotalAlloc,
		SysBytes:     m.Sys,
		NumGC:        m.NumGC,
		HeapObjects:  m.HeapObjects,
		AllocMB:      formatFloat(allocMB, 2),
		TimestampSec: time.Now().Unix(),
	}
}

func formatFloat(v float64, decimals int) string {
	intPart := int(v)
	fracPart := int((v - float64(intPart)) * 100)
	if fracPart < 0 {
		fracPart = 0
	}
	return formatTwoDigits(intPart, fracPart)
}

func formatTwoDigits(i, f int) string {
	b := make([]byte, 0, 16)
	b = appendInt(b, i)
	b = append(b, '.')
	if f < 10 {
		b = append(b, '0')
	}
	b = appendInt(b, f)
	return string(b)
}

func appendInt(b []byte, n int) []byte {
	if n == 0 {
		return append(b, '0')
	}
	var digits [20]byte
	idx := len(digits)
	for n > 0 {
		idx--
		digits[idx] = byte('0' + (n % 10))
		n /= 10
	}
	return append(b, digits[idx:]...)
}
