# streamgate — Architecture and Engineering Master Plan

> **High-Throughput HTTP Event-Stream (SSE) Multiplexer & Fan-Out Broadcast Gateway (< 45MB RAM)**  
> Production-grade alternative to Pusher, Ably, and Mercure in a single self-contained Go binary with an embedded React 19 telemetry console.

---

## 1. Executive Summary and Architecture Overview

### Problem Statement
Modern applications (AI token streaming, real-time analytics, financial tickers, live notifications, collaborative canvases) need persistent real-time message streams delivered to web and mobile clients.

Holding tens of thousands of long-lived HTTP/2 or HTTP/1.1 connections directly inside application backends (Python/FastAPI, Node.js, Ruby on Rails, Django, PHP) leads to severe failure modes:
1. **Connection Exhaustion and Memory Bloat**: Worker pools and file descriptors deplete rapidly, causing standard request/response endpoints to queue and time out.
2. **Expensive SaaS Lock-In**: Commercial hosted pub/sub services charge high recurring fees scaled on peak concurrent connections and monthly message volumes.
3. **Disconnections on Mobile Networks**: Cellular handovers and network dropouts sever streams. Without deterministic event ID replay buffers, clients permanently lose messages during reconnection windows.

### The Solution: streamgate
`streamgate` acts as a dedicated event-stream multiplexer and fan-out broadcast gateway. Backend services publish an event once through an internal HTTP POST endpoint, and `streamgate` broadcasts that event to tens of thousands of connected subscribers over Server-Sent Events (SSE) with sub-millisecond dispatch latency and under 45MB of memory usage.

```
+--------------------------------------------------------------------------+
| Backend Services (Go, Python, Node, Ruby, Java, Elixir, Rust)            |
+--------------------------------------------------------------------------+
                                    |
                                    | POST /api/v1/publish (Single or Batch)
                                    v
+--------------------------------------------------------------------------+
| streamgate Engine                                                        |
|                                                                          |
|  +--------------------------------+   +-------------------------------+  |
|  | Authentication & Rate Limiter  |-->| Event ID Generator            |  |
|  | - API Key / JWT Claims         |   | - Monotonic 64-bit Timestamp  |  |
|  +--------------------------------+   +---------------+---------------+  |
|                                                       |                  |
|                                                       v                  |
|  +--------------------------------------------------------------------+  |
|  | Per-Channel Circular Ring Buffer (1,024 to 65,536 items)           |  |
|  | - In-memory O(1) append                                            |  |
|  | - Zero-allocation slice retrieval for Last-Event-ID replay          |  |
|  +------------------------------------+-------------------------------+  |
|                                       |                                  |
|                                       | Broadcast Fan-Out                |
|                                       v                                  |
|  +--------------------------------------------------------------------+  |
|  | Sharded Subscriber Table (64 Stripes with RWMutex)                 |  |
|  | - Fast path reads with lock striping                               |  |
|  | - Bounded non-blocking client write queues (backpressure safe)     |  |
|  | - Automatic keep-alive heartbeat generator (:ping comments)        |  |
|  +------------------------------------+-------------------------------+  |
|                                       |                                  |
|         +-----------------------------+-----------------------------+    |
|         |                                                           |    |
|         v                                                           v    |
|  +-----------------------------+             +-----------------------------+
|  | Client Browser / App        |             | Client Browser / App        |
|  | GET /v1/subscribe?channel=x |             | GET /v1/subscribe?channel=y |
|  | (HTTP/2 or HTTP/1.1 SSE)    |             | (HTTP/2 or HTTP/1.1 SSE)    |
|  +-----------------------------+             +-----------------------------+
|                                                                          |
|  +--------------------------------------------------------------------+  |
|  | Embedded React 19 Operations Cockpit                               |  |
|  | - Real-time channel inspector, subscriber gauges, message charts   |  |
|  | - Interactive test publisher and live event tagger                 |  |
|  +--------------------------------------------------------------------+  |
+--------------------------------------------------------------------------+
```

---

## 2. Core Technical Specifications

| Component | Technical Choice | Rationale |
| :--- | :--- | :--- |
| **Language & Runtime** | Go 1.23+ | Lightweight goroutine scheduling (~2KB initial stack per connection), predictable garbage collection, zero external runtime dependencies. |
| **Transport Layer** | `net/http` + HTTP/2 | Native HTTP/2 multiplexing across a single TCP connection, fallback to HTTP/1.1 Chunked Transfer-Encoding for older clients. |
| **Fan-Out Dispatch** | 64-way Lock-Striped Registry | Reduces mutex contention across tens of thousands of subscribers during concurrent broadcast bursts. |
| **Replay Cache** | Circular Ring Buffer | Fixed-capacity array with modular indexing. Avoids dynamic allocations during steady-state broadcast operations. |
| **Backpressure Control** | Bounded Buffer with Drop/Disconnect Policy | Slow clients do not block the broadcaster goroutine or exhaust gateway memory. |
| **Authentication** | Bearer API Key & JWT | Secret API tokens protect publish endpoints; JWT tokens with channel wildcard claims validate subscriber connections. |
| **Telemetry & Metrics** | Prometheus Exporter + Internal SSE | Standard `/metrics` endpoint for Prometheus/Grafana, plus internal SSE channel streaming stats directly to the web console. |
| **Embedded Web Console** | React 19 + TypeScript + Vite | Built as static single-page application assets and embedded into the Go binary with `//go:embed`. |

---

## 3. Directory Layout

```
streamgate/
├── cmd/
│   ├── streamgate/
│   │   └── main.go                 # Daemon entrypoint, flags, config, signal handling
│   └── streamgate-bench/
│       └── main.go                 # High-concurrency load testing and benchmark CLI
├── internal/
│   ├── config/
│   │   └── config.go               # Configuration parser (flags, env vars, defaults)
│   ├── hub/
│   │   ├── ringbuffer.go           # Fixed-capacity circular ring buffer for event replay
│   │   ├── client.go               # Client connection abstraction with bounded write queue
│   │   ├── channel.go              # Single channel state and subscriber lock-striping
│   │   └── hub.go                  # Master channel manager, wildcard routing, global stats
│   ├── auth/
│   │   └── auth.go                 # API key and JWT channel permission verification
│   ├── heartbeat/
│   │   └── heartbeat.go            # Periodic keep-alive ping dispatcher
│   ├── telemetry/
│   │   └── metrics.go              # Prometheus metrics and real-time runtime statistics
│   └── api/
│       ├── router.go               # HTTP routing, CORS, security headers, web console serving
│       ├── subscribe.go            # GET /v1/subscribe SSE handler with Last-Event-ID replay
│       ├── publish.go              # POST /api/v1/publish single and batch ingestion
│       └── channels.go             # REST API for channel introspection and statistics
├── web/
│   ├── src/
│   │   ├── components/
│   │   │   ├── OverviewGauges.tsx  # Active clients, throughput, memory, channel count
│   │   │   ├── ChannelExplorer.tsx # Searchable, sortable channel cards with live stats
│   │   │   ├── LiveEventFeed.tsx   # Streaming console of events flowing through channels
│   │   │   ├── TestPublisher.tsx   # Interactive JSON event publisher with validation
│   │   │   └── ReplayTester.tsx    # Interactive Last-Event-ID reconnection tester
│   │   ├── types.ts                # TypeScript interfaces for events and metrics
│   │   ├── App.tsx                 # Root operations dashboard
│   │   ├── main.tsx                # React 19 bootstrap
│   │   └── index.css               # Dark theme Tailwind styling
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts
├── examples/
│   ├── browser.html                # Native browser EventSource demo
│   ├── node-publisher.js           # Node.js event publisher script
│   └── python-publisher.py         # Python event publisher script
├── Dockerfile                      # Multi-stage production container
├── docker-compose.yml              # Local testing with publisher service
├── Makefile                        # Build, test, lint, and benchmark automation
├── .gitignore
├── LICENSE
├── README.md
└── MASTER_PLAN.md
```

---

## 4. API and Protocol Specifications

### 4.1 Server-Sent Events Subscription
- **Path**: `GET /v1/subscribe` or `GET /v1/subscribe/channels/{channel}`
- **Query Parameters**:
  - `channel`: Comma-separated list of channel names (e.g. `crypto,stocks.btc,chat.general`).
  - `token`: Optional JWT token for private channel authorization.
  - `last_event_id`: Optional event ID to replay missed events (alternative to HTTP header).
- **HTTP Headers**:
  - `Last-Event-ID`: Previous event ID for replay recovery.
  - `Authorization: Bearer <jwt>`: Alternative way to pass authorization token.
- **Response**: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`.

### 4.2 Event Publication
- **Path**: `POST /api/v1/publish` or `POST /api/v1/publish/channels/{channel}`
- **Headers**: `Authorization: Bearer <publish_secret_key>`, `Content-Type: application/json`
- **Single Event Payload**:
```json
{
  "channel": "finance-trades",
  "event": "trade.executed",
  "data": {
    "symbol": "BTC/USD",
    "price": 94250.00,
    "amount": 0.45,
    "timestamp": 1730937600
  }
}
```
- **Batch Publication**: `POST /api/v1/publish/batch`
```json
{
  "events": [
    { "channel": "chat.lobby", "event": "message", "data": { "text": "hello" } },
    { "channel": "alerts", "event": "system", "data": { "level": "info" } }
  ]
}
```

### 4.3 Channel Introspection and Health
- `GET /api/v1/channels`: List active channels with subscriber counts and throughput.
- `GET /api/v1/stats`: Gateway operational statistics (RAM, goroutines, messages/sec, total delivered).
- `GET /metrics`: Standard Prometheus metrics.
- `GET /healthz`: Health and readiness check.

---

## 5. Detailed Implementation Tasks

### Phase 1: High-Performance Concurrency Core
- [x] Task 1.1: Initialize Git repository, configure topics, and register with portfolio index.
- [x] Task 1.2: Implement `internal/config` supporting command-line flags, environment variables, and safe defaults.
- [x] Task 1.3: Implement `internal/hub/ringbuffer.go` featuring monotonic event ID tracking, bounded capacity, thread-safe appends, and zero-allocation slice replay.
- [x] Task 1.4: Implement `internal/hub/client.go` with non-blocking write channels, backpressure handling, and graceful drop counters.
- [x] Task 1.5: Implement `internal/hub/channel.go` with lock-striped subscriber storage.
- [x] Task 1.6: Implement `internal/hub/hub.go` managing multi-channel subscription, wildcard prefix routing (`orders.*`), channel lifecycle, and metrics aggregation.
- [x] Task 1.7: Write comprehensive concurrent unit tests verifying race freedom (`go test -race`).

### Phase 2: HTTP SSE Streaming and Ingestion Engine
- [x] Task 2.1: Implement `internal/heartbeat/heartbeat.go` for periodic `:ping` comments preventing proxy idle timeouts.
- [x] Task 2.2: Implement `internal/auth/auth.go` verifying publish API keys and JWT subscriber signatures with wildcard channel claims.
- [x] Task 2.3: Implement `internal/telemetry/metrics.go` exporting Prometheus metrics and internal gateway state snapshots.
- [x] Task 2.4: Implement `internal/api/subscribe.go` handling HTTP/2 and HTTP/1.1 SSE client loops, Last-Event-ID recovery, and connection lifecycle events.
- [x] Task 2.5: Implement `internal/api/publish.go` handling single and batch event ingestion with input validation.
- [x] Task 2.6: Implement `internal/api/channels.go` for REST queries and channel introspection.
- [x] Task 2.7: Implement `internal/api/router.go` wiring CORS, recovery, security headers, and request routing.
- [x] Task 2.8: Write integration tests covering replay, heartbeat dispatch, and batch publication.

### Phase 3: Embedded React 19 Operations Cockpit
- [x] Task 3.1: Scaffold Vite + React 19 + TypeScript frontend in `web/` with Tailwind CSS and Lucide icons.
- [x] Task 3.2: Implement real-time SSE listener in the UI connecting to gateway telemetry.
- [x] Task 3.3: Implement `OverviewGauges.tsx` displaying real-time memory usage, active connections, messages per second, and channel counts.
- [x] Task 3.4: Implement `ChannelExplorer.tsx` with search, filtering, and channel detail cards.
- [x] Task 3.5: Implement `LiveEventFeed.tsx` with pause/resume, channel filtering, and event inspector drawer.
- [x] Task 3.6: Implement `TestPublisher.tsx` with interactive JSON payload composer and instant publish feedback.
- [x] Task 3.7: Implement `ReplayTester.tsx` allowing developers to test `Last-Event-ID` recovery directly inside the browser.
- [x] Task 3.8: Bundle frontend and wire into Go daemon using `//go:embed`.

### Phase 4: Benchmarking and Production Hardening
- [x] Task 4.1: Implement `cmd/streamgate-bench` load generator capable of maintaining thousands of concurrent SSE connections and measuring fan-out latency percentiles.
- [x] Task 4.2: Verify memory profile under 10,000+ simulated subscribers stays below 45MB RAM.
- [x] Task 4.3: Implement multi-stage Dockerfile and docker-compose demo setup.
- [x] Task 4.4: Author comprehensive humanized README and integration examples for Browser, Node.js, and Python.
- [x] Task 4.5: Release automated verification scripts and verify clean end-to-end functionality.
