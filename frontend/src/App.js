import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { useState, useMemo } from 'react';
import { Header } from './components/Header';
import { FeedAlertBanner } from './components/FeedAlertBanner';
import { SummaryCards } from './components/SummaryCards';
import { FilterControls } from './components/FilterControls';
import { MarketTable } from './components/MarketTable';
import { MarketCardGrid } from './components/MarketCardGrid';
import { SymbolDetailModal } from './components/SymbolDetailModal';
import { MetricsDrawer } from './components/MetricsDrawer';
import { usePricingEngine } from './hooks/usePricingEngine';
import { COMMODITIES, CRYPTO_SYMBOLS, filterSymbols, FOREX_MAJORS } from './utils/marketUtils';
export function App() {
    const { snapshots, symbolsList, feedStatus, feedStatusDetail, isBackendHealthy, health, metrics, isLoading, error, lastUpdated, wsStatus, priceFlashes, refreshNow, isPollingPaused, setIsPollingPaused, stats, } = usePricingEngine(3000);
    const [category, setCategory] = useState('all');
    const [searchQuery, setSearchQuery] = useState('');
    const [sortField, setSortField] = useState('symbol');
    const [sortOrder, setSortOrder] = useState('asc');
    const [viewMode, setViewMode] = useState('table');
    const [selectedSymbol, setSelectedSymbol] = useState(null);
    const [isMetricsOpen, setIsMetricsOpen] = useState(false);
    // Compute category counts
    const categoryCounts = useMemo(() => {
        let forexMajors = 0;
        let forexCrosses = 0;
        let crypto = 0;
        let commodities = 0;
        symbolsList.forEach((item) => {
            const s = item.symbol.toUpperCase();
            if (FOREX_MAJORS.has(s))
                forexMajors++;
            else if (CRYPTO_SYMBOLS.has(s))
                crypto++;
            else if (COMMODITIES.has(s))
                commodities++;
            else
                forexCrosses++;
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
    return (_jsxs("div", { className: "min-h-screen bg-[#0B0E14] text-slate-100 flex flex-col font-sans", children: [_jsx(Header, { feedStatus: feedStatus, feedStatusDetail: feedStatusDetail, isBackendHealthy: isBackendHealthy, health: health, wsStatus: wsStatus, lastUpdated: lastUpdated, refreshNow: refreshNow, isPollingPaused: isPollingPaused, setIsPollingPaused: setIsPollingPaused, onOpenMetrics: () => setIsMetricsOpen(true) }), _jsxs("main", { className: "flex-1 pb-16", children: [_jsx(FeedAlertBanner, { feedStatus: feedStatus, feedStatusDetail: feedStatusDetail, metrics: metrics, totalSymbols: symbolsList.length, lastUpdated: lastUpdated }), _jsx(SummaryCards, { stats: stats, metrics: metrics, onSelectSymbol: (sym) => setSelectedSymbol(sym) }), _jsx(FilterControls, { category: category, setCategory: setCategory, searchQuery: searchQuery, setSearchQuery: setSearchQuery, sortField: sortField, setSortField: setSortField, sortOrder: sortOrder, setSortOrder: setSortOrder, viewMode: viewMode, setViewMode: setViewMode, counts: categoryCounts }), viewMode === 'table' ? (_jsx(MarketTable, { items: filteredAndSortedItems, isLoading: isLoading, error: error, priceFlashes: priceFlashes, sortField: sortField, sortOrder: sortOrder, onSort: (field) => {
                            if (sortField === field) {
                                setSortOrder(sortOrder === 'asc' ? 'desc' : 'asc');
                            }
                            else {
                                setSortField(field);
                                setSortOrder('asc');
                            }
                        }, onSelectSymbol: (sym) => setSelectedSymbol(sym) })) : (_jsx(MarketCardGrid, { items: filteredAndSortedItems, priceFlashes: priceFlashes, onSelectSymbol: (sym) => setSelectedSymbol(sym) }))] }), _jsx("footer", { className: "border-t border-slate-800/80 bg-[#0B0E14] px-4 lg:px-8 py-5 text-xs text-slate-500", children: _jsxs("div", { className: "max-w-7xl mx-auto flex flex-col sm:flex-row items-center justify-between gap-3", children: [_jsxs("div", { className: "flex items-center gap-2", children: [_jsx("span", { className: "w-2 h-2 rounded-full bg-cyan-500" }), _jsxs("span", { children: ["Real-Time Pricing Engine \u2022 REST API on ", _jsx("code", { className: "text-slate-400", children: "http://localhost:8082" })] })] }), _jsxs("div", { className: "flex items-center gap-4 text-slate-400", children: [_jsx("span", { children: "Sub-Millisecond Processing SLA" }), _jsx("span", { children: "\u2022" }), _jsx("span", { children: "PostgreSQL Persistence" }), _jsx("span", { children: "\u2022" }), _jsx("span", { children: "O(1) Rolling Window" })] })] }) }), _jsx(SymbolDetailModal, { snapshot: selectedSnapshot, onClose: () => setSelectedSymbol(null) }), _jsx(MetricsDrawer, { isOpen: isMetricsOpen, onClose: () => setIsMetricsOpen(false), metrics: metrics, serverUptime: health?.uptime_seconds })] }));
}
export default App;
