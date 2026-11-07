package hub

import (
	"sync"
	"sync/atomic"
	"time"
)

// Client represents an active HTTP SSE subscriber connection.
type Client struct {
	ID           string
	Send         chan *Event
	Channels     map[string]struct{}
	channelsMu   sync.RWMutex
	ConnectedAt  time.Time
	RemoteAddr   string
	UserAgent    string
	DroppedCount uint64
	DropPolicy   string // "drop_oldest", "drop_newest", "disconnect"
	Done         chan struct{}
	closed       int32
}

// NewClient initializes a new subscriber connection abstraction.
func NewClient(id string, bufferSize int, dropPolicy string, remoteAddr, userAgent string) *Client {
	if bufferSize <= 0 {
		bufferSize = 256
	}
	if dropPolicy == "" {
		dropPolicy = "drop_oldest"
	}
	return &Client{
		ID:          id,
		Send:        make(chan *Event, bufferSize),
		Channels:    make(map[string]struct{}),
		ConnectedAt: time.Now(),
		RemoteAddr:  remoteAddr,
		UserAgent:   userAgent,
		DropPolicy:  dropPolicy,
		Done:        make(chan struct{}),
	}
}

// Enqueue attempts to dispatch an event to the client's buffered channel non-blockingly.
// If the buffer is full, it executes the configured backpressure drop policy.
func (c *Client) Enqueue(e *Event) bool {
	if c.IsClosed() {
		return false
	}

	select {
	case c.Send <- e:
		return true
	default:
		// Client write buffer is full
		atomic.AddUint64(&c.DroppedCount, 1)

		switch c.DropPolicy {
		case "drop_newest":
			return false

		case "disconnect":
			c.CloseConnection()
			return false

		case "drop_oldest":
			fallthrough
		default:
			// Discard the oldest event in the buffer and queue the newest
			select {
			case <-c.Send:
			default:
			}

			select {
			case c.Send <- e:
				return true
			default:
				return false
			}
		}
	}
}

// Subscribe marks the client as listening to a specific channel.
func (c *Client) Subscribe(channel string) {
	c.channelsMu.Lock()
	defer c.channelsMu.Unlock()
	c.Channels[channel] = struct{}{}
}

// Unsubscribe removes a channel from the client's active subscriptions.
func (c *Client) Unsubscribe(channel string) {
	c.channelsMu.Lock()
	defer c.channelsMu.Unlock()
	delete(c.Channels, channel)
}

// SubscribedChannels returns a snapshot slice of subscribed channel names.
func (c *Client) SubscribedChannels() []string {
	c.channelsMu.RLock()
	defer c.channelsMu.RUnlock()
	list := make([]string, 0, len(c.Channels))
	for ch := range c.Channels {
		list = append(list, ch)
	}
	return list
}

// IsSubscribed returns true if the client is subscribed to the specified channel.
func (c *Client) IsSubscribed(channel string) bool {
	c.channelsMu.RLock()
	defer c.channelsMu.RUnlock()
	_, ok := c.Channels[channel]
	return ok
}

// CloseConnection shuts down the client send loop safely.
func (c *Client) CloseConnection() {
	if atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		close(c.Done)
	}
}

// IsClosed checks if the connection has been terminated.
func (c *Client) IsClosed() bool {
	return atomic.LoadInt32(&c.closed) == 1
}

// Dropped returns the total number of dropped messages for this client.
func (c *Client) Dropped() uint64 {
	return atomic.LoadUint64(&c.DroppedCount)
}
