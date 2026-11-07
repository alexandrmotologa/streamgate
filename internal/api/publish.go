package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/alexandrmotologa/streamgate/internal/auth"
	"github.com/alexandrmotologa/streamgate/internal/hub"
	"github.com/alexandrmotologa/streamgate/internal/telemetry"
)

// PublishHandler handles HTTP event ingestion.
type PublishHandler struct {
	hub     *hub.Hub
	auth    *auth.Authenticator
	metrics *telemetry.Metrics
}

// NewPublishHandler creates a new publish handler.
func NewPublishHandler(h *hub.Hub, a *auth.Authenticator, m *telemetry.Metrics) *PublishHandler {
	return &PublishHandler{
		hub:     h,
		auth:    a,
		metrics: m,
	}
}

// PublishRequest represents a single publish request body.
type PublishRequest struct {
	Channel string          `json:"channel"`
	Event   string          `json:"event"`
	Data    json.RawMessage `json:"data"`
}

// PublishResponse contains execution feedback for a published event.
type PublishResponse struct {
	Success     bool   `json:"success"`
	ID          string `json:"id"`
	Channel     string `json:"channel"`
	Event       string `json:"event"`
	DeliveredTo int    `json:"delivered_to"`
}

// BatchPublishRequest represents multiple events ingested simultaneously.
type BatchPublishRequest struct {
	Events []PublishRequest `json:"events"`
}

// BatchPublishResponse reports bulk delivery status.
type BatchPublishResponse struct {
	Success        bool              `json:"success"`
	TotalEvents    int               `json:"total_events"`
	TotalDelivered int               `json:"total_delivered"`
	Results        []PublishResponse `json:"results"`
}

// ServeHTTP routes single vs batch publish requests.
func (ph *PublishHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed, use POST", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()
	defer func() {
		if ph.metrics != nil {
			ph.metrics.PublishDuration.Observe(time.Since(start).Seconds())
		}
	}()

	// Validate publisher credentials
	if err := ph.auth.ValidatePublishRequest(r); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	path := r.URL.Path
	if strings.HasSuffix(path, "/batch") {
		ph.handleBatchPublish(w, r)
		return
	}

	ph.handleSinglePublish(w, r)
}

func (ph *PublishHandler) handleSinglePublish(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024*1024*4)) // 4MB payload limit
	if err != nil {
		http.Error(w, "Payload too large or unreadable", http.StatusBadRequest)
		return
	}

	var req PublishRequest
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "Invalid JSON payload: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Channel can also be extracted from URL: /api/v1/publish/channels/{channel}
	if req.Channel == "" {
		if idx := strings.Index(r.URL.Path, "/api/v1/publish/channels/"); idx != -1 {
			chFromPath := strings.TrimPrefix(r.URL.Path, "/api/v1/publish/channels/")
			chFromPath = strings.Trim(chFromPath, "/")
			if chFromPath != "" {
				req.Channel = chFromPath
			}
		}
	}

	if req.Channel == "" {
		http.Error(w, "Field 'channel' is required in JSON body or URL path", http.StatusBadRequest)
		return
	}

	if req.Event == "" {
		req.Event = "message"
	}

	if len(req.Data) == 0 {
		req.Data = json.RawMessage(`{}`)
	}

	ev, delivered := ph.hub.Publish(req.Channel, req.Event, req.Data)
	if ph.metrics != nil {
		ph.metrics.MessagesPublished.WithLabelValues(req.Channel).Inc()
	}

	resp := PublishResponse{
		Success:     true,
		ID:          ev.ID,
		Channel:     req.Channel,
		Event:       req.Event,
		DeliveredTo: delivered,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (ph *PublishHandler) handleBatchPublish(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024*1024*16)) // 16MB batch limit
	if err != nil {
		http.Error(w, "Payload too large or unreadable", http.StatusBadRequest)
		return
	}

	var req BatchPublishRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON batch payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.Events) == 0 {
		http.Error(w, "Batch array 'events' cannot be empty", http.StatusBadRequest)
		return
	}

	results := make([]PublishResponse, 0, len(req.Events))
	totalDelivered := 0

	for _, item := range req.Events {
		if item.Channel == "" {
			continue
		}
		if item.Event == "" {
			item.Event = "message"
		}
		if len(item.Data) == 0 {
			item.Data = json.RawMessage(`{}`)
		}

		ev, delivered := ph.hub.Publish(item.Channel, item.Event, item.Data)
		if ph.metrics != nil {
			ph.metrics.MessagesPublished.WithLabelValues(item.Channel).Inc()
		}

		totalDelivered += delivered
		results = append(results, PublishResponse{
			Success:     true,
			ID:          ev.ID,
			Channel:     item.Channel,
			Event:       item.Event,
			DeliveredTo: delivered,
		})
	}

	resp := BatchPublishResponse{
		Success:        true,
		TotalEvents:    len(results),
		TotalDelivered: totalDelivered,
		Results:        results,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
