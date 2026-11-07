package hub

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestRingBuffer_AddAndSize(t *testing.T) {
	rb := NewRingBuffer(5)
	if rb.Capacity() != 5 {
		t.Fatalf("expected capacity 5, got %d", rb.Capacity())
	}
	if rb.Size() != 0 {
		t.Fatalf("expected initial size 0, got %d", rb.Size())
	}

	for i := 1; i <= 3; i++ {
		rb.Add(&Event{ID: fmt.Sprintf("ev-%d", i), Seq: uint64(i)})
	}
	if rb.Size() != 3 {
		t.Fatalf("expected size 3, got %d", rb.Size())
	}

	// Add more to trigger wrap-around
	for i := 4; i <= 8; i++ {
		rb.Add(&Event{ID: fmt.Sprintf("ev-%d", i), Seq: uint64(i)})
	}
	if rb.Size() != 5 {
		t.Fatalf("expected capped size 5, got %d", rb.Size())
	}

	latest := rb.GetLatest(3)
	if len(latest) != 3 {
		t.Fatalf("expected 3 latest events, got %d", len(latest))
	}
	if latest[0].ID != "ev-6" || latest[1].ID != "ev-7" || latest[2].ID != "ev-8" {
		t.Fatalf("unexpected latest events: %v, %v, %v", latest[0].ID, latest[1].ID, latest[2].ID)
	}
}

func TestRingBuffer_GetSince(t *testing.T) {
	rb := NewRingBuffer(5)

	// Add 5 events: 100, 101, 102, 103, 104
	for i := 100; i <= 104; i++ {
		rb.Add(&Event{ID: fmt.Sprintf("%04d", i), Seq: uint64(i)})
	}

	// Replay since 102 -> should return 103, 104
	replayed := rb.GetSince("0102")
	if len(replayed) != 2 {
		t.Fatalf("expected 2 replayed events, got %d", len(replayed))
	}
	if replayed[0].ID != "0103" || replayed[1].ID != "0104" {
		t.Fatalf("unexpected replay: %v, %v", replayed[0].ID, replayed[1].ID)
	}

	// Replay since 104 (latest) -> should return nil/0
	replayed = rb.GetSince("0104")
	if len(replayed) != 0 {
		t.Fatalf("expected 0 events since latest, got %d", len(replayed))
	}

	// Replay since 0050 (older than oldest 0100) -> should return all 5
	replayed = rb.GetSince("0050")
	if len(replayed) != 5 {
		t.Fatalf("expected 5 events for ancient lastEventID, got %d", len(replayed))
	}
	if replayed[0].ID != "0100" || replayed[4].ID != "0104" {
		t.Fatalf("unexpected full replay bounds: %s to %s", replayed[0].ID, replayed[4].ID)
	}

	// Replay empty -> should return nil
	if len(rb.GetSince("")) != 0 {
		t.Fatalf("expected 0 events for empty lastEventID")
	}
}

func TestRingBuffer_ConcurrentReadWrite(t *testing.T) {
	rb := NewRingBuffer(100)
	var wg sync.WaitGroup

	// 5 concurrent writers
	for w := 0; w < 5; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				id, seq := NextEventID()
				rb.Add(&Event{
					ID:        id,
					Seq:       seq,
					Channel:   "test",
					Timestamp: time.Now(),
				})
			}
		}(w)
	}

	// 5 concurrent readers
	for r := 0; r < 5; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				_ = rb.Size()
				_ = rb.GetLatest(10)
			}
		}()
	}

	wg.Wait()
	if rb.Size() != 100 {
		t.Fatalf("expected full buffer size 100, got %d", rb.Size())
	}
}
