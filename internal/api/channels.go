package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/alexandrmotologa/streamgate/internal/hub"
	"github.com/alexandrmotologa/streamgate/internal/telemetry"
)

// ChannelStatsHandler handles REST queries for channel metrics, presence members, and history.
type ChannelStatsHandler struct {
	hub *hub.Hub
}

// NewChannelStatsHandler creates a new channels API handler.
func NewChannelStatsHandler(h *hub.Hub) *ChannelStatsHandler {
	return &ChannelStatsHandler{hub: h}
}

// ServeHTTP routes channel list, single channel detail, presence members, and history queries.
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

	// Check if querying /members or /history
	parts := strings.Split(path, "/")
	channelName := parts[0]

	if len(parts) == 2 && parts[1] == "members" {
		csh.handleChannelMembers(w, channelName)
		return
	}

	if len(parts) == 2 && parts[1] == "history" {
		csh.handleChannelHistory(w, r, channelName)
		return
	}

	// Single channel detail
	ch := csh.hub.GetOrCreateChannel(channelName)
	subs, pub, del, lastPub := ch.Stats()
	members := ch.GetMembers()

	info := hub.ChannelInfo{
		Name:           ch.Name,
		Subscribers:    subs,
		TotalPublished: pub,
		TotalDelivered: del,
		BufferSize:     ch.Buffer.Size(),
		BufferCapacity: ch.Buffer.Capacity(),
		CreatedAt:      ch.CreatedAt,
		LastPublished:  lastPub,
		IsPresence:     ch.IsPresence(),
		MemberCount:    len(members),
	}
	_ = json.NewEncoder(w).Encode(info)
}

func (csh *ChannelStatsHandler) handleChannelMembers(w http.ResponseWriter, channelName string) {
	members := csh.hub.GetChannelMembers(channelName)
	if members == nil {
		members = []*hub.MemberInfo{}
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"channel":       channelName,
		"members_count": len(members),
		"members":       members,
	})
}

func (csh *ChannelStatsHandler) handleChannelHistory(w http.ResponseWriter, r *http.Request, channelName string) {
	limit := 50
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}
	since := r.URL.Query().Get("since")

	events := csh.hub.GetChannelHistory(channelName, limit, since)
	if events == nil {
		events = []*hub.Event{}
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"channel": channelName,
		"count":   len(events),
		"events":  events,
	})
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
