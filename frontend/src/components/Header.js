import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { useEffect, useState } from 'react';
import { Activity, Wifi, WifiOff, RefreshCw, Clock, Pause, Play, Server, Zap, BarChart3 } from 'lucide-react';
export const Header = ({ feedStatus, feedStatusDetail, isBackendHealthy, health, wsStatus, lastUpdated, refreshNow, isPollingPaused, setIsPollingPaused, onOpenMetrics, }) => {
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
        }
        finally {
            setTimeout(() => setIsRefreshing(false), 500);
        }
    };
    const formatUptime = (seconds) => {
        if (seconds === undefined)
            return '-';
        if (seconds < 60)
            return `${seconds}s`;
        if (seconds < 3600)
            return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
        const h = Math.floor(seconds / 3600);
        const m = Math.floor((seconds % 3600) / 60);
        return `${h}h ${m}m`;
    };
    return (_jsx("header", { className: "sticky top-0 z-40 bg-[#0B0E14]/90 backdrop-blur-md border-b border-slate-800/80 px-4 lg:px-8 py-3.5 transition-all", children: _jsxs("div", { className: "max-w-7xl mx-auto flex flex-col md:flex-row md:items-center justify-between gap-3", children: [_jsxs("div", { className: "flex items-center gap-3", children: [_jsxs("div", { className: "relative flex items-center justify-center w-10 h-10 rounded-xl bg-gradient-to-tr from-cyan-600 via-blue-600 to-indigo-600 shadow-lg shadow-cyan-500/20 ring-1 ring-white/20", children: [_jsx(Zap, { className: "w-5 h-5 text-white animate-pulse" }), _jsx("div", { className: "absolute -bottom-0.5 -right-0.5 w-3 h-3 rounded-full bg-emerald-500 border-2 border-[#0B0E14]" })] }), _jsxs("div", { children: [_jsxs("div", { className: "flex items-center gap-2.5", children: [_jsx("h1", { className: "text-xl font-bold tracking-tight text-white m-0 flex items-center gap-2", children: "Real-Time Pricing Engine" }), _jsx("span", { className: "text-[10px] font-semibold tracking-wider uppercase px-2 py-0.5 rounded-full bg-slate-800 text-cyan-400 border border-cyan-500/30", children: "PRO v1.0" })] }), _jsxs("p", { className: "text-xs text-slate-400 flex items-center gap-1.5 mt-0.5", children: [_jsx("span", { className: "inline-block w-1.5 h-1.5 rounded-full bg-emerald-400" }), "High-Frequency FX & Crypto Market Data Pipeline"] })] })] }), _jsxs("div", { className: "flex flex-wrap items-center gap-2 sm:gap-3 text-xs", children: [_jsxs("div", { title: feedStatusDetail, className: `flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg border font-medium cursor-help transition-all ${feedStatus === 'live'
                                ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30 shadow-sm shadow-emerald-500/10'
                                : feedStatus === 'stale'
                                    ? 'bg-amber-500/10 text-amber-300 border-amber-500/30'
                                    : feedStatus === 'mock'
                                        ? 'bg-purple-500/10 text-purple-300 border-purple-500/30'
                                        : 'bg-rose-500/10 text-rose-400 border-rose-500/30 animate-pulse'}`, children: [_jsx("span", { className: `w-2 h-2 rounded-full ${feedStatus === 'live' ? 'bg-emerald-400 animate-ping' :
                                        feedStatus === 'stale' ? 'bg-amber-400' :
                                            feedStatus === 'mock' ? 'bg-purple-400' : 'bg-rose-400'}` }), _jsx("span", { className: "capitalize font-semibold", children: feedStatus === 'live' ? 'Live Stream' :
                                        feedStatus === 'stale' ? 'Stale Feed' :
                                            feedStatus === 'mock' ? 'Mock Feed' : 'Unavailable' })] }), _jsxs("div", { title: isBackendHealthy ? 'Backend Go REST API on port 8082 is healthy' : 'Cannot reach backend REST API', className: `flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg border font-medium ${isBackendHealthy
                                ? 'bg-slate-900 text-slate-300 border-slate-700/80'
                                : 'bg-rose-950/40 text-rose-300 border-rose-700/60'}`, children: [_jsx(Server, { className: `w-3.5 h-3.5 ${isBackendHealthy ? 'text-cyan-400' : 'text-rose-400'}` }), _jsx("span", { className: "hidden sm:inline", children: "REST:" }), _jsx("span", { className: isBackendHealthy ? 'text-emerald-400' : 'text-rose-400', children: isBackendHealthy ? '8082 OK' : 'Down' }), health?.uptime_seconds !== undefined && (_jsx("span", { className: "text-[10px] text-slate-500 border-l border-slate-700 pl-1.5 ml-0.5", children: formatUptime(health.uptime_seconds) }))] }), _jsxs("div", { title: `WebSocket protocol status: ${wsStatus}`, className: "hidden lg:flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-slate-900 text-slate-300 border border-slate-700/80 font-medium", children: [wsStatus === 'connected' ? (_jsx(Wifi, { className: "w-3.5 h-3.5 text-emerald-400" })) : wsStatus === 'connecting' ? (_jsx(Activity, { className: "w-3.5 h-3.5 text-amber-400 animate-spin" })) : (_jsx(WifiOff, { className: "w-3.5 h-3.5 text-slate-500" })), _jsx("span", { children: "WS:" }), _jsx("span", { className: `capitalize ${wsStatus === 'connected' ? 'text-emerald-400' :
                                        wsStatus === 'connecting' ? 'text-amber-400' : 'text-slate-400'}`, children: wsStatus })] }), _jsxs("div", { className: "hidden xl:flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-slate-900 text-slate-300 border border-slate-800 font-mono text-[11px]", children: [_jsx(Clock, { className: "w-3.5 h-3.5 text-cyan-400" }), _jsx("span", { children: utcTime })] }), _jsxs("button", { onClick: onOpenMetrics, title: "Inspect Go Engine Internal Metrics & Diagnostics", className: "flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-slate-800/80 hover:bg-slate-700 text-slate-200 border border-slate-700/60 font-medium transition", children: [_jsx(BarChart3, { className: "w-3.5 h-3.5 text-cyan-400" }), _jsx("span", { className: "hidden sm:inline", children: "Metrics" })] }), _jsx("button", { onClick: () => setIsPollingPaused(!isPollingPaused), title: isPollingPaused ? 'Resume 3s REST Auto-Refresh' : 'Pause Auto-Refresh', className: `p-1.5 rounded-lg border transition ${isPollingPaused
                                ? 'bg-amber-500/20 text-amber-300 border-amber-500/40 hover:bg-amber-500/30'
                                : 'bg-slate-800/80 text-slate-300 border-slate-700 hover:bg-slate-700'}`, children: isPollingPaused ? _jsx(Play, { className: "w-3.5 h-3.5" }) : _jsx(Pause, { className: "w-3.5 h-3.5" }) }), _jsxs("button", { onClick: handleManualRefresh, disabled: isRefreshing, title: "Force immediate reload from backend /prices", className: "flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-cyan-600 hover:bg-cyan-500 active:scale-95 text-white font-medium shadow-md shadow-cyan-600/20 transition disabled:opacity-50 cursor-pointer", children: [_jsx(RefreshCw, { className: `w-3.5 h-3.5 ${isRefreshing ? 'animate-spin' : ''}` }), _jsx("span", { children: "Refresh" })] })] })] }) }));
};
