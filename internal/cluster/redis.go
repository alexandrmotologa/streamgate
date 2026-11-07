package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/alexandrmotologa/streamgate/internal/hub"
	"github.com/redis/go-redis/v9"
)

// Message is the cross-node envelope distributed over the Redis backbone.
type Message struct {
	Origin    string          `json:"origin"`
	Channel   string          `json:"channel"`
	Event     string          `json:"event"`
	Data      json.RawMessage `json:"data"`
	Timestamp int64           `json:"timestamp"`
}

// Manager coordinates Redis/Valkey pub/sub synchronization between cluster nodes.
type Manager struct {
	instanceID string
	prefix     string
	client     *redis.Client
	hub        *hub.Hub
	topic      string
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewManager creates a Redis cluster synchronization adapter.
func NewManager(addr, password, prefix string, h *hub.Hub) (*Manager, error) {
	if addr == "" {
		return nil, nil // Standalone mode
	}
	if prefix == "" {
		prefix = "streamgate"
	}

	instanceID := fmt.Sprintf("node-%d-%04d", time.Now().UnixMilli(), rand.Intn(10000))
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
	})

	ctx, cancel := context.WithCancel(context.Background())

	// Test connection
	pingCtx, pingCancel := context.WithTimeout(ctx, 3*time.Second)
	defer pingCancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		cancel()
		_ = rdb.Close()
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	m := &Manager{
		instanceID: instanceID,
		prefix:     prefix,
		client:     rdb,
		hub:        h,
		topic:      prefix + ":events",
		ctx:        ctx,
		cancel:     cancel,
	}

	h.SetClusterBroadcaster(m)
	go m.listen()

	log.Printf("[INFO] Redis cluster pub/sub backbone connected on %s (node: %s)", addr, instanceID)
	return m, nil
}

// PublishToCluster broadcasts an event envelope to other nodes in the cluster.
func (m *Manager) PublishToCluster(channel, event string, data []byte) error {
	if m == nil || m.client == nil {
		return nil
	}

	msg := Message{
		Origin:    m.instanceID,
		Channel:   channel,
		Event:     event,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return m.client.Publish(m.ctx, m.topic, payload).Err()
}

func (m *Manager) listen() {
	pubsub := m.client.Subscribe(m.ctx, m.topic)
	defer pubsub.Close()

	ch := pubsub.Channel()

	for {
		select {
		case <-m.ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}

			var clusterMsg Message
			if err := json.Unmarshal([]byte(msg.Payload), &clusterMsg); err != nil {
				continue
			}

			// Ignore events originating from this node to prevent loop
			if clusterMsg.Origin == m.instanceID {
				continue
			}

			// Deliver locally to subscribers of this node
			m.hub.PublishLocal(clusterMsg.Channel, clusterMsg.Event, clusterMsg.Data)
		}
	}
}

// Close shuts down the cluster subscription and redis client.
func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.cancel()
	if m.client != nil {
		_ = m.client.Close()
	}
}
