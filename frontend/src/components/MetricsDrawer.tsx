import React from 'react';
import {
  X,
  Cpu,
  Database,
  Radio,
  AlertTriangle,
  Clock,
  Activity,
  Layers,
  ShieldCheck
} from 'lucide-react';
import { SystemMetrics } from '../types/market';

interface MetricsDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  metrics: SystemMetrics | null;
  serverUptime?: number;
}

export const MetricsDrawer: React.FC<MetricsDrawerProps> = ({
  isOpen,
  onClose,
  metrics,
  serverUptime,
}) => {
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex justify-end bg-black/60 backdrop-blur-sm animate-in fade-in duration-200">
      <div
        className="relative w-full max-w-md h-full bg-[#101520] border-l border-slate-700/80 p-6 overflow-y-auto text-white space-y-6 shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-slate-800 pb-4">
          <div className="flex items-center gap-2.5">
            <Cpu className="w-5 h-5 text-cyan-400" />
            <h3 className="text-lg font-bold text-white m-0">Engine Diagnostics</h3>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 text-slate-400 hover:text-white hover:bg-slate-800 rounded-lg transition"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {!metrics ? (
          <div className="text-slate-400 text-sm">Loading telemetry metrics...</div>
        ) : (
          <div className="space-y-6 text-xs">
            {/* Latency & Ingestion Throughput */}
            <div className="space-y-2">
              <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block">
                Latency & Processing SLA
              </span>
              <div className="grid grid-cols-2 gap-2 font-mono">
                <div className="p-3 rounded-xl bg-slate-900 border border-slate-800">
                  <span className="text-slate-500 block text-[10px]">Avg Latency</span>
                  <span className="text-emerald-400 font-bold text-base">
                    {metrics.avg_latency_micros > 0 ? `${metrics.avg_latency_micros.toFixed(2)} µs` : '< 10 µs'}
                  </span>
                  <span className="text-[9px] text-slate-500 block mt-0.5">SLA: &lt; 50,000 µs</span>
                </div>

                <div className="p-3 rounded-xl bg-slate-900 border border-slate-800">
                  <span className="text-slate-500 block text-[10px]">Processed Ticks</span>
                  <span className="text-cyan-400 font-bold text-base">
                    {metrics.processed_ticks.toLocaleString()}
                  </span>
                  <span className="text-[9px] text-slate-500 block mt-0.5">
                    Incoming: {metrics.incoming_ticks.toLocaleString()}
                  </span>
                </div>
              </div>
            </div>

            {/* Persistence & Database Store */}
            <div className="space-y-2">
              <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block">
                PostgreSQL Binary Ingestion
              </span>
              <div className="grid grid-cols-2 gap-2 font-mono">
                <div className="p-3 rounded-xl bg-slate-900 border border-slate-800">
                  <span className="text-slate-500 block text-[10px]">Database Writes</span>
                  <span className="text-slate-200 font-bold text-base">
                    {metrics.database_writes.toLocaleString()}
                  </span>
                </div>

                <div className="p-3 rounded-xl bg-slate-900 border border-slate-800">
                  <span className="text-slate-500 block text-[10px]">Database Errors</span>
                  <span className={`font-bold text-base ${metrics.database_errors > 0 ? 'text-rose-400' : 'text-emerald-400'}`}>
                    {metrics.database_errors}
                  </span>
                </div>
              </div>
            </div>

            {/* Downstream WebSocket Hub */}
            <div className="space-y-2">
              <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block">
                Downstream Distribution
              </span>
              <div className="grid grid-cols-2 gap-2 font-mono">
                <div className="p-3 rounded-xl bg-slate-900 border border-slate-800">
                  <span className="text-slate-500 block text-[10px]">Active Clients</span>
                  <span className="text-slate-200 font-bold text-base">
                    {metrics.downstream_clients}
                  </span>
                </div>

                <div className="p-3 rounded-xl bg-slate-900 border border-slate-800">
                  <span className="text-slate-500 block text-[10px]">Messages Emitted</span>
                  <span className="text-slate-200 font-bold text-base">
                    {metrics.outbound_messages.toLocaleString()}
                  </span>
                </div>
              </div>
            </div>

            {/* Payload Efficiency */}
            <div className="p-3 rounded-xl bg-slate-900 border border-slate-800 space-y-1 font-mono">
              <span className="text-slate-500 block text-[10px]">Payload Efficiency</span>
              <div className="flex justify-between text-slate-300">
                <span>Avg Payload Size:</span>
                <span>{metrics.avg_payload_bytes.toFixed(0)} B (Req: &lt; 10 KB)</span>
              </div>
              <div className="flex justify-between text-slate-300">
                <span>Max Payload Observed:</span>
                <span>{metrics.max_payload_bytes} B</span>
              </div>
            </div>

            {/* Upstream Connection & Failures */}
            <div className="p-3.5 rounded-xl bg-slate-900/90 border border-slate-800 space-y-2 font-mono">
              <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block">
                Upstream Feed Resilience
              </span>
              <div className="flex justify-between">
                <span className="text-slate-400">Auth Failures:</span>
                <span className={`font-bold ${metrics.auth_failures > 0 ? 'text-rose-400' : 'text-emerald-400'}`}>
                  {metrics.auth_failures}
                </span>
              </div>
              <div className="flex justify-between">
                <span className="text-slate-400">Reconnect Attempts:</span>
                <span className="text-slate-300">{metrics.upstream_reconnects}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-slate-400">Total Dropped Events:</span>
                <span className={metrics.dropped_events > 0 ? 'text-amber-400' : 'text-slate-300'}>
                  {metrics.dropped_events}
                </span>
              </div>

              {metrics.dropped_by_reason && Object.keys(metrics.dropped_by_reason).length > 0 && (
                <div className="pt-2 border-t border-slate-800">
                  <span className="text-[10px] text-slate-500 block mb-1">Dropped Breakdown:</span>
                  {Object.entries(metrics.dropped_by_reason).map(([reason, count]) => (
                    <div key={reason} className="flex justify-between text-[11px] text-amber-300/80">
                      <span>{reason}:</span>
                      <span>{count}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
