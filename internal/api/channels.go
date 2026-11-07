package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/alexandrmotologa/streamgate/internal/hub"
	"github.com/alexandrmotologa/streamgate/internal/telemetry"
)

// ChannelStatsHandler handles REST queries for channel metrics.
type ChannelStatsHandler struct {
	hub *hub.Hub
}

// NewChannelStatsHandler creates a new channels API handler.
func NewChannelStatsHandler(h *hub.Hub) *ChannelStatsHandler {
	return &ChannelStatsHandler{hub: h}
}

// ServeHTTP routes channel list and single channel detail queries.
func (csh *ChannelStatsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/channels")
	path = strings.Trim(path, "/")

	w.Header().Set("Content-Type", "application/json")

	if path == "" {
		// List all channels
		channels := csh.hub.ListChannels()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total":    len(channels),
			"channels": channels,
		})
		return
	}

	// Single channel detail
	ch := csh.hub.GetOrCreateChannel(path)
	subs, pub, del, lastPub := ch.Stats()
	info := hub.ChannelInfo{
		Name:           ch.Name,
		Subscribers:    subs,
		TotalPublished: pub,
		TotalDelivered: del,
		BufferSize:     ch.Buffer.Size(),
		BufferCapacity: ch.Buffer.Capacity(),
		CreatedAt:      ch.CreatedAt,
		LastPublished:  lastPub,
	}
	_ = json.NewEncoder(w).Encode(info)
}

// GatewayStatsHandler returns overall operational statistics and runtime telemetry.
type GatewayStatsHandler struct {
	hub *hub.Hub
}

// NewGatewayStatsHandler creates a new stats handler.
func NewGatewayStatsHandler(h *hub.Hub) *GatewayStatsHandler {
	return &GatewayStatsHandler{hub: h}
}

func (gsh *GatewayStatsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := gsh.hub.GetStats()
	rt := telemetry.GetRuntimeStats()

	resp := map[string]any{
		"gateway": stats,
		"runtime": rt,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// HealthHandler returns a clean healthz response.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "healthy",
		"service": "streamgate",
	})
}
