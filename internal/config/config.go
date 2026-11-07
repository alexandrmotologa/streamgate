package config

import (
	"flag"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime configuration options for the streamgate daemon.
type Config struct {
	Host              string
	Port              int
	PublishAPIKey     string
	JWTSecret         string
	RingBufferSize    int
	ClientBufferSize  int
	HeartbeatInterval time.Duration
	AllowedOrigins    []string
	EnableWebUI       bool
	DropPolicy        string // "drop_oldest", "drop_newest", "disconnect"
}

// DefaultConfig returns reasonable production-ready defaults.
func DefaultConfig() *Config {
	return &Config{
		Host:              "0.0.0.0",
		Port:              8080,
		PublishAPIKey:     "",
		JWTSecret:         "",
		RingBufferSize:    1024,
		ClientBufferSize:  256,
		HeartbeatInterval: 15 * time.Second,
		AllowedOrigins:    []string{"*"},
		EnableWebUI:       true,
		DropPolicy:        "drop_oldest",
	}
}

// Load populates configuration from defaults, environment variables, and CLI flags.
func Load(args []string) (*Config, error) {
	cfg := DefaultConfig()

	// Environment overrides
	if host := os.Getenv("STREAMGATE_HOST"); host != "" {
		cfg.Host = host
	}
	if portStr := os.Getenv("STREAMGATE_PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
			cfg.Port = p
		}
	}
	if key := os.Getenv("STREAMGATE_API_KEY"); key != "" {
		cfg.PublishAPIKey = key
	}
	if secret := os.Getenv("STREAMGATE_JWT_SECRET"); secret != "" {
		cfg.JWTSecret = secret
	}
	if rbs := os.Getenv("STREAMGATE_RING_BUFFER_SIZE"); rbs != "" {
		if n, err := strconv.Atoi(rbs); err == nil && n > 0 {
			cfg.RingBufferSize = n
		}
	}
	if cbs := os.Getenv("STREAMGATE_CLIENT_BUFFER_SIZE"); cbs != "" {
		if n, err := strconv.Atoi(cbs); err == nil && n > 0 {
			cfg.ClientBufferSize = n
		}
	}
	if hb := os.Getenv("STREAMGATE_HEARTBEAT_INTERVAL"); hb != "" {
		if sec, err := strconv.Atoi(hb); err == nil && sec > 0 {
			cfg.HeartbeatInterval = time.Duration(sec) * time.Second
		}
	}
	if origins := os.Getenv("STREAMGATE_ALLOWED_ORIGINS"); origins != "" {
		cfg.AllowedOrigins = strings.Split(origins, ",")
	}
	if web := os.Getenv("STREAMGATE_ENABLE_WEB_UI"); web != "" {
		cfg.EnableWebUI = web == "true" || web == "1"
	}
	if dp := os.Getenv("STREAMGATE_DROP_POLICY"); dp != "" {
		cfg.DropPolicy = dp
	}

	// Command line flags override environment variables
	fs := flag.NewFlagSet("streamgate", flag.ContinueOnError)
	fs.StringVar(&cfg.Host, "host", cfg.Host, "Host address to bind HTTP server")
	fs.IntVar(&cfg.Port, "port", cfg.Port, "Port to listen on")
	fs.StringVar(&cfg.PublishAPIKey, "api-key", cfg.PublishAPIKey, "API key required for publishing events")
	fs.StringVar(&cfg.JWTSecret, "jwt-secret", cfg.JWTSecret, "Secret for validating subscriber JWT tokens")
	fs.IntVar(&cfg.RingBufferSize, "ring-size", cfg.RingBufferSize, "Per-channel ring buffer replay capacity")
	fs.IntVar(&cfg.ClientBufferSize, "client-buffer", cfg.ClientBufferSize, "Per-client connection message buffer")
	var heartbeatSec int
	fs.IntVar(&heartbeatSec, "heartbeat", int(cfg.HeartbeatInterval.Seconds()), "Keep-alive ping interval in seconds")
	fs.BoolVar(&cfg.EnableWebUI, "web-ui", cfg.EnableWebUI, "Enable embedded React 19 web console")
	fs.StringVar(&cfg.DropPolicy, "drop-policy", cfg.DropPolicy, "Backpressure policy: drop_oldest, drop_newest, disconnect")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if heartbeatSec > 0 {
		cfg.HeartbeatInterval = time.Duration(heartbeatSec) * time.Second
	}

	return cfg, nil
}
