// Zero-dependency Node.js publisher for streamgate
const http = require('http');

const GATEWAY_HOST = process.env.STREAMGATE_HOST || 'localhost';
const GATEWAY_PORT = process.env.STREAMGATE_PORT || 8080;
const CHANNEL = process.env.STREAMGATE_CHANNEL || 'finance-trades';

function publishEvent(channel, eventType, data) {
  const payload = JSON.stringify({
    channel,
    event: eventType,
    data,
  });

  const options = {
    hostname: GATEWAY_HOST,
    port: GATEWAY_PORT,
    path: '/api/v1/publish',
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Content-Length': Buffer.byteLength(payload),
    },
  };

  const req = http.request(options, (res) => {
    let body = '';
    res.on('data', (chunk) => body += chunk);
    res.on('end', () => {
      try {
        const resp = JSON.parse(body);
        console.log(`[PUBLISHED] ID: ${resp.id} | Delivered to: ${resp.delivered_to} subscribers`);
      } catch {
        console.log(`[RESPONSE] HTTP ${res.statusCode}: ${body}`);
      }
    });
  });

  req.on('error', (err) => {
    console.error(`[ERROR] Publish failed: ${err.message}`);
  });

  req.write(payload);
  req.end();
}

console.log(`Publishing events to http://${GATEWAY_HOST}:${GATEWAY_PORT}/api/v1/publish every 1.5s...`);
let seq = 0;

setInterval(() => {
  seq++;
  publishEvent(CHANNEL, 'trade.executed', {
    symbol: 'BTC/USD',
    price: +(94200 + Math.random() * 400).toFixed(2),
    volume: +(Math.random() * 1.5).toFixed(4),
    seq,
    timestamp: Date.now(),
  });
}, 1500);
