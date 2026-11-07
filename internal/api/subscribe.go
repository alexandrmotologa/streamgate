package api

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/alexandrmotologa/streamgate/internal/auth"
	"github.com/alexandrmotologa/streamgate/internal/heartbeat"
	"github.com/alexandrmotologa/streamgate/internal/hub"
	"github.com/alexandrmotologa/streamgate/internal/telemetry"
)

var clientIDSeq uint64

// SubscribeHandler handles Server-Sent Events subscription connections.
type SubscribeHandler struct {
	hub               *hub.Hub
	auth              *auth.Authenticator
	metrics           *telemetry.Metrics
	heartbeatInterval time.Duration
	clientBufferSize  int
	dropPolicy        string
}

// NewSubscribeHandler creates a new SSE subscription handler.
func NewSubscribeHandler(
	h *hub.Hub,
	a *auth.Authenticator,
	m *telemetry.Metrics,
	heartbeatInterval time.Duration,
	clientBufferSize int,
	dropPolicy string,
) *SubscribeHandler {
	if heartbeatInterval <= 0 {
		heartbeatInterval = 15 * time.Second
	}
	return &SubscribeHandler{
		hub:               h,
		auth:              a,
		metrics:           m,
		heartbeatInterval: heartbeatInterval,
		clientBufferSize:  clientBufferSize,
		dropPolicy:        dropPolicy,
	}
}

// ServeHTTP streams SSE events to the connecting HTTP client.
func (sh *SubscribeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported by underlying transport", http.StatusInternalServerError)
		return
	}

	// 1. Resolve requested channels
	requestedChannels := extractRequestedChannels(r)
	if len(requestedChannels) == 0 {
		http.Error(w, "Query parameter 'channel' or URL path channel required", http.StatusBadRequest)
		return
	}

	// 2. Validate authentication and channel access
	if err := sh.auth.ValidateSubscribeRequest(r, requestedChannels); err != nil {
		if err == auth.ErrForbiddenChannel {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else {
			http.Error(w, err.Error(), http.StatusUnauthorized)
		}
		return
	}

	// 3. Extract Last-Event-ID for replay recovery
	lastEventID := strings.TrimSpace(r.Header.Get("Last-Event-ID"))
	if lastEventID == "" {
		lastEventID = strings.TrimSpace(r.URL.Query().Get("last_event_id"))
	}

	// 4. Set standard SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Prevents NGINX from buffering SSE stream

	// 5. Initialize Client abstraction
	clientID := fmt.Sprintf("sub-%d-%06d", time.Now().UnixMilli(), atomic.AddUint64(&clientIDSeq, 1)%1000000)
	client := hub.NewClient(
		clientID,
		sh.clientBufferSize,
		sh.dropPolicy,
		r.RemoteAddr,
		r.UserAgent(),
	)

	// Register with Hub and replay missed messages
	sh.hub.RegisterClient(client, requestedChannels, lastEventID)
	if sh.metrics != nil {
		sh.metrics.ActiveConnections.Inc()
	}

	defer func() {
		sh.hub.UnregisterClient(client)
		if sh.metrics != nil {
			sh.metrics.ActiveConnections.Dec()
		}
	}()

	// Send initial greeting comment and flush
	fmt.Fprintf(w, ":connected client=%s channels=%s\n\n", clientID, strings.Join(requestedChannels, ","))
	flusher.Flush()

	// 6. Setup heartbeat ping ticker
	pingTicker := time.NewTicker(sh.heartbeatInterval)
	defer pingTicker.Stop()

	ctx := r.Context()

	// 7. Event delivery loop
	for {
		select {
		case <-ctx.Done():
			return

		case <-client.Done:
			return

		case event, ok := <-client.Send:
			if !ok {
				return
			}
			_, err := w.Write(event.FormatSSE())
			if err != nil {
				return
			}
			flusher.Flush()
			if sh.metrics != nil {
				sh.metrics.MessagesDelivered.WithLabelValues(event.Channel).Inc()
			}

		case <-pingTicker.C:
			_, err := w.Write(heartbeat.Frame)
			if err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func extractRequestedChannels(r *http.Request) []string {
	var channels []string

	// From path: /v1/subscribe/channels/{channels}
	path := r.URL.Path
	if idx := strings.Index(path, "/v1/subscribe/channels/"); idx != -1 {
		raw := strings.TrimPrefix(path, "/v1/subscribe/channels/")
		raw = strings.Trim(raw, "/")
		if raw != "" {
			for _, ch := range strings.Split(raw, ",") {
				ch = strings.TrimSpace(ch)
				if ch != "" {
					channels = append(channels, ch)
				}
			}
		}
	}

	// From query params: ?channel=a,b or ?channels=a,b
	for _, param := range []string{"channel", "channels"} {
		if q := r.URL.Query().Get(param); q != "" {
			for _, ch := range strings.Split(q, ",") {
				ch = strings.TrimSpace(ch)
				if ch != "" {
					channels = append(channels, ch)
				}
			}
		}
	}

	return deduplicateStrings(channels)
}

func deduplicateStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if _, exists := seen[item]; !exists {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}
