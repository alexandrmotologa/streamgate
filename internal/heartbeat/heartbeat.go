package heartbeat

import (
	"context"
	"time"
)

// Frame is the static SSE comment byte sequence sent as a keep-alive signal.
var Frame = []byte(":keepalive\n\n")

// SnapshotDispatcher defines the interface for periodic telemetry broadcast.
type SnapshotDispatcher interface {
	BroadcastMetricsSnapshot()
}

// Service runs background heartbeat and telemetry intervals.
type Service struct {
	dispatcher        SnapshotDispatcher
	telemetryInterval time.Duration
}

// NewService creates a new heartbeat and telemetry scheduler.
func NewService(dispatcher SnapshotDispatcher, telemetryInterval time.Duration) *Service {
	if telemetryInterval <= 0 {
		telemetryInterval = time.Second
	}
	return &Service{
		dispatcher:        dispatcher,
		telemetryInterval: telemetryInterval,
	}
}

// Start begins the periodic telemetry broadcast loop until context cancellation.
func (s *Service) Start(ctx context.Context) {
	ticker := time.NewTicker(s.telemetryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.dispatcher != nil {
				s.dispatcher.BroadcastMetricsSnapshot()
			}
		}
	}
}
