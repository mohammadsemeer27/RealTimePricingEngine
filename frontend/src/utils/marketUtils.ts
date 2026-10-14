import { AssetCategory, MarketSnapshot } from '../types/market';

export const FOREX_MAJORS = new Set([
  'EURUSD', 'GBPUSD', 'USDJPY', 'USDCHF', 'USDCAD', 'AUDUSD', 'NZDUSD'
]);

export const COMMODITIES = new Set([
  'XAUUSD', 'XAGUSD'
]);

export const CRYPTO_SYMBOLS = new Set([
  'BTCUSD', 'ETHUSD', 'BNBUSD', 'SOLUSD', 'XRPUSD', 'ADAUSD',
  'DOGUSD', 'DOTUSD', 'LTCUSD', 'BCHUSD', 'XLMUSD', 'TRXUSD',
  'UNIUSD', 'FILUSD', 'AVXUSD'
]);

export function getAssetClass(symbol: string): 'Forex Major' | 'Forex Cross' | 'Crypto' | 'Commodity' {
  const s = symbol.toUpperCase();
  if (FOREX_MAJORS.has(s)) return 'Forex Major';
  if (COMMODITIES.has(s)) return 'Commodity';
  if (CRYPTO_SYMBOLS.has(s)) return 'Crypto';
  return 'Forex Cross';
}

export function formatPrice(value: string | number, symbol: string): string {
  const num = typeof value === 'string' ? parseFloat(value) : value;
  if (isNaN(num)) return '-';

  const s = symbol.toUpperCase();
  if (s === 'BTCUSD' || s === 'ETHUSD' || s === 'XAUUSD') {
    return num.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  }
  if (s.endsWith('JPY')) {
    return num.toLocaleString('en-US', { minimumFractionDigits: 3, maximumFractionDigits: 3 });
  }
  if (CRYPTO_SYMBOLS.has(s) && num > 100) {
    return num.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  }
  // Standard forex typically 4-5 decimals
  return num.toLocaleString('en-US', { minimumFractionDigits: 4, maximumFractionDigits: 5 });
}

export function calculateSpread(buy: string, sell: string): { diff: number; bps: number; text: string } {
  const b = parseFloat(buy);
  const s = parseFloat(sell);
  if (isNaN(b) || isNaN(s) || b <= 0) return { diff: 0, bps: 0, text: '-' };

  const diff = Math.max(0, s - b);
  const bps = (diff / b) * 10000;

  // Choose precision based on diff magnitude
  let text = '';
  if (diff < 0.001) {
    text = diff.toFixed(5);
  } else if (diff < 1) {
    text = diff.toFixed(4);
  } else {
    text = diff.toFixed(2);
  }

  return { diff, bps, text };
}

export function formatFreshness(ms: number): string {
  if (ms < 0) return '0ms';
  if (ms < 1000) return `${Math.round(ms)}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  if (ms < 3600000) {
    const mins = Math.floor(ms / 60000);
    const secs = Math.floor((ms % 60000) / 1000);
    return `${mins}m ${secs}s`;
  }
  const hours = Math.floor(ms / 3600000);
  const mins = Math.floor((ms % 3600000) / 60000);
  return `${hours}h ${mins}m`;
}

export function filterSymbols(
  snapshots: MarketSnapshot[],
  category: AssetCategory,
  searchQuery: string
): MarketSnapshot[] {
  const query = searchQuery.trim().toUpperCase();

  return snapshots.filter((item) => {
    // 1. Text Search
    if (query && !item.symbol.toUpperCase().includes(query)) {
      return false;
    }

    // 2. Category Filter
    switch (category) {
      case 'forex-majors':
        return FOREX_MAJORS.has(item.symbol.toUpperCase());
      case 'forex-crosses':
        return (
          !FOREX_MAJORS.has(item.symbol.toUpperCase()) &&
          !CRYPTO_SYMBOLS.has(item.symbol.toUpperCase()) &&
          !COMMODITIES.has(item.symbol.toUpperCase())
        );
      case 'crypto':
        return CRYPTO_SYMBOLS.has(item.symbol.toUpperCase());
      case 'commodities':
        return COMMODITIES.has(item.symbol.toUpperCase());
      case 'gainers':
        return parseFloat(item.change_24h_pct) > 0;
      case 'losers':
        return parseFloat(item.change_24h_pct) < 0;
      case 'stale':
        return item.is_stale;
      case 'all':
      default:
        return true;
    }
  });
}
