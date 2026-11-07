import React, { useState, useRef, useEffect } from 'react';
import { Play, Pause, Trash2, Filter, Copy, Check, Terminal, ExternalLink } from 'lucide-react';
import { LiveEventItem } from '../types';

interface Props {
  events: LiveEventItem[];
  activeChannel: string;
  onClear: () => void;
  onChannelChange: (channel: string) => void;
}

export const LiveEventFeed: React.FC<Props> = ({
  events,
  activeChannel,
  onClear,
  onChannelChange,
}) => {
  const [isPaused, setIsPaused] = useState(false);
  const [selectedEvent, setSelectedEvent] = useState<LiveEventItem | null>(null);
  const [copied, setCopied] = useState(false);
  const [filterText, setFilterText] = useState('');
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isPaused && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [events, isPaused]);

  const filteredEvents = events.filter(
    ev =>
      ev.channel.toLowerCase().includes(filterText.toLowerCase()) ||
      ev.event.toLowerCase().includes(filterText.toLowerCase()) ||
      ev.data.toLowerCase().includes(filterText.toLowerCase())
  );

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const formatPayload = (data: string) => {
    try {
      const parsed = JSON.parse(data);
      return JSON.stringify(parsed, null, 2);
    } catch {
      return data;
    }
  };

  return (
    <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-5 backdrop-blur flex flex-col h-[650px]">
      {/* Header controls */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-4 border-b border-slate-800 flex-shrink-0">
        <div>
          <h2 className="text-lg font-semibold text-white flex items-center gap-2">
            <Terminal className="w-5 h-5 text-emerald-400" />
            <span>Live Event Stream Inspector</span>
            <span className="text-xs bg-slate-800 text-slate-300 font-mono px-2 py-0.5 rounded-full">
              {filteredEvents.length} frames
            </span>
          </h2>
          <div className="text-xs text-slate-400 mt-0.5 flex items-center gap-2">
            <span>Listening on channel:</span>
            <span className="font-mono text-emerald-400 font-medium bg-emerald-500/10 px-1.5 py-0.5 rounded">
              {activeChannel || 'all channels'}
            </span>
          </div>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          <div className="relative">
            <Filter className="w-3.5 h-3.5 text-slate-400 absolute left-2.5 top-1/2 -translate-y-1/2" />
            <input
              type="text"
              placeholder="Filter payload or event..."
              value={filterText}
              onChange={e => setFilterText(e.target.value)}
              className="bg-slate-950 border border-slate-800 rounded-lg pl-8 pr-2.5 py-1 text-xs text-white placeholder-slate-400 focus:outline-none focus:border-emerald-500 w-44"
            />
          </div>

          <button
            onClick={() => setIsPaused(!isPaused)}
            className={`px-2.5 py-1 text-xs font-medium rounded-lg flex items-center gap-1.5 transition ${
              isPaused
                ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30'
                : 'bg-slate-800 hover:bg-slate-700 text-slate-300'
            }`}
          >
            {isPaused ? <Play className="w-3.5 h-3.5" /> : <Pause className="w-3.5 h-3.5" />}
            <span>{isPaused ? 'Resume' : 'Pause'}</span>
          </button>

          <button
            onClick={onClear}
            title="Clear stream log"
            className="p-1.5 bg-slate-800 hover:bg-rose-950/40 hover:text-rose-400 text-slate-400 rounded-lg transition"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Event list view */}
      <div className="flex-1 overflow-hidden flex flex-col md:flex-row gap-4 mt-4 min-h-0">
        <div
          ref={scrollRef}
          className="flex-1 overflow-y-auto space-y-1.5 pr-1 font-mono text-xs select-text"
        >
          {filteredEvents.length === 0 ? (
            <div className="h-full flex flex-col items-center justify-center text-slate-400 py-16">
              <Terminal className="w-8 h-8 mb-2 opacity-30" />
              <p className="text-sm">Awaiting events...</p>
              <p className="text-xs text-slate-400 mt-1">
                Publish a test event or connect an upstream producer.
              </p>
            </div>
          ) : (
            filteredEvents.map((ev, i) => {
              const isSelected = selectedEvent?.id === ev.id;
              return (
                <div
                  key={`${ev.id}-${i}`}
                  onClick={() => setSelectedEvent(ev)}
                  className={`p-2.5 rounded-lg border cursor-pointer transition flex items-start gap-3 ${
                    isSelected
                      ? 'bg-slate-800/90 border-emerald-500/50'
                      : 'bg-slate-950/60 border-slate-800/60 hover:bg-slate-800/40 hover:border-slate-700'
                  }`}
                >
                  <div className="flex items-center gap-1.5 text-slate-400 text-[10px] w-20 flex-shrink-0 pt-0.5">
                    <span>{ev.timestamp}</span>
                  </div>

                  {ev.isReplay ? (
                    <span className="text-[10px] px-1.5 py-0.5 bg-amber-500/10 text-amber-400 border border-amber-500/20 rounded font-semibold uppercase tracking-wider flex-shrink-0">
                      Replay
                    </span>
                  ) : (
                    <span className="text-[10px] px-1.5 py-0.5 bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 rounded font-semibold uppercase tracking-wider flex-shrink-0">
                      Live
                    </span>
                  )}

                  <span
                    onClick={e => {
                      e.stopPropagation();
                      onChannelChange(ev.channel);
                    }}
                    className="text-sky-400 hover:underline flex-shrink-0 font-medium"
                  >
                    #{ev.channel}
                  </span>

                  <span className="text-purple-400 font-semibold flex-shrink-0">
                    [{ev.event}]
                  </span>

                  <span className="text-slate-300 truncate flex-1 opacity-90">
                    {ev.data}
                  </span>
                </div>
              );
            })
          )}
        </div>

        {/* Payload inspector drawer */}
        {selectedEvent && (
          <div className="w-full md:w-80 lg:w-96 bg-slate-950 border border-slate-800 rounded-xl p-4 flex flex-col justify-between flex-shrink-0 overflow-hidden">
            <div>
              <div className="flex items-center justify-between pb-3 border-b border-slate-800">
                <span className="text-xs font-semibold uppercase tracking-wider text-slate-300">
                  Event Frame Inspector
                </span>
                <button
                  onClick={() => setSelectedEvent(null)}
                  className="text-slate-400 hover:text-white text-xs"
                >
                  Close
                </button>
              </div>

              <div className="mt-3 space-y-2 text-xs font-mono">
                <div>
                  <span className="text-slate-400 block text-[10px] uppercase">Event ID</span>
                  <span className="text-emerald-400 font-semibold">{selectedEvent.id}</span>
                </div>
                <div>
                  <span className="text-slate-400 block text-[10px] uppercase">Channel</span>
                  <span className="text-sky-400">#{selectedEvent.channel}</span>
                </div>
                <div>
                  <span className="text-slate-400 block text-[10px] uppercase">Event Type</span>
                  <span className="text-purple-400">{selectedEvent.event}</span>
                </div>
                <div>
                  <div className="flex items-center justify-between mt-3 mb-1">
                    <span className="text-slate-400 text-[10px] uppercase">Payload (data)</span>
                    <button
                      onClick={() => handleCopy(selectedEvent.data)}
                      className="text-slate-400 hover:text-white flex items-center gap-1 text-[11px]"
                    >
                      {copied ? <Check className="w-3 h-3 text-emerald-400" /> : <Copy className="w-3 h-3" />}
                      <span>{copied ? 'Copied' : 'Copy'}</span>
                    </button>
                  </div>
                  <pre className="bg-slate-900 border border-slate-800/80 rounded-lg p-2.5 text-[11px] text-slate-200 overflow-x-auto max-h-56">
                    {formatPayload(selectedEvent.data)}
                  </pre>
                </div>
              </div>
            </div>

            <div className="mt-4 pt-3 border-t border-slate-800 text-[11px] text-slate-400 flex items-center justify-between">
              <span>Time: {selectedEvent.timestamp}</span>
              <span className="flex items-center gap-1 text-emerald-400">
                <ExternalLink className="w-3 h-3" />
                <span>Framed SSE</span>
              </span>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
