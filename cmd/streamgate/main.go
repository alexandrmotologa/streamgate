package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexandrmotologa/streamgate/internal/auth"
	"github.com/alexandrmotologa/streamgate/internal/config"
	"github.com/alexandrmotologa/streamgate/internal/heartbeat"
	"github.com/alexandrmotologa/streamgate/internal/hub"
	"github.com/alexandrmotologa/streamgate/internal/telemetry"
	"github.com/alexandrmotologa/streamgate/internal/api"
	"github.com/alexandrmotologa/streamgate/web"
)

const version = "1.0.0"

func main() {
	// Quick check for version flag
	for _, arg := range os.Args[1:] {
		if arg == "-version" || arg == "--version" || arg == "-v" {
			fmt.Printf("streamgate v%s (Go %s)\n", version, "1.23+")
			os.Exit(0)
		}
	}

	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		log.Fatalf("[FATAL] Configuration error: %v", err)
	}

	// 1. Initialize Core Concurrency Engine
	hubInstance := hub.NewHub(cfg.RingBufferSize, cfg.ClientBufferSize, cfg.DropPolicy)

	// 2. Initialize Auth & Telemetry
	authInstance := auth.NewAuthenticator(cfg.PublishAPIKey, cfg.JWTSecret)
	metricsInstance := telemetry.NewMetrics(nil)

	// 3. Start Heartbeat & Live Telemetry Dispatcher
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	heartbeatService := heartbeat.NewService(hubInstance, time.Second)
	go heartbeatService.Start(ctx)

	// 4. Resolve Web Assets
	webFS, err := web.Assets()
	if err != nil {
		log.Printf("[WARN] Failed to mount embedded web UI assets: %v", err)
	}

	// 5. Build Router
	router := api.NewRouter(cfg, hubInstance, authInstance, metricsInstance, webFS)

	// 6. HTTP Server Setup
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	server := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		// Note: WriteTimeout is deliberately omitted because Server-Sent Events streams are long-lived!
	}

	// 7. Banner output
	fmt.Println()
	fmt.Println("  ┌──────────────────────────────────────────────────────────┐")
	fmt.Printf("  │  streamgate v%-44s│\n", version)
	fmt.Println("  │  High-Throughput HTTP Event-Stream (SSE) Gateway         │")
	fmt.Println("  │  Memory Target: < 45MB RAM | Zero-Allocation Fan-Out     │")
	fmt.Println("  ├──────────────────────────────────────────────────────────┤")
	fmt.Printf("  │  HTTP Server:     http://%s%-27s│\n", addr, " ")
	fmt.Printf("  │  Web Cockpit:     http://localhost:%-24d│\n", cfg.Port)
	fmt.Printf("  │  SSE Stream:      GET  /v1/subscribe?channel=...        │\n")
	fmt.Printf("  │  Event Ingestion: POST /api/v1/publish                   │\n")
	fmt.Printf("  │  Prometheus:      GET  /metrics                          │\n")
	fmt.Println("  └──────────────────────────────────────────────────────────┘")
	fmt.Println()

	// 8. Graceful Shutdown Listener
	shutdownSig := make(chan os.Signal, 1)
	signal.Notify(shutdownSig, os.Interrupt, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("[INFO] Gateway listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		log.Fatalf("[FATAL] Server startup failed: %v", err)
	case sig := <-shutdownSig:
		log.Printf("[INFO] Received signal %v, initiating graceful shutdown...", sig)
	}

	// Graceful shutdown with 5s timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] Server shutdown error: %v", err)
	} else {
		log.Printf("[INFO] Gateway stopped cleanly.")
	}
}
