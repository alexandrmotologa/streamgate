package api

import (
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/alexandrmotologa/streamgate/internal/auth"
	"github.com/alexandrmotologa/streamgate/internal/config"
	"github.com/alexandrmotologa/streamgate/internal/hub"
	"github.com/alexandrmotologa/streamgate/internal/telemetry"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Router sets up HTTP route dispatch and middleware.
type Router struct {
	cfg            *config.Config
	hub            *hub.Hub
	auth           *auth.Authenticator
	metrics        *telemetry.Metrics
	webFS          fs.FS
	mux            *http.ServeMux
	handlerWithMid http.Handler
}

// NewRouter constructs the API router and mounts endpoints.
func NewRouter(cfg *config.Config, h *hub.Hub, a *auth.Authenticator, m *telemetry.Metrics, webFS fs.FS) *Router {
	mux := http.NewServeMux()

	r := &Router{
		cfg:     cfg,
		hub:     h,
		auth:    a,
		metrics: m,
		webFS:   webFS,
		mux:     mux,
	}

	r.registerRoutes()
	r.handlerWithMid = r.applyMiddleware(mux)

	return r
}

func (r *Router) registerRoutes() {
	// 1. Health & Prometheus metrics
	r.mux.HandleFunc("/healthz", HealthHandler)
	if r.metrics != nil && r.metrics.Registry != nil {
		r.mux.Handle("/metrics", promhttp.HandlerFor(r.metrics.Registry, promhttp.HandlerOpts{}))
	} else {
		r.mux.Handle("/metrics", promhttp.Handler())
	}

	// 2. Event publishing
	publishHandler := NewPublishHandler(r.hub, r.auth, r.metrics)
	r.mux.Handle("/api/v1/publish", publishHandler)
	r.mux.Handle("/api/v1/publish/", publishHandler)

	// 3. Channel statistics and introspection
	channelStatsHandler := NewChannelStatsHandler(r.hub)
	r.mux.Handle("/api/v1/channels", channelStatsHandler)
	r.mux.Handle("/api/v1/channels/", channelStatsHandler)

	gatewayStatsHandler := NewGatewayStatsHandler(r.hub)
	r.mux.Handle("/api/v1/stats", gatewayStatsHandler)

	// 4. SSE Subscription stream
	subscribeHandler := NewSubscribeHandler(
		r.hub,
		r.auth,
		r.metrics,
		r.cfg.HeartbeatInterval,
		r.cfg.ClientBufferSize,
		r.cfg.DropPolicy,
	)
	r.mux.Handle("/v1/subscribe", subscribeHandler)
	r.mux.Handle("/v1/subscribe/", subscribeHandler)

	// 5. Embedded Web UI (React 19)
	if r.cfg.EnableWebUI && r.webFS != nil {
		r.mux.Handle("/", r.spaFileServer(r.webFS))
	}
}

// spaFileServer serves static files with SPA index.html fallback for client routing.
func (r *Router) spaFileServer(fileSys fs.FS) http.Handler {
	httpFS := http.FS(fileSys)
	fileServer := http.FileServer(httpFS)

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		path := strings.TrimPrefix(req.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Try opening the requested file
		f, err := fileSys.Open(path)
		if err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, req)
			return
		}

		// If file doesn't exist and not an API call, serve index.html (SPA fallback)
		if !strings.HasPrefix(req.URL.Path, "/api/") && !strings.HasPrefix(req.URL.Path, "/v1/") {
			indexFile, err := fileSys.Open("index.html")
			if err == nil {
				defer indexFile.Close()
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = io.Copy(w, indexFile)
				return
			}
		}

		fileServer.ServeHTTP(w, req)
	})
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.handlerWithMid.ServeHTTP(w, req)
}

func (r *Router) applyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// CORS Headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Last-Event-ID, X-API-Key, X-Auth-Token")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Type, Last-Event-ID")

		// Security Headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")

		// Handle OPTIONS preflight
		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, req)
	})
}
