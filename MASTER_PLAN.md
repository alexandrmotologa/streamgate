# `streamgate` — Architecture & Engineering Master Plan

> **High-Throughput HTTP Event-Stream (SSE) Multiplexer & Fan-Out Broadcast Gateway (< 45MB RAM)**  
> *Pusher, Ably, and Mercure Alternative in a Single Binary (Go + React 19)*

---

## 1. Executive Summary & Problem Space

### The Core Problem
The rapid rise of AI applications (streaming token-by-token LLM completions), real-time financial dashboards, live sports tickers, and collaborative web software demands persistent, real-time data streaming to client browsers.

However, holding tens of thousands of long-lived HTTP connections directly inside traditional application backends (Python/FastAPI, Node.js, Ruby on Rails, Django, PHP) causes severe architectural problems:
1. **Connection Exhaustion & Memory Bloat**: Web servers run out of worker threads or file descriptors, causing incoming API requests to queue and time out.
2. **Expensive SaaS Lock-In (Pusher, Ably)**: Commercial cloud pub/sub platforms charge exorbitant fees (\$99–\$999+/month) based on peak concurrent connections and monthly message quotas.
3. **Connection Drops on Mobile Networks**: Cellular network handovers and subway tunnels frequently break connections. Without an automated `Last-Event-ID` replay buffer, users miss critical events during reconnections.

### The Solution: `streamgate`
`streamgate` is an ultra-fast, single-binary event-stream multiplexer written in Go with an embedded React 19 operations cockpit. It sits between client browsers and your backend services. Your backend publishes an event once via an internal HTTP POST or Redis Pub/Sub message, and `streamgate` fans out that single event to 50,000+ connected clients simultaneously over Server-Sent Events (SSE) on HTTP/2 or HTTP/1.1 with sub-millisecond latencies and less than 45MB of idle RAM.

---

## 2. System Architecture & Fan-Out Pipeline

```
[ Backend Application (Python, Node, Go, Ruby) ]
                       │
                       │  POST /api/v1/publish/channels/{channel_id}
                       ▼
┌────────────────────────────────────────────────────────────────────────┐
│                        `streamgate` Gateway Engine                     │
│                                                                        │
│  ┌───────────────────────┐         ┌────────────────────────────────┐  │
│  │ Channel Auth & JWT    │ ──────► │ Ingestion Buffer & Event ID    │  │
│  │ Permission Verifier   │         │ Generator (ULID / Monotonic)   │  │
│  └───────────────────────┘         └────────────────┬───────────────┘  │
│                                                     │                  │
│                                                     ▼                  │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │ Circular Memory Ring Buffer (Per-Channel Replay Cache, 1k items) │  │
│  │ - Instant fast-forward replay for reconnecting `Last-Event-ID`   │  │
│  └──────────────────────────────────┬───────────────────────────────┘  │
│                                     │ Zero-Allocation Broadcast        │
│                                     ▼                                  │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │ Lock-Free Sharded Subscriber Table (sync.Map / Lock-Striping)    │  │
│  │ - 50,000+ Goroutine-backed client streams                        │  │
│  │ - Periodic Keep-Alive Ping Generator (`:keepalive\n\n`)          │  │
│  │ - Adaptive Gzip / Deflate Stream Compression                     │  │
│  └──────────────────────────────────┬───────────────────────────────┘  │
│                                     │                                  │
│         ┌───────────────────────────┴──────────────────────────┐       │
│         ▼                                                      ▼       │
│  [ Client Browsers / Apps ]                          [ Client Browsers ]
│  GET /v1/subscribe/channels/ai-chat                  GET /v1/subscribe/channels/crypto
│  (HTTP/2 & HTTP/1.1 SSE Stream)                      (HTTP/2 & HTTP/1.1 SSE Stream)
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │ Embedded React 19 Operations Cockpit                             │  │
│  │ - Real-time channel inspector, subscriber gauges, message chart  │  │
│  │ - Interactive Test Publisher & Live Event Tagger                 │  │
│  └──────────────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Technology Stack

| Layer | Component | Specification |
| :--- | :--- | :--- |
| **Language & Runtime** | Go 1.23+ | Native concurrency with lightweight goroutines (2KB stack each), zero CGo |
| **HTTP Transport** | `net/http` + HTTP/2 | Native HTTP/2 multiplexing allowing multiple SSE channels over a single TCP connection |
| **Connection Fan-Out** | Lock-Free Sharded Table | Lock-striped subscriber dispatch achieving > 100,000 msgs/sec fan-out |
| **Replay Cache** | Circular Ring Buffer | Per-channel in-memory ring buffer (1,000 events) for `Last-Event-ID` reconnection recovery |
| **Authorization** | JWT & HMAC Tokens | Validates channel read/write permissions via standard JWT claims or presigned tokens |
| **Embedded Web UI** | React 19 + TypeScript + Vite | Built as a single bundle and served via Go `//go:embed` |
| **UI Styling** | Tailwind CSS + Lucide Icons | High-density dark-mode real-time operations console |

---

## 4. Repository & Directory Structure

```
B:\workgit\streamgate\
├── cmd\
│   └── streamgate\
│       └── main.go                 # Gateway daemon entrypoint, CLI flags, graceful shutdown
├── internal\
│   ├── config\
│   │   └── config.go               # Configuration (ports, buffer sizes, JWT secret, CORS)
│   ├── api\
│   │   ├── router.go               # HTTP route setup, CORS middleware, gzip negotiation
│   │   ├── publish.go              # POST /api/v1/publish handler
│   │   ├── subscribe.go            # GET /v1/subscribe/:channel SSE stream handler
│   │   ├── channels.go             # Channel listing and subscriber telemetry endpoints
│   │   └── auth.go                 # JWT validation and channel permission checks
│   ├── hub\
│   │   ├── hub.go                  # Master channel manager and sharded registry
│   │   ├── channel.go              # Single channel state, subscriber list, and lock-striping
│   │   ├── ringbuffer.go           # High-speed circular ring buffer for event replay
│   │   └── client.go               # Client connection abstraction with backpressure handling
│   ├── heartbeat\
│   │   └── ping.go                 # Background keep-alive comment dispatcher (every 15s)
│   └── telemetry\
│       └── metrics.go              # Prometheus metrics (subscribers, msg/sec, drops, latency)
├── web\
│   ├── src\
│   │   ├── components\
│   │   │   ├── ChannelCard.tsx     # Channel overview card: active clients, throughput
│   │   │   ├── LiveEventFeed.tsx   # Real-time streaming log of broadcast events
│   │   │   ├── TestPublisher.tsx   # Interactive GUI to publish JSON events directly
│   │   │   └── MetricsChart.tsx    # Live canvas chart of messages/sec and active sockets
│   │   ├── App.tsx                 # Root streaming cockpit
│   │   ├── main.tsx                # React 19 entrypoint
│   │   └── index.css               # Tailwind CSS declarations
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts
├── Makefile                        # Multi-platform compilation and benchmark suites
├── Dockerfile                      # Scratch container image (< 20MB)
├── README.md                       # Comprehensive documentation and integration guide
└── MASTER_PLAN.md                  # This master specification
```

---

## 5. API Specification & Client Integration

### 1. Publishing an Event (Backend to `streamgate`)

```bash
POST /api/v1/publish/channels/finance-trades
Authorization: Bearer <publish_secret_key>
Content-Type: application/json

{
  "event": "trade.executed",
  "data": {
    "symbol": "BTC/USD",
    "price": 94250.00,
    "amount": 0.45,
    "timestamp": 1730937600
  }
}
```

### 2. Subscribing to an Event Stream (Browser to `streamgate`)

```javascript
// Native browser EventSource with automatic reconnection
const eventSource = new EventSource("https://stream.example.com/v1/subscribe/channels/finance-trades?token=eyJhbGciOi...");

eventSource.addEventListener("trade.executed", (event) => {
  const trade = JSON.parse(event.data);
  console.log("New trade received:", trade);
});

eventSource.onerror = (err) => {
  console.warn("Connection lost. Browser will auto-reconnect with Last-Event-ID header...");
};
```

---

## 6. Implementation Roadmap

### Phase 1: Core Fan-Out Engine & Ring Buffer
- [ ] Initialize Go module `github.com/alexandrmotologa/streamgate`.
- [ ] Implement `internal/hub/ringbuffer.go` (circular memory buffer with monotonic event IDs).
- [ ] Implement `internal/hub/channel.go` with lock-striped subscriber maps.
- [ ] Unit test concurrent broadcast under heavy load with zero race conditions (`go test -race`).

### Phase 2: HTTP SSE Server & Connection Lifecycle
- [ ] Implement `internal/api/subscribe.go` with HTTP/2 and HTTP/1.1 SSE streaming.
- [ ] Support `Last-Event-ID` header: replay missed events from ring buffer on reconnection.
- [ ] Implement background keep-alive ping generator (`:keepalive\n\n` every 15 seconds).
- [ ] Implement client backpressure detection (drop or disconnect slow clients before RAM exhausts).

### Phase 3: Ingestion API & Channel Authorization
- [ ] Implement `internal/api/publish.go` supporting single and batch event publication.
- [ ] Implement JWT token validation allowing fine-grained read/write channel wildcards (`chat.*`).
- [ ] Implement Prometheus metrics exporter (`/metrics`).

### Phase 4: Embedded React 19 Operations Cockpit
- [ ] Scaffold Vite + React 19 frontend in `web/`.
- [ ] Build live channel explorer showing real-time subscriber counts and bandwidth.
- [ ] Build interactive test event publisher with syntax-highlighted JSON editor.
- [ ] Embed static assets directly into the Go executable via `//go:embed`.

### Phase 5: Production Hardening, Benchmarking & Documentation
- [ ] Benchmark fan-out throughput (target: 50,000 connected clients with < 40MB RAM).
- [ ] Write integration guide for React, Vue, Next.js, Python, and Node.js.
- [ ] Release single-binary binaries for Linux, macOS, and Windows.
