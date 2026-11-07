package hub

import (
	"fmt"
	"sync"
	"testing"
)

func TestClient_Backpressure(t *testing.T) {
	// Client with buffer size 2 and drop_oldest policy
	c := NewClient("client-1", 2, "drop_oldest", "127.0.0.1", "test-agent")

	ev1 := &Event{ID: "1", Data: []byte("first")}
	ev2 := &Event{ID: "2", Data: []byte("second")}
	ev3 := &Event{ID: "3", Data: []byte("third")}

	c.Enqueue(ev1)
	c.Enqueue(ev2)
	// Buffer is now full [ev1, ev2]. Enqueuing ev3 should drop oldest (ev1) and insert ev3
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

	c1 := NewClient("c1", 10, "drop_oldest", "127.0.0.1", "agent1")
	c2 := NewClient("c2", 10, "drop_oldest", "127.0.0.1", "agent2")

	h.RegisterClient(c1, []string{"sports"}, "")
	h.RegisterClient(c2, []string{"sports", "news"}, "")

	stats := h.GetStats()
	if stats.ActiveConnections != 2 {
		t.Fatalf("expected 2 active connections, got %d", stats.ActiveConnections)
	}

	// Publish to sports
	ev, delivered := h.Publish("sports", "score", []byte(`{"score":"1-0"}`))
	if delivered != 2 {
		t.Fatalf("expected delivery to 2 clients, got %d", delivered)
	}
	if ev.ID == "" {
		t.Fatalf("expected non-empty event ID")
	}

	// Verify both clients received it
	select {
	case msg := <-c1.Send:
		if string(msg.Data) != `{"score":"1-0"}` {
			t.Fatalf("c1 received unexpected data: %s", string(msg.Data))
		}
	default:
		t.Fatalf("c1 did not receive message")
	}

	select {
	case msg := <-c2.Send:
		if string(msg.Data) != `{"score":"1-0"}` {
			t.Fatalf("c2 received unexpected data: %s", string(msg.Data))
		}
	default:
		t.Fatalf("c2 did not receive message")
	}

	// Publish to news -> only c2 should receive
	_, deliveredNews := h.Publish("news", "headline", []byte(`{"title":"breaking"}`))
	if deliveredNews != 1 {
		t.Fatalf("expected delivery to 1 client on news, got %d", deliveredNews)
	}

	select {
	case msg := <-c2.Send:
		if string(msg.Data) != `{"title":"breaking"}` {
			t.Fatalf("c2 received unexpected data: %s", string(msg.Data))
		}
	default:
		t.Fatalf("c2 did not receive news message")
	}

	// Unregister c1
	h.UnregisterClient(c1)
	if h.GetStats().ActiveConnections != 1 {
		t.Fatalf("expected 1 active connection after unregistering c1")
	}
}

func TestHub_WildcardRouting(t *testing.T) {
	h := NewHub(10, 10, "drop_oldest")

	c := NewClient("wild-client", 10, "drop_oldest", "127.0.0.1", "agent")
	h.RegisterClient(c, []string{"sensors.*"}, "")

	// Publish to sensors.temperature
	_, delivered := h.Publish("sensors.temperature", "reading", []byte(`{"celsius":22}`))
	if delivered != 1 {
		t.Fatalf("expected 1 delivery via wildcard, got %d", delivered)
	}

	select {
	case msg := <-c.Send:
		if msg.Channel != "sensors.temperature" {
			t.Fatalf("expected channel sensors.temperature, got %s", msg.Channel)
		}
	default:
		t.Fatalf("wildcard subscriber did not receive message")
	}
}

func TestHub_ReplayOnRegister(t *testing.T) {
	h := NewHub(10, 10, "drop_oldest")

	// Publish 3 events to channel "audit" before client connects
	ev1, _ := h.Publish("audit", "step1", []byte("data1"))
	ev2, _ := h.Publish("audit", "step2", []byte("data2"))
	ev3, _ := h.Publish("audit", "step3", []byte("data3"))

	// Client registers requesting replay since ev1.ID
	c := NewClient("replaying-client", 10, "drop_oldest", "127.0.0.1", "agent")
	h.RegisterClient(c, []string{"audit"}, ev1.ID)

	// Client should immediately have received ev2 and ev3
	var replayed []*Event
	for i := 0; i < 2; i++ {
		select {
		case msg := <-c.Send:
			replayed = append(replayed, msg)
		default:
			t.Fatalf("missing replayed event index %d", i)
		}
	}

	if replayed[0].ID != ev2.ID || replayed[1].ID != ev3.ID {
		t.Fatalf("unexpected replayed IDs: %s, %s", replayed[0].ID, replayed[1].ID)
	}
}

func TestHub_ConcurrentFanOut(t *testing.T) {
	h := NewHub(50, 100, "drop_oldest")

	// Register 100 clients
	numClients := 100
	clients := make([]*Client, numClients)
	for i := 0; i < numClients; i++ {
		c := NewClient(fmt.Sprintf("client-%d", i), 100, "drop_oldest", "127.0.0.1", "bench")
		clients[i] = c
		h.RegisterClient(c, []string{"benchmark"}, "")
	}

	var wg sync.WaitGroup
	// 5 concurrent publishers
	for p := 0; p < 5; p++ {
		wg.Add(1)
		go func(pubID int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				h.Publish("benchmark", "tick", []byte(fmt.Sprintf(`{"publisher":%d,"i":%d}`, pubID, i)))
			}
		}(p)
	}

	wg.Wait()

	// Drain messages
	for _, c := range clients {
		received := 0
	DrainLoop:
		for {
			select {
			case <-c.Send:
				received++
			default:
				break DrainLoop
			}
		}
		if received == 0 {
			t.Fatalf("client received 0 messages")
		}
	}
}
