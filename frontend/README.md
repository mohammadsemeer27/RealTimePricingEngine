# Real-Time Pricing Engine — Trading Frontend Dashboard

A high-performance, responsive dark-mode financial trading dashboard built with **React 19**, **TypeScript**, **Vite 8**, and **Tailwind CSS**. It connects to the Go Real-Time Pricing Engine running via Docker Compose on port `8082`.

---

## Architecture & Features

- **Real-Time Data Pipeline**:
  - Connects to `GET http://localhost:8082/prices` for initial state and periodic REST refresh.
  - Subscribes to downstream WebSockets on `ws://localhost:8082/ws` (proxied via `/ws`) for compact delta updates (`< 150 B/delta`).
  - Graceful fallback: seamlessly falls back to periodic REST polling (every 3 seconds) if WebSockets disconnect.
- **Accurate Feed Status Evaluation**:
  - Distinguishes **Live**, **Stale**, **Mock**, and **Unavailable** states based on actual backend data without fabricating prices.
  - Detects upstream authentication failures (e.g., live provider auth close code 4003) and clearly indicates that PostgreSQL restored historical state is being served.
- **Trading Dashboard UI**:
  - **Summary KPI Cards**: Active symbol universe (61 pairs), average data freshness, 24h top gainer, and 24h top loser.
  - **Interactive Market Table**: Real-time quotes for all 61 trading pairs (Forex Majors, Crosses, Crypto, Commodities) with:
    - Symbol & Asset Class badge
    - Real-time Midpoint price with green/red flash animations on tick updates
    - Buy (Bid) & Sell (Ask) quotes
    - Real-time Spread (absolute and basis points)
    - 24-hour High & Low computed from monotonic deques ($O(1)$)
    - 24-hour Percentage Change with trend indicators
    - 24-hour Range Bar visualizer
    - Freshness badge (sub-second or stale duration)
  - **Search & Multi-Category Filtering**: Instant ticker search (`BTC`, `JPY`, `EUR`, etc.) and category tabs (All, Forex Majors, Cross Pairs, Crypto, Metals & Commodities, Gainers, Losers, Stale).
  - **Dual Layout Modes**: Switch between high-density Data Table and responsive Grid Cards.
  - **Symbol Detail Modal**: Deep dive into the 24-hour monotonic rolling window for any currency or crypto pair.
  - **Engine Diagnostics Drawer**: Live operational telemetry displaying average processing latency ($\mu$s), database bulk writes, downstream WebSocket clients, and dropped event breakdowns directly from `GET /metrics`.

---

## Prerequisites

- **Node.js**: `v20+` or `v22+` (tested on Node `v22.23.1`)
- **npm**: `v10+`
- **Docker Compose backend**: running on host port `8082`

---

## Installation & Setup

### 1. Navigate to the frontend directory
```bash
cd frontend
```

### 2. Install dependencies
```bash
npm install
```

### 3. Start the development server
```bash
npm run dev
```

The Vite dev server will start and be accessible at:
```
http://localhost:5173
```

---

## Environment Variables (Optional)

By default, Vite's dev server automatically proxies API requests (`/prices`, `/healthz`, `/readyz`, `/metrics`, and `/ws`) to `http://localhost:8082`. This bypasses browser cross-origin (CORS) restrictions.

To target an alternative backend URL directly:
Create a `.env` file inside `frontend/`:
```env
VITE_API_BASE_URL=http://localhost:8082
```

---

## Available Scripts

| Script | Command | Description |
| :--- | :--- | :--- |
| `dev` | `npm run dev` | Runs the Vite development server on port 5173 with hot module replacement |
| `build` | `npm run build` | Compiles TypeScript and builds production assets to `dist/` |
| `preview` | `npm run preview` | Locally serves the production build |
| `lint` | `npm run lint` | Runs the Oxlint static analyzer |

---

## Production Build

To build the static frontend for deployment:
```bash
npm run build
```
The optimized bundle will be created in `frontend/dist/`.
