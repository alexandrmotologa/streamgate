import React, { useState, useRef } from 'react';
import { History, Play, Square, RefreshCcw, Layers } from 'lucide-react';
import { LiveEventItem } from '../types';

export const ReplayTester: React.FC = () => {
  const [channel, setChannel] = useState('finance.btc');
  const [lastEventId, setLastEventId] = useState('0');
  const [status, setStatus] = useState<'idle' | 'connected' | 'error'>('idle');
  const [replayedEvents, setReplayedEvents] = useState<LiveEventItem[]>([]);
  const eventSourceRef = useRef<EventSource | null>(null);

  const startReplayTest = () => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }

    setReplayedEvents([]);
    setStatus('connected');

    const url = `/v1/subscribe?channel=${encodeURIComponent(channel)}&last_event_id=${encodeURIComponent(lastEventId)}`;
    const es = new EventSource(url);
    eventSourceRef.current = es;

    es.onopen = () => {
      setStatus('connected');
    };

    es.onerror = () => {
      setStatus('error');
    };

    // Generic message listener
    es.onmessage = (e) => {
      const now = new Date().toLocaleTimeString();
      setReplayedEvents(prev => [
        ...prev,
        {
          id: e.lastEventId || 'unknown',
          channel,
          event: 'message',
          data: e.data,
          timestamp: now,
          isReplay: true,
        }
      ]);
    };

    // Also catch custom event types if any
    const customTypes = ['trade.tick', 'token.delta', 'health.warning', 'user.message', 'score', 'init'];
    for (const evtName of customTypes) {
      es.addEventListener(evtName, (e: MessageEvent) => {
        const now = new Date().toLocaleTimeString();
        setReplayedEvents(prev => [
          ...prev,
          {
            id: e.lastEventId || 'unknown',
            channel,
            event: evtName,
            data: e.data,
            timestamp: now,
            isReplay: true,
          }
        ]);
      });
    }
  };

  const stopReplayTest = () => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
    setStatus('idle');
  };

  return (
    <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-5 backdrop-blur">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 pb-4 border-b border-slate-800">
        <div>
          <h2 className="text-lg font-semibold text-white flex items-center gap-2">
            <History className="w-5 h-5 text-amber-400" />
            <span>Last-Event-ID Reconnection Buffer Verifier</span>
          </h2>
          <p className="text-xs text-slate-400 mt-0.5">
            Test seamless fast-forward recovery of missed events across mobile network dropouts.
          </p>
        </div>

        <div className="flex items-center gap-2">
          {status === 'connected' ? (
            <button
              onClick={stopReplayTest}
              className="px-3 py-1.5 bg-rose-500/20 hover:bg-rose-500/30 text-rose-300 text-xs font-semibold rounded-lg flex items-center gap-1.5 border border-rose-500/30 transition"
            >
              <Square className="w-3.5 h-3.5" />
              <span>Disconnect</span>
            </button>
          ) : (
            <button
              onClick={startReplayTest}
              className="px-4 py-1.5 bg-amber-500 hover:bg-amber-600 text-slate-950 text-xs font-semibold rounded-lg flex items-center gap-1.5 transition shadow-sm shadow-amber-500/20"
            >
              <Play className="w-3.5 h-3.5" />
              <span>Start Replay Test</span>
            </button>
          )}
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-4">
        <div>
          <label className="block text-xs font-medium text-slate-300 mb-1">
            Target Channel
          </label>
          <input
            type="text"
            value={channel}
            onChange={e => setChannel(e.target.value)}
            placeholder="e.g. finance.btc"
            className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs font-mono text-white focus:outline-none focus:border-amber-500 transition"
          />
        </div>

        <div>
          <div className="flex items-center justify-between mb-1">
            <label className="text-xs font-medium text-slate-300">
              Last-Event-ID (<code className="text-amber-400">0</code> = replay entire buffer)
            </label>
            <div className="flex items-center gap-1 text-[11px]">
              <button
                type="button"
                onClick={() => setLastEventId('0')}
                className="text-slate-400 hover:text-white"
              >
                Reset to 0
              </button>
            </div>
          </div>
          <input
            type="text"
            value={lastEventId}
            onChange={e => setLastEventId(e.target.value)}
            placeholder="e.g. 1730937600000-000001 or 0"
            className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs font-mono text-white focus:outline-none focus:border-amber-500 transition"
          />
        </div>
      </div>

      {/* Stream results */}
      <div className="mt-4 pt-3 border-t border-slate-800">
        <div className="flex items-center justify-between text-xs mb-2">
          <span className="text-slate-400 font-mono">
            Recovered Frames: <strong className="text-white">{replayedEvents.length}</strong>
          </span>
          <div className="flex items-center gap-2">
            {status === 'connected' && (
              <span className="inline-flex items-center gap-1 text-emerald-400 font-mono text-[11px]">
                <RefreshCcw className="w-3 h-3 animate-spin" />
                <span>Stream Active</span>
              </span>
            )}
            {status === 'error' && (
              <span className="text-rose-400 font-mono text-[11px]">Stream Dropped</span>
            )}
          </div>
        </div>

        <div className="bg-slate-950 border border-slate-800 rounded-xl p-3 min-h-[220px] max-h-[350px] overflow-y-auto space-y-1.5 font-mono text-xs">
          {replayedEvents.length === 0 ? (
            <div className="h-44 flex flex-col items-center justify-center text-slate-400">
              <Layers className="w-8 h-8 mb-2 opacity-30" />
              <p>No replay frames received yet</p>
              <p className="text-[11px] text-slate-400 mt-1">
                Click &apos;Start Replay Test&apos; to query the circular ring buffer.
              </p>
            </div>
          ) : (
            replayedEvents.map((ev, i) => (
              <div
                key={`${ev.id}-${i}`}
                className="p-2 rounded bg-slate-900/80 border border-slate-800/80 flex items-start gap-2.5"
              >
                <span className="px-1.5 py-0.2 bg-amber-500/10 text-amber-400 border border-amber-500/20 rounded text-[10px] uppercase font-bold flex-shrink-0">
                  Buffer Replay
                </span>
                <span className="text-emerald-400 font-semibold">{ev.id}</span>
                <span className="text-purple-400 font-medium">[{ev.event}]</span>
                <span className="text-slate-300 truncate flex-1">{ev.data}</span>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
};
