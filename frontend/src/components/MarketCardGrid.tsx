import React from 'react';
import { ArrowUpRight, ArrowDownRight, Clock } from 'lucide-react';
import { MarketSnapshot } from '../types/market';
import { calculateSpread, formatFreshness, formatPrice, getAssetClass } from '../utils/marketUtils';
import { FlashDirection } from '../hooks/usePricingEngine';

interface MarketCardGridProps {
  items: MarketSnapshot[];
  priceFlashes: Record<string, FlashDirection>;
  onSelectSymbol: (symbol: string) => void;
}

export const MarketCardGrid: React.FC<MarketCardGridProps> = ({
  items,
  priceFlashes,
  onSelectSymbol,
}) => {
  return (
    <div className="max-w-7xl mx-auto px-4 lg:px-8 mt-4">
      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3.5 sm:gap-4">
        {items.map((item) => {
          const flash = priceFlashes[item.symbol];
          const changePct = parseFloat(item.change_24h_pct);
          const isPositive = !isNaN(changePct) && changePct > 0;
          const isNegative = !isNaN(changePct) && changePct < 0;
          const spread = calculateSpread(item.buy, item.sell);
          const assetClass = getAssetClass(item.symbol);

          return (
            <div
              key={item.symbol}
              onClick={() => onSelectSymbol(item.symbol)}
              className={`p-4 rounded-2xl bg-[#101520] border border-slate-800/80 shadow-lg shadow-black/20 hover:border-slate-700/80 transition-all cursor-pointer group ${
                flash === 'up' ? 'flash-up' : flash === 'down' ? 'flash-down' : ''
              }`}
            >
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-base font-bold text-white group-hover:text-cyan-300 transition-colors">
                    {item.symbol}
                  </h3>
                  <span className="text-[10px] text-slate-400">
                    {assetClass}
                  </span>
                </div>

                <div className={`flex items-center gap-0.5 px-2 py-0.5 rounded-lg text-xs font-bold font-mono ${
                  isPositive
                    ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30'
                    : isNegative
                    ? 'bg-rose-500/15 text-rose-400 border border-rose-500/30'
                    : 'bg-slate-800 text-slate-400'
                }`}>
                  {isPositive ? <ArrowUpRight className="w-3.5 h-3.5" /> : isNegative ? <ArrowDownRight className="w-3.5 h-3.5" /> : null}
                  <span>{isPositive ? '+' : ''}{changePct.toFixed(2)}%</span>
                </div>
              </div>

              {/* Midpoint Price */}
              <div className="mt-3">
                <span className="text-xs text-slate-500 font-medium block">Midpoint Price</span>
                <div className="text-xl font-bold font-mono text-white tracking-tight">
                  {formatPrice(item.mid_price, item.symbol)}
                </div>
              </div>

              {/* Buy / Sell / Spread */}
              <div className="mt-3 pt-2.5 border-t border-slate-800/60 grid grid-cols-2 gap-2 text-xs font-mono">
                <div>
                  <span className="text-[10px] text-slate-500 block">Buy (Bid)</span>
                  <span className="text-emerald-400 font-medium">{formatPrice(item.buy, item.symbol)}</span>
                </div>
                <div>
                  <span className="text-[10px] text-slate-500 block">Sell (Ask)</span>
                  <span className="text-rose-400 font-medium">{formatPrice(item.sell, item.symbol)}</span>
                </div>
              </div>

              {/* 24h High/Low and Freshness Footer */}
              <div className="mt-3 pt-2.5 border-t border-slate-800/60 flex items-center justify-between text-[11px] text-slate-400">
                <span className="font-mono text-[10px] text-slate-500">
                  H: {formatPrice(item.high_24h, item.symbol)}
                </span>
                <span className="flex items-center gap-1 font-mono text-[10px]">
                  <span className={`w-1.5 h-1.5 rounded-full ${item.is_stale ? 'bg-amber-400' : 'bg-emerald-400'}`} />
                  {item.is_stale ? 'Stale' : formatFreshness(item.freshness_ms)}
                </span>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};
