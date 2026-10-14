export interface MarketSnapshot {
  symbol: string;
  buy: string;
  sell: string;
  mid_price: string;
  high_24h: string;
  low_24h: string;
  change_24h_pct: string;
  reference_price: string;
  event_time: string;
  processed_time: string;
  sequence: number;
  freshness_ms: number;
  is_stale: boolean;
}

export interface PricesApiResponse {
  count: number;
  data: Record<string, MarketSnapshot>;
}

export interface MarketDelta {
  t: string;       // "d" for delta
  s: string;       // symbol
  b: string;       // buy price
  a: string;       // sell (ask) price
  m: string;       // mid price
  h: string;       // 24h high
  l: string;       // 24h low
  c: string;       // 24h change pct
  e: number;       // unix timestamp ms
  seq: number;     // sequence
}

export interface ServerAck {
  type: 'ack';
  action: string;
  status: string;
  symbols?: string[];
}

export interface ServerSnapshotMsg {
  type: 'snapshot';
  data: MarketSnapshot;
}

export interface ServerPongMsg {
  type: 'pong';
  action?: string;
  status?: string;
}

export interface ServerErrorMsg {
  type: 'error';
  message: string;
}

export interface SystemMetrics {
  incoming_ticks: number;
  processed_ticks: number;
  dropped_events: number;
  dropped_by_reason: Record<string, number>;
  upstream_reconnects: number;
  auth_failures: number;
  downstream_clients: number;
  database_errors: number;
  database_writes: number;
  outbound_messages: number;
  avg_latency_micros: number;
  avg_payload_bytes: number;
  max_payload_bytes: number;
}

export interface HealthStatus {
  status: string;
  uptime_seconds?: number;
}

export type FeedStatus = 'live' | 'stale' | 'mock' | 'unavailable' | 'connecting';

export type AssetCategory = 'all' | 'forex-majors' | 'forex-crosses' | 'crypto' | 'commodities' | 'gainers' | 'losers' | 'stale';

export type SortField = 'symbol' | 'mid_price' | 'change_24h_pct' | 'spread' | 'freshness_ms';
export type SortOrder = 'asc' | 'desc';
