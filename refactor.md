# Refactor Plan — SvelteKit + FastAPI + WebSockets

Goal: complete refactor to a SvelteKit frontend with a FastAPI backend and real‑time WebSocket updates, while preserving trading logic, configuration, and data model behavior.

## Phase 0 — Baseline & Scope
- Freeze current behavior (tests pass, sample data works).
- Document current UI flows (Strategy, Backtest, Monitoring, Watchlist).
- Enumerate required user‑visible features and current data models.

## Phase 1 — Target Architecture & Contracts
- Define service boundaries: core bot logic vs. API service.
- Design REST API (strategy CRUD, backtest run/history, watchlist, config, logs).
- Design WebSocket channels (prices, bot state, orders, backtest progress, alerts).
- Define shared schemas (Pydantic models + OpenAPI).

## Phase 2 — Backend Service (FastAPI)
- Create `api` service package alongside existing core modules.
- Implement REST endpoints with strict validation.
- Implement WebSocket manager (broadcast, per‑topic subscriptions).
- Add background tasks for backtest progress streaming.
- Add auth gate (optional) to match current config behavior.

## Phase 3 — Frontend (SvelteKit)
- Scaffold SvelteKit app with routing and layout.
- Implement data layer (REST client + WebSocket client + stores).
- Build pages: Strategy Builder, Backtest, Monitoring, Watchlist.
- Implement i18n with current translation keys.

## Phase 4 — Feature Parity
- Strategy rule builder parity (rules, operators, validation, saving/loading).
- Backtest configuration + results + charts (equity, trades, metrics).
- Monitoring parity (bot status, positions, orders, logs).
- Notifications and command actions.

## Phase 5 — Performance & UX
- WebSocket‑first updates for live data.
- Client‑side caching for symbol lists and watchlists.
- Optimistic UI updates where safe.
- Instrumentation (latency, reconnect, error visibility).

### Phase 5 Progress (2026‑01‑28)
- ✅ WebSocket reconnect helper with backoff and status badges (connection, lag, reconnect count) on Monitoring + Notifications.
- ✅ Notifications persistence (DB‑backed), pagination, per‑page selector, and “new alerts” indicator.
- ✅ Client‑side caching for strategy, config, watchlist, symbols, backtest status/results.
- ✅ Optimistic UI updates for watchlist.
- ✅ REST latency instrumentation (per‑endpoint timing) with status badges in Monitoring + Notifications.
- ✅ Cache invalidation controls (reload bypass cache + clear cache buttons) on Strategy, Backtest, Watchlist, Notifications.
- ✅ API error surfacing in UI with parsed server details.
- ✅ REST latency/error visibility badges on Strategy + Backtest.
- ✅ Cache policy tuning controls (TTL and auto‑refresh toggles).

### Phase 5 Remaining (Proposed)
- ☐ None.

## Phase 6 — Migration & Rollout
- Run API + SvelteKit UI side‑by‑side with the bot.
- Validate outputs against current tests and sample data.
- Update docs, scripts, and Docker compose for new services.
- Deprecate Streamlit UI and remove unused UI modules. (Completed)

### Phase 6 Progress (2026‑01‑28)
- ✅ Added API + SvelteKit web services to docker‑compose for side‑by‑side runs.
- ✅ Added web Dockerfile and `run_web.sh` convenience script.
- ✅ Updated README to document new SvelteKit UI + API flow.
- ✅ Removed Streamlit UI, scripts, and dependencies.

## Phase 7 — Cleanup & Hardening
- Remove legacy UI dependencies.
- Consolidate config paths and env variables.
- Add integration tests for API and WebSocket flows.
- Security review (CORS, auth, rate limits).

### Phase 7 Progress (2026‑01‑28)
- ✅ Added integration tests for REST endpoints and WebSocket streams.
- ✅ Consolidated config/env paths and documented overrides.
- ✅ Security review: configurable CORS, optional basic auth, basic rate limiting.

## Acceptance Criteria
- All current capabilities are available in the new UI.
- Real‑time updates are WebSocket‑driven.
- Backtests and live monitoring are fully functional.
- Documentation and deployment steps updated.

---

# Phase 0 Execution (Baseline & Scope)

## Baseline Snapshot
- Date: 2026‑01‑28
- Last known test run: 128 passed (per terminal log).
- Current UI: SvelteKit app (Monitoring, Strategy, Backtest, Watchlist, Notifications).

## Current UI Flow Map
### Live Trading (Monitoring)
- Auto‑refresh toggle (default on) with 5s interval.
- Connection status (state age), TWS status (provider), bot status & errors.
- TWS account snapshot (manual/interval refresh).
- Metrics (equity, PnL, open positions, etc.).
- Positions table, orders table.
- Bot controls (start/stop/emergency stop).
- Logs section with recent entries.

### Watchlist
- Watchlist manager for local tickers file.
- Used as a fallback source for backtest tickers.

### Strategy Builder
- Strategy metadata (name/description), rule counts and status.
- Rules list with enable/disable and delete.
- Rule builder to create conditions and actions.
- Backtest configuration (tickers, dates, capital, timeframe, data source).
- Backtest execution + results (metrics, equity curve, trades, QuantStats).
- Actions: import/export, save draft, deploy, reset.

## User‑Visible Feature Inventory
- Strategy CRUD (create, edit, save, load, import/export JSON).
- Rule builder: indicators, conditions, actions, enable/disable, priority.
- Backtesting with sample or TWS data (optional Nautilus).
- Live monitoring (positions, orders, account snapshot, logs).
- Watchlist management (local file, used for tickers).
- Optional auth gate via config.
- i18n (EN/FR) across UI text.
- Notifications (Telegram/Discord), ML signal support (from prior notes).

## Core Data Model Inventory
- Strategy models: `Strategy`, `Rule`, `Condition`, `Indicator` + enums.
- Bot state models: `BotState`, `Position`, `Order` + enums.
- Config models: settings and database config (DuckDB default).
- Backtest outputs: `BacktestResult` (metrics, trades, equity).

## Files of Record
- Web UI: web/src/routes/
- API entry: src/api/main.py
- Strategy models: src/bot/strategy/rules/models.py
- Bot state: src/bot/state.py

---

# Phase 1 Execution (Target Architecture & Contracts)

## Service Boundaries
### Core (Python package)
- Strategy models and rule evaluation.
- Backtest engine and data loaders.
- Live runner + state updates + notifications.
- TWS data provider + adapters.

### API Service (FastAPI)
- Exposes REST + WebSocket interfaces over the core package.
- Owns auth gate, request validation, background tasks, streaming.

### Frontend (SvelteKit)
- UI state, rendering, charts, and real‑time updates.
- No direct file access; all persistence via API.

## REST API Contract (Draft)
Base: `/api/v1`

### Strategy
- `GET /strategy` → current strategy
- `PUT /strategy` → update/replace current strategy
- `POST /strategy/validate` → validate and return errors
- `POST /strategy/import` → import JSON
- `GET /strategy/export` → export JSON

### Backtest
- `POST /backtest/run` → start backtest (returns job id)
- `GET /backtest/{id}` → backtest status + summary
- `GET /backtest/{id}/results` → full results (metrics, trades, equity)
- `DELETE /backtest/{id}` → delete cached results

### Watchlist
- `GET /watchlist` → list symbols
- `PUT /watchlist` → replace list
- `POST /watchlist/add` → add symbol
- `POST /watchlist/remove` → remove symbol

### Monitoring / State
- `GET /state` → current bot state snapshot
- `POST /bot/start` → start bot
- `POST /bot/stop` → stop bot
- `POST /bot/emergency_stop` → emergency stop
- `GET /logs` → recent logs

### Config
- `GET /config` → current config (redacted secrets)
- `PUT /config` → update config

### Symbols
- `GET /symbols` → cached symbol list (for search/autocomplete)

## WebSocket Channels (Draft)
Base: `/ws`

- `/ws/state` → bot state updates
- `/ws/logs` → log stream
- `/ws/positions` → position updates
- `/ws/orders` → order updates
- `/ws/account` → TWS account snapshot updates
- `/ws/backtest/{id}` → backtest progress + partial results
- `/ws/market/{symbol}` → live market data (if enabled)

## Schema Notes (Draft)
- Strategy models mirror current Pydantic models (`Strategy`, `Rule`, `Condition`, `Indicator`).
- Bot state mirrors `BotState` with nested `Position` and `Order` types.
- Backtest results contain `metrics`, `trades`, `equity_curve`, and `data_source`.
- All timestamps are ISO‑8601 strings in UTC.

## Non‑Functional Requirements
- Authentication gate consistent with current config (optional).
- CORS for SvelteKit origin only.
- WebSocket reconnect + backoff guidance.
- Avoid blocking operations on request threads (use background tasks).

---

# Phase 2 Execution (Backend Service Scaffolding)

## Implemented
- FastAPI app scaffold with CORS for SvelteKit dev origin.
- REST endpoints for strategy, backtest, watchlist, state/logs, config, symbols.
- WebSocket endpoints for state, logs, backtest progress.
- Backtest job manager with background execution and serialized results.
- API helper utilities for watchlist/strategy/symbol cache and redacted config.

## Files Added
- src/api/main.py
- src/api/utils.py
- src/api/schemas.py
- src/api/services/backtest.py
- src/api/routers/*.py
- run_api.sh

## Dependencies
- fastapi
- uvicorn

---

# Phase 3 Execution (Frontend Scaffold)

## Implemented
- SvelteKit app scaffolded in web/ (TypeScript).
- Global layout with navigation shell.
- API health check on landing page.
- Placeholder routes for Monitoring, Strategy, Backtest, Watchlist.
- Environment template for API base URL.

## Files Added/Updated
- web/src/routes/+layout.svelte
- web/src/routes/+page.svelte
- web/src/routes/monitoring/+page.svelte
- web/src/routes/strategy/+page.svelte
- web/src/routes/backtest/+page.svelte
- web/src/routes/watchlist/+page.svelte
- web/src/lib/api.ts
- web/src/app.css
- web/.env.example
