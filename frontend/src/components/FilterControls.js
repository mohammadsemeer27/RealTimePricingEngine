import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { Search, X, ArrowUpDown, LayoutGrid, ListFilter, SlidersHorizontal } from 'lucide-react';
export const FilterControls = ({ category, setCategory, searchQuery, setSearchQuery, sortField, setSortField, sortOrder, setSortOrder, viewMode, setViewMode, counts, }) => {
    const tabs = [
        { id: 'all', label: 'All Pairs', count: counts.all },
        { id: 'forex-majors', label: 'Forex Majors', count: counts.forexMajors },
        { id: 'forex-crosses', label: 'Cross Pairs', count: counts.forexCrosses },
        { id: 'crypto', label: 'Crypto', count: counts.crypto },
        { id: 'commodities', label: 'Metals & Commodities', count: counts.commodities },
        { id: 'gainers', label: 'Top Gainers' },
        { id: 'losers', label: 'Top Losers' },
        { id: 'stale', label: 'Stale' },
    ];
    const handleSortChange = (e) => {
        const value = e.target.value;
        if (value === sortField) {
            setSortOrder(sortOrder === 'asc' ? 'desc' : 'asc');
        }
        else {
            setSortField(value);
            setSortOrder(value === 'symbol' ? 'asc' : 'desc');
        }
    };
    const toggleOrder = () => {
        setSortOrder(sortOrder === 'asc' ? 'desc' : 'asc');
    };
    return (_jsx("div", { className: "max-w-7xl mx-auto px-4 lg:px-8 mt-6", children: _jsxs("div", { className: "flex flex-col gap-3.5", children: [_jsx("div", { className: "flex items-center gap-1.5 overflow-x-auto pb-1 scrollbar-none text-xs", children: tabs.map((tab) => {
                        const isActive = category === tab.id;
                        return (_jsxs("button", { onClick: () => setCategory(tab.id), className: `whitespace-nowrap px-3.5 py-2 rounded-xl font-medium transition-all flex items-center gap-2 cursor-pointer ${isActive
                                ? 'bg-cyan-500/15 text-cyan-300 border border-cyan-500/40 shadow-sm shadow-cyan-500/10'
                                : 'bg-[#101520] hover:bg-slate-800/80 text-slate-400 hover:text-slate-200 border border-slate-800/80'}`, children: [_jsx("span", { children: tab.label }), tab.count !== undefined && (_jsx("span", { className: `text-[10px] px-1.5 py-0.2 rounded-md ${isActive ? 'bg-cyan-500/30 text-cyan-200' : 'bg-slate-800 text-slate-400'}`, children: tab.count }))] }, tab.id));
                    }) }), _jsxs("div", { className: "flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 bg-[#101520] p-2.5 rounded-2xl border border-slate-800/80", children: [_jsxs("div", { className: "relative flex-1", children: [_jsx(Search, { className: "absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" }), _jsx("input", { type: "text", value: searchQuery, onChange: (e) => setSearchQuery(e.target.value), placeholder: "Search ticker (e.g. BTC, EURUSD, XAU, JPY)...", className: "w-full bg-[#0B0E14] text-white text-xs sm:text-sm pl-9 pr-8 py-2 rounded-xl border border-slate-700/60 focus:outline-none focus:border-cyan-500 focus:ring-1 focus:ring-cyan-500 transition-all placeholder:text-slate-500" }), searchQuery && (_jsx("button", { onClick: () => setSearchQuery(''), className: "absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-200", children: _jsx(X, { className: "w-3.5 h-3.5" }) }))] }), _jsxs("div", { className: "flex items-center gap-2 text-xs", children: [_jsxs("div", { className: "flex items-center gap-1.5 bg-[#0B0E14] px-2.5 py-1.5 rounded-xl border border-slate-700/60", children: [_jsx(SlidersHorizontal, { className: "w-3.5 h-3.5 text-slate-400" }), _jsx("span", { className: "text-slate-400 hidden sm:inline", children: "Sort:" }), _jsxs("select", { value: sortField, onChange: handleSortChange, className: "bg-transparent text-slate-200 focus:outline-none cursor-pointer", children: [_jsx("option", { value: "symbol", className: "bg-[#101520]", children: "Symbol" }), _jsx("option", { value: "mid_price", className: "bg-[#101520]", children: "Midpoint" }), _jsx("option", { value: "change_24h_pct", className: "bg-[#101520]", children: "24h Change %" }), _jsx("option", { value: "spread", className: "bg-[#101520]", children: "Spread" }), _jsx("option", { value: "freshness_ms", className: "bg-[#101520]", children: "Freshness" })] }), _jsx("button", { onClick: toggleOrder, title: `Sort ${sortOrder === 'asc' ? 'Ascending' : 'Descending'}`, className: "p-1 hover:bg-slate-800 rounded text-cyan-400 cursor-pointer", children: _jsx(ArrowUpDown, { className: "w-3.5 h-3.5" }) })] }), _jsxs("div", { className: "flex items-center bg-[#0B0E14] p-1 rounded-xl border border-slate-700/60", children: [_jsx("button", { onClick: () => setViewMode('table'), title: "Table View", className: `p-1.5 rounded-lg transition ${viewMode === 'table' ? 'bg-cyan-500/20 text-cyan-300' : 'text-slate-400 hover:text-slate-200'}`, children: _jsx(ListFilter, { className: "w-4 h-4" }) }), _jsx("button", { onClick: () => setViewMode('grid'), title: "Grid Card View", className: `p-1.5 rounded-lg transition ${viewMode === 'grid' ? 'bg-cyan-500/20 text-cyan-300' : 'text-slate-400 hover:text-slate-200'}`, children: _jsx(LayoutGrid, { className: "w-4 h-4" }) })] })] })] })] }) }));
};
