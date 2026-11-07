package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestWebhookDispatcher(t *testing.T) {
	secret := "test-secret"
	var receivedPayload Payload
	var receivedSig string
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		receivedSig = r.Header.Get("X-Streamgate-Signature")
		_ = json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dispatcher := NewDispatcher(server.URL, secret)
	defer dispatcher.Close()

	dispatcher.Trigger(EventChannelOccupied, "finance-trades")

	// Allow worker to dispatch
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if receivedPayload.Event != EventChannelOccupied {
		t.Fatalf("expected event %s, got %s", EventChannelOccupied, receivedPayload.Event)
	}
	if receivedPayload.Channel != "finance-trades" {
		t.Fatalf("expected channel finance-trades, got %s", receivedPayload.Channel)
	}

	// Validate HMAC signature
	raw, _ := json.Marshal(receivedPayload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(raw)
	expectedSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if receivedSig != expectedSig {
		t.Fatalf("signature mismatch: expected %s, got %s", expectedSig, receivedSig)
	}
}
