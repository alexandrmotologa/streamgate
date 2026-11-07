package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// EventType represents the lifecycle event triggered by channel state change.
type EventType string

const (
	EventChannelOccupied EventType = "channel_occupied"
	EventChannelVacated  EventType = "channel_vacated"
)

// Payload represents the webhook notification body.
type Payload struct {
	Event     EventType `json:"event"`
	Channel   string    `json:"channel"`
	Timestamp int64     `json:"timestamp"`
}

// Dispatcher manages outbound HTTP webhook notifications asynchronously.
type Dispatcher struct {
	url        string
	secret     string
	httpClient *http.Client
	queue      chan Payload
	done       chan struct{}
}

// NewDispatcher creates a non-blocking webhook dispatcher.
func NewDispatcher(url, secret string) *Dispatcher {
	if url == "" {
		return nil
	}

	d := &Dispatcher{
		url:    url,
		secret: secret,
		httpClient: &http.Client{
			Timeout: 4 * time.Second,
		},
		queue: make(chan Payload, 512),
		done:  make(chan struct{}),
	}

	go d.worker()
	return d
}

// Trigger queues a lifecycle event for asynchronous dispatch.
func (d *Dispatcher) Trigger(event EventType, channel string) {
	if d == nil {
		return
	}

	payload := Payload{
		Event:     event,
		Channel:   channel,
		Timestamp: time.Now().Unix(),
	}

	select {
	case d.queue <- payload:
	default:
		log.Printf("[WARN] Webhook queue full, dropping event: %s for %s", event, channel)
	}
}

// Close gracefully stops the background dispatcher.
func (d *Dispatcher) Close() {
	if d == nil {
		return
	}
	close(d.done)
}

func (d *Dispatcher) worker() {
	for {
		select {
		case <-d.done:
			return
		case item := <-d.queue:
			d.send(item)
		}
	}
}

func (d *Dispatcher) send(p Payload) {
	data, err := json.Marshal(p)
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodPost, d.url, bytes.NewReader(data))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "streamgate-webhook/1.0")

	// Optional HMAC-SHA256 signature
	if d.secret != "" {
		mac := hmac.New(sha256.New, []byte(d.secret))
		mac.Write(data)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Streamgate-Signature", "sha256="+sig)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		log.Printf("[WARN] Webhook dispatch failed to %s: %v", d.url, err)
		return
	}
	_ = resp.Body.Close()
}
