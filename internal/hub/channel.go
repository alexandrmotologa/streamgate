package hub

import (
	"encoding/json"
	"hash/fnv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const numStripes = 64

// LifecycleListener defines notifications for channel occupancy changes.
type LifecycleListener interface {
	OnChannelOccupied(channel string)
	OnChannelVacated(channel string)
}

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

	// Presence channel support
	isPresence      bool
	presenceMu      sync.RWMutex
	presenceMembers map[string]*MemberInfo

	// Lifecycle hooks
	listener LifecycleListener
}

// NewChannel initializes a channel with lock-striped subscriber storage.
func NewChannel(name string, ringCapacity int, listener LifecycleListener) *Channel {
	ch := &Channel{
		Name:            name,
		Buffer:          NewRingBuffer(ringCapacity),
		CreatedAt:       time.Now(),
		isPresence:      strings.HasPrefix(name, "presence-"),
		presenceMembers: make(map[string]*MemberInfo),
		listener:        listener,
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

// AddSubscriber registers a client with the channel and fires presence / lifecycle hooks.
func (ch *Channel) AddSubscriber(c *Client) {
	idx := getStripeIndex(c.ID)
	stripe := &ch.stripes[idx]

	isNew := false
	stripe.mu.Lock()
	if _, exists := stripe.clients[c.ID]; !exists {
		stripe.clients[c.ID] = c
		isNew = true
	}
	stripe.mu.Unlock()

	if !isNew {
		return
	}

	c.Subscribe(ch.Name)
	newTotal := atomic.AddInt64(&ch.totalSubCount, 1)

	// Trigger lifecycle hook if channel transitioned 0 -> 1
	if newTotal == 1 && ch.listener != nil {
		ch.listener.OnChannelOccupied(ch.Name)
	}

	// Presence tracking
	if ch.isPresence {
		member := &MemberInfo{
			ClientID: c.ID,
			UserID:   c.UserID,
			UserInfo: c.UserInfo,
			JoinedAt: time.Now(),
		}

		ch.presenceMu.Lock()
		ch.presenceMembers[c.ID] = member
		ch.presenceMu.Unlock()

		// Broadcast member_added event
		payload, err := json.Marshal(map[string]any{
			"user_id":   member.UserID,
			"client_id": member.ClientID,
			"info":      member.UserInfo,
			"action":    "member_added",
		})
		if err == nil {
			id, seq := NextEventID()
			ev := &Event{
				ID:        id,
				Seq:       seq,
				Channel:   ch.Name,
				Event:     "presence.member_added",
				Data:      payload,
				Timestamp: time.Now(),
			}
			ev.FormatSSE()
			ch.Broadcast(ev)
		}
	}
}

// RemoveSubscriber unregisters a client from the channel and fires presence / lifecycle hooks.
func (ch *Channel) RemoveSubscriber(c *Client) {
	idx := getStripeIndex(c.ID)
	stripe := &ch.stripes[idx]

	removed := false
	stripe.mu.Lock()
	if _, exists := stripe.clients[c.ID]; exists {
		delete(stripe.clients, c.ID)
		removed = true
	}
	stripe.mu.Unlock()

	if !removed {
		return
	}

	c.Unsubscribe(ch.Name)
	newTotal := atomic.AddInt64(&ch.totalSubCount, -1)

	// Trigger lifecycle hook if channel vacated 1 -> 0
	if newTotal == 0 && ch.listener != nil {
		ch.listener.OnChannelVacated(ch.Name)
	}

	// Presence tracking
	if ch.isPresence {
		ch.presenceMu.Lock()
		delete(ch.presenceMembers, c.ID)
		ch.presenceMu.Unlock()

		// Broadcast member_removed event
		payload, err := json.Marshal(map[string]any{
			"user_id":   c.UserID,
			"client_id": c.ID,
			"action":    "member_removed",
		})
		if err == nil {
			id, seq := NextEventID()
			ev := &Event{
				ID:        id,
				Seq:       seq,
				Channel:   ch.Name,
				Event:     "presence.member_removed",
				Data:      payload,
				Timestamp: time.Now(),
			}
			ev.FormatSSE()
			ch.Broadcast(ev)
		}
	}
}

// Broadcast sends an event to all subscribers of this channel and caches it in the ring buffer.
func (ch *Channel) Broadcast(e *Event) int {
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

// GetMembers returns snapshot of members in a presence channel.
func (ch *Channel) GetMembers() []*MemberInfo {
	if !ch.isPresence {
		return nil
	}

	ch.presenceMu.RLock()
	defer ch.presenceMu.RUnlock()

	result := make([]*MemberInfo, 0, len(ch.presenceMembers))
	for _, m := range ch.presenceMembers {
		result = append(result, m)
	}
	return result
}

// IsPresence returns true if this channel tracks member presence.
func (ch *Channel) IsPresence() bool {
	return ch.isPresence
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
