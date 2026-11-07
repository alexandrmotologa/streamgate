package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alexandrmotologa/streamgate/internal/auth"
	"github.com/alexandrmotologa/streamgate/internal/config"
	"github.com/alexandrmotologa/streamgate/internal/hub"
	"github.com/alexandrmotologa/streamgate/internal/telemetry"
)

func setupTestRouter() (*Router, *hub.Hub) {
	cfg := config.DefaultConfig()
	cfg.HeartbeatInterval = 50 * time.Millisecond
	h := hub.NewHub(100, 100, "drop_oldest")
	a := auth.NewAuthenticator("", "")
	m := telemetry.NewMetrics(nil)
	r := NewRouter(cfg, h, a, m, nil)
	return r, h
}

func TestAPI_HealthAndStats(t *testing.T) {
	router, _ := setupTestRouter()

	// 1. /healthz
	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /healthz, got %d", rec.Code)
	}

	// 2. /api/v1/stats
	req = httptest.NewRequest("GET", "/api/v1/stats", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/v1/stats, got %d", rec.Code)
	}

	var statsResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &statsResp); err != nil {
		t.Fatalf("failed to decode stats: %v", err)
	}
	if _, ok := statsResp["gateway"]; !ok {
		t.Fatalf("expected 'gateway' in stats response")
	}
}

func TestAPI_PublishSingleAndBatch(t *testing.T) {
	router, _ := setupTestRouter()

	// 1. Single Publish
	payload := []byte(`{"channel":"orders","event":"created","data":{"order_id":1234}}`)
	req := httptest.NewRequest("POST", "/api/v1/publish", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for single publish, got %d body=%s", rec.Code, rec.Body.String())
	}

	var pubResp PublishResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &pubResp); err != nil {
		t.Fatalf("failed to decode publish response: %v", err)
	}
	if !pubResp.Success || pubResp.Channel != "orders" {
		t.Fatalf("unexpected publish response: %+v", pubResp)
	}

	// 2. Batch Publish
	batchPayload := []byte(`{
		"events": [
			{"channel":"orders","event":"updated","data":{"status":"shipped"}},
			{"channel":"notifications","event":"alert","data":"email sent"}
		]
	}`)
	req = httptest.NewRequest("POST", "/api/v1/publish/batch", bytes.NewReader(batchPayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for batch publish, got %d body=%s", rec.Code, rec.Body.String())
	}

	var batchResp BatchPublishResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &batchResp); err != nil {
		t.Fatalf("failed to decode batch response: %v", err)
	}
	if batchResp.TotalEvents != 2 {
		t.Fatalf("expected 2 total events, got %d", batchResp.TotalEvents)
	}

	// 3. Check Channel list
	req = httptest.NewRequest("GET", "/api/v1/channels", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var chList map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &chList)
	if chList["total"].(float64) < 2 {
		t.Fatalf("expected at least 2 channels, got %v", chList["total"])
	}
}

func TestAPI_SSESubscribeAndReplay(t *testing.T) {
	router, h := setupTestRouter()

	// Publish an event before subscription
	ev1, _ := h.Publish("live", "init", []byte(`{"init":true}`))

	server := httptest.NewServer(router)
	defer server.Close()

	// Connect SSE with Last-Event-ID
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/v1/subscribe?channel=live", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	// Replay request using header
	req.Header.Set("Last-Event-ID", "0000000000-000000") // older than ev1 -> will replay ev1!

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to connect SSE: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from SSE, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("expected Content-Type text/event-stream, got %s", resp.Header.Get("Content-Type"))
	}

	reader := bufio.NewReader(resp.Body)

	// Read initial greeting ":connected"
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read greeting: %v", err)
	}
	if !strings.HasPrefix(line, ":connected") {
		t.Fatalf("expected :connected greeting, got: %s", line)
	}

	// Next, we should receive the replayed ev1
	var receivedID string
	var receivedEvent string
	var receivedData string

	for i := 0; i < 5; i++ {
		l, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "id: ") {
			receivedID = strings.TrimPrefix(l, "id: ")
		} else if strings.HasPrefix(l, "event: ") {
			receivedEvent = strings.TrimPrefix(l, "event: ")
		} else if strings.HasPrefix(l, "data: ") {
			receivedData = strings.TrimPrefix(l, "data: ")
			break
		}
	}

	if receivedID != ev1.ID {
		t.Fatalf("expected replayed event ID %s, got %s", ev1.ID, receivedID)
	}
	if receivedEvent != "init" {
		t.Fatalf("expected event 'init', got %s", receivedEvent)
	}
	if receivedData != `{"init":true}` {
		t.Fatalf("expected data '{\"init\":true}', got %s", receivedData)
	}

	// Now publish live event while connected
	go func() {
		time.Sleep(20 * time.Millisecond)
		_, _ = h.Publish("live", "live_tick", []byte(`{"tick":1}`))
	}()

	var liveEvent string
	var liveData string
	for i := 0; i < 6; i++ {
		l, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "event: ") {
			liveEvent = strings.TrimPrefix(l, "event: ")
		} else if strings.HasPrefix(l, "data: ") {
			liveData = strings.TrimPrefix(l, "data: ")
			break
		}
	}

	if liveEvent != "live_tick" || liveData != `{"tick":1}` {
		t.Fatalf("live event mismatch: event=%s data=%s", liveEvent, liveData)
	}
}
