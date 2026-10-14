import React from 'react';
import { AlertCircle, AlertTriangle, CheckCircle2, Database, Info } from 'lucide-react';
import { FeedStatus, SystemMetrics } from '../types/market';

interface FeedAlertBannerProps {
  feedStatus: FeedStatus;
  feedStatusDetail: string;
  metrics: SystemMetrics | null;
  totalSymbols: number;
  lastUpdated: Date | null;
}

export const FeedAlertBanner: React.FC<FeedAlertBannerProps> = ({
  feedStatus,
  feedStatusDetail,
  metrics,
  totalSymbols,
  lastUpdated,
}) => {
  if (feedStatus === 'live') {
    return null; // Keep screen clean when completely live and green
  }

  const isAuthFailed = metrics && metrics.auth_failures > 0;

  return (
    <div className="max-w-7xl mx-auto px-4 lg:px-8 mt-4">
      {feedStatus === 'unavailable' ? (
        <div className="flex items-start gap-3 p-4 rounded-xl bg-rose-950/40 border border-rose-800/80 text-rose-200">
          <AlertCircle className="w-5 h-5 text-rose-400 shrink-0 mt-0.5" />
          <div className="text-xs sm:text-sm">
            <h4 className="font-semibold text-rose-300">Backend Pricing Engine Disconnected</h4>
            <p className="mt-1 text-rose-300/80 leading-relaxed">
              Could not communicate with the Go pricing engine at <code className="bg-rose-900/60 px-1 py-0.5 rounded text-rose-200">http://localhost:8082</code>.
              Please ensure your Docker Compose container <code className="bg-rose-900/60 px-1 py-0.5 rounded text-rose-200">pricing-engine</code> is running.
            </p>
          </div>
        </div>
      ) : feedStatus === 'stale' ? (
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 p-3.5 sm:p-4 rounded-xl bg-amber-950/30 border border-amber-800/50 text-amber-200">
          <div className="flex items-start gap-3">
            <AlertTriangle className="w-5 h-5 text-amber-400 shrink-0 mt-0.5" />
            <div className="text-xs sm:text-sm">
              <div className="flex items-center gap-2">
                <h4 className="font-semibold text-amber-300">
                  {isAuthFailed ? 'Upstream Provider Authentication Failed' : 'Feed Marked Stale (>60s)'}
                </h4>
                <span className="text-[11px] px-2 py-0.5 rounded bg-amber-900/60 text-amber-300 border border-amber-700/60">
                  PostgreSQL Seeded State
                </span>
              </div>
              <p className="mt-1 text-amber-300/80 leading-relaxed">
                {isAuthFailed ? (
                  <>
                    The engine attempted upstream WebSocket authentication but the provider returned code <code className="bg-amber-900/60 px-1 py-0.5 rounded text-amber-200">4003 (auth failed)</code>.
                    Displaying <strong>{totalSymbols}</strong> cached snapshots restored from PostgreSQL. Prices are not live-ticking.
                  </>
                ) : (
                  feedStatusDetail
                )}
              </p>
            </div>
          </div>

          <div className="shrink-0 flex items-center gap-2 text-xs bg-amber-950/60 px-3 py-1.5 rounded-lg border border-amber-800/40 text-amber-400">
            <Database className="w-3.5 h-3.5 text-amber-400" />
            <span>Cached Snapshot</span>
            {lastUpdated && (
              <span className="text-amber-500 font-mono">
                {lastUpdated.toLocaleTimeString()}
              </span>
            )}
          </div>
        </div>
      ) : feedStatus === 'mock' ? (
        <div className="flex items-center gap-3 p-3.5 rounded-xl bg-purple-950/30 border border-purple-800/50 text-purple-200 text-xs sm:text-sm">
          <Info className="w-5 h-5 text-purple-400 shrink-0" />
          <p>
            <strong>Built-in Mock Feed:</strong> Generating realistic simulated ticks for 60 symbols (~100ms interval).
          </p>
        </div>
      ) : null}
    </div>
  );
};
