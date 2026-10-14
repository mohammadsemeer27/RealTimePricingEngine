import React, { useEffect } from 'react';
import {
  X,
  ArrowUpRight,
  ArrowDownRight,
  Clock,
  Database,
  Hash,
  Activity,
  Layers,
  ShieldCheck,
  ShieldAlert
} from 'lucide-react';
import { MarketSnapshot } from '../types/market';
import {
  calculateSpread,
  formatFreshness,
  formatPrice,
  getAssetClass
} from '../utils/marketUtils';

interface SymbolDetailModalProps {
  snapshot: MarketSnapshot | null;
  onClose: () => void;
}

export const SymbolDetailModal: React.FC<SymbolDetailModalProps> = ({
  snapshot,
  onClose,
}) => {
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [onClose]);

  if (!snapshot) return null;

  const changePct = parseFloat(snapshot.change_24h_pct);
  const isPositive = !isNaN(changePct) && changePct > 0;
  const isNegative = !isNaN(changePct) && changePct < 0;
  const spread = calculateSpread(snapshot.buy, snapshot.sell);
  const assetClass = getAssetClass(snapshot.symbol);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-in fade-in duration-200">
      <div
        className="relative w-full max-w-xl rounded-3xl bg-[#101520] border border-slate-700/80 shadow-2xl p-6 text-white space-y-6"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-start justify-between border-b border-slate-800/80 pb-4">
          <div className="flex items-center gap-3">
            <div className="w-12 h-12 rounded-2xl bg-cyan-950/60 border border-cyan-800/50 flex items-center justify-center text-cyan-400 font-bold text-lg">
              {snapshot.symbol.slice(0, 3)}
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-2xl font-bold tracking-tight text-white m-0">
                  {snapshot.symbol}
                </h2>
                <span className="text-xs px-2.5 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700">
                  {assetClass}
                </span>
              </div>
              <p className="text-xs text-slate-400 mt-0.5 flex items-center gap-1.5 font-mono">
                Seq #{snapshot.sequence} • Event: {new Date(snapshot.event_time).toUTCString()}
              </p>
            </div>
          </div>

          <button
            onClick={onClose}
            className="p-2 rounded-xl text-slate-400 hover:text-white hover:bg-slate-800 transition"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Primary Pricing Banner */}
        <div className="p-4 rounded-2xl bg-[#0B0E14] border border-slate-800/80 flex flex-wrap items-center justify-between gap-4">
          <div>
            <span className="text-xs text-slate-500 font-medium block">Rolling Window Midpoint</span>
            <div className="text-3xl font-extrabold font-mono text-white tracking-tight mt-0.5">
              {formatPrice(snapshot.mid_price, snapshot.symbol)}
            </div>
          </div>

          <div className="flex items-center gap-3">
            <div className={`flex items-center gap-1 px-3 py-1.5 rounded-xl text-sm font-bold font-mono ${
              isPositive
                ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30'
                : isNegative
                ? 'bg-rose-500/15 text-rose-400 border border-rose-500/30'
                : 'bg-slate-800 text-slate-300'
            }`}>
              {isPositive ? <ArrowUpRight className="w-4 h-4" /> : isNegative ? <ArrowDownRight className="w-4 h-4" /> : null}
              <span>{isPositive ? '+' : ''}{changePct.toFixed(2)}%</span>
            </div>

            <div className={`flex items-center gap-1 px-2.5 py-1.5 rounded-xl text-xs font-medium border ${
              snapshot.is_stale
                ? 'bg-amber-950/60 text-amber-300 border-amber-800/60'
                : 'bg-emerald-950/60 text-emerald-300 border-emerald-800/60'
            }`}>
              <span className={`w-2 h-2 rounded-full ${snapshot.is_stale ? 'bg-amber-400' : 'bg-emerald-400'}`} />
              <span>{snapshot.is_stale ? 'Stale' : 'Live'}</span>
            </div>
          </div>
        </div>

        {/* 24-Hour Monotonic Statistics Grid */}
        <div>
          <h4 className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-3">
            24-Hour Rolling Window Parameters (O(1) Monotonic Deques)
          </h4>
          <div className="grid grid-cols-2 sm:grid-cols-3 gap-3 text-xs font-mono">
            <div className="p-3 rounded-xl bg-slate-900/70 border border-slate-800">
              <span className="text-slate-500 block text-[11px]">Buy (Bid)</span>
              <span className="text-emerald-400 font-bold text-sm">
                {formatPrice(snapshot.buy, snapshot.symbol)}
              </span>
            </div>

            <div className="p-3 rounded-xl bg-slate-900/70 border border-slate-800">
              <span className="text-slate-500 block text-[11px]">Sell (Ask)</span>
              <span className="text-rose-400 font-bold text-sm">
                {formatPrice(snapshot.sell, snapshot.symbol)}
              </span>
            </div>

            <div className="p-3 rounded-xl bg-slate-900/70 border border-slate-800">
              <span className="text-slate-500 block text-[11px]">Spread</span>
              <span className="text-slate-200 font-bold text-sm">
                {spread.text} ({spread.bps.toFixed(1)} bps)
              </span>
            </div>

            <div className="p-3 rounded-xl bg-slate-900/70 border border-slate-800">
              <span className="text-slate-500 block text-[11px]">24h High (Max)</span>
              <span className="text-slate-200 font-bold text-sm">
                {formatPrice(snapshot.high_24h, snapshot.symbol)}
              </span>
            </div>

            <div className="p-3 rounded-xl bg-slate-900/70 border border-slate-800">
              <span className="text-slate-500 block text-[11px]">24h Low (Min)</span>
              <span className="text-slate-200 font-bold text-sm">
                {formatPrice(snapshot.low_24h, snapshot.symbol)}
              </span>
            </div>

            <div className="p-3 rounded-xl bg-slate-900/70 border border-slate-800">
              <span className="text-slate-500 block text-[11px]">Earliest 24h Midpoint (Ref)</span>
              <span className="text-cyan-300 font-bold text-sm">
                {formatPrice(snapshot.reference_price, snapshot.symbol)}
              </span>
            </div>
          </div>
        </div>

        {/* Engine Pipeline Diagnostics */}
        <div className="pt-2 border-t border-slate-800/80 text-xs text-slate-400 space-y-1.5 font-mono">
          <div className="flex justify-between">
            <span>Tick Freshness:</span>
            <span className={snapshot.is_stale ? 'text-amber-400' : 'text-emerald-400'}>
              {formatFreshness(snapshot.freshness_ms)} ({snapshot.freshness_ms.toLocaleString()} ms)
            </span>
          </div>
          <div className="flex justify-between">
            <span>Processed Time:</span>
            <span className="text-slate-300">{new Date(snapshot.processed_time).toISOString()}</span>
          </div>
          <div className="flex justify-between">
            <span>Sequence Number:</span>
            <span className="text-slate-300">{snapshot.sequence}</span>
          </div>
        </div>
      </div>
    </div>
  );
};
