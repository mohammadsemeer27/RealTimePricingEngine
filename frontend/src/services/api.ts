import { HealthStatus, MarketSnapshot, PricesApiResponse, SystemMetrics } from '../types/market';

// Default to relative path in dev mode to leverage Vite proxy (which targets http://localhost:8082)
// This avoids browser CORS restrictions since the Go server does not inject Access-Control-Allow-Origin.
const API_BASE = import.meta.env.VITE_API_BASE_URL || '';

export class ApiError extends Error {
  status?: number;
  constructor(message: string, status?: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

async function request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const url = `${API_BASE}${endpoint}`;
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 8000);

  try {
    const res = await fetch(url, {
      ...options,
      signal: controller.signal,
      headers: {
        'Accept': 'application/json',
        ...(options.headers || {}),
      },
    });

    if (!res.ok) {
      let errorMsg = `HTTP ${res.status} (${res.statusText})`;
      try {
        const body = await res.json();
        if (body.error) errorMsg = body.error;
      } catch {
        // Fallback to status text
      }
      throw new ApiError(errorMsg, res.status);
    }

    return await res.json() as T;
  } catch (err: unknown) {
    if (err instanceof ApiError) throw err;
    if (err instanceof DOMException && err.name === 'AbortError') {
      throw new ApiError('Request timed out while connecting to pricing engine.');
    }
    const message = err instanceof Error ? err.message : 'Unknown network failure';
    throw new ApiError(`Network error: ${message}`);
  } finally {
    clearTimeout(timeoutId);
  }
}

export const apiService = {
  async getPrices(symbols?: string[]): Promise<PricesApiResponse> {
    const query = symbols && symbols.length > 0 ? `?symbols=${symbols.join(',')}` : '';
    return request<PricesApiResponse>(`/prices${query}`);
  },

  async getPriceSymbol(symbol: string): Promise<MarketSnapshot> {
    return request<MarketSnapshot>(`/prices/${encodeURIComponent(symbol)}`);
  },

  async getHealth(): Promise<HealthStatus> {
    return request<HealthStatus>('/healthz');
  },

  async getReady(): Promise<{ status: string; error?: string }> {
    return request<{ status: string; error?: string }>('/readyz');
  },

  async getMetrics(): Promise<SystemMetrics> {
    return request<SystemMetrics>('/metrics', {
      headers: { 'Accept': 'application/json' },
    });
  },
};
