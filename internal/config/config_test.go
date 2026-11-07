package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Port)
	}
	if cfg.RingBufferSize != 1024 {
		t.Errorf("expected ring buffer size 1024, got %d", cfg.RingBufferSize)
	}
	if cfg.ClientBufferSize != 256 {
		t.Errorf("expected client buffer size 256, got %d", cfg.ClientBufferSize)
	}
	if cfg.HeartbeatInterval != 15*time.Second {
		t.Errorf("expected heartbeat 15s, got %v", cfg.HeartbeatInterval)
	}
	if cfg.RetryInterval != 3*time.Second {
		t.Errorf("expected retry interval 3s, got %v", cfg.RetryInterval)
	}
	if !cfg.EnableWebUI {
		t.Errorf("expected EnableWebUI true")
	}
	if !cfg.EnableCompression {
		t.Errorf("expected EnableCompression true")
	}
}

func TestConfigLoadWithFlags(t *testing.T) {
	args := []string{
		"-port", "9090",
		"-host", "127.0.0.1",
		"-api-key", "test-secret",
		"-ring-size", "2048",
		"-client-buffer", "512",
		"-heartbeat", "30",
		"-retry-interval", "5",
		"-web-ui=false",
		"-drop-policy", "disconnect",
		"-webhook-url", "http://internal/hook",
		"-webhook-secret", "hook-key",
		"-compression=false",
		"-redis-addr", "127.0.0.1:6379",
	}

	cfg, err := Load(args)
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Port)
	}
	if cfg.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", cfg.Host)
	}
	if cfg.PublishAPIKey != "test-secret" {
		t.Errorf("expected api-key test-secret, got %s", cfg.PublishAPIKey)
	}
	if cfg.RingBufferSize != 2048 {
		t.Errorf("expected ring-size 2048, got %d", cfg.RingBufferSize)
	}
	if cfg.ClientBufferSize != 512 {
		t.Errorf("expected client-buffer 512, got %d", cfg.ClientBufferSize)
	}
	if cfg.HeartbeatInterval != 30*time.Second {
		t.Errorf("expected heartbeat 30s, got %v", cfg.HeartbeatInterval)
	}
	if cfg.RetryInterval != 5*time.Second {
		t.Errorf("expected retry-interval 5s, got %v", cfg.RetryInterval)
	}
	if cfg.EnableWebUI {
		t.Errorf("expected EnableWebUI false")
	}
	if cfg.DropPolicy != "disconnect" {
		t.Errorf("expected drop-policy disconnect, got %s", cfg.DropPolicy)
	}
	if cfg.WebhookURL != "http://internal/hook" {
		t.Errorf("expected webhook-url http://internal/hook, got %s", cfg.WebhookURL)
	}
	if cfg.EnableCompression {
		t.Errorf("expected compression false")
	}
	if cfg.RedisAddr != "127.0.0.1:6379" {
		t.Errorf("expected redis-addr 127.0.0.1:6379, got %s", cfg.RedisAddr)
	}
}

func TestConfigLoadWithEnv(t *testing.T) {
	os.Setenv("STREAMGATE_PORT", "7777")
	os.Setenv("STREAMGATE_API_KEY", "env-secret")
	os.Setenv("STREAMGATE_WEBHOOK_URL", "http://env-hook")
	defer func() {
		os.Unsetenv("STREAMGATE_PORT")
		os.Unsetenv("STREAMGATE_API_KEY")
		os.Unsetenv("STREAMGATE_WEBHOOK_URL")
	}()

	cfg, err := Load([]string{})
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.Port != 7777 {
		t.Errorf("expected port 7777 from env, got %d", cfg.Port)
	}
	if cfg.PublishAPIKey != "env-secret" {
		t.Errorf("expected api-key env-secret, got %s", cfg.PublishAPIKey)
	}
	if cfg.WebhookURL != "http://env-hook" {
		t.Errorf("expected webhook-url from env, got %s", cfg.WebhookURL)
	}
}
