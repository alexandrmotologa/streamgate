const http = require('http');

const PORT = 8999;
const HOST = '127.0.0.1';

// 1. Connect simulated presence subscribers
const users = [
  { id: 'usr_sarah', name: 'Sarah Connor', role: 'Staff Eng' },
  { id: 'usr_alex', name: 'Alex M.', role: 'Architect' },
  { id: 'usr_david', name: 'David K.', role: 'DevOps' },
  { id: 'usr_elena', name: 'Elena R.', role: 'SRE' }
];

users.forEach(u => {
  const infoParam = encodeURIComponent(JSON.stringify({ name: u.name, role: u.role }));
  const url = `http://${HOST}:${PORT}/v1/subscribe?channel=presence-chat,finance-trades,ai.completion&user_id=${u.id}&info=${infoParam}`;
  http.get(url, (res) => {
    res.on('data', () => {}); // keep stream alive
  }).on('error', () => {});
});

console.log('Connected 4 simulated subscribers to presence-chat and topics.');

// 2. Publish loop
let counter = 0;
const interval = setInterval(() => {
  counter++;

  // Publish trade
  publish('finance-trades', 'trade.executed', {
    symbol: counter % 2 === 0 ? 'BTC/USD' : 'ETH/USD',
    price: counter % 2 === 0 ? +(94800 + Math.random() * 200).toFixed(2) : +(2840 + Math.random() * 20).toFixed(2),
    amount: +(Math.random() * 1.5).toFixed(4),
    seq: counter,
    timestamp: Date.now()
  });

  // Publish AI token
  const tokens = ['The', 'streamgate', 'multiplexer', 'broadcasts', 'events', 'sub-millisecond', 'latency', 'under', '45MB'];
  publish('ai.completion', 'token.delta', {
    model: 'gemini-2.5-pro',
    token: tokens[counter % tokens.length],
    index: counter,
    is_final: counter % tokens.length === tokens.length - 1
  });

  // Periodic alert
  if (counter % 5 === 0) {
    publish('ops.alerts', 'health.check', {
      status: 'nominal',
      active_connections: 4,
      heap_mb: '3.4 MB',
      fanout_rate: '40 msgs/sec'
    });
  }

  // Periodic chat
  if (counter % 4 === 0) {
    const user = users[counter % users.length];
    publish('presence-chat', 'chat.message', {
      name: user.name,
      text: `Testing real-time fan-out delivery frame #${counter}!`,
      time: new Date().toLocaleTimeString()
    });
  }
}, 350);

function publish(channel, event, data) {
  const body = JSON.stringify({ channel, event, data });
  const req = http.request({
    hostname: HOST,
    port: PORT,
    path: '/api/v1/publish',
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Content-Length': Buffer.byteLength(body)
    }
  }, (res) => {
    res.on('data', () => {});
  });
  req.on('error', () => {});
  req.write(body);
  req.end();
}
