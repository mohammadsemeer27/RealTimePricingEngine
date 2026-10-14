import React from 'react';
import {
  TrendingUp,
  TrendingDown,
  Layers,
  Clock,
  Cpu,
  Flame,
  ArrowUpRight,
  ArrowDownRight,
  ShieldAlert
} from 'lucide-react';
import { MarketSnapshot, SystemMetrics } from '../types/market';
import { formatFreshness, formatPrice } from '../utils/marketUtils';

interface SummaryCardsProps {
  stats: {
    totalSymbols: number;
    staleCount: number;
    freshCount: number;
    avgFreshnessMs: number;
    topGainer: MarketSnapshot | null;
    topLoser: MarketSnapshot | null;
    highestSpreadSymbol: { symbol: string; spreadBps: number } | null;
  };
  metrics: SystemMetrics | null;
  onSelectSymbol?: (symbol: string) => void;
}

export const SummaryCards: React.FC<SummaryCardsProps> = ({
  stats,
  metrics,
  onSelectSymbol,
}) => {
  return (
    <div className="max-w-7xl mx-auto px-4 lg:px-8 mt-5">
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3.5 sm:gap-4">
        {/* Card 1: Total Symbols */}
        <div className="relative overflow-hidden p-4 rounded-2xl bg-[#101520] border border-slate-800/80 shadow-lg shadow-black/20 hover:border-slate-700/80 transition-all">
          <div className="flex items-center justify-between text-slate-400 text-xs font-medium">
            <span className="flex items-center gap-1.5">
              <Layers className="w-4 h-4 text-cyan-400" />
              Active Universe
            </span>
            <span className="px-2 py-0.5 rounded-full bg-cyan-950/60 text-cyan-300 border border-cyan-800/40 text-[10px] font-semibold">
              61 PAIRS
            </span>
          </div>

          <div className="mt-3 flex items-baseline justify-between">
            <div className="text-3xl font-extrabold text-white tracking-tight font-mono">
              {stats.totalSymbols}
            </div>
            <div className="text-xs text-slate-400 flex flex-col items-end gap-0.5">
              <span className="text-[11px] text-slate-300 font-medium">FX • Crypto • Metals</span>
              <span className="text-[10px] text-slate-500">24h Monotonic Window</span>
            </div>
          </div>

          <div className="mt-3 pt-2.5 border-t border-slate-800/60 flex items-center justify-between text-[11px] text-slate-400">
            <span className="flex items-center gap-1">
              <span className={`w-1.5 h-1.5 rounded-full ${stats.freshCount > 0 ? 'bg-emerald-400' : 'bg-amber-400'}`} />
              {stats.freshCount} Fresh
            </span>
            <span className="flex items-center gap-1">
              <span className="w-1.5 h-1.5 rounded-full bg-amber-400" />
              {stats.staleCount} Stale
            </span>
            <span className="text-slate-500">Amortized O(1)</span>
          </div>
        </div>

        {/* Card 2: Freshness / Feed Health */}
        <div className="relative overflow-hidden p-4 rounded-2xl bg-[#101520] border border-slate-800/80 shadow-lg shadow-black/20 hover:border-slate-700/80 transition-all">
          <div className="flex items-center justify-between text-slate-400 text-xs font-medium">
            <span className="flex items-center gap-1.5">
              <Clock className="w-4 h-4 text-amber-400" />
              Avg Data Freshness
            </span>
            <span className={`px-2 py-0.5 rounded-full text-[10px] font-semibold border ${
              stats.staleCount === 0
                ? 'bg-emerald-950/60 text-emerald-300 border-emerald-800/40'
                : 'bg-amber-950/60 text-amber-300 border-amber-800/40'
            }`}>
              {stats.staleCount === 0 ? 'SUB-SECOND' : 'CACHED'}
            </span>
          </div>

          <div className="mt-3 flex items-baseline justify-between">
            <div className="text-3xl font-extrabold text-white tracking-tight font-mono">
              {formatFreshness(stats.avgFreshnessMs)}
            </div>
            <div className="text-xs flex flex-col items-end text-slate-400">
              <span className="text-[11px] text-amber-400/90 font-medium">Staleness Limit</span>
              <span className="text-[10px] text-slate-500">60,000 ms</span>
            </div>
          </div>

          <div className="mt-3 pt-2.5 border-t border-slate-800/60 flex items-center justify-between text-[11px] text-slate-400">
            <span>Window: 24 Hours</span>
            <span className="font-mono text-slate-300">
              {stats.staleCount > 0 ? `${stats.staleCount} flagged stale` : 'All ticks real-time'}
            </span>
          </div>
        </div>

        {/* Card 3: Top Gainer */}
        <div
          onClick={() => stats.topGainer && onSelectSymbol?.(stats.topGainer.symbol)}
          className="relative overflow-hidden p-4 rounded-2xl bg-[#101520] border border-slate-800/80 shadow-lg shadow-black/20 hover:border-emerald-500/40 transition-all cursor-pointer group"
        >
          <div className="flex items-center justify-between text-slate-400 text-xs font-medium">
            <span className="flex items-center gap-1.5 text-emerald-400">
              <TrendingUp className="w-4 h-4" />
              24h Top Gainer
            </span>
            {stats.topGainer && (
              <span className="font-mono font-bold text-emerald-400 flex items-center text-xs">
                <ArrowUpRight className="w-3.5 h-3.5" />
                +{parseFloat(stats.topGainer.change_24h_pct).toFixed(2)}%
              </span>
            )}
          </div>

          <div className="mt-3 flex items-baseline justify-between">
            <div className="text-2xl font-bold text-white tracking-tight group-hover:text-cyan-300 transition-colors">
              {stats.topGainer?.symbol || '-'}
            </div>
            <div className="text-sm font-mono text-slate-200">
              {stats.topGainer ? formatPrice(stats.topGainer.mid_price, stats.topGainer.symbol) : '-'}
            </div>
          </div>

          <div className="mt-3 pt-2.5 border-t border-slate-800/60 flex items-center justify-between text-[11px] text-slate-400 font-mono">
            <span>High: {stats.topGainer ? formatPrice(stats.topGainer.high_24h, stats.topGainer.symbol) : '-'}</span>
            <span className="text-slate-500">Ref: {stats.topGainer ? formatPrice(stats.topGainer.reference_price, stats.topGainer.symbol) : '-'}</span>
          </div>
        </div>

        {/* Card 4: Top Loser */}
        <div
          onClick={() => stats.topLoser && onSelectSymbol?.(stats.topLoser.symbol)}
          className="relative overflow-hidden p-4 rounded-2xl bg-[#101520] border border-slate-800/80 shadow-lg shadow-black/20 hover:border-rose-500/40 transition-all cursor-pointer group"
        >
          <div className="flex items-center justify-between text-slate-400 text-xs font-medium">
            <span className="flex items-center gap-1.5 text-rose-400">
              <TrendingDown className="w-4 h-4" />
              24h Top Loser
            </span>
            {stats.topLoser && (
              <span className="font-mono font-bold text-rose-400 flex items-center text-xs">
                <ArrowDownRight className="w-3.5 h-3.5" />
                {parseFloat(stats.topLoser.change_24h_pct).toFixed(2)}%
              </span>
            )}
          </div>

          <div className="mt-3 flex items-baseline justify-between">
            <div className="text-2xl font-bold text-white tracking-tight group-hover:text-rose-300 transition-colors">
              {stats.topLoser?.symbol || '-'}
            </div>
            <div className="text-sm font-mono text-slate-200">
              {stats.topLoser ? formatPrice(stats.topLoser.mid_price, stats.topLoser.symbol) : '-'}
            </div>
          </div>

          <div className="mt-3 pt-2.5 border-t border-slate-800/60 flex items-center justify-between text-[11px] text-slate-400 font-mono">
            <span>Low: {stats.topLoser ? formatPrice(stats.topLoser.low_24h, stats.topLoser.symbol) : '-'}</span>
            <span className="text-slate-500">Ref: {stats.topLoser ? formatPrice(stats.topLoser.reference_price, stats.topLoser.symbol) : '-'}</span>
          </div>
        </div>
      </div>
    </div>
  );
};
