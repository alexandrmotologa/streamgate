<p align="center">
  <a href="https://github.com/alexandrmotologa/streamgate">
    <img src="docs/images/logo.png?raw=true" alt="streamgate Logo" width="130" style="border-radius: 24px;" />
  </a>
</p>

<h1 align="center">streamgate</h1>

<p align="center">
  <strong>High-Throughput HTTP Server-Sent Events (SSE) Multiplexer and Fan-Out Broadcast Gateway</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go Version" />
  <img src="https://img.shields.io/badge/React-19-61DAFB?style=flat-square&logo=react&logoColor=black" alt="React 19" />
  <img src="https://img.shields.io/badge/RAM_Budget-%3C_45_MB-10b981?style=flat-square" alt="Memory Target" />
  <img src="https://img.shields.io/badge/License-MIT-blue?style=flat-square" alt="License" />
</p>

<p align="center">
  <img src="docs/images/streamgate_demo.gif?raw=true" alt="streamgate Operations Cockpit Demo" width="100%" style="border-radius: 12px; border: 1px solid #334155;" />
</p>

streamgate distributes real-time events to concurrent subscribers over HTTP/2 and HTTP/1.1 from a single backend HTTP POST request. It includes per-channel in-memory circular ring buffers for automatic `Last-Event-ID` reconnection recovery, lock-striped subscriber tables, presence member tracking, channel lifecycle webhooks, adaptive stream compression, Redis pub/sub clustering, and an operations dashboard running in less than 45MB of RAM.

---

## Operations Cockpit & Visual Tour

streamgate includes an embedded single-page operations dashboard built in React 19, TypeScript, and Tailwind CSS. The cockpit communicates with the Go gateway over its own real-time SSE stream and management endpoints.

<table width="100%">
  <tr>
    <td width="50%" valign="top">
      <strong>Real-Time Overview & Metrics Gauges</strong><br />
      Live subscriber counters, message delivery rates, memory allocation budget tracker, and channel topology.
      <br /><br />
      <img src="docs/images/streamgate_overview.png?raw=true" alt="Overview Gauges" width="100%" style="border-radius: 8px; border: 1px solid #334155;" />
    </td>
    <td width="50%" valign="top">
      <strong>Live Event Stream Inspector</strong><br />
      Live streaming feed with sub-millisecond arrival stamps, search filtering, and JSON payload inspection drawer.
      <br /><br />
      <img src="docs/images/streamgate_live_feed.png?raw=true" alt="Live Event Stream Inspector" width="100%" style="border-radius: 8px; border: 1px solid #334155;" />
    </td>
  </tr>
  <tr>
    <td width="50%" valign="top">
      <strong>Interactive Test Publisher</strong><br />
      Send mock events with one-click presets (trades, AI tokens, health alerts, chat) and immediate fan-out delivery verification.
      <br /><br />
      <img src="docs/images/streamgate_publisher.png?raw=true" alt="Test Publisher" width="100%" style="border-radius: 8px; border: 1px solid #334155;" />
    </td>
    <td width="50%" valign="top">
      <strong>Presence Members Modal</strong><br />
      Track active subscribers, user identifiers, custom JSON metadata, and join timestamps across presence channels.
      <br /><br />
      <img src="docs/images/streamgate_presence.png?raw=true" alt="Presence Members Modal" width="100%" style="border-radius: 8px; border: 1px solid #334155;" />
    </td>
  </tr>
  <tr>
    <td width="50%" valign="top">
      <strong>JWT Token Generator</strong><br />
      Generate and test subscriber authentication tokens directly in the cockpit using standard HMAC-SHA256 signatures.
      <br /><br />
      <img src="docs/images/streamgate_jwt.png?raw=true" alt="JWT Token Generator" width="100%" style="border-radius: 8px; border: 1px solid #334155;" />
    </td>
    <td width="50%" valign="top">
      <strong>Mobile Responsive Cockpit Layout</strong><br />
      Full operational capabilities adapted for mobile and tablet screens, from quick triage to subscriber monitoring.
      <br /><br />
      <img src="docs/images/streamgate_mobile.png?raw=true" alt="Mobile Cockpit Layout" width="100%" style="border-radius: 8px; border: 1px solid #334155;" />
    </td>
  </tr>
</table>

---

## Architecture

```
[ Application Backends (Python, Go, Node, Ruby, Java) ]
                          │
                          │  POST /api/v1/publish
                          ▼
┌────────────────────────────────────────────────────────┐
│                   streamgate Core                      │
│                                                        │
│  ┌───────────────────────┐  ┌───────────────────────┐  │
│  │ Channel Authorization │  │ Monotonic Sequence ID │  │
│  │ API Key & JWT Claims  │  │ Generator             │  │
│  └───────────────────────┘  └───────────┬───────────┘  │
│                                         │              │
│                                         ▼              │
│  ┌──────────────────────────────────────────────────┐  │
│  │ Per-Channel Circular Ring Buffer (1,024 items)   │  │
│  │ Fast-forward recovery for Last-Event-ID replay   │  │
│  └──────────────────────┬───────────────────────────┘  │
│                         │ Pre-rendered SSE frame       │
│                         ▼                              │
│  ┌──────────────────────────────────────────────────┐  │
│  │ 64-Stripe Sharded Subscriber Registry            │  │
│  │ - Bounded non-blocking client write channels     │  │
│  │ - Heartbeat comment generator (:keepalive)       │  │
│  │ - Presence member tracker (presence-*)           │  │
│  │ - Channel occupancy webhooks (occupied/vacated)  │  │
│  └──────────────────────┬───────────────────────────┘  │
│                         │                              │
│         ┌───────────────┴───────────────┐              │
│         ▼                               ▼              │
│  [ Web Browsers ]              [ Mobile Apps ]         │
│  GET /v1/subscribe?channel=x   GET /v1/subscribe?ch=y  │
│                                                        │
│  ┌──────────────────────────────────────────────────┐  │
│  │ Embedded React 19 Operations Cockpit             │  │
│  │ Real-time channel inspector, JWT tool & publisher│  │
│  └──────────────────────────────────────────────────┘  │
│                         ▲                              │
│                         │ Optional Backbone            │
│  ┌──────────────────────┴───────────────────────────┐  │
│  │ Distributed Redis / Valkey Pub/Sub Mesh          │  │
│  └──────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────┘
```

---

## Key Capabilities

- **Zero-Allocation Fan-Out**: Ingested events are formatted into the Server-Sent Events frame specification (`id`, `event`, `data\n\n`) once upon receipt. The same byte slice is written directly to subscriber connections.
- **Lock-Striped Subscriber Tables**: Subscribers within each channel are partitioned across 64 discrete mutex stripes. Broadcasting does not block incoming connections or subscriber teardown.
- **Presence Channels**: Channels prefixed with `presence-` track connected users, broadcasting `presence.member_added` and `presence.member_removed` events automatically with user metadata.
- **Channel Lifecycle Webhooks**: Sends HTTP POST requests (`channel_occupied` and `channel_vacated`) with HMAC-SHA256 signatures to backends when subscriber counts transition between 0 and 1.
- **Replay Buffers & REST History**: Each channel maintains an in-memory ring buffer. Missed events are replayed on reconnection with `Last-Event-ID`, or queried directly via `GET /api/v1/channels/{channel}/history`.
- **Adaptive Stream Compression**: Supports gzip streaming compression with immediate frame-by-frame flushing.
- **Multi-Channel & Wildcard Routing**: Clients can subscribe to multiple comma-separated channels or wildcard patterns (`orders.*`).
- **Distributed Redis Clustering**: Connect multiple instances across a cluster via `-redis-addr` to distribute events across hundreds of thousands of connections.
- **Backpressure Protection**: Each client connection uses a bounded channel buffer. If a client stalls, streamgate applies a configurable drop policy (`drop_oldest`, `drop_newest`, or `disconnect`) to prevent memory exhaustion.
- **Low Memory Footprint**: Runs in 3MB to 35MB of RAM under thousands of concurrent connections.
- **Single Static Executable**: The React 19 operations cockpit is embedded directly into the Go binary.

---

## Quick Start

### 1. Download or Build

With Go installed:

```bash
git clone https://github.com/alexandrmotologa/streamgate.git
cd streamgate
go build -o bin/streamgate ./cmd/streamgate
```

### 2. Start the Daemon

```bash
./bin/streamgate -port 8080
```

Open `http://localhost:8080` in your browser to access the embedded operations cockpit.

---

## API Reference

### Publish an Event

Send an HTTP POST request to `/api/v1/publish`:

```bash
curl -X POST http://localhost:8080/api/v1/publish \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "finance-trades",
    "event": "trade.executed",
    "data": {
      "symbol": "BTC/USD",
      "price": 94820.50,
      "volume": 0.42
    }
  }'
```

Response:

```json
{
  "success": true,
  "id": "1730937600000-000001",
  "channel": "finance-trades",
  "event": "trade.executed",
  "delivered_to": 128
}
```

### Batch Publish

Publish multiple events to different channels in a single request:

```bash
curl -X POST http://localhost:8080/api/v1/publish/batch \
  -H "Content-Type: application/json" \
  -d '{
    "events": [
      {
        "channel": "orders",
        "event": "created",
        "data": { "order_id": 101 }
      },
      {
        "channel": "alerts",
        "event": "system",
        "data": { "level": "info", "message": "Cache warmed" }
      }
    ]
  }'
```

### Subscribe to a Stream

Connect with standard HTTP/2 or HTTP/1.1 SSE clients:

```bash
curl -N "http://localhost:8080/v1/subscribe?channel=finance-trades"
```

Stream output:

```
retry: 3000
:connected client=sub-1730937600000-000001 channels=finance-trades

id: 1730937600000-000001
event: trade.executed
data: {"symbol":"BTC/USD","price":94820.5,"volume":0.42}

:keepalive
```

### Presence Channels & Member Metadata

Subscribe to a presence channel with `user_id` and optional JSON `info`:

```bash
curl -N "http://localhost:8080/v1/subscribe?channel=presence-chat&user_id=usr_42&info=%7B%22name%22%3A%22Alex%22%7D"
```

Query current online members:

```bash
curl http://localhost:8080/api/v1/channels/presence-chat/members
```

Response:

```json
{
  "channel": "presence-chat",
  "members_count": 1,
  "members": [
    {
      "client_id": "sub-1730937600000-000001",
      "user_id": "usr_42",
      "info": { "name": "Alex" },
      "joined_at": "2026-11-07T15:00:00Z"
    }
  ]
}
```

### Query Channel History

Fetch recent events directly from the in-memory ring buffer:

```bash
curl "http://localhost:8080/api/v1/channels/finance-trades/history?limit=20"
```

---

## Client Integration

### Using streamgate-client (Zero-Dependency Browser & Node.js SDK)

Import the included lightweight client:

```html
<script src="/client/streamgate.js"></script>
<script>
  const client = new Streamgate('http://localhost:8080', {
    userId: 'usr_101',
    userInfo: { name: 'Alex' }
  });

  // Standard channel subscription
  const trades = client.subscribe('finance-trades');
  trades.on('trade.executed', (trade) => {
    console.log('Trade received:', trade);
  });

  // Presence channel subscription
  const room = client.subscribe('presence-room');
  room.on('presence.member_added', (member) => {
    console.log('User joined:', member.user_id, member.info);
  });
  room.on('presence.member_removed', (member) => {
    console.log('User left:', member.user_id);
  });

  client.connect();
</script>
```

### Python Publisher

```python
import json
import urllib.request

data = json.dumps({
    "channel": "finance-trades",
    "event": "trade.executed",
    "data": {"symbol": "ETH/USD", "price": 2850.0}
}).encode("utf-8")

req = urllib.request.Request(
    "http://localhost:8080/api/v1/publish",
    data=data,
    headers={"Content-Type": "application/json"},
    method="POST"
)

with urllib.request.urlopen(req) as resp:
    print(resp.read().decode("utf-8"))
```

---

## Configuration

streamgate can be configured using command line flags or environment variables:

| Flag | Environment Variable | Default | Description |
| :--- | :--- | :--- | :--- |
| `-port` | `STREAMGATE_PORT` | `8080` | Port for the HTTP and SSE server |
| `-host` | `STREAMGATE_HOST` | `0.0.0.0` | Bind host address |
| `-api-key` | `STREAMGATE_API_KEY` | `""` | Optional Bearer API key required to publish |
| `-jwt-secret` | `STREAMGATE_JWT_SECRET` | `""` | Secret key for verifying subscriber JWT tokens |
| `-ring-size` | `STREAMGATE_RING_BUFFER_SIZE` | `1024` | Per-channel circular buffer replay capacity |
| `-client-buffer` | `STREAMGATE_CLIENT_BUFFER_SIZE` | `256` | Per-client queued message limit |
| `-heartbeat` | `STREAMGATE_HEARTBEAT_INTERVAL` | `15` | Keep-alive ping interval in seconds |
| `-retry-interval` | `STREAMGATE_RETRY_INTERVAL` | `3` | SSE client reconnect delay directive in seconds |
| `-webhook-url` | `STREAMGATE_WEBHOOK_URL` | `""` | Target endpoint for channel occupancy webhooks |
| `-webhook-secret` | `STREAMGATE_WEBHOOK_SECRET` | `""` | Shared secret for HMAC-SHA256 signature on webhooks |
| `-compression` | `STREAMGATE_ENABLE_COMPRESSION` | `true` | Enable gzip streaming compression |
| `-redis-addr` | `STREAMGATE_REDIS_ADDR` | `""` | Redis address for distributed pub/sub clustering |
| `-drop-policy` | `STREAMGATE_DROP_POLICY` | `drop_oldest` | Backpressure policy: `drop_oldest`, `drop_newest`, `disconnect` |
| `-web-ui` | `STREAMGATE_ENABLE_WEB_UI` | `true` | Serve the embedded React 19 cockpit at `/` |

---

## Telemetry and Metrics

- **Operational Health**: `GET /healthz`
- **Gateway Statistics**: `GET /api/v1/stats`
- **Active Channel Inspection**: `GET /api/v1/channels`
- **Prometheus Metrics**: `GET /metrics`

Available Prometheus metrics:
- `streamgate_active_connections`: Current number of open subscriber connections.
- `streamgate_messages_published_total`: Total published messages by channel.
- `streamgate_messages_delivered_total`: Total delivered messages by channel.
- `streamgate_messages_dropped_total`: Total dropped messages by policy.
- `streamgate_fanout_duration_seconds`: Fan-out dispatch latency histogram.
- `streamgate_publish_request_duration_seconds`: Publish endpoint duration histogram.

---

## Performance Verification

streamgate includes an automated load generation utility (`streamgate-bench`) to benchmark fan-out performance and memory consumption locally.

Run the benchmark suite:

```bash
# Build the benchmark tool
go build -o bin/streamgate-bench ./cmd/streamgate-bench

# Execute test with 1,000 concurrent subscribers at 50 msgs/sec
./bin/streamgate-bench -target http://localhost:8080 -clients 1000 -rate 50 -duration 10s
```

Sample benchmark output:

```
================================================================
 STREAMGATE HIGH-CONCURRENCY FAN-OUT BENCHMARK
================================================================
 Target:                 http://127.0.0.1:8080
 Connected Subscribers:  1,000 concurrent streams
 Total Event Fan-Outs:   50,000 deliveries
 Effective Throughput:   5,000.0 msgs/sec fan-out
 Delivery Latency p50:   2.85 ms
 Delivery Latency p95:   4.12 ms
 Delivery Latency p99:   5.88 ms
----------------------------------------------------------------
 Gateway Heap Alloc:     3.42 MB (Target: < 45 MB)
 Gateway Sys Alloc:      18.20 MB
 Active Goroutines:      2,008
================================================================
```

---

## Docker Deployment

Build and run using Docker:

```bash
docker build -t streamgate .
docker run -p 8080:8080 streamgate
```

Or deploy with Docker Compose:

```bash
docker compose up -d
```

---

## License

MIT License. See [LICENSE](LICENSE) for details.
