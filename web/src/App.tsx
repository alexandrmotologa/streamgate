import { useState, useEffect, useRef } from 'react';
import {
  Layers,
  Radio,
  Send,
  History,
  Activity,
  Terminal,
  ExternalLink,
  Code2,
  BookOpen,
} from 'lucide-react';
import { GatewayStats, RuntimeStats, ChannelInfo, LiveEventItem } from './types';
import { OverviewGauges } from './components/OverviewGauges';
import { ChannelExplorer } from './components/ChannelExplorer';
import { LiveEventFeed } from './components/LiveEventFeed';
import { TestPublisher } from './components/TestPublisher';
import { ReplayTester } from './components/ReplayTester';

export function App() {
  const [tab, setTab] = useState<'channels' | 'live' | 'publish' | 'replay'>('channels');
  const [gatewayStats, setGatewayStats] = useState<GatewayStats | null>(null);
  const [runtimeStats, setRuntimeStats] = useState<RuntimeStats | null>(null);
  const [channels, setChannels] = useState<ChannelInfo[]>([]);
  const [events, setEvents] = useState<LiveEventItem[]>([]);
  const [activeChannel, setActiveChannel] = useState('');
  const [isConnected, setIsConnected] = useState(false);
  const [ratePerSec, setRatePerSec] = useState(0);
  const [showDocs, setShowDocs] = useState(false);

  // Rate tracking
  const messageCountRef = useRef(0);
  const lastDeliveredTotalRef = useRef<number | null>(null);

  // 1. Fetch channels and stats initially and periodically
  const fetchChannelData = async () => {
    try {
      const [resCh, resSt] = await Promise.all([
        fetch('/api/v1/channels'),
        fetch('/api/v1/stats'),
      ]);
      if (resCh.ok) {
        const data = await resCh.json();
        setChannels(data.channels || []);
      }
      if (resSt.ok) {
        const st = await resSt.json();
        setGatewayStats(st.gateway);
        setRuntimeStats(st.runtime);

        // Calculate rate per second
        if (lastDeliveredTotalRef.current !== null) {
          const delta = st.gateway.total_messages_delivered - lastDeliveredTotalRef.current;
          setRatePerSec(Math.max(0, delta));
        }
        lastDeliveredTotalRef.current = st.gateway.total_messages_delivered;
      }
    } catch {
      // offline or initial loading
    }
  };

  useEffect(() => {
    fetchChannelData();
    const interval = setInterval(fetchChannelData, 2000);
    return () => clearInterval(interval);
  }, []);

  // 2. Dogfooding: Connect to gateway's own SSE stream!
  useEffect(() => {
    // Listen to metrics and any selected channels
    const channelsToListen = activeChannel
      ? `_streamgate.metrics,${activeChannel}`
      : '_streamgate.metrics,chat.lobby,finance.btc';

    const url = `/v1/subscribe?channel=${encodeURIComponent(channelsToListen)}`;
    const es = new EventSource(url);

    es.onopen = () => {
      setIsConnected(true);
    };

    es.onerror = () => {
      setIsConnected(false);
    };

    // Telemetry event handler
    es.addEventListener('telemetry', (e: MessageEvent) => {
      try {
        const stats: GatewayStats = JSON.parse(e.data);
        setGatewayStats(stats);
      } catch {
        // ignore
      }
    });

    // Handle all messages for live feed
    es.onmessage = (e: MessageEvent) => {
      messageCountRef.current += 1;
      const now = new Date().toLocaleTimeString();
      const item: LiveEventItem = {
        id: e.lastEventId || 'frame',
        channel: activeChannel || 'broadcast',
        event: 'message',
        data: e.data,
        timestamp: now,
      };

      setEvents(prev => {
        const next = [...prev, item];
        if (next.length > 300) next.shift();
        return next;
      });
    };

    // Custom event handlers
    const customTypes = ['trade.tick', 'token.delta', 'health.warning', 'user.message', 'score', 'init'];
    customTypes.forEach(evtType => {
      es.addEventListener(evtType, (e: MessageEvent) => {
        messageCountRef.current += 1;
        const now = new Date().toLocaleTimeString();
        const item: LiveEventItem = {
          id: e.lastEventId || 'frame',
          channel: activeChannel || 'broadcast',
          event: evtType,
          data: e.data,
          timestamp: now,
        };
        setEvents(prev => {
          const next = [...prev, item];
          if (next.length > 300) next.shift();
          return next;
        });
      });
    });

    return () => {
      es.close();
    };
  }, [activeChannel]);

  return (
    <div className="min-h-screen flex flex-col bg-slate-950 text-slate-100 selection:bg-emerald-500 selection:text-slate-950">
      {/* Top Navigation Bar */}
      <header className="border-b border-slate-800/80 bg-slate-900/60 backdrop-blur sticky top-0 z-30">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-xl bg-gradient-to-tr from-emerald-600 to-teal-400 flex items-center justify-center shadow-lg shadow-emerald-500/20 text-slate-950 font-bold font-mono">
              <Activity className="w-5 h-5 text-slate-950 stroke-[2.5]" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <span className="font-bold text-base tracking-tight text-white font-mono">streamgate</span>
                <span className="text-[10px] font-mono uppercase tracking-wider bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 px-1.5 py-0.2 rounded font-semibold">
                  v1.0.0
                </span>
                <span className="text-[10px] font-mono text-slate-400 hidden sm:inline-block">
                  &lt; 45MB RAM Target
                </span>
              </div>
              <p className="text-[11px] text-slate-400 hidden sm:block">
                High-Throughput Server-Sent Events Multiplexer & Fan-Out Broadcast Gateway
              </p>
            </div>
          </div>

          <div className="flex items-center gap-3">
            {/* Live connection badge */}
            <div className="flex items-center gap-1.5 bg-slate-950 border border-slate-800 px-2.5 py-1 rounded-full text-xs font-mono">
              <span
                className={`w-2 h-2 rounded-full ${
                  isConnected ? 'bg-emerald-400 animate-pulse' : 'bg-rose-500'
                }`}
              />
              <span className="text-slate-300">
                {isConnected ? 'Telemetry Live' : 'Disconnected'}
              </span>
            </div>

            <button
              onClick={() => setShowDocs(!showDocs)}
              className="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg transition text-xs flex items-center gap-1.5 px-2.5"
            >
              <BookOpen className="w-3.5 h-3.5" />
              <span className="hidden sm:inline">Docs</span>
            </button>

            <a
              href="https://github.com/alexandrmotologa/streamgate"
              target="_blank"
              rel="noreferrer"
              className="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg transition"
              title="GitHub Repository"
            >
              <Code2 className="w-4 h-4" />
            </a>
          </div>
        </div>
      </header>

      {/* Main Content Area */}
      <main className="flex-1 max-w-7xl mx-auto w-full px-4 sm:px-6 lg:px-8 py-6 space-y-6">
        {/* Metric Overview Gauges */}
        <OverviewGauges
          gateway={gatewayStats}
          runtime={runtimeStats}
          ratePerSec={ratePerSec}
        />

        {/* Navigation Tabs */}
        <div className="border-b border-slate-800 flex items-center gap-2 overflow-x-auto pb-px">
          <button
            onClick={() => setTab('channels')}
            className={`px-4 py-2.5 text-xs font-medium rounded-t-lg transition flex items-center gap-2 border-b-2 font-mono ${
              tab === 'channels'
                ? 'border-emerald-400 text-emerald-400 bg-slate-900/40'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            <Layers className="w-4 h-4" />
            <span>Channels ({channels.length})</span>
          </button>

          <button
            onClick={() => setTab('live')}
            className={`px-4 py-2.5 text-xs font-medium rounded-t-lg transition flex items-center gap-2 border-b-2 font-mono ${
              tab === 'live'
                ? 'border-emerald-400 text-emerald-400 bg-slate-900/40'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            <Terminal className="w-4 h-4" />
            <span>Live Stream Inspector</span>
            {events.length > 0 && (
              <span className="bg-emerald-500/20 text-emerald-300 text-[10px] px-1.5 py-0.2 rounded-full font-semibold">
                {events.length}
              </span>
            )}
          </button>

          <button
            onClick={() => setTab('publish')}
            className={`px-4 py-2.5 text-xs font-medium rounded-t-lg transition flex items-center gap-2 border-b-2 font-mono ${
              tab === 'publish'
                ? 'border-emerald-400 text-emerald-400 bg-slate-900/40'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            <Send className="w-4 h-4" />
            <span>Test Publisher</span>
          </button>

          <button
            onClick={() => setTab('replay')}
            className={`px-4 py-2.5 text-xs font-medium rounded-t-lg transition flex items-center gap-2 border-b-2 font-mono ${
              tab === 'replay'
                ? 'border-emerald-400 text-emerald-400 bg-slate-900/40'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            <History className="w-4 h-4" />
            <span>Replay Verifier</span>
          </button>
        </div>

        {/* Tab Views */}
        {tab === 'channels' && (
          <ChannelExplorer
            channels={channels}
            onSelectChannel={ch => {
              setActiveChannel(ch);
              setTab('live');
            }}
            onPublishToChannel={ch => {
              setActiveChannel(ch);
              setTab('publish');
            }}
            onRefresh={fetchChannelData}
          />
        )}

        {tab === 'live' && (
          <LiveEventFeed
            events={events}
            activeChannel={activeChannel}
            onClear={() => setEvents([])}
            onChannelChange={ch => setActiveChannel(ch)}
          />
        )}

        {tab === 'publish' && (
          <TestPublisher
            defaultChannel={activeChannel || 'chat.lobby'}
            onEventPublished={() => {
              fetchChannelData();
            }}
          />
        )}

        {tab === 'replay' && <ReplayTester />}

        {/* API Docs & Quick-Start Card */}
        {showDocs && (
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 font-mono text-xs text-slate-300 space-y-4">
            <div className="flex items-center justify-between pb-2 border-b border-slate-800">
              <span className="font-semibold text-white uppercase text-xs tracking-wider">
                Streamgate API Quick-Start Cheatsheet
              </span>
              <button onClick={() => setShowDocs(false)} className="text-slate-400 hover:text-white">
                ✕ Close
              </button>
            </div>

            <div className="space-y-3">
              <div>
                <span className="text-emerald-400 font-semibold block mb-1">
                  1. Browser SSE Subscription (Native EventSource)
                </span>
                <pre className="bg-slate-950 p-3 rounded-lg border border-slate-800 text-[11px] overflow-x-auto text-slate-200">
{`const stream = new EventSource('/v1/subscribe?channel=finance-trades');

stream.addEventListener('trade.executed', (e) => {
  const data = JSON.parse(e.data);
  console.log('Trade received:', data);
});

// Automatic reconnection with Last-Event-ID header is handled natively by the browser`}
                </pre>
              </div>

              <div>
                <span className="text-emerald-400 font-semibold block mb-1">
                  2. Backend Publishing Ingestion (cURL / HTTP POST)
                </span>
                <pre className="bg-slate-950 p-3 rounded-lg border border-slate-800 text-[11px] overflow-x-auto text-slate-200">
{`curl -X POST http://localhost:8080/api/v1/publish \\
  -H "Content-Type: application/json" \\
  -d '{
    "channel": "finance-trades",
    "event": "trade.executed",
    "data": { "symbol": "BTC/USD", "price": 94820.50 }
  }'`}
                </pre>
              </div>

              <div>
                <span className="text-emerald-400 font-semibold block mb-1">
                  3. Multi-Channel & Wildcard Subscription
                </span>
                <pre className="bg-slate-950 p-3 rounded-lg border border-slate-800 text-[11px] overflow-x-auto text-slate-200">
{`curl -N "http://localhost:8080/v1/subscribe?channel=orders.*,notifications,alerts"`}
                </pre>
              </div>
            </div>
          </div>
        )}
      </main>

      {/* Footer */}
      <footer className="border-t border-slate-800/80 bg-slate-950/80 py-4 text-xs text-slate-400 font-mono">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 flex flex-col sm:flex-row items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <span>streamgate</span>
            <span>•</span>
            <span>Go 1.23+ Engine</span>
            <span>•</span>
            <span>React 19 Telemetry</span>
          </div>
          <div className="flex items-center gap-4">
            <span className="flex items-center gap-1 text-slate-400">
              <Radio className="w-3 h-3 text-emerald-400" />
              <span>HTTP/2 Multiplexed SSE</span>
            </span>
            <a
              href="https://github.com/alexandrmotologa/streamgate"
              target="_blank"
              rel="noreferrer"
              className="text-slate-400 hover:text-emerald-400 transition flex items-center gap-1"
            >
              <span>GitHub</span>
              <ExternalLink className="w-3 h-3" />
            </a>
          </div>
        </div>
      </footer>
    </div>
  );
}
