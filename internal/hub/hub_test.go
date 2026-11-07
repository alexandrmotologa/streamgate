package hub

import (
	"fmt"
	"sync"
	"testing"
)

type mockLifecycle struct {
	mu       sync.Mutex
	occupied []string
	vacated  []string
}

func (m *mockLifecycle) OnChannelOccupied(channel string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.occupied = append(m.occupied, channel)
}

func (m *mockLifecycle) OnChannelVacated(channel string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.vacated = append(m.vacated, channel)
}

func TestClient_Backpressure(t *testing.T) {
	c := NewClient("client-1", "user-1", nil, 2, "drop_oldest", "127.0.0.1", "test-agent")

	ev1 := &Event{ID: "1", Data: []byte("first")}
	ev2 := &Event{ID: "2", Data: []byte("second")}
	ev3 := &Event{ID: "3", Data: []byte("third")}

	c.Enqueue(ev1)
	c.Enqueue(ev2)
	c.Enqueue(ev3)

	if c.Dropped() != 1 {
		t.Fatalf("expected 1 dropped message, got %d", c.Dropped())
	}

	got1 := <-c.Send
	got2 := <-c.Send
	if got1.ID != "2" || got2.ID != "3" {
		t.Fatalf("expected events 2 and 3, got %s and %s", got1.ID, got2.ID)
	}
}

func TestHub_PublishAndSubscribe(t *testing.T) {
	h := NewHub(10, 10, "drop_oldest")

	c1 := NewClient("c1", "user1", nil, 10, "drop_oldest", "127.0.0.1", "agent1")
	c2 := NewClient("c2", "user2", nil, 10, "drop_oldest", "127.0.0.1", "agent2")

	h.RegisterClient(c1, []string{"sports"}, "")
	h.RegisterClient(c2, []string{"sports", "news"}, "")

	stats := h.GetStats()
	if stats.ActiveConnections != 2 {
		t.Fatalf("expected 2 active connections, got %d", stats.ActiveConnections)
	}

	ev, delivered := h.Publish("sports", "score", []byte(`{"score":"1-0"}`))
	if delivered != 2 {
		t.Fatalf("expected delivery to 2 clients, got %d", delivered)
	}
	if ev.ID == "" {
		t.Fatalf("expected non-empty event ID")
	}

	select {
	case msg := <-c1.Send:
		if string(msg.Data) != `{"score":"1-0"}` {
			t.Fatalf("c1 received unexpected data: %s", string(msg.Data))
		}
	default:
		t.Fatalf("c1 did not receive message")
	}

	h.UnregisterClient(c1)
	if h.GetStats().ActiveConnections != 1 {
		t.Fatalf("expected 1 active connection after unregistering c1")
	}
}

func TestHub_PresenceChannels(t *testing.T) {
	h := NewHub(10, 10, "drop_oldest")

	c1 := NewClient("c1", "usr_alice", map[string]any{"name": "Alice"}, 10, "drop_oldest", "127.0.0.1", "agent1")
	c2 := NewClient("c2", "usr_bob", map[string]any{"name": "Bob"}, 10, "drop_oldest", "127.0.0.1", "agent2")

	// c1 joins presence-chat
	h.RegisterClient(c1, []string{"presence-chat"}, "")

	members := h.GetChannelMembers("presence-chat")
	if len(members) != 1 {
		t.Fatalf("expected 1 member in presence channel, got %d", len(members))
	}
	if members[0].UserID != "usr_alice" {
		t.Fatalf("expected user_id usr_alice, got %s", members[0].UserID)
	}

	// c2 joins presence-chat -> c1 should receive presence.member_added event!
	h.RegisterClient(c2, []string{"presence-chat"}, "")

	members = h.GetChannelMembers("presence-chat")
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}

	// Check that c1 received member_added notification for c2
	receivedAdded := false
	for i := 0; i < 2; i++ {
		select {
		case msg := <-c1.Send:
			if msg.Event == "presence.member_added" {
				receivedAdded = true
			}
		default:
		}
	}
	if !receivedAdded {
		t.Fatalf("c1 did not receive presence.member_added event")
	}

	// c2 leaves
	h.UnregisterClient(c2)
	members = h.GetChannelMembers("presence-chat")
	if len(members) != 1 {
		t.Fatalf("expected 1 member after c2 left, got %d", len(members))
	}
}

func TestHub_LifecycleHooks(t *testing.T) {
	h := NewHub(10, 10, "drop_oldest")
	mock := &mockLifecycle{}
	h.SetLifecycleListener(mock)

	c1 := NewClient("c1", "usr_1", nil, 10, "drop_oldest", "127.0.0.1", "agent1")

	// First client joins -> channel_occupied
	h.RegisterClient(c1, []string{"stocks"}, "")

	mock.mu.Lock()
	if len(mock.occupied) != 1 || mock.occupied[0] != "stocks" {
		t.Fatalf("expected 1 occupied event for stocks, got %v", mock.occupied)
	}
	mock.mu.Unlock()

	// Client leaves -> channel_vacated
	h.UnregisterClient(c1)

	mock.mu.Lock()
	if len(mock.vacated) != 1 || mock.vacated[0] != "stocks" {
		t.Fatalf("expected 1 vacated event for stocks, got %v", mock.vacated)
	}
	mock.mu.Unlock()
}

func TestHub_ChannelHistory(t *testing.T) {
	h := NewHub(10, 10, "drop_oldest")

	for i := 1; i <= 5; i++ {
		h.Publish("history-test", "tick", []byte(fmt.Sprintf(`{"i":%d}`, i)))
	}

	// Retrieve last 3
	history := h.GetChannelHistory("history-test", 3, "")
	if len(history) != 3 {
		t.Fatalf("expected 3 history events, got %d", len(history))
	}
	if string(history[2].Data) != `{"i":5}` {
		t.Fatalf("expected latest event i:5, got %s", string(history[2].Data))
	}
}
