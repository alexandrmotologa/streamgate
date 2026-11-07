package hub

import (
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"
)

const numStripes = 64

// subscriberStripe is a sharded bucket of subscribers with its own RWMutex.
type subscriberStripe struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

// Channel manages the subscriber set and replay buffer for a single topic.
type Channel struct {
	Name            string
	Buffer          *RingBuffer
	stripes         [numStripes]subscriberStripe
	totalSubCount   int64
	totalPublished  uint64
	totalDelivered  uint64
	lastPublishedAt atomic.Value // holds time.Time
	CreatedAt       time.Time
}

// NewChannel initializes a channel with lock-striped subscriber storage.
func NewChannel(name string, ringCapacity int) *Channel {
	ch := &Channel{
		Name:      name,
		Buffer:    NewRingBuffer(ringCapacity),
		CreatedAt: time.Now(),
	}
	for i := 0; i < numStripes; i++ {
		ch.stripes[i].clients = make(map[string]*Client)
	}
	ch.lastPublishedAt.Store(time.Time{})
	return ch
}

func getStripeIndex(clientID string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(clientID))
	return h.Sum32() % numStripes
}

// AddSubscriber registers a client with the channel.
func (ch *Channel) AddSubscriber(c *Client) {
	idx := getStripeIndex(c.ID)
	stripe := &ch.stripes[idx]

	stripe.mu.Lock()
	if _, exists := stripe.clients[c.ID]; !exists {
		stripe.clients[c.ID] = c
		atomic.AddInt64(&ch.totalSubCount, 1)
	}
	stripe.mu.Unlock()

	c.Subscribe(ch.Name)
}

// RemoveSubscriber unregisters a client from the channel.
func (ch *Channel) RemoveSubscriber(c *Client) {
	idx := getStripeIndex(c.ID)
	stripe := &ch.stripes[idx]

	stripe.mu.Lock()
	if _, exists := stripe.clients[c.ID]; exists {
		delete(stripe.clients, c.ID)
		atomic.AddInt64(&ch.totalSubCount, -1)
	}
	stripe.mu.Unlock()

	c.Unsubscribe(ch.Name)
}

// Broadcast sends an event to all subscribers of this channel and caches it in the ring buffer.
// Returns the number of subscribers the event was delivered to.
func (ch *Channel) Broadcast(e *Event) int {
	// Cache in ring buffer
	ch.Buffer.Add(e)
	atomic.AddUint64(&ch.totalPublished, 1)
	ch.lastPublishedAt.Store(time.Now())

	delivered := 0

	// Iterate across all lock stripes
	for i := 0; i < numStripes; i++ {
		stripe := &ch.stripes[i]
		stripe.mu.RLock()
		for _, client := range stripe.clients {
			if client.Enqueue(e) {
				delivered++
			}
		}
		stripe.mu.RUnlock()
	}

	atomic.AddUint64(&ch.totalDelivered, uint64(delivered))
	return delivered
}

// SubscriberCount returns the current number of active subscribers on this channel.
func (ch *Channel) SubscriberCount() int {
	cnt := atomic.LoadInt64(&ch.totalSubCount)
	if cnt < 0 {
		return 0
	}
	return int(cnt)
}

// Stats returns a summary snapshot of channel metrics.
func (ch *Channel) Stats() (subscribers int, published uint64, delivered uint64, lastPub time.Time) {
	subscribers = ch.SubscriberCount()
	published = atomic.LoadUint64(&ch.totalPublished)
	delivered = atomic.LoadUint64(&ch.totalDelivered)
	if val := ch.lastPublishedAt.Load(); val != nil {
		lastPub = val.(time.Time)
	}
	return
}
