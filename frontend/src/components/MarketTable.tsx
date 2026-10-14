import React from 'react';
import {
  ArrowUpRight,
  ArrowDownRight,
  Clock,
  ExternalLink,
  ChevronRight,
  TrendingUp,
  AlertCircle
} from 'lucide-react';
import { MarketSnapshot, SortField, SortOrder } from '../types/market';
import {
  calculateSpread,
  formatFreshness,
  formatPrice,
  getAssetClass
} from '../utils/marketUtils';
import { FlashDirection } from '../hooks/usePricingEngine';

interface MarketTableProps {
  items: MarketSnapshot[];
  isLoading: boolean;
  error: string | null;
  priceFlashes: Record<string, FlashDirection>;
  sortField: SortField;
  sortOrder: SortOrder;
  onSort: (field: SortField) => void;
  onSelectSymbol: (symbol: string) => void;
}

export const MarketTable: React.FC<MarketTableProps> = ({
  items,
  isLoading,
  error,
  priceFlashes,
  sortField,
  sortOrder,
  onSort,
  onSelectSymbol,
}) => {
  if (isLoading && items.length === 0) {
    return (
      <div className="max-w-7xl mx-auto px-4 lg:px-8 mt-4">
        <div className="p-8 rounded-2xl bg-[#101520] border border-slate-800/80 animate-pulse space-y-4">
          <div className="h-6 bg-slate-800 rounded w-1/4"></div>
          <div className="space-y-3">
            {[...Array(8)].map((_, i) => (
              <div key={i} className="h-12 bg-slate-800/50 rounded-xl"></div>
            ))}
          </div>
        </div>
      </div>
    );
  }

  if (error && items.length === 0) {
    return (
      <div className="max-w-7xl mx-auto px-4 lg:px-8 mt-4">
        <div className="p-12 text-center rounded-2xl bg-[#101520] border border-rose-900/50">
          <AlertCircle className="w-12 h-12 text-rose-500 mx-auto mb-3" />
          <h3 className="text-lg font-bold text-white">Connection Error</h3>
          <p className="text-sm text-slate-400 mt-1 max-w-md mx-auto">
            {error}
          </p>
        </div>
      </div>
    );
  }

  if (items.length === 0) {
    return (
      <div className="max-w-7xl mx-auto px-4 lg:px-8 mt-4">
        <div className="p-12 text-center rounded-2xl bg-[#101520] border border-slate-800/80">
          <p className="text-slate-400 text-sm">No currency pairs or crypto symbols match your current filters.</p>
        </div>
      </div>
    );
  }

  const renderSortIndicator = (field: SortField) => {
    if (sortField !== field) return null;
    return <span className="text-cyan-400 ml-1">{sortOrder === 'asc' ? '↑' : '↓'}</span>;
  };

  return (
    <div className="max-w-7xl mx-auto px-4 lg:px-8 mt-4">
      <div className="overflow-hidden rounded-2xl bg-[#101520] border border-slate-800/80 shadow-xl shadow-black/40">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs border-collapse">
            <thead>
              <tr className="border-b border-slate-800/90 bg-[#0B0E14]/70 text-slate-400 select-none uppercase tracking-wider text-[11px] font-semibold">
                <th
                  onClick={() => onSort('symbol')}
                  className="py-3.5 px-4 cursor-pointer hover:text-white transition"
                >
                  <div className="flex items-center gap-1">
                    Symbol {renderSortIndicator('symbol')}
                  </div>
                </th>
                <th
                  onClick={() => onSort('mid_price')}
                  className="py-3.5 px-3 text-right cursor-pointer hover:text-white transition"
                >
                  <div className="flex items-center justify-end gap-1">
                    Midpoint {renderSortIndicator('mid_price')}
                  </div>
                </th>
                <th className="py-3.5 px-3 text-right">Buy (Bid)</th>
                <th className="py-3.5 px-3 text-right">Sell (Ask)</th>
                <th
                  onClick={() => onSort('spread')}
                  className="py-3.5 px-3 text-right cursor-pointer hover:text-white transition hidden sm:table-cell"
                >
                  <div className="flex items-center justify-end gap-1">
                    Spread {renderSortIndicator('spread')}
                  </div>
                </th>
                <th className="py-3.5 px-3 text-right hidden md:table-cell">24h High</th>
                <th className="py-3.5 px-3 text-right hidden md:table-cell">24h Low</th>
                <th
                  onClick={() => onSort('change_24h_pct')}
                  className="py-3.5 px-3 text-right cursor-pointer hover:text-white transition"
                >
                  <div className="flex items-center justify-end gap-1">
                    24h Change {renderSortIndicator('change_24h_pct')}
                  </div>
                </th>
                <th className="py-3.5 px-3 text-center hidden lg:table-cell">24h Range</th>
                <th
                  onClick={() => onSort('freshness_ms')}
                  className="py-3.5 px-4 text-right cursor-pointer hover:text-white transition"
                >
                  <div className="flex items-center justify-end gap-1">
                    Freshness {renderSortIndicator('freshness_ms')}
                  </div>
                </th>
                <th className="py-3.5 px-3 text-center w-12">Action</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/50 font-mono">
              {items.map((item) => {
                const flash = priceFlashes[item.symbol];
                const changePct = parseFloat(item.change_24h_pct);
                const isPositive = !isNaN(changePct) && changePct > 0;
                const isNegative = !isNaN(changePct) && changePct < 0;
                const spread = calculateSpread(item.buy, item.sell);
                const assetClass = getAssetClass(item.symbol);

                // Range position calculation
                const high = parseFloat(item.high_24h);
                const low = parseFloat(item.low_24h);
                const mid = parseFloat(item.mid_price);
                let rangePct = 50;
                if (!isNaN(high) && !isNaN(low) && high > low) {
                  rangePct = Math.min(100, Math.max(0, ((mid - low) / (high - low)) * 100));
                }

                return (
                  <tr
                    key={item.symbol}
                    onClick={() => onSelectSymbol(item.symbol)}
                    className={`hover:bg-slate-800/40 transition-colors group cursor-pointer ${
                      flash === 'up' ? 'flash-up' : flash === 'down' ? 'flash-down' : ''
                    }`}
                  >
                    {/* Symbol & Category */}
                    <td className="py-3 px-4 font-sans font-medium text-white">
                      <div className="flex items-center gap-2">
                        <span className="font-bold tracking-tight text-slate-100 group-hover:text-cyan-300 transition-colors">
                          {item.symbol}
                        </span>
                        <span className={`text-[10px] px-1.5 py-0.5 rounded font-mono font-medium ${
                          assetClass === 'Forex Major' ? 'bg-blue-500/10 text-blue-300 border border-blue-500/20' :
                          assetClass === 'Crypto' ? 'bg-purple-500/10 text-purple-300 border border-purple-500/20' :
                          assetClass === 'Commodity' ? 'bg-amber-500/10 text-amber-300 border border-amber-500/20' :
                          'bg-slate-800 text-slate-400 border border-slate-700/50'
                        }`}>
                          {assetClass}
                        </span>
                      </div>
                    </td>

                    {/* Midpoint Price */}
                    <td className="py-3 px-3 text-right font-bold text-white text-sm">
                      <span className="px-1.5 py-0.5 rounded transition">
                        {formatPrice(item.mid_price, item.symbol)}
                      </span>
                    </td>

                    {/* Buy (Bid) */}
                    <td className="py-3 px-3 text-right text-emerald-400 font-medium">
                      {formatPrice(item.buy, item.symbol)}
                    </td>

                    {/* Sell (Ask) */}
                    <td className="py-3 px-3 text-right text-rose-400 font-medium">
                      {formatPrice(item.sell, item.symbol)}
                    </td>

                    {/* Spread */}
                    <td className="py-3 px-3 text-right text-slate-300 hidden sm:table-cell">
                      <div className="flex flex-col items-end">
                        <span>{spread.text}</span>
                        <span className="text-[10px] text-slate-500">
                          {spread.bps.toFixed(1)} bps
                        </span>
                      </div>
                    </td>

                    {/* 24h High */}
                    <td className="py-3 px-3 text-right text-slate-400 hidden md:table-cell">
                      {formatPrice(item.high_24h, item.symbol)}
                    </td>

                    {/* 24h Low */}
                    <td className="py-3 px-3 text-right text-slate-400 hidden md:table-cell">
                      {formatPrice(item.low_24h, item.symbol)}
                    </td>

                    {/* 24h Change % */}
                    <td className="py-3 px-3 text-right">
                      <div className={`inline-flex items-center gap-0.5 px-2 py-0.5 rounded-lg font-bold text-[11px] ${
                        isPositive
                          ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30'
                          : isNegative
                          ? 'bg-rose-500/15 text-rose-400 border border-rose-500/30'
                          : 'bg-slate-800 text-slate-400'
                      }`}>
                        {isPositive ? (
                          <ArrowUpRight className="w-3.5 h-3.5" />
                        ) : isNegative ? (
                          <ArrowDownRight className="w-3.5 h-3.5" />
                        ) : null}
                        <span>{isPositive ? '+' : ''}{changePct.toFixed(2)}%</span>
                      </div>
                    </td>

                    {/* 24h Range Bar */}
                    <td className="py-3 px-3 hidden lg:table-cell">
                      <div className="w-24 mx-auto">
                        <div className="flex justify-between text-[9px] text-slate-500 mb-0.5">
                          <span>L</span>
                          <span>H</span>
                        </div>
                        <div className="h-1.5 w-full bg-slate-800 rounded-full overflow-hidden relative">
                          <div
                            className="absolute top-0 bottom-0 bg-gradient-to-r from-cyan-500 to-blue-500 rounded-full"
                            style={{ left: 0, width: `${rangePct}%` }}
                          />
                        </div>
                      </div>
                    </td>

                    {/* Freshness */}
                    <td className="py-3 px-4 text-right">
                      <div className="inline-flex items-center gap-1.5">
                        <span className={`w-1.5 h-1.5 rounded-full ${
                          item.is_stale ? 'bg-amber-400' : 'bg-emerald-400'
                        }`} />
                        <span className={`text-[11px] font-medium ${
                          item.is_stale ? 'text-amber-300' : 'text-emerald-400'
                        }`}>
                          {item.is_stale ? `Stale (${formatFreshness(item.freshness_ms)})` : formatFreshness(item.freshness_ms)}
                        </span>
                      </div>
                    </td>

                    {/* Quick Detail Modal Trigger */}
                    <td className="py-3 px-3 text-center">
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          onSelectSymbol(item.symbol);
                        }}
                        className="p-1 text-slate-500 hover:text-cyan-400 hover:bg-slate-800 rounded transition"
                        title="View Symbol Window Statistics"
                      >
                        <ChevronRight className="w-4 h-4" />
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};
