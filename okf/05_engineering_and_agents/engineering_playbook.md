# Engineering Playbook (`05_engineering_and_agents`)

> **Navigation:** [Master Index](../00_index.md) | **Related:** [System Architecture](../02_present_state/system_architecture.md) · [Capability Matrix](../02_present_state/capability_matrix.md) · [Root `AGENTS.md`](../../AGENTS.md)  
> **Status:** `Active`

---

## 1. Wails v3 Service & IPC Conventions

1. **Service Registration (`main.go`, `app*.go`):**
   - Register the single `App` service in [`main.go`](../../main.go) via `app.RegisterService(application.NewService(svc))`.
   - Implement `ServiceStartup(ctx context.Context, options application.ServiceOptions) error` rather than Wails v2 `startup(ctx)`.
   - Keep exported service methods cleanly grouped across [`app.go`](../../app.go) (TWS, runtime, cockpit with `cockpitMu` serialization), [`app_config.go`](../../app_config.go) (config, strategy & native file dialogs), [`app_watchlist.go`](../../app_watchlist.go) (TradingView import, watchlist dialogs & CRUD), [`app_strategy.go`](../../app_strategy.go) (strategy presets, pin/order stack & validation), and [`app_diagnostics.go`](../../app_diagnostics.go) (diagnostics, dry-run scanner, breaker reset & trade ledger).
2. **Struct Serialization & Binding Regeneration:**
   - Tag every field on structs in [`backend/models/`](../../backend/models/runtime.go) (and sibling model files `symbol.go`, `watchlist.go`, `strategy.go`, `cockpit.go`, `diagnostics.go`, `ledger.go`) with explicit `json:"snake_case"` tags (including `omitempty` only where optional on the TS side).
   - Regenerate TypeScript/JS stubs whenever any exported `App` signature or `backend/models` struct changes:
     ```bash
     wails3 generate bindings
     ```
     Generated files live in `frontend/bindings/tws_traderbot/`. **Never edit generated binding files by hand**, and do not add new imports from the legacy `frontend/wailsjs/` directory.
3. **Frontend IPC Invocation (`frontend/src/lib/`):**
   - Import Wails v3 methods directly from `frontend/bindings/tws_traderbot/app` and runtime events from `@wailsio/runtime` (`Events.On`, `Events.Off`).
   - [`frontend/src/lib/api.ts`](../../frontend/src/lib/api.ts) uses 100% native Wails v3 IPC (`41` bindings via `$Call.ByID`) with zero `/api/v1/*` HTTP fallbacks or `(window as any).go` shims.
4. **Native OS Dialogs:**
   - Use `a.app.Dialog.OpenFileWithOptions(...)` and `SaveFileWithOptions(...)` (`PromptForSingleSelection()`) in Go rather than browser `<input type="file">` uploads whenever reading or exporting local JSON files.

---

## 2. SQLite Persistence Patterns (`backend/db/`)

1. **Pure-Go Driver (`modernc.org/sqlite`):**
   - Import `_ "modernc.org/sqlite"` so the database layer compiles without SQLite CGO headers and supports `db.Connect(":memory:")` in fast unit tests.
2. **Idempotent UPSERT Pattern:**
   - Persist document configuration blobs in `system_config` using parameterized queries:
     ```sql
     INSERT INTO system_config (key, value)
     VALUES (?, ?)
     ON CONFLICT(key) DO UPDATE SET value=excluded.value
     ```
3. **Key Ownership & Schema Discipline:**
   - Current canonical keys in `system_config`: `tws_connection`, `config`, `strategy`, `cockpit_state`, `migration_cobalt_v2_clean_holdings`.
   - Relational `watchlists`, `watchlist_items`, and `strategies` (`pinned`, `sort_order`) tables in [`backend/db/sqlite.go`](../../backend/db/sqlite.go) are the canonical store (`SetMaxOpenConns(1)` / `SetMaxIdleConns(1)` with `WAL`, `foreign_keys(1)`, and `busy_timeout(5000)`).
   - Never persist broker credentials, API tokens, or personal contact data in SQLite or logs.

---

## 3. `scmhub/ibapi` Wrapper, Error Suppression & Pacing Rules

1. **Wrapper Embedding ([`backend/bot/ibkr.go`](../../backend/bot/ibkr.go) & `third_party/ibapi`):**
   - `IBWrapper` lives in [`backend/bot/ibkr.go`](../../backend/bot/ibkr.go) (no `backend/wrapper/` package) and embeds `ibapi.Wrapper` from the locally patched `third_party/ibapi@v0.10.50` module (`watchDone`/CAS disconnect synchronization and atomic `IsConnected()`).
   - Implements live telemetry callbacks (`ConnectAck`, `ConnectionClosed`, `ManagedAccounts`, `AccountSummary`, `UpdateAccountValue`, `Position`, `PositionEnd`, `UpdatePortfolio`, `OpenOrder`, `OpenOrderEnd`, `OrderStatus`, `Pnl`, `NextValidID`, `HistoricalData`, `HistoricalDataEnd`, `Error`).
2. **Informational Error Code Suppression & `BuildBrokerAlert`:**
   - Suppress benign TWS/IB Gateway farm status codes (`2104`, `2106`, `2158`) in `IBWrapper.Error` while surfacing actionable codes (`10089`, `504`, `502`, `1100`–`1102`, `200`, `201`, `321`, `354`) through `BuildBrokerAlert` with operator hints.
3. **Pacing, Request Tracking & Dry-Run Safety Invariants:**
   - **Track `reqID -> (instrument_id, timeframe)` safely:** Guard `IBWrapper` bar buffers and request maps with `sync.Mutex` (never hardcode placeholder symbols like `SYM_<reqID>` in production paths).
   - **Respect TWS Historical Data Pacing:** Interactive Brokers throttles concurrent and burst historical requests. When expanding multi-symbol / multi-timeframe fetching, stagger `ReqHistoricalData` requests with explicit pacing intervals and cache completed bar frames.
   - **Full-Universe Subscriptions vs. Targeted Execution:** Subscribe to market data for the watchlist universe, but only allow order evaluation/submission when `cockpit_state.global_enabled`, the active workspace, the single active strategy slot, and the target ticker are all enabled.
   - **Order IDs & Dry-Run Guard:** Use TWS `NextValidId` (incremented atomically) for `PlaceOrder` rather than historical request IDs, normalize `ActionType` (`"buy"`/`"sell"` $\rightarrow$ `"BUY"`/`"SELL"`), and enforce a strict dry-run gate so dry-run evaluations **never** invoke `PlaceOrder`.

---

## 4. Strategy Engine & Multi-Timeframe Evaluation (`backend/strategy/`)

1. **Pure Deterministic Functions:**
   - Keep indicator math (`SMA`, `EMA`) and condition evaluators (`CrossAbove`, `Slope`, `EvaluateConditions`) stateless and independent of TWS/SQLite so all rule logic runs in fast `go test` suites.
   - Pad warm-up indices (`i < period-1`) with `math.NaN()` and guard against empty slices or `lookback <= 0`.
2. **Flat Map & Multi-Timeframe Bundle Compatibility:**
   - When wiring indicator pre-computation into `Engine.EvaluateTick`, maintain compatibility with both flat bar series maps and nested `symbol -> timeframe -> frame` bundles so multi-timeframe rules (`1m`, `5m`, `15m`, `1h`, `1d`) resolve consistently.
3. **Single Active Strategy Slot:**
   - Preserve the cockpit invariant enforcing at most one enabled strategy slot (`1/1`) across workspaces unless explicitly redesigned in [`decisions_and_gaps.md`](../04_evidence_and_sources/decisions_and_gaps.md).

---

## 5. Svelte 5 & Bilingual i18n (`en` / `fr`) Rules

1. **Compiler & Adapter Mode ([`frontend/svelte.config.js`](../../frontend/svelte.config.js)):**
   - Svelte 5 runs with `compilerOptions: { runes: false }` and `@sveltejs/adapter-static` (`fallback: 'index.html'`, output `dist/`). Use consistent `$:` reactive declarations across components rather than mixing Svelte 5 runes syntax unless runes mode is deliberately enabled everywhere.
   - Root layout [`+layout.ts`](../../frontend/src/routes/+layout.ts) enforces `prerender = true` and `ssr = false`.
2. **Cobalt Brand System, Density & Interactive Controls ([`cobalt_brand_and_ui_guide.md`](../02_present_state/cobalt_brand_and_ui_guide.md)):**
   - Use canonical `:root` tokens (`#05070b`/`#0b111e` Uni Noir surfaces, `#263c8a`/`#1d63ed`/`#38bdf8` Cobalt blues, `#faa031` accents, `IoskeleyMono`), global dark `<select>`/`<option>` rules (`color-scheme: dark`, `appearance: none`, `#0b1320`), locked 3-row `.cockpit-shell` (`auto auto minmax(0, 1fr)`), fixed single-bar `.cockpit-hud-strip`, single-line ticker rows (`minmax(0, 1fr)` grid truncation), `SlideToArm.svelte` (`>= 78%`, `pointercancel`-safe), `HelpDrawer.svelte` (`F1`/`?`), and 3-track `OnboardingTour.svelte`.
   - Persist transient input drafts (e.g., TradingView URL) in [`frontend/src/lib/stores/ui.ts`](../../frontend/src/lib/stores/ui.ts) and onboarding progress in `localStorage` (`cobalt_onboarding_state`).
3. **Bilingual i18n Discipline ([`frontend/src/lib/i18n/translations.ts`](../../frontend/src/lib/i18n/translations.ts)):**
   - Add every new user-visible string to **both** `en` and `fr` maps in `translations.ts` (`928` symmetric keys).
   - Use reactive `$: _lang = $language; $: tr = (k, p) => { void _lang; return t(k, p); };` (or `{lang && t(...)}`) instead of `{#key $language}` in `+layout.svelte` so switching `en <-> fr` updates all text in place without discarding unsaved route state. Never leave raw English UI literals in `.svelte` templates.
4. **Vite Dev & Wails Dependency Sync:**
   - Run `npm install` inside `frontend/` whenever `frontend/package.json` updates (such as `@wailsio/runtime`) so `svelte-check` and `vite build` resolve all runtime module declarations.

---

## 6. Build, Test & OKF Hygiene Commands

Run all relevant gates from `/tws_traderbot` before committing changes:

```bash
# 1. Go backend unit & race tests
go test -race ./...
# or: task test:backend

# 2. Frontend type-check & production build
(cd frontend && npm run check && npm run build)

# 3. Wails bindings, desktop/Windows builds & static release portal
wails3 generate bindings
wails3 task linux:build
wails3 task windows:build
node site/build.mjs
# Output binaries: build/bin/tws_traderbot & build/bin/tws_traderbot.exe

# 4. OKF hygiene, dependency lockstep, i18n parity & packaging/UI gates
python3 scripts/check-okf-hygiene.py
python3 scripts/check-dep-lockstep.py
python3 scripts/check-i18n-parity.py
python3 -m unittest discover -s scripts/tests -p 'test_*.py'
node scripts/tests/verify_v2_ui.mjs --dist frontend/dist

# 5. Full opaque-box E2E test suite (Tiers 1–5 + V2 & Tier-5 Backend tests)
task test:e2e
```

### Verification & Delivery Invariants

- **Full Go Suite & Race Detection:** `go test -race ./...` (and `TWS_LIVE_TEST=1 go test -race` against local TWS Paper) run clean across all root, backend, and E2E packages without data races.
- **E2E & Adversarial Hardening:** `task test:e2e` executes the full opaque-box E2E suite across Tiers 1–5 (including `v2_requirements_test.go` and `tier5_v2_backend_test.go`) with zero failures.
- **OKF Curation Sync Rule:** Per [`okf/AGENTS.md`](../AGENTS.md) and [`../../AGENTS.md`](../../AGENTS.md), any commit altering Wails IPC bindings, SQLite schema, `backend/strategy/`, `backend/bot/`, or `frontend/` routes must update `okf/` in the same commit, leave zero `.md` files outside the allowlist, keep rule files (`AGENTS.md`, `okf/AGENTS.md`, `.github/copilot-instructions.md`) under `24,000` bytes, and pass [`scripts/check-okf-hygiene.py`](../../scripts/check-okf-hygiene.py).
