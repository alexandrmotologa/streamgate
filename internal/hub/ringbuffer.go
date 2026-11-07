package hub

import (
	"strings"
	"sync"
)

// RingBuffer represents a thread-safe circular memory buffer for storing events per channel.
type RingBuffer struct {
	mu       sync.RWMutex
	capacity int
	events   []*Event
	head     int // index where next event will be written
	count    int // number of events currently held
}

// NewRingBuffer creates a circular buffer with the specified capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 1024
	}
	return &RingBuffer{
		capacity: capacity,
		events:   make([]*Event, capacity),
	}
}

// Add appends an event to the circular buffer, overwriting the oldest event if full.
func (rb *RingBuffer) Add(e *Event) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.events[rb.head] = e
	rb.head = (rb.head + 1) % rb.capacity
	if rb.count < rb.capacity {
		rb.count++
	}
}

// Capacity returns the maximum buffer capacity.
func (rb *RingBuffer) Capacity() int {
	return rb.capacity
}

// Size returns the current number of events stored in the buffer.
func (rb *RingBuffer) Size() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}

// GetSince returns all events published strictly after the given lastEventID.
// If lastEventID is older than the oldest event in the buffer, all buffered events are returned.
// If lastEventID is the latest or newer, an empty slice is returned.
func (rb *RingBuffer) GetSince(lastEventID string) []*Event {
	lastEventID = strings.TrimSpace(lastEventID)
	if lastEventID == "" {
		return nil
	}

	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.count == 0 {
		return nil
	}

	// Calculate index of the oldest item
	oldestIdx := (rb.head - rb.count + rb.capacity) % rb.capacity
	newestIdx := (rb.head - 1 + rb.capacity) % rb.capacity

	oldestEvent := rb.events[oldestIdx]
	newestEvent := rb.events[newestIdx]

	if oldestEvent == nil || newestEvent == nil {
		return nil
	}

	// If client is already up to date with the latest event
	if newestEvent.ID == lastEventID {
		return nil
	}

	// Scan buffer from oldest to newest
	foundIdx := -1
	for i := 0; i < rb.count; i++ {
		idx := (oldestIdx + i) % rb.capacity
		ev := rb.events[idx]
		if ev != nil && ev.ID == lastEventID {
			foundIdx = i
			break
		}
	}

	var result []*Event
	if foundIdx >= 0 {
		// Event was found: return all subsequent events
		remaining := rb.count - (foundIdx + 1)
		if remaining <= 0 {
			return nil
		}
		result = make([]*Event, remaining)
		for j := 0; j < remaining; j++ {
			idx := (oldestIdx + foundIdx + 1 + j) % rb.capacity
			result[j] = rb.events[idx]
		}
	} else {
		// Event was not found. If lastEventID lexicographically/chronologically older than oldest, replay all.
		if lastEventID < oldestEvent.ID {
			result = make([]*Event, rb.count)
			for i := 0; i < rb.count; i++ {
				idx := (oldestIdx + i) % rb.capacity
				result[i] = rb.events[idx]
			}
		}
		// If lastEventID > newestEvent.ID, it represents a future or unknown ID, return nil
	}

	return result
}

// GetLatest returns up to n most recent events in chronological order.
func (rb *RingBuffer) GetLatest(n int) []*Event {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.count == 0 || n <= 0 {
		return nil
	}

	if n > rb.count {
		n = rb.count
	}

	result := make([]*Event, n)
	startOffset := rb.count - n
	oldestIdx := (rb.head - rb.count + rb.capacity) % rb.capacity

	for i := 0; i < n; i++ {
		idx := (oldestIdx + startOffset + i) % rb.capacity
		result[i] = rb.events[idx]
	}

	return result
}
