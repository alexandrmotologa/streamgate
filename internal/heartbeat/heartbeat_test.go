package heartbeat

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type mockDispatcher struct {
	count uint64
}

func (m *mockDispatcher) BroadcastMetricsSnapshot() {
	atomic.AddUint64(&m.count, 1)
}

func TestHeartbeatService(t *testing.T) {
	mock := &mockDispatcher{}
	svc := NewService(mock, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go svc.Start(ctx)

	time.Sleep(50 * time.Millisecond)
	cancel()

	cnt := atomic.LoadUint64(&mock.count)
	if cnt < 2 {
		t.Fatalf("expected at least 2 snapshot dispatches, got %d", cnt)
	}
}
