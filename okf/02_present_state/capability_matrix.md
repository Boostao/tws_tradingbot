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

| Capability | Status | Evidence & Implementation Summary |
|---|---|---|
| Wails v3 app shell & static embed | **Implemented & Tested** | [`main.go`](../../main.go), [`Taskfile.yml`](../../Taskfile.yml), [`build/config.yml`](../../build/config.yml) embed `frontend/dist` and register `App`. Clean builds via `task build`, `npm run check`, and `npm run build`. |
| Generated Wails v3 JS/TS bindings | **Implemented & Tested** | [`frontend/bindings/tws_traderbot/app.js`](../../frontend/bindings/tws_traderbot/app.js) exposes all 32 `App` methods via `$Call.ByID`. Stale Wails v2 bindings purged. |
| `frontend/src/lib/api.ts` IPC routing | **Implemented & Tested** | 100% native Wails v3 IPC bindings through `$Call.ByID`. Zero `/api/v1/*` HTTP stubs, zero `(window as any).go` shims. |
| CI/CD GitHub Actions workflow | **Implemented & Tested** | Matrix workflows configured in [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml), [`.github/workflows/release.yml`](../../.github/workflows/release.yml), and [`.github/workflows/pages.yml`](../../.github/workflows/pages.yml). |
| Linters & parity gates | **Implemented & Tested** | [`scripts/check-okf-hygiene.py`](../../scripts/check-okf-hygiene.py), [`scripts/check-dep-lockstep.py`](../../scripts/check-dep-lockstep.py), [`scripts/check-i18n-parity.py`](../../scripts/check-i18n-parity.py), and script tests in `scripts/tests/` (24/24 passing). |

### 2.2 SQLite Persistence & Domain State

| Capability | Status | Evidence & Implementation Summary |
|---|---|---|
| Pure-Go SQLite connection & schema | **Implemented & Tested** | [`backend/db/sqlite.go`](../../backend/db/sqlite.go) (`modernc.org/sqlite`) creates `watchlists`, `watchlist_items`, `strategies`, `trade_ledger`, and `system_config` tables with WAL mode and foreign keys enabled. |
| `system_config` JSON KV persistence | **Implemented & Tested** | [`config.go`](../../backend/db/config.go) and [`cockpit.go`](../../backend/db/cockpit.go) implement UPSERTs for global configuration and runtime state. |
| Normalized relational watchlist tables | **Implemented & Tested** | Relational `watchlists` and `watchlist_items` tables with cascade deletion, multi-column indexes, pre-seeded with 5 official Cobalt lists, and smart diff/merge support. |
| Persistent trade ledger & preset library | **Implemented & Tested** | `trade_ledger` and `strategies` tables implemented in [`backend/db/ledger.go`](../../backend/db/ledger.go) and [`backend/db/strategy.go`](../../backend/db/strategy.go), persisting all simulated and live trade executions. |

### 2.3 Cockpit Operational Surface (`/cockpit` & `/`)

| Capability | Status | Evidence & Implementation Summary |
|---|---|---|
| Workspace tabs (`Achat`, `Short`, `Swing`) | **Implemented & Tested** | [`routes/cockpit/+page.svelte`](../../frontend/src/routes/cockpit/+page.svelte) renders 3 workspaces (`Achat`, `Short`, `Swing`) pre-seeded in `app.go` and `engine.go`. |
| Single active strategy slot invariant | **Implemented & Tested** | Strict `1/1` active strategy slot concurrency enforced in UI (`buildExclusiveStrategyWorkspaces`) and validated fail-closed in `bot.Engine.CanExecute`. |
| Cockpit state save & hot updates | **Implemented & Tested** | `UpdateCockpitState` in [`app.go`](../../app.go) saves cockpit state and updates runtime execution parameters via native Wails v3 IPC. |
| Cockpit $\rightarrow$ Bot `Engine` wiring | **Implemented & Tested** | `Engine` strictly checks `global_enabled`, workspace enablement, active strategy slot, and per-ticker toggles during tick evaluation. |
| Default landing route (`/` vs `/cockpit`) | **Implemented & Tested** | [`+page.ts`](../../frontend/src/routes/+page.ts) issues a `307` redirect to `/cockpit`, making the operator cockpit the default landing surface. |

### 2.4 Watchlists & TradingView Feed Sync

| Capability | Status | Evidence & Implementation Summary |
|---|---|---|
| TradingView shared watchlist import | **Implemented & Tested** | `ImportTradingViewWatchlist` in [`app_watchlist.go`](../../app_watchlist.go) fetches HTML, extracts `application/prs.init-data+json`, parses section headers, and normalizes tickers to `SYMBOL.VENUE`. |
| Watchlist JSON file import / export | **Implemented & Tested** | `ImportWatchlistFile` and `ExportWatchlistFile` in [`app_watchlist.go`](../../app_watchlist.go) use native OS file dialogs with clean round-trip serialization. |
| Manual add / remove ticker | **Implemented & Tested** | `AddWatchlistSymbol` and `RemoveWatchlistSymbol` in [`app_watchlist.go`](../../app_watchlist.go) mutate relational watchlist items with automatic venue qualification. |
| Feed refresh button (`Refresh`) | **Implemented & Tested** | `RefreshWatchlistFeed` in [`app_watchlist.go`](../../app_watchlist.go) performs smart diff/merge, updating ticker universe while preserving operator overrides for enabled state, tiering, and sizing. |
| `SYMBOL.VENUE` `instrument_id` normalization | **Implemented & Tested** | Normalized in [`backend/models/symbol.go`](../../backend/models/symbol.go) and [`backend/bot/resolver.go`](../../backend/bot/resolver.go), eliminating all 8 exchange-stripped collisions. |
| Regime reference vs orderable equity gating | **Implemented & Tested** | `IsOrderable` and `IsRegimeReference` tags in `WatchlistItem` enforce Gate 7, ensuring benchmark and regime feeds inform indicators without placing orders. |

### 2.5 Strategy Builder, Library & Evaluation Engine

| Capability | Status | Evidence & Implementation Summary |
|---|---|---|
| Active strategy persistence & hot-reload | **Implemented & Tested** | `GetStrategy` and `UpdateStrategy` in [`app_config.go`](../../app_config.go) persist strategy state to SQLite and hot-reload `Engine.ActiveStrategy`. |
| Native strategy JSON import / export | **Implemented & Tested** | `ImportStrategyFile` and `ExportStrategyFile` in [`app_config.go`](../../app_config.go) provide native OS file import/export. |
| Strategy library CRUD & validation | **Implemented & Tested** | `GetStrategyLibrary`, `GetStrategyPreset`, `SaveStrategyPreset`, `DeleteStrategyPreset`, `ApplyStrategyPreset`, and `ValidateStrategy` in [`app_strategy.go`](../../app_strategy.go). |
| Array math & memoized indicator pipeline | **Implemented & Tested** | [`backend/strategy/indicators.go`](../../backend/strategy/indicators.go) provides thread-safe memoized indicator calculations (HA, RVOL, OBV, MACD, RSI, BB, Stoch, Alligator, Guppy, ATR, RS vs SPX) with zero-division protection. |
| Multi-timeframe BarFrame bundle | **Implemented & Tested** | [`backend/strategy/barframe.go`](../../backend/strategy/barframe.go) supports hierarchical `symbol -> timeframe -> BarFrame` OHLCV bundles across all 9 Cobalt timeframes. |
| Full rule & indicator evaluator coverage | **Implemented & Tested** | [`backend/strategy/evaluator.go`](../../backend/strategy/evaluator.go) supports all 7 condition types (`cross_above`, `cross_below`, `greater_than`, `less_than`, `slope_above`, `slope_below`, `within_range`). |
| Pre-seeded 20 Cobalt & Screener 2.0 presets | **Implemented & Tested** | [`backend/strategy/catalog.go`](../../backend/strategy/catalog.go) pre-seeds all 13 chart models, 4 Screener 2.0 presets, and 3 Cours setups into SQLite. |

### 2.6 Broker Connectivity, Market Data, Sizing & Execution

| Capability | Status | Evidence & Implementation Summary |
|---|---|---|
| TWS socket connect / disconnect & settings | **Implemented & Tested** | [`app.go`](../../app.go), [`backend/bot/ibkr.go`](../../backend/bot/ibkr.go), and [`Sidebar.svelte`](../../frontend/src/lib/components/Sidebar.svelte) connecting via `scmhub/ibapi` on port `7497` with settings hydration. |
| 3-Mode position sizing (# shares, $, %) | **Implemented & Tested** | `CalculatePositionSize` in [`backend/bot/order.go`](../../backend/bot/order.go) computes sizes via fixed shares (priority 1), fixed dollar notional, or portfolio equity percentage. |
| Multi-venue contract resolver & dot-class | **Implemented & Tested** | `ResolveContract` and `ConvertDotClass` in [`backend/bot/resolver.go`](../../backend/bot/resolver.go) handle US (`SMART/USD`), Canadian (`SMART/CAD`), and dot-class equities (`BRK B`, `CTC A`). |
| 9-Gate fail-closed engine | **Implemented & Tested** | `CanExecute` in [`backend/bot/engine.go`](../../backend/bot/engine.go) evaluates all 9 fail-closed execution gates before order creation. |
| Master circuit breaker & daily loss breaker | **Implemented & Tested** | Coupe-circuit principal (`global_enabled`) and automated daily loss risk breaker with manual reset in `ResetCircuitBreaker()`. |
| Atomic `NextValidId` sequencing | **Implemented & Tested** | Atomic thread-safe counter in [`backend/bot/order.go`](../../backend/bot/order.go) synchronized from TWS socket `NextValidId`. |
| Session gating (`RTH`/`ETH`) | **Implemented & Tested** | [`backend/bot/session.go`](../../backend/bot/session.go) enforces session-safe routing, rejecting `MKT` orders during ETH. |
| OCA child bracket orders | **Implemented & Tested** | `CreateBracketOrderPair` in [`backend/bot/order.go`](../../backend/bot/order.go) generates parent `MKT` + OCA `LMT` profit target + OCA `STP` stop loss. |
| Strict dry-run guard & audit ledger | **Implemented & Tested** | `IsDryRun` guard in [`backend/bot/ibkr.go`](../../backend/bot/ibkr.go) prevents live socket order transmission and records simulated fills to SQLite `trade_ledger`. |
| Bilingual UI (`en` / `fr`) | **Implemented & Tested** | 100% key parity in [`frontend/src/lib/i18n/translations.ts`](../../frontend/src/lib/i18n/translations.ts) verified by `check-i18n-parity.py` with authentic Cobalt lexicon. |
