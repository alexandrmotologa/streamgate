import React from 'react';
import { Activity, Radio, Cpu, Layers, HardDrive } from 'lucide-react';
import { GatewayStats, RuntimeStats } from '../types';

interface Props {
  gateway: GatewayStats | null;
  runtime: RuntimeStats | null;
  ratePerSec: number;
}

export const OverviewGauges: React.FC<Props> = ({ gateway, runtime, ratePerSec }) => {
  const allocMB = runtime?.alloc_mb ? parseFloat(runtime.alloc_mb) : 0;
  // Calculate percentage of 45MB RAM target budget
  const memoryBudgetPercent = Math.min(100, Math.round((allocMB / 45) * 100));

  const formatUptime = (seconds: number) => {
    const hrs = Math.floor(seconds / 3600);
    const mins = Math.floor((seconds % 3600) / 60);
    const secs = seconds % 60;
    if (hrs > 0) return `${hrs}h ${mins}m ${secs}s`;
    if (mins > 0) return `${mins}m ${secs}s`;
    return `${secs}s`;
  };

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
      {/* 1. Active Subscribers */}
      <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-5 backdrop-blur shadow-sm relative overflow-hidden group hover:border-slate-700 transition">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium uppercase tracking-wider text-slate-400">Active Subscribers</span>
          <div className="p-2 bg-emerald-500/10 text-emerald-400 rounded-lg">
            <Radio className="w-5 h-5 animate-pulse" />
          </div>
        </div>
        <div className="mt-3 flex items-baseline gap-2">
          <span className="text-3xl font-bold font-mono tracking-tight text-white">
            {gateway ? gateway.active_connections.toLocaleString() : '0'}
          </span>
          <span className="text-xs text-slate-400 font-mono">sockets</span>
        </div>
        <div className="mt-2 text-xs text-slate-400 flex items-center gap-1.5">
          <span className="inline-block w-2 h-2 rounded-full bg-emerald-400"></span>
          <span>Total connected: {gateway ? gateway.total_connections_count.toLocaleString() : '0'}</span>
        </div>
      </div>

      {/* 2. Fan-Out Throughput */}
      <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-5 backdrop-blur shadow-sm relative overflow-hidden group hover:border-slate-700 transition">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium uppercase tracking-wider text-slate-400">Dispatch Rate</span>
          <div className="p-2 bg-sky-500/10 text-sky-400 rounded-lg">
            <Activity className="w-5 h-5" />
          </div>
        </div>
        <div className="mt-3 flex items-baseline gap-2">
          <span className="text-3xl font-bold font-mono tracking-tight text-white">
            {ratePerSec.toLocaleString()}
          </span>
          <span className="text-xs text-slate-400 font-mono">msgs/sec</span>
        </div>
        <div className="mt-2 text-xs text-slate-400 flex items-center justify-between">
          <span>Delivered: {gateway ? gateway.total_messages_delivered.toLocaleString() : '0'}</span>
          {gateway && gateway.total_messages_dropped > 0 && (
            <span className="text-amber-400">Dropped: {gateway.total_messages_dropped}</span>
          )}
        </div>
      </div>

      {/* 3. Memory Footprint (<45MB Budget) */}
      <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-5 backdrop-blur shadow-sm relative overflow-hidden group hover:border-slate-700 transition">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium uppercase tracking-wider text-slate-400">Memory Budget</span>
          <div className="p-2 bg-indigo-500/10 text-indigo-400 rounded-lg">
            <HardDrive className="w-5 h-5" />
          </div>
        </div>
        <div className="mt-3 flex items-baseline gap-2">
          <span className="text-3xl font-bold font-mono tracking-tight text-white">
            {runtime ? runtime.alloc_mb : '0.00'}
          </span>
          <span className="text-xs text-slate-400 font-mono">MB / 45 MB</span>
        </div>
        <div className="mt-2.5">
          <div className="w-full bg-slate-800 rounded-full h-1.5 overflow-hidden">
            <div
              className={`h-full transition-all duration-500 ${
                memoryBudgetPercent > 80 ? 'bg-amber-400' : 'bg-emerald-400'
              }`}
              style={{ width: `${Math.max(2, memoryBudgetPercent)}%` }}
            />
          </div>
          <div className="mt-1.5 flex justify-between text-[11px] text-slate-400">
            <span>{memoryBudgetPercent}% of budget</span>
            <span>Sys: {runtime ? (runtime.sys_bytes / 1024 / 1024).toFixed(1) : '0'} MB</span>
          </div>
        </div>
      </div>

      {/* 4. Active Topics & Goroutines */}
      <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-5 backdrop-blur shadow-sm relative overflow-hidden group hover:border-slate-700 transition">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium uppercase tracking-wider text-slate-400">System Topology</span>
          <div className="p-2 bg-purple-500/10 text-purple-400 rounded-lg">
            <Layers className="w-5 h-5" />
          </div>
        </div>
        <div className="mt-3 flex items-baseline gap-2">
          <span className="text-3xl font-bold font-mono tracking-tight text-white">
            {gateway ? gateway.total_channels_count : '0'}
          </span>
          <span className="text-xs text-slate-400 font-mono">channels</span>
        </div>
        <div className="mt-2 text-xs text-slate-400 flex items-center justify-between">
          <span className="flex items-center gap-1">
            <Cpu className="w-3.5 h-3.5 text-slate-400" />
            <span>Goroutines: {runtime ? runtime.goroutines : '0'}</span>
          </span>
          <span>Up: {gateway ? formatUptime(gateway.uptime_seconds) : '0s'}</span>
        </div>
      </div>
    </div>
  );
};
