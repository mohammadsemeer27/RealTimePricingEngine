import { useEffect, useRef, useState, useCallback, useMemo } from 'react';
import { apiService, ApiError } from '../services/api';
export function usePricingEngine(pollingIntervalMs = 3000) {
    const [snapshots, setSnapshots] = useState({});
    const [health, setHealth] = useState(null);
    const [metrics, setMetrics] = useState(null);
    const [isLoading, setIsLoading] = useState(true);
    const [error, setError] = useState(null);
    const [lastUpdated, setLastUpdated] = useState(null);
    const [isBackendHealthy, setIsBackendHealthy] = useState(false);
    const [feedStatus, setFeedStatus] = useState('connecting');
    const [feedStatusDetail, setFeedStatusDetail] = useState('Initializing engine connection...');
    const [wsStatus, setWsStatus] = useState('disconnected');
    const [priceFlashes, setPriceFlashes] = useState({});
    const [isPollingPaused, setIsPollingPaused] = useState(false);
    const wsRef = useRef(null);
    const reconnectTimeoutRef = useRef(null);
    const flashTimeoutsRef = useRef({});
    const snapshotsRef = useRef({});
    snapshotsRef.current = snapshots;
    // Trigger flash animation for a symbol
    const triggerFlash = useCallback((symbol, direction) => {
        if (flashTimeoutsRef.current[symbol]) {
            clearTimeout(flashTimeoutsRef.current[symbol]);
        }
        setPriceFlashes((prev) => ({ ...prev, [symbol]: direction }));
        flashTimeoutsRef.current[symbol] = setTimeout(() => {
            setPriceFlashes((prev) => {
                const next = { ...prev };
                delete next[symbol];
                return next;
            });
            delete flashTimeoutsRef.current[symbol];
        }, 1200);
    }, []);
    // Update a snapshot with a compact delta from WebSocket
    const applyDelta = useCallback((delta) => {
        setSnapshots((prev) => {
            const existing = prev[delta.s];
            const prevMid = existing ? parseFloat(existing.mid_price) : 0;
            const newMid = parseFloat(delta.m);
            if (prevMid > 0 && newMid !== prevMid) {
                triggerFlash(delta.s, newMid > prevMid ? 'up' : 'down');
            }
            const updatedSnapshot = {
                symbol: delta.s,
                buy: delta.b,
                sell: delta.a,
                mid_price: delta.m,
                high_24h: delta.h,
                low_24h: delta.l,
                change_24h_pct: delta.c,
                reference_price: existing ? existing.reference_price : delta.m,
                event_time: new Date(delta.e).toISOString(),
                processed_time: new Date().toISOString(),
                sequence: delta.seq,
                freshness_ms: Math.max(0, Date.now() - delta.e),
                is_stale: false, // Actively streaming tick
            };
            return {
                ...prev,
                [delta.s]: updatedSnapshot,
            };
        });
        setLastUpdated(new Date());
    }, [triggerFlash]);
    // REST refresh function
    const fetchAllData = useCallback(async () => {
        try {
            // 1. Fetch prices
            const pricesRes = await apiService.getPrices();
            const newSnapshots = pricesRes.data || {};
            // Compare mid prices for flash animations during REST refresh
            Object.values(newSnapshots).forEach((item) => {
                const prev = snapshotsRef.current[item.symbol];
                if (prev) {
                    const oldMid = parseFloat(prev.mid_price);
                    const newMid = parseFloat(item.mid_price);
                    if (oldMid > 0 && newMid !== oldMid) {
                        triggerFlash(item.symbol, newMid > oldMid ? 'up' : 'down');
                    }
                }
            });
            setSnapshots(newSnapshots);
            setError(null);
            setLastUpdated(new Date());
            setIsBackendHealthy(true);
            // 2. Fetch health & metrics in parallel
            const [healthData, metricsData] = await Promise.allSettled([
                apiService.getHealth(),
                apiService.getMetrics(),
            ]);
            let currentMetrics = null;
            if (healthData.status === 'fulfilled') {
                setHealth(healthData.value);
            }
            if (metricsData.status === 'fulfilled') {
                currentMetrics = metricsData.value;
                setMetrics(currentMetrics);
            }
            // 3. Determine feed status accurately
            const items = Object.values(newSnapshots);
            if (items.length === 0) {
                setFeedStatus('stale');
                setFeedStatusDetail('Engine connected, but no symbols currently loaded.');
            }
            else {
                const allStale = items.every((i) => i.is_stale);
                const someStale = items.some((i) => i.is_stale);
                const avgFreshness = items.reduce((acc, curr) => acc + curr.freshness_ms, 0) / items.length;
                if (currentMetrics && currentMetrics.auth_failures > 0 && currentMetrics.incoming_ticks === 0) {
                    setFeedStatus('stale');
                    setFeedStatusDetail(`Upstream authentication failed (Code 4003). Preserving ${items.length} PostgreSQL historical symbols.`);
                }
                else if (allStale || avgFreshness > 60000) {
                    setFeedStatus('stale');
                    setFeedStatusDetail(`Feed is Stale (Avg freshness ${Math.round(avgFreshness / 1000)}s ago). Showing PostgreSQL cached data.`);
                }
                else if (currentMetrics && currentMetrics.incoming_ticks > 0) {
                    setFeedStatus('live');
                    setFeedStatusDetail(`Live Feed Active — ${currentMetrics.processed_ticks.toLocaleString()} ticks processed.`);
                }
                else if (!someStale) {
                    setFeedStatus('live');
                    setFeedStatusDetail('Feed is Active & Fresh.');
                }
                else {
                    setFeedStatus('stale');
                    setFeedStatusDetail('Partially stale market observations.');
                }
            }
        }
        catch (err) {
            setIsBackendHealthy(false);
            setFeedStatus('unavailable');
            const msg = err instanceof ApiError ? err.message : 'Failed to connect to backend engine';
            setError(msg);
            setFeedStatusDetail(`Backend unreachable on http://localhost:8082 (${msg})`);
        }
        finally {
            setIsLoading(false);
        }
    }, [triggerFlash]);
    // Connect WebSocket
    const connectWebSocket = useCallback(() => {
        // Build WS URL matching current window location or direct host
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        // Use Vite proxy endpoint /ws if hosted on dev server, or direct port
        const wsUrl = `${protocol}//${window.location.host}/ws`;
        try {
            setWsStatus('connecting');
            const ws = new WebSocket(wsUrl);
            wsRef.current = ws;
            ws.onopen = () => {
                setWsStatus('connected');
                // Subscribe to all available symbols
                const symbolsToSub = Object.keys(snapshotsRef.current);
                const initialSubs = symbolsToSub.length > 0 ? symbolsToSub : [
                    'EURUSD', 'GBPUSD', 'USDJPY', 'BTCUSD', 'ETHUSD', 'XAUUSD'
                ];
                ws.send(JSON.stringify({
                    action: 'subscribe',
                    symbols: initialSubs,
                }));
            };
            ws.onmessage = (event) => {
                const raw = event.data;
                if (typeof raw !== 'string')
                    return;
                // Support newline-delimited coalesced frames
                const lines = raw.split('\n');
                for (const line of lines) {
                    const trimmed = line.trim();
                    if (!trimmed)
                        continue;
                    try {
                        const parsed = JSON.parse(trimmed);
                        // Handle compact delta
                        if (parsed.t === 'd' && parsed.s) {
                            applyDelta(parsed);
                        }
                        else if (parsed.type === 'snapshot' && parsed.data) {
                            const snap = parsed.data;
                            setSnapshots((prev) => ({ ...prev, [snap.symbol]: snap }));
                        }
                    }
                    catch {
                        // Ignore non-json frames
                    }
                }
            };
            ws.onerror = () => {
                setWsStatus('error');
            };
            ws.onclose = () => {
                setWsStatus('disconnected');
                // Attempt reconnect in 5 seconds
                if (!reconnectTimeoutRef.current) {
                    reconnectTimeoutRef.current = setTimeout(() => {
                        reconnectTimeoutRef.current = null;
                        connectWebSocket();
                    }, 5000);
                }
            };
        }
        catch {
            setWsStatus('error');
        }
    }, [applyDelta]);
    // Initialize REST data fetch and WebSocket connection
    useEffect(() => {
        void fetchAllData();
        // Initial WS connect
        connectWebSocket();
        return () => {
            if (wsRef.current) {
                wsRef.current.close();
            }
            if (reconnectTimeoutRef.current) {
                clearTimeout(reconnectTimeoutRef.current);
            }
        };
    }, [fetchAllData, connectWebSocket]);
    // Periodic REST Polling
    useEffect(() => {
        if (isPollingPaused)
            return;
        const timer = setInterval(() => {
            void fetchAllData();
        }, pollingIntervalMs);
        return () => clearInterval(timer);
    }, [fetchAllData, pollingIntervalMs, isPollingPaused]);
    // Derived symbols list sorted
    const symbolsList = useMemo(() => {
        return Object.values(snapshots);
    }, [snapshots]);
    // Calculated Market Statistics
    const stats = useMemo(() => {
        const list = symbolsList;
        const totalSymbols = list.length;
        if (totalSymbols === 0) {
            return {
                totalSymbols: 0,
                staleCount: 0,
                freshCount: 0,
                avgFreshnessMs: 0,
                topGainer: null,
                topLoser: null,
                highestSpreadSymbol: null,
            };
        }
        let staleCount = 0;
        let freshCount = 0;
        let totalFreshness = 0;
        let topGainer = null;
        let topLoser = null;
        let highestSpreadSymbol = null;
        list.forEach((snap) => {
            if (snap.is_stale) {
                staleCount++;
            }
            else {
                freshCount++;
            }
            totalFreshness += snap.freshness_ms || 0;
            const pctChange = parseFloat(snap.change_24h_pct);
            if (!isNaN(pctChange)) {
                if (!topGainer || pctChange > parseFloat(topGainer.change_24h_pct)) {
                    topGainer = snap;
                }
                if (!topLoser || pctChange < parseFloat(topLoser.change_24h_pct)) {
                    topLoser = snap;
                }
            }
            const b = parseFloat(snap.buy);
            const s = parseFloat(snap.sell);
            if (b > 0 && s > 0) {
                const spreadBps = ((s - b) / b) * 10000;
                if (!highestSpreadSymbol || spreadBps > highestSpreadSymbol.spreadBps) {
                    highestSpreadSymbol = { symbol: snap.symbol, spreadBps };
                }
            }
        });
        const avgFreshnessMs = totalSymbols > 0 ? Math.round(totalFreshness / totalSymbols) : 0;
        return {
            totalSymbols,
            staleCount,
            freshCount,
            avgFreshnessMs,
            topGainer,
            topLoser,
            highestSpreadSymbol,
        };
    }, [symbolsList]);
    return {
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
        refreshNow: fetchAllData,
        isPollingPaused,
        setIsPollingPaused,
        stats,
    };
}
