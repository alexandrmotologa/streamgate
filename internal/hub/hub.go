package hub

import (
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ChannelInfo provides metadata about an active channel.
type ChannelInfo struct {
	Name           string    `json:"name"`
	Subscribers    int       `json:"subscribers"`
	TotalPublished uint64    `json:"total_published"`
	TotalDelivered uint64    `json:"total_delivered"`
	BufferSize     int       `json:"buffer_size"`
	BufferCapacity int       `json:"buffer_capacity"`
	CreatedAt      time.Time `json:"created_at"`
	LastPublished  time.Time `json:"last_published"`
}

// HubStats provides high-level telemetry on gateway runtime performance.
type HubStats struct {
	ActiveConnections      int    `json:"active_connections"`
	TotalConnectionsCount  uint64 `json:"total_connections_count"`
	TotalChannelsCount     int    `json:"total_channels_count"`
	TotalMessagesPublished uint64 `json:"total_messages_published"`
	TotalMessagesDelivered uint64 `json:"total_messages_delivered"`
	TotalMessagesDropped   uint64 `json:"total_messages_dropped"`
	UptimeSeconds          int64  `json:"uptime_seconds"`
}

// Hub is the central broker orchestrating channels, routing, and subscriber lifecycles.
type Hub struct {
	channelsMu        sync.RWMutex
	channels          map[string]*Channel
	clients           sync.Map // string -> *Client
	defaultRingCap    int
	defaultClientCap  int
	defaultDropPolicy string
	startedAt         time.Time

	// Counters
	activeConnections  int64
	totalConnections   uint64
	totalPublished     uint64
	totalDelivered     uint64
	totalDroppedManual uint64
}

// NewHub creates a new Hub instance.
func NewHub(ringCap, clientCap int, dropPolicy string) *Hub {
	if ringCap <= 0 {
		ringCap = 1024
	}
	if clientCap <= 0 {
		clientCap = 256
	}
	if dropPolicy == "" {
		dropPolicy = "drop_oldest"
	}

	return &Hub{
		channels:          make(map[string]*Channel),
		defaultRingCap:    ringCap,
		defaultClientCap:  clientCap,
		defaultDropPolicy: dropPolicy,
		startedAt:         time.Now(),
	}
}

// GetOrCreateChannel returns an existing channel or safely instantiates a new one.
func (h *Hub) GetOrCreateChannel(name string) *Channel {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "default"
	}

	h.channelsMu.RLock()
	ch, exists := h.channels[name]
	h.channelsMu.RUnlock()
	if exists {
		return ch
	}

	h.channelsMu.Lock()
	defer h.channelsMu.Unlock()

	// Double check
	if ch, exists = h.channels[name]; exists {
		return ch
	}

	ch = NewChannel(name, h.defaultRingCap)
	h.channels[name] = ch
	return ch
}

// RegisterClient stores the client in the hub and hooks it into requested channels.
// Replays missed events if lastEventID is supplied.
func (h *Hub) RegisterClient(c *Client, channels []string, lastEventID string) {
	h.clients.Store(c.ID, c)
	atomic.AddInt64(&h.activeConnections, 1)
	atomic.AddUint64(&h.totalConnections, 1)

	for _, chName := range channels {
		chName = strings.TrimSpace(chName)
		if chName == "" {
			continue
		}

		ch := h.GetOrCreateChannel(chName)

		// Replay missed events prior to hooking live broadcast
		if lastEventID != "" {
			missed := ch.Buffer.GetSince(lastEventID)
			for _, ev := range missed {
				c.Enqueue(ev)
			}
		}

		ch.AddSubscriber(c)
	}
}

// UnregisterClient removes the client from all channels and releases resources.
func (h *Hub) UnregisterClient(c *Client) {
	if _, loaded := h.clients.LoadAndDelete(c.ID); !loaded {
		return
	}

	atomic.AddInt64(&h.activeConnections, -1)
	c.CloseConnection()

	// Remove from channels
	subscribed := c.SubscribedChannels()
	h.channelsMu.RLock()
	for _, chName := range subscribed {
		if ch, ok := h.channels[chName]; ok {
			ch.RemoveSubscriber(c)
		}
	}
	h.channelsMu.RUnlock()
}

// Publish broadcasts an event to the target channel and any wildcard patterns matching the channel.
func (h *Hub) Publish(channelName, eventType string, data []byte) (*Event, int) {
	channelName = strings.TrimSpace(channelName)
	if channelName == "" {
		channelName = "default"
	}

	id, seq := NextEventID()
	ev := &Event{
		ID:        id,
		Seq:       seq,
		Channel:   channelName,
		Event:     eventType,
		Data:      data,
		Timestamp: time.Now(),
	}
	// Pre-format SSE frame once for all subscribers
	ev.FormatSSE()

	delivered := 0

	// 1. Direct channel broadcast
	targetChannel := h.GetOrCreateChannel(channelName)
	delivered += targetChannel.Broadcast(ev)

	// 2. Wildcard channel broadcast (e.g., "sensors.*" matches "sensors.temp")
	h.channelsMu.RLock()
	for name, ch := range h.channels {
		if name == channelName {
			continue
		}
		if isWildcardMatch(name, channelName) {
			delivered += ch.Broadcast(ev)
		}
	}
	h.channelsMu.RUnlock()

	atomic.AddUint64(&h.totalPublished, 1)
	atomic.AddUint64(&h.totalDelivered, uint64(delivered))

	return ev, delivered
}

// isWildcardMatch checks if a pattern like "stocks.*" matches "stocks.btc" or "stocks.apple.us"
func isWildcardMatch(pattern, channel string) bool {
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		return strings.HasPrefix(channel, prefix+".")
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(channel, prefix)
	}
	return false
}

// ListChannels returns details of all registered channels.
func (h *Hub) ListChannels() []ChannelInfo {
	h.channelsMu.RLock()
	defer h.channelsMu.RUnlock()

	result := make([]ChannelInfo, 0, len(h.channels))
	for name, ch := range h.channels {
		subs, pub, del, lastPub := ch.Stats()
		result = append(result, ChannelInfo{
			Name:           name,
			Subscribers:    subs,
			TotalPublished: pub,
			TotalDelivered: del,
			BufferSize:     ch.Buffer.Size(),
			BufferCapacity: ch.Buffer.Capacity(),
			CreatedAt:      ch.CreatedAt,
			LastPublished:  lastPub,
		})
	}
	return result
}

// GetStats returns gateway runtime statistics.
func (h *Hub) GetStats() HubStats {
	active := atomic.LoadInt64(&h.activeConnections)
	if active < 0 {
		active = 0
	}

	h.channelsMu.RLock()
	channelCount := len(h.channels)
	h.channelsMu.RUnlock()

	var dropped uint64
	h.clients.Range(func(_, val any) bool {
		if c, ok := val.(*Client); ok {
			dropped += c.Dropped()
		}
		return true
	})

	return HubStats{
		ActiveConnections:      int(active),
		TotalConnectionsCount:  atomic.LoadUint64(&h.totalConnections),
		TotalChannelsCount:     channelCount,
		TotalMessagesPublished: atomic.LoadUint64(&h.totalPublished),
		TotalMessagesDelivered: atomic.LoadUint64(&h.totalDelivered),
		TotalMessagesDropped:   dropped,
		UptimeSeconds:          int64(time.Since(h.startedAt).Seconds()),
	}
}

// BroadcastMetricsSnapshot dispatches internal telemetry snapshot to the "_streamgate.metrics" channel.
func (h *Hub) BroadcastMetricsSnapshot() {
	stats := h.GetStats()
	payload, err := json.Marshal(stats)
	if err == nil {
		h.Publish("_streamgate.metrics", "telemetry", payload)
	}
}
