import React, { useState } from 'react';
import { Search, Radio, Send, RefreshCw, Database } from 'lucide-react';
import { ChannelInfo } from '../types';

interface Props {
  channels: ChannelInfo[];
  onSelectChannel: (channel: string) => void;
  onPublishToChannel: (channel: string) => void;
  onRefresh: () => void;
}

export const ChannelExplorer: React.FC<Props> = ({
  channels,
  onSelectChannel,
  onPublishToChannel,
  onRefresh,
}) => {
  const [search, setSearch] = useState('');

  const filtered = channels.filter(ch =>
    ch.name.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-5 backdrop-blur">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-4 border-b border-slate-800">
        <div>
          <h2 className="text-lg font-semibold text-white flex items-center gap-2">
            <span>Active Channels & Topics</span>
            <span className="text-xs bg-slate-800 text-slate-300 font-mono px-2 py-0.5 rounded-full">
              {channels.length}
            </span>
          </h2>
          <p className="text-xs text-slate-400 mt-0.5">
            Real-time subscriber maps, ring buffer replay windows, and fan-out totals.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <div className="relative">
            <Search className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
            <input
              type="text"
              placeholder="Filter channels..."
              value={search}
              onChange={e => setSearch(e.target.value)}
              className="bg-slate-950 border border-slate-800 rounded-lg pl-9 pr-3 py-1.5 text-xs text-white placeholder-slate-400 focus:outline-none focus:border-emerald-500 w-48 sm:w-64 transition"
            />
          </div>
          <button
            onClick={onRefresh}
            title="Refresh channels"
            aria-label="Refresh channels"
            className="p-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg transition"
          >
            <RefreshCw className="w-4 h-4" />
          </button>
        </div>
      </div>

      {filtered.length === 0 ? (
        <div className="py-12 text-center text-slate-400">
          <Radio className="w-8 h-8 mx-auto mb-2 opacity-40" />
          <p className="text-sm">No channels found</p>
          <p className="text-xs text-slate-400 mt-1">
            Publish an event or connect a subscriber to instantiate a channel.
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 mt-4">
          {filtered.map(ch => {
            const bufferPercent = Math.round((ch.buffer_size / Math.max(1, ch.buffer_capacity)) * 100);
            const isSystem = ch.name.startsWith('_streamgate');

            return (
              <div
                key={ch.name}
                className="bg-slate-950/70 border border-slate-800/80 rounded-xl p-4 hover:border-slate-700 transition flex flex-col justify-between group"
              >
                <div>
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <span className="font-mono text-sm font-semibold text-white truncate block">
                        {ch.name}
                      </span>
                      {isSystem && (
                        <span className="inline-block mt-0.5 text-[10px] font-mono uppercase tracking-wider bg-indigo-500/10 text-indigo-400 px-1.5 py-0.2 rounded">
                          System Metric Stream
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-1.5 bg-emerald-500/10 text-emerald-400 px-2 py-0.5 rounded-full text-xs font-mono font-medium flex-shrink-0">
                      <Radio className="w-3 h-3" />
                      <span>{ch.subscribers}</span>
                    </div>
                  </div>

                  <div className="grid grid-cols-2 gap-2 mt-4 text-xs font-mono">
                    <div className="bg-slate-900/60 p-2 rounded-lg border border-slate-800/40">
                      <span className="text-slate-400 block text-[10px] uppercase">Published</span>
                      <span className="text-white font-semibold">{ch.total_published.toLocaleString()}</span>
                    </div>
                    <div className="bg-slate-900/60 p-2 rounded-lg border border-slate-800/40">
                      <span className="text-slate-400 block text-[10px] uppercase">Delivered</span>
                      <span className="text-emerald-400 font-semibold">{ch.total_delivered.toLocaleString()}</span>
                    </div>
                  </div>

                  {/* Ring buffer occupancy */}
                  <div className="mt-3">
                    <div className="flex justify-between text-[11px] text-slate-400 mb-1">
                      <span className="flex items-center gap-1">
                        <Database className="w-3 h-3 text-slate-400" />
                        <span>Replay Buffer</span>
                      </span>
                      <span className="font-mono">{ch.buffer_size} / {ch.buffer_capacity}</span>
                    </div>
                    <div className="w-full bg-slate-900 rounded-full h-1 overflow-hidden">
                      <div
                        className="h-full bg-sky-400 transition-all duration-300"
                        style={{ width: `${Math.max(2, bufferPercent)}%` }}
                      />
                    </div>
                  </div>
                </div>

                <div className="mt-4 pt-3 border-t border-slate-800/60 flex items-center justify-between gap-2">
                  <button
                    onClick={() => onSelectChannel(ch.name)}
                    className="flex-1 py-1.5 px-3 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium rounded-lg transition flex items-center justify-center gap-1.5"
                  >
                    <Radio className="w-3.5 h-3.5 text-emerald-400" />
                    <span>Listen Live</span>
                  </button>
                  <button
                    onClick={() => onPublishToChannel(ch.name)}
                    className="flex-1 py-1.5 px-3 bg-emerald-600/20 hover:bg-emerald-600/30 text-emerald-300 text-xs font-medium rounded-lg transition flex items-center justify-center gap-1.5 border border-emerald-500/20"
                  >
                    <Send className="w-3.5 h-3.5" />
                    <span>Publish</span>
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};
