import React, { useEffect, useState } from 'react';
import {
  Activity,
  Wifi,
  WifiOff,
  RefreshCw,
  Clock,
  Pause,
  Play,
  Server,
  Zap,
  BarChart3
} from 'lucide-react';
import { FeedStatus, HealthStatus } from '../types/market';

interface HeaderProps {
  feedStatus: FeedStatus;
  feedStatusDetail: string;
  isBackendHealthy: boolean;
  health: HealthStatus | null;
  wsStatus: 'connected' | 'connecting' | 'disconnected' | 'error';
  lastUpdated: Date | null;
  refreshNow: () => Promise<void>;
  isPollingPaused: boolean;
  setIsPollingPaused: (paused: boolean) => void;
  onOpenMetrics: () => void;
}

export const Header: React.FC<HeaderProps> = ({
  feedStatus,
  feedStatusDetail,
  isBackendHealthy,
  health,
  wsStatus,
  lastUpdated,
  refreshNow,
  isPollingPaused,
  setIsPollingPaused,
  onOpenMetrics,
}) => {
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [utcTime, setUtcTime] = useState(new Date().toUTCString().slice(17, 25) + ' UTC');

  useEffect(() => {
    const timer = setInterval(() => {
      setUtcTime(new Date().toUTCString().slice(17, 25) + ' UTC');
    }, 1000);
    return () => clearInterval(timer);
  }, []);

  const handleManualRefresh = async () => {
    setIsRefreshing(true);
    try {
      await refreshNow();
    } finally {
      setTimeout(() => setIsRefreshing(false), 500);
    }
  };

  const formatUptime = (seconds?: number): string => {
    if (seconds === undefined) return '-';
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    return `${h}h ${m}m`;
  };

  return (
    <header className="sticky top-0 z-40 bg-[#0B0E14]/90 backdrop-blur-md border-b border-slate-800/80 px-4 lg:px-8 py-3.5 transition-all">
      <div className="max-w-7xl mx-auto flex flex-col md:flex-row md:items-center justify-between gap-3">
        {/* Left Branding */}
        <div className="flex items-center gap-3">
          <div className="relative flex items-center justify-center w-10 h-10 rounded-xl bg-gradient-to-tr from-cyan-600 via-blue-600 to-indigo-600 shadow-lg shadow-cyan-500/20 ring-1 ring-white/20">
            <Zap className="w-5 h-5 text-white animate-pulse" />
            <div className="absolute -bottom-0.5 -right-0.5 w-3 h-3 rounded-full bg-emerald-500 border-2 border-[#0B0E14]" />
          </div>
          <div>
            <div className="flex items-center gap-2.5">
              <h1 className="text-xl font-bold tracking-tight text-white m-0 flex items-center gap-2">
                Real-Time Pricing Engine
              </h1>
              <span className="text-[10px] font-semibold tracking-wider uppercase px-2 py-0.5 rounded-full bg-slate-800 text-cyan-400 border border-cyan-500/30">
                PRO v1.0
              </span>
            </div>
            <p className="text-xs text-slate-400 flex items-center gap-1.5 mt-0.5">
              <span className="inline-block w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
              High-Frequency FX & Crypto Market Data Pipeline
            </p>
          </div>
        </div>

        {/* Center / Right Control & Status Bar */}
        <div className="flex flex-wrap items-center gap-2 sm:gap-3 text-xs">
          {/* Feed Status Pill */}
          <div
            title={feedStatusDetail}
            className={`flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg border font-medium cursor-help transition-all ${
              feedStatus === 'live'
                ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30 shadow-sm shadow-emerald-500/10'
                : feedStatus === 'stale'
                ? 'bg-amber-500/10 text-amber-300 border-amber-500/30'
                : feedStatus === 'mock'
                ? 'bg-purple-500/10 text-purple-300 border-purple-500/30'
                : 'bg-rose-500/10 text-rose-400 border-rose-500/30 animate-pulse'
            }`}
          >
            <span className={`w-2 h-2 rounded-full ${
              feedStatus === 'live' ? 'bg-emerald-400 animate-ping' :
              feedStatus === 'stale' ? 'bg-amber-400' :
              feedStatus === 'mock' ? 'bg-purple-400' : 'bg-rose-400'
            }`} />
            <span className="capitalize font-semibold">
              {feedStatus === 'live' ? 'Live Stream' :
               feedStatus === 'stale' ? 'Stale Feed' :
               feedStatus === 'mock' ? 'Mock Feed' : 'Unavailable'}
            </span>
          </div>

          {/* Backend Connection Indicator */}
          <div
            title={isBackendHealthy ? 'Backend Go REST API on port 8082 is healthy' : 'Cannot reach backend REST API'}
            className={`flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg border font-medium ${
              isBackendHealthy
                ? 'bg-slate-900 text-slate-300 border-slate-700/80'
                : 'bg-rose-950/40 text-rose-300 border-rose-700/60'
            }`}
          >
            <Server className={`w-3.5 h-3.5 ${isBackendHealthy ? 'text-cyan-400' : 'text-rose-400'}`} />
            <span className="hidden sm:inline">REST:</span>
            <span className={isBackendHealthy ? 'text-emerald-400' : 'text-rose-400'}>
              {isBackendHealthy ? '8082 OK' : 'Down'}
            </span>
            {health?.uptime_seconds !== undefined && (
              <span className="text-[10px] text-slate-500 border-l border-slate-700 pl-1.5 ml-0.5">
                {formatUptime(health.uptime_seconds)}
              </span>
            )}
          </div>

          {/* WebSocket Status */}
          <div
            title={`WebSocket protocol status: ${wsStatus}`}
            className="hidden lg:flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-slate-900 text-slate-300 border border-slate-700/80 font-medium"
          >
            {wsStatus === 'connected' ? (
              <Wifi className="w-3.5 h-3.5 text-emerald-400" />
            ) : wsStatus === 'connecting' ? (
              <Activity className="w-3.5 h-3.5 text-amber-400 animate-spin" />
            ) : (
              <WifiOff className="w-3.5 h-3.5 text-slate-500" />
            )}
            <span>WS:</span>
            <span className={`capitalize ${
              wsStatus === 'connected' ? 'text-emerald-400' :
              wsStatus === 'connecting' ? 'text-amber-400' : 'text-slate-400'
            }`}>
              {wsStatus}
            </span>
          </div>

          {/* UTC Clock */}
          <div className="hidden xl:flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-slate-900 text-slate-300 border border-slate-800 font-mono text-[11px]">
            <Clock className="w-3.5 h-3.5 text-cyan-400" />
            <span>{utcTime}</span>
          </div>

          {/* Open Operational Metrics Button */}
          <button
            onClick={onOpenMetrics}
            title="Inspect Go Engine Internal Metrics & Diagnostics"
            className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-slate-800/80 hover:bg-slate-700 text-slate-200 border border-slate-700/60 font-medium transition"
          >
            <BarChart3 className="w-3.5 h-3.5 text-cyan-400" />
            <span className="hidden sm:inline">Metrics</span>
          </button>

          {/* Polling Pause / Resume Button */}
          <button
            onClick={() => setIsPollingPaused(!isPollingPaused)}
            title={isPollingPaused ? 'Resume 3s REST Auto-Refresh' : 'Pause Auto-Refresh'}
            className={`p-1.5 rounded-lg border transition ${
              isPollingPaused
                ? 'bg-amber-500/20 text-amber-300 border-amber-500/40 hover:bg-amber-500/30'
                : 'bg-slate-800/80 text-slate-300 border-slate-700 hover:bg-slate-700'
            }`}
          >
            {isPollingPaused ? <Play className="w-3.5 h-3.5" /> : <Pause className="w-3.5 h-3.5" />}
          </button>

          {/* Manual Refresh Button */}
          <button
            onClick={handleManualRefresh}
            disabled={isRefreshing}
            title="Force immediate reload from backend /prices"
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-cyan-600 hover:bg-cyan-500 active:scale-95 text-white font-medium shadow-md shadow-cyan-600/20 transition disabled:opacity-50 cursor-pointer"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${isRefreshing ? 'animate-spin' : ''}`} />
            <span>Refresh</span>
          </button>
        </div>
      </div>
    </header>
  );
};
