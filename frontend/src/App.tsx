import React, { useState, useMemo } from 'react';
import { Header } from './components/Header';
import { FeedAlertBanner } from './components/FeedAlertBanner';
import { SummaryCards } from './components/SummaryCards';
import { FilterControls } from './components/FilterControls';
import { MarketTable } from './components/MarketTable';
import { MarketCardGrid } from './components/MarketCardGrid';
import { SymbolDetailModal } from './components/SymbolDetailModal';
import { MetricsDrawer } from './components/MetricsDrawer';
import { usePricingEngine } from './hooks/usePricingEngine';
import { AssetCategory, SortField, SortOrder } from './types/market';
import {
  COMMODITIES,
  CRYPTO_SYMBOLS,
  filterSymbols,
  FOREX_MAJORS
} from './utils/marketUtils';

export function App() {
  const {
    snapshots,
    symbolsList,
    feedStatus,
    feedStatusDetail,
    isBackendHealthy,
    health,
    metrics,
    isLoading,
    error,
    lastUpdated,
    wsStatus,
    priceFlashes,
    refreshNow,
    isPollingPaused,
    setIsPollingPaused,
    stats,
  } = usePricingEngine(3000);

  const [category, setCategory] = useState<AssetCategory>('all');
  const [searchQuery, setSearchQuery] = useState<string>('');
  const [sortField, setSortField] = useState<SortField>('symbol');
  const [sortOrder, setSortOrder] = useState<SortOrder>('asc');
  const [viewMode, setViewMode] = useState<'table' | 'grid'>('table');
  const [selectedSymbol, setSelectedSymbol] = useState<string | null>(null);
  const [isMetricsOpen, setIsMetricsOpen] = useState<boolean>(false);

  // Compute category counts
  const categoryCounts = useMemo(() => {
    let forexMajors = 0;
    let forexCrosses = 0;
    let crypto = 0;
    let commodities = 0;

    symbolsList.forEach((item) => {
      const s = item.symbol.toUpperCase();
      if (FOREX_MAJORS.has(s)) forexMajors++;
      else if (CRYPTO_SYMBOLS.has(s)) crypto++;
      else if (COMMODITIES.has(s)) commodities++;
      else forexCrosses++;
    });

    return {
      all: symbolsList.length,
      forexMajors,
      forexCrosses,
      crypto,
      commodities,
    };
  }, [symbolsList]);

  // Filter and sort items
  const filteredAndSortedItems = useMemo(() => {
    const filtered = filterSymbols(symbolsList, category, searchQuery);

    return filtered.sort((a, b) => {
      let comparison = 0;
      switch (sortField) {
        case 'symbol':
          comparison = a.symbol.localeCompare(b.symbol);
          break;
        case 'mid_price':
          comparison = parseFloat(a.mid_price) - parseFloat(b.mid_price);
          break;
        case 'change_24h_pct':
          comparison = parseFloat(a.change_24h_pct) - parseFloat(b.change_24h_pct);
          break;
        case 'spread': {
          const spreadA = parseFloat(a.sell) - parseFloat(a.buy);
          const spreadB = parseFloat(b.sell) - parseFloat(b.buy);
          comparison = spreadA - spreadB;
          break;
        }
        case 'freshness_ms':
          comparison = (a.freshness_ms || 0) - (b.freshness_ms || 0);
          break;
      }

      return sortOrder === 'asc' ? comparison : -comparison;
    });
  }, [symbolsList, category, searchQuery, sortField, sortOrder]);

  const selectedSnapshot = selectedSymbol ? snapshots[selectedSymbol] || null : null;

  return (
    <div className="min-h-screen bg-[#0B0E14] text-slate-100 flex flex-col font-sans">
      {/* Sticky Header with Navigation and Engine Indicators */}
      <Header
        feedStatus={feedStatus}
        feedStatusDetail={feedStatusDetail}
        isBackendHealthy={isBackendHealthy}
        health={health}
        wsStatus={wsStatus}
        lastUpdated={lastUpdated}
        refreshNow={refreshNow}
        isPollingPaused={isPollingPaused}
        setIsPollingPaused={setIsPollingPaused}
        onOpenMetrics={() => setIsMetricsOpen(true)}
      />

      <main className="flex-1 pb-16">
        {/* Feed Status and Health Banner */}
        <FeedAlertBanner
          feedStatus={feedStatus}
          feedStatusDetail={feedStatusDetail}
          metrics={metrics}
          totalSymbols={symbolsList.length}
          lastUpdated={lastUpdated}
        />

        {/* Real-Time Summary Metric Cards */}
        <SummaryCards
          stats={stats}
          metrics={metrics}
          onSelectSymbol={(sym) => setSelectedSymbol(sym)}
        />

        {/* Filter, Search & Layout Controls */}
        <FilterControls
          category={category}
          setCategory={setCategory}
          searchQuery={searchQuery}
          setSearchQuery={setSearchQuery}
          sortField={sortField}
          setSortField={setSortField}
          sortOrder={sortOrder}
          setSortOrder={setSortOrder}
          viewMode={viewMode}
          setViewMode={setViewMode}
          counts={categoryCounts}
        />

        {/* Main Market View (Table or Card Grid) */}
        {viewMode === 'table' ? (
          <MarketTable
            items={filteredAndSortedItems}
            isLoading={isLoading}
            error={error}
            priceFlashes={priceFlashes}
            sortField={sortField}
            sortOrder={sortOrder}
            onSort={(field) => {
              if (sortField === field) {
                setSortOrder(sortOrder === 'asc' ? 'desc' : 'asc');
              } else {
                setSortField(field);
                setSortOrder('asc');
              }
            }}
            onSelectSymbol={(sym) => setSelectedSymbol(sym)}
          />
        ) : (
          <MarketCardGrid
            items={filteredAndSortedItems}
            priceFlashes={priceFlashes}
            onSelectSymbol={(sym) => setSelectedSymbol(sym)}
          />
        )}
      </main>

      {/* Footer */}
      <footer className="border-t border-slate-800/80 bg-[#0B0E14] px-4 lg:px-8 py-5 text-xs text-slate-500">
        <div className="max-w-7xl mx-auto flex flex-col sm:flex-row items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <span className="w-2 h-2 rounded-full bg-cyan-500"></span>
            <span>Real-Time Pricing Engine • REST API on <code className="text-slate-400">http://localhost:8082</code></span>
          </div>
          <div className="flex items-center gap-4 text-slate-400">
            <span>Sub-Millisecond Processing SLA</span>
            <span>•</span>
            <span>PostgreSQL Persistence</span>
            <span>•</span>
            <span>O(1) Rolling Window</span>
          </div>
        </div>
      </footer>

      {/* Modals & Drawers */}
      <SymbolDetailModal
        snapshot={selectedSnapshot}
        onClose={() => setSelectedSymbol(null)}
      />

      <MetricsDrawer
        isOpen={isMetricsOpen}
        onClose={() => setIsMetricsOpen(false)}
        metrics={metrics}
        serverUptime={health?.uptime_seconds}
      />
    </div>
  );
}

export default App;
