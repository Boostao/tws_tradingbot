# Capability Matrix (`02_present_state`)

> **Navigation:** [Master Index](../00_index.md) | **Related:** [System Architecture](./system_architecture.md) · [Roadmap](../03_future_architecture/roadmap.md) · [Decisions & Gaps](../04_evidence_and_sources/decisions_and_gaps.md)  
> **Status:** `Active`

---

## 1. Status Legend

| Status | Meaning |
|---|---|
| **Implemented & Tested** | Present in Go/Svelte `HEAD` and verified by passing automated tests (`go test -v ./backend/...` or OKF suite). |
| **Implemented (Partial / Untested)** | Present in Go/Svelte `HEAD`, but lacks unit test coverage, hits minor wiring bugs, or requires manual prerequisites. |
| **UI / Legacy HTTP Stub** | UI surface exists in `frontend/`, but [`frontend/src/lib/api.ts`](../../frontend/src/lib/api.ts) still calls non-existent FastAPI HTTP routes (`/api/v1/*`) or uses a Wails v2 `(window as any).go` check. |
| **Not Wired / Missing in Go** | Domain concept exists in legacy Python specs or [`NOTES.md`](../../../NOTES.md), not yet implemented in the Go runtime. |

---

## 2. End-to-End Feature Matrix

### 2.1 Application Shell, Wails v3 IPC & Build Pipeline

| Capability | Status | Evidence & Implementation Gap |
|---|---|---|
| Wails v3 app shell & static embed | **Implemented (Partial / Untested)** | [`main.go`](../../main.go), [`Taskfile.yml`](../../Taskfile.yml), [`build/config.yml`](../../build/config.yml) embed `frontend/dist` and register `App`. Locally `frontend/node_modules/@wailsio/runtime` is not installed yet (`npm run check` fails with 6 errors in `runtime.ts` and `+page.svelte`). |
| Generated Wails v3 JS/TS bindings | **Implemented (Partial / Untested)** | [`frontend/bindings/tws_traderbot/app.js`](../../frontend/bindings/tws_traderbot/app.js) exposes all 21 `App` methods via `$Call.ByID`. Stale Wails v2 `frontend/wailsjs/` files still exist alongside it. |
| `frontend/src/lib/api.ts` IPC routing | **UI / Legacy HTTP Stub** | Only `importTradingViewWatchlist`, `exportWatchlistFile`, `importWatchlistFile` call Wails v3 bindings unconditionally. Ten methods (`getStrategy`, `getConfig`, `updateConfig`, `getState`, `connectTws`, `disconnectTws`, `startBot`, `stopBot`, `getWatchlist`, `getCockpit`) gate Wails calls behind `(window as any).go` (a Wails v2 global). Fifteen others (`saveCockpit`, `replaceWatchlist`, `refreshWatchlistFeed`, `saveStrategy`, `validateStrategy`, `importStrategy*`, `*StrategyPreset*`, `dryRunBot`, `getDiagnostics`, `getSymbols`) fall back to `fetch(${API_BASE}/api/v1/...)`. |
| CI/CD GitHub Actions workflow | **Implemented (Partial / Untested)** | [`.github/workflows/release.yml`](../../.github/workflows/release.yml) still triggers on `main` and invokes Wails v2 `wails build` rather than `wails3` / `Taskfile.yml` on `wails3`. |
| OKF curation & hygiene linter | **Implemented & Tested** | [`scripts/check-okf-hygiene.py`](../../scripts/check-okf-hygiene.py) and `scripts/tests/test_okf_hygiene.py` (4/4 tests passing). |

### 2.2 SQLite Persistence & Domain State

| Capability | Status | Evidence & Implementation Gap |
|---|---|---|
| Pure-Go SQLite connection & schema | **Implemented & Tested** | [`backend/db/sqlite.go`](../../backend/db/sqlite.go) (`modernc.org/sqlite`) creates `watchlist` and `system_config` tables; verified in-memory via `backend/db/*_test.go`. |
| `system_config` JSON KV persistence | **Implemented & Tested** | [`config.go`](../../backend/db/config.go), [`cockpit.go`](../../backend/db/cockpit.go), [`watchlist.go`](../../backend/db/watchlist.go) implement UPSERTs (`TestCockpitState_Persistence`, `TestWatchlistState_Persistence` pass). |
| Normalized `watchlist` SQL table | **Not Wired / Missing in Go** | Created in `InitSchema()`, never queried or written; all watchlist data is serialized as one JSON blob under `system_config['watchlist_state']`. |
| Persistent trade ledger & symbol cache | **Not Wired / Missing in Go** | No SQLite tables or Go stores exist for executed orders, closed trades, or cached symbol search records. |

### 2.3 Cockpit Operational Surface (`/cockpit` & `/`)

| Capability | Status | Evidence & Implementation Gap |
|---|---|---|
| Workspace tabs (`Achat`, `Short`, `Swing`) | **Implemented (Partial / Untested)** | [`routes/cockpit/+page.svelte`](../../frontend/src/routes/cockpit/+page.svelte) renders workspace tabs, mode badge, main breaker, and grouped ticker toggles. However, `GetCockpitState()` in [`app.go`](../../app.go#L139-L150) seeds only a single `"Main Trading"` workspace (`id: "default", kind: "long"`), not `Achat`/`Short`/`Swing`. |
| Single active strategy slot invariant | **Implemented (Partial / Untested)** | `buildExclusiveStrategyWorkspaces()` in `cockpit/+page.svelte` disables all other workspace slots so at most `1/1` strategy slot is active globally. |
| Cockpit state save (`UpdateCockpitState`) | **UI / Legacy HTTP Stub** | [`app.go`](../../app.go#L162-L180) exposes `UpdateCockpitState`, but `api.ts:saveCockpit()` still executes `PUT ${API_BASE}/api/v1/cockpit` instead of calling the Wails binding. |
| Cockpit $\rightarrow$ Bot `Engine` wiring | **Not Wired / Missing in Go** | `App.StartBot()` passes an empty `&models.Strategy{}` ([`app.go`](../../app.go#L211)); `Engine` never consults `cockpit_state` (`global_enabled`, active workspace, active strategy slot, or enabled watchlist groups/items). |
| Default landing route (`/` vs `/cockpit`) | **Implemented (Partial / Untested)** | [`+page.ts`](../../frontend/src/routes/+page.ts) redirects `/` (`307`) to `/cockpit`, making `+page.svelte` (the Wails v3 3-panel Pulse/Matrix/Ledger dashboard) unreachable via `/`. |

### 2.4 Watchlists & TradingView Feed Sync

| Capability | Status | Evidence & Implementation Gap |
|---|---|---|
| TradingView shared watchlist import | **Implemented (Partial / Untested)** | `ImportTradingViewWatchlist` in [`app_watchlist.go`](../../app_watchlist.go#L21-L185) scrapes `application/prs.init-data+json`, splits `###` section headers into `WatchlistGroup`s, parses `EXCHANGE:SYMBOL`, and saves to `system_config['watchlist_state']`. No HTTP mock unit test exists. |
| Watchlist JSON file import / export | **Implemented (Partial / Untested)** | `ImportWatchlistFile` and `ExportWatchlistFile` ([`app_watchlist.go`](../../app_watchlist.go#L188-L232)) use native Wails v3 file dialogs; JSON round-trip logic is duplicated in `app_watchlist_test.go`, which fails `go test .` due to duplicate `package main` on lines 1–2. |
| Manual add / remove ticker | **Implemented (Partial / Untested)** | `AddWatchlistSymbol` / `RemoveWatchlistSymbol` exist in [`app_watchlist.go`](../../app_watchlist.go#L235-L315) (hardcoding `Exchange: "SMART"`), while `watchlist/+page.svelte` calls `replaceWatchlist` (HTTP `PUT /api/v1/watchlist`) and `SymbolSearch.svelte` calls `getSymbols` (HTTP `GET /api/v1/symbols`). |
| Feed refresh button (`Refresh`) | **UI / Legacy HTTP Stub** | `refreshWatchlistFeed()` in `api.ts` calls HTTP `POST /api/v1/watchlist/feed/refresh` instead of re-invoking `ImportTradingViewWatchlist(watchlist.feed.url)`. |
| `SYMBOL.VENUE` `instrument_id` normalization | **Not Wired / Missing in Go** | `WatchlistItem.InstrumentID` exists in [`backend/models/schemas.go`](../../backend/models/schemas.go#L10), but is never populated during import or symbol addition. |

### 2.5 Strategy Builder, Library & Evaluation Engine

| Capability | Status | Evidence & Implementation Gap |
|---|---|---|
| Active strategy persistence & hot-reload | **Implemented (Partial / Untested)** | `GetStrategy` / `UpdateStrategy` ([`app_config.go`](../../app_config.go#L41-L71)) persist JSON to `system_config['strategy']` and unmarshal into `a.engine.ActiveStrategy`. Overwritten with `&models.Strategy{}` if `StartBot()` is invoked afterward. |
| Native strategy JSON import / export | **Implemented (Partial / Untested)** | `ImportStrategyFile` / `ExportStrategyFile` ([`app_config.go`](../../app_config.go#L74-L113)) wired to `strategy/+page.svelte` via Wails v3 file dialogs. |
| Strategy save, validation & library CRUD | **UI / Legacy HTTP Stub** | `strategy/+page.svelte` exposes preset Save/Update/Load/Activate/Delete and Validate buttons, but `api.ts` routes all of them to legacy `/api/v1/strategy*` HTTP endpoints. |
| Array math (`SMA`, `EMA`, `CrossAbove`, `Slope`) | **Implemented & Tested** | [`backend/strategy/math.go`](../../backend/strategy/math.go) and [`evaluator.go`](../../backend/strategy/evaluator.go) pass `TestSMA`, `TestCrossAbove`, `TestEvaluateCondition`. |
| Full rule & indicator evaluator coverage | **Not Wired / Missing in Go** | `EvaluateConditions` supports only `cross_above`, `greater_than`, and `slope_below` against pre-populated `"indicatorA"` / `"indicatorB"` slice keys. Missing: `cross_below`, `less_than`, `slope_above`, `within_range`; indicator calculators (`EMA`/`SMA` auto-compute, `RSI`, `MACD`, `VIX`, `Time`, Bollinger, Stochastic, OBV, Alligator); and nested `symbol -> timeframe -> frame` bundle resolution. |
| UI $\leftrightarrow$ Go strategy schema alignment | **Not Wired / Missing in Go** | [`strategy/+page.svelte`](../../frontend/src/routes/strategy/+page.svelte) emits `crosses_above`, `crosses_below`, `per_ticker`, `length`, `params`, `component`, `symbol`, `position_size_mode`, `position_size_value`, while [`backend/models/strategy.go`](../../backend/models/strategy.go) defines `cross_above`, `cross_below`, `symbol`/`instrument`, `period` (and omits sizing fields and `params`). |

### 2.6 Broker Connectivity, Market Data, Sizing & Execution

| Capability | Status | Evidence & Implementation Gap |
|---|---|---|
| TWS socket connect / disconnect & settings | **Implemented (Partial / Untested)** | [`app.go`](../../app.go) + [`backend/bot/ibkr.go`](../../backend/bot/ibkr.go) connect via `scmhub/ibapi` using `tws_connection` in SQLite; `IBWrapper.Error` suppresses `2104`, `2106`, `2158`. `Sidebar.svelte` `loadSidebarConfig()` is currently an empty stub (`{}`), so persisted sidebar inputs do not hydrate on mount. |
| Market-data streaming & historical pacing | **Not Wired / Missing in Go** | No scheduler in `App` or `Engine` requests historical bars or streaming quotes for watchlist symbols; `HistoricalDataEnd` maps `reqID` to placeholder `"SYM_<reqID>"` and forwards only `"Close"` / `"Open"` series. |
| Order routing (`IBWrapper.ExecuteAction`) | **Implemented (Partial / Untested)** | Places `MKT` `DAY` orders via `w.client.PlaceOrder` and emits `ledger_entry`. **Gaps:** `actionMap` expects uppercase `"BUY"` / `"SELL"` while `models.ActionBuy` / `ActionSell` are lowercase `"buy"` / `"sell"`, `orderId` uses `w.currentReqID++` instead of TWS `NextValidId`, and `TotalQuantity` is hardcoded to `"100"`. |
| Position sizing modes (`# shares`, `$`, `%`) & brackets | **Not Wired / Missing in Go** | Sidebar captures `fixed_notional`, `bracket_enabled`, `stop_loss_pct`, `take_profit_pct`; strategy UI models `position_size_mode` / `position_size_value`. Neither is wired into `Engine` or `IBWrapper.ExecuteAction`. |
| Dry-run preview & live monitoring telemetry | **UI / Legacy HTTP Stub** | `FailsafeStop` works natively in [`app.go`](../../app.go#L190-L194). [`monitoring/+page.svelte`](../../frontend/src/routes/monitoring/+page.svelte) requires `dryRunBot()`, `getDiagnostics()`, and rich `BotState` telemetry (`equity`, `daily_pnl`, `recent_trades`), whereas Go `models.BotState` only contains `{running: bool}`. |
| Bilingual UI (`en` / `fr`) | **Implemented (Partial / Untested)** | [`translations.ts`](../../frontend/src/lib/i18n/translations.ts) covers sidebar, manual, strategy, and watchlist. Newer UI labels in `monitoring/+page.svelte`, `+page.svelte`, and `"Filter tickers"` in `cockpit/+page.svelte` are currently hardcoded in English. |
