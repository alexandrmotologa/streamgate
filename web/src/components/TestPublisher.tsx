import React, { useState } from 'react';
import { Send, Sparkles, CheckCircle2, AlertCircle, Clock } from 'lucide-react';

interface Props {
  defaultChannel?: string;
  onEventPublished?: () => void;
}

export const TestPublisher: React.FC<Props> = ({ defaultChannel = 'chat.lobby', onEventPublished }) => {
  const [channel, setChannel] = useState(defaultChannel);
  const [event, setEvent] = useState('trade.tick');
  const [payload, setPayload] = useState(`{
  "symbol": "BTC/USD",
  "price": 94820.50,
  "volume": 1.45,
  "timestamp": ${Date.now()}
}`);
  const [apiKey, setApiKey] = useState('');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<{
    success: boolean;
    id?: string;
    delivered?: number;
    error?: string;
    durationMs?: number;
  } | null>(null);

  const applyPreset = (type: 'trade' | 'ai' | 'alert' | 'chat') => {
    switch (type) {
      case 'trade':
        setChannel('finance.btc');
        setEvent('trade.tick');
        setPayload(JSON.stringify({
          symbol: "BTC/USD",
          price: (94500 + Math.random() * 500).toFixed(2),
          volume: +(Math.random() * 2).toFixed(4),
          timestamp: Date.now()
        }, null, 2));
        break;
      case 'ai':
        setChannel('ai.completion');
        setEvent('token.delta');
        setPayload(JSON.stringify({
          model: "claude-3-7-sonnet",
          token: "optimized",
          index: 42,
          is_final: false
        }, null, 2));
        break;
      case 'alert':
        setChannel('ops.alerts');
        setEvent('health.warning');
        setPayload(JSON.stringify({
          severity: "high",
          service: "auth-gateway",
          message: "Database connection pool saturated > 85%",
          code: 503
        }, null, 2));
        break;
      case 'chat':
        setChannel('chat.general');
        setEvent('user.message');
        setPayload(JSON.stringify({
          user: "alex",
          message: "Streamgate handles 50,000 clients smoothly with under 45MB RAM!",
          sent_at: new Date().toISOString()
        }, null, 2));
        break;
    }
  };

  const handlePublish = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setResult(null);

    const startTime = performance.now();

    try {
      let parsedData: unknown;
      try {
        parsedData = JSON.parse(payload);
      } catch {
        parsedData = payload;
      }

      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
      };
      if (apiKey) {
        headers['Authorization'] = `Bearer ${apiKey}`;
      }

      const res = await fetch('/api/v1/publish', {
        method: 'POST',
        headers,
        body: JSON.stringify({
          channel,
          event,
          data: parsedData,
        }),
      });

      const durationMs = Math.round(performance.now() - startTime);

      if (!res.ok) {
        const errorText = await res.text();
        setResult({
          success: false,
          error: errorText || `HTTP ${res.status}`,
          durationMs,
        });
        return;
      }

      const data = await res.json();
      setResult({
        success: true,
        id: data.id,
        delivered: data.delivered_to,
        durationMs,
      });

      if (onEventPublished) {
        onEventPublished();
      }
    } catch (err: unknown) {
      setResult({
        success: false,
        error: (err as Error).message || 'Failed to reach gateway endpoint',
        durationMs: Math.round(performance.now() - startTime),
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-5 backdrop-blur">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 pb-4 border-b border-slate-800">
        <div>
          <h2 className="text-lg font-semibold text-white flex items-center gap-2">
            <Send className="w-5 h-5 text-emerald-400" />
            <span>Interactive Event Publisher</span>
          </h2>
          <p className="text-xs text-slate-400 mt-0.5">
            Test fan-out ingestion via POST /api/v1/publish with live delivery receipts.
          </p>
        </div>

        {/* Quick preset buttons */}
        <div className="flex items-center gap-1.5 flex-wrap">
          <span className="text-[11px] text-slate-400 mr-1 flex items-center gap-1">
            <Sparkles className="w-3 h-3 text-amber-400" />
            <span>Presets:</span>
          </span>
          <button
            type="button"
            onClick={() => applyPreset('trade')}
            className="px-2 py-1 bg-slate-800 hover:bg-slate-700 text-[11px] text-slate-300 rounded font-mono transition"
          >
            Crypto Trade
          </button>
          <button
            type="button"
            onClick={() => applyPreset('ai')}
            className="px-2 py-1 bg-slate-800 hover:bg-slate-700 text-[11px] text-slate-300 rounded font-mono transition"
          >
            AI Token
          </button>
          <button
            type="button"
            onClick={() => applyPreset('alert')}
            className="px-2 py-1 bg-slate-800 hover:bg-slate-700 text-[11px] text-slate-300 rounded font-mono transition"
          >
            Alert
          </button>
          <button
            type="button"
            onClick={() => applyPreset('chat')}
            className="px-2 py-1 bg-slate-800 hover:bg-slate-700 text-[11px] text-slate-300 rounded font-mono transition"
          >
            Chat
          </button>
        </div>
      </div>

      <form onSubmit={handlePublish} className="mt-4 space-y-4">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Target Channel
            </label>
            <input
              type="text"
              required
              value={channel}
              onChange={e => setChannel(e.target.value)}
              placeholder="e.g. finance-trades or chat.lobby"
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs font-mono text-white placeholder-slate-400 focus:outline-none focus:border-emerald-500 transition"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Event Type Frame (<code className="text-emerald-400">event:</code>)
            </label>
            <input
              type="text"
              required
              value={event}
              onChange={e => setEvent(e.target.value)}
              placeholder="e.g. trade.executed or message"
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs font-mono text-white placeholder-slate-400 focus:outline-none focus:border-emerald-500 transition"
            />
          </div>
        </div>

        <div>
          <div className="flex items-center justify-between mb-1">
            <label className="text-xs font-medium text-slate-300">
              JSON Event Payload (<code className="text-emerald-400">data:</code>)
            </label>
            <button
              type="button"
              onClick={() => {
                try {
                  setPayload(JSON.stringify(JSON.parse(payload), null, 2));
                } catch {
                  // ignore
                }
              }}
              className="text-[11px] text-slate-400 hover:text-white transition"
            >
              Format JSON
            </button>
          </div>
          <textarea
            rows={7}
            required
            value={payload}
            onChange={e => setPayload(e.target.value)}
            className="w-full bg-slate-950 border border-slate-800 rounded-lg p-3 text-xs font-mono text-slate-200 placeholder-slate-400 focus:outline-none focus:border-emerald-500 transition"
          />
        </div>

        <div className="flex flex-col sm:flex-row items-center justify-between gap-4 pt-2">
          <div className="w-full sm:w-72">
            <input
              type="password"
              value={apiKey}
              onChange={e => setApiKey(e.target.value)}
              placeholder="Optional Bearer API Key"
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white placeholder-slate-400 focus:outline-none focus:border-emerald-500 transition"
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full sm:w-auto px-6 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:opacity-50 text-slate-950 font-semibold text-xs rounded-lg transition flex items-center justify-center gap-2 shadow-sm shadow-emerald-500/20"
          >
            <Send className="w-4 h-4" />
            <span>{loading ? 'Broadcasting...' : 'Publish to Streamgate'}</span>
          </button>
        </div>
      </form>

      {/* Result feedback */}
      {result && (
        <div
          className={`mt-4 p-3.5 rounded-xl border flex items-start gap-3 text-xs font-mono transition ${
            result.success
              ? 'bg-emerald-950/20 border-emerald-500/30 text-emerald-300'
              : 'bg-rose-950/20 border-rose-500/30 text-rose-300'
          }`}
        >
          {result.success ? (
            <CheckCircle2 className="w-5 h-5 text-emerald-400 flex-shrink-0 mt-0.5" />
          ) : (
            <AlertCircle className="w-5 h-5 text-rose-400 flex-shrink-0 mt-0.5" />
          )}

          <div className="flex-1 space-y-1">
            <div className="font-semibold text-white">
              {result.success ? 'Event Ingested and Broadcasted' : 'Publication Failed'}
            </div>
            {result.success ? (
              <div className="flex items-center gap-4 flex-wrap text-slate-300">
                <span>Assigned Event ID: <code className="text-emerald-400">{result.id}</code></span>
                <span>Fanned out to: <code className="text-emerald-400">{result.delivered} subscribers</code></span>
                <span className="flex items-center gap-1 text-slate-400">
                  <Clock className="w-3.5 h-3.5" />
                  <span>{result.durationMs}ms</span>
                </span>
              </div>
            ) : (
              <div className="text-rose-300">{result.error}</div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
