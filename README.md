# streamgate

High-throughput HTTP Server-Sent Events multiplexer and fan-out broadcast gateway written in Go with an embedded React 19 operations cockpit.

streamgate distributes real-time events to concurrent subscribers over HTTP/2 and HTTP/1.1 from a single backend HTTP POST request. It includes per-channel in-memory circular ring buffers for automatic `Last-Event-ID` reconnection recovery, lock-striped subscriber tables, periodic keep-alive comments, and an operations dashboard running in less than 45MB of RAM.

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
│  └──────────────────────┬───────────────────────────┘  │
│                         │                              │
│         ┌───────────────┴───────────────┐              │
│         ▼                               ▼              │
│  [ Web Browsers ]              [ Mobile Apps ]         │
│  GET /v1/subscribe?channel=x   GET /v1/subscribe?ch=y  │
│                                                        │
│  ┌──────────────────────────────────────────────────┐  │
│  │ Embedded React 19 Operations Cockpit             │  │
│  │ Real-time channel inspector and test publisher   │  │
│  └──────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────┘
```

---

## Key Capabilities

- **Zero-Allocation Fan-Out**: Ingested events are formatted into the Server-Sent Events frame specification (`id`, `event`, `data\n\n`) once upon receipt. The same byte slice is written directly to subscriber connections.
- **Lock-Striped Subscriber Tables**: Subscribers within each channel are partitioned across 64 discrete mutex stripes. Broadcasting does not block incoming connections or subscriber teardown.
- **Replay Buffers**: Each channel maintains an in-memory ring buffer. When a client reconnects after a network drop with `Last-Event-ID`, missed events are replayed before live events resume.
- **Multi-Channel and Wildcard Routing**: Clients can subscribe to multiple comma-separated channels or wildcard patterns (`orders.*`).
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
:connected client=sub-1730937600000-000001 channels=finance-trades

id: 1730937600000-000001
event: trade.executed
data: {"symbol":"BTC/USD","price":94820.5,"volume":0.42}

:keepalive
```

### Multi-Channel and Wildcard Subscription

```bash
curl -N "http://localhost:8080/v1/subscribe?channel=orders.*,notifications,crypto"
```

---

## Client Integration

### Browser (Native JavaScript)

```javascript
const stream = new EventSource('/v1/subscribe?channel=finance-trades');

stream.addEventListener('trade.executed', (event) => {
  const trade = JSON.parse(event.data);
  console.log('Trade received:', trade);
});

stream.onerror = () => {
  console.log('Connection lost. Browser reconnects automatically with Last-Event-ID.');
};
```

### Node.js Publisher

```javascript
const http = require('http');

const payload = JSON.stringify({
  channel: 'finance-trades',
  event: 'trade.executed',
  data: { symbol: 'BTC/USD', price: 94820.50 }
});

const req = http.request({
  hostname: 'localhost',
  port: 8080,
  path: '/api/v1/publish',
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Content-Length': Buffer.byteLength(payload)
  }
}, (res) => {
  res.on('data', chunk => console.log(chunk.toString()));
});

req.write(payload);
req.end();
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
