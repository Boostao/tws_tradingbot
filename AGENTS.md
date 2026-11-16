# `tws_traderbot` — Engineering & Multi-Agent Router

Welcome to **`tws_traderbot`**, a local Interactive Brokers (TWS) trading bot built with **Go `1.26`**, **Wails v3**, **Svelte 5**, **`scmhub/ibapi`**, and **SQLite (`modernc.org/sqlite`)**.

## 1. Authoritative Routing Map (Read Scoped Contracts Before Editing)

Instructions are layered to avoid context bloat while preserving engineering invariants:

| Scope | Scoped Agent Contract | Knowledge Base / Playbook |
|---|---|---|
| **Repository Root (`/*`)** | **`AGENTS.md`** *(this file)* · [`.github/copilot-instructions.md`](./.github/copilot-instructions.md) | [`okf/00_index.md`](./okf/00_index.md) · [`okf/04_evidence_and_sources/decisions_and_gaps.md`](./okf/04_evidence_and_sources/decisions_and_gaps.md) |
| **Knowledge & Curation (`okf/`)** | **[`okf/AGENTS.md`](./okf/AGENTS.md)** | [`okf/00_index.md`](./okf/00_index.md) · [`okf/04_evidence_and_sources/citations_index.md`](./okf/04_evidence_and_sources/citations_index.md) · [`okf/04_evidence_and_sources/decisions_and_gaps.md`](./okf/04_evidence_and_sources/decisions_and_gaps.md) |
| **Cobalt Domain, Indicators & Universe** | **`AGENTS.md`** · [`okf/AGENTS.md`](./okf/AGENTS.md) | [`okf/02_present_state/cobalt_indicators_and_courses.md`](./okf/02_present_state/cobalt_indicators_and_courses.md) · [`okf/02_present_state/cobalt_trading_universe.md`](./okf/02_present_state/cobalt_trading_universe.md) |
| **Capabilities, Roadmap & History** | **`AGENTS.md`** · [`okf/AGENTS.md`](./okf/AGENTS.md) | [`okf/02_present_state/capability_matrix.md`](./okf/02_present_state/capability_matrix.md) · [`okf/03_future_architecture/roadmap.md`](./okf/03_future_architecture/roadmap.md) · [`okf/01_history_and_evolution/legacy_and_migration.md`](./okf/01_history_and_evolution/legacy_and_migration.md) |
| **Engineering & Runtime (`backend/`, `frontend/`)** | **`AGENTS.md`** · [`.github/copilot-instructions.md`](./.github/copilot-instructions.md) | [`okf/02_present_state/system_architecture.md`](./okf/02_present_state/system_architecture.md) · [`okf/05_engineering_and_agents/engineering_playbook.md`](./okf/05_engineering_and_agents/engineering_playbook.md) |

---

## 2. Repository Directory Layout

- **`main.go`, `app.go`, `app_config.go`, `app_watchlist.go`** — Wails v3 application entry point, IPC service bindings, config, and watchlist management.
- **`backend/`** — Go backend packages:
  - `api/` — Backend API context exposed to Wails.
  - `bot/` — Execution engine (`Engine.EvaluateTick()`), order routing, and dry-run guards.
  - `db/` — SQLite persistence (`system_config`, watchlists, runtime state).
  - `models/` — Shared domain structs (with `json:"..."` tags for Wails TypeScript bindings).
  - `strategy/` — Native Go array math evaluators (SMA, EMA, Crossover, Slope).
  - `wrapper/` — Custom `IBWrapper` embedding `ibapi.Wrapper` for TWS market data and execution.
- **`frontend/`** — Svelte 5 + SvelteKit + TypeScript UI (`@wailsio/runtime`, `frontend/bindings/`, `frontend/src/lib/i18n/translations.ts`).
- **`okf/`** — **Operational Knowledge Framework**: authoritative specifications across `01_history_and_evolution/`, `02_present_state/`, `03_future_architecture/`, `04_evidence_and_sources/`, and `05_engineering_and_agents/`, governed by [`okf/AGENTS.md`](./okf/AGENTS.md) (in-place sync on architectural/schema/strategy changes; zero stray `.md` files outside allowlisted paths).
- **`scripts/`** — OKF hygiene linter (`check-okf-hygiene.py`) and test suites (`scripts/tests/`).
- **`.agents/`** — Project skills (including [`okf-curator`](./.agents/skills/okf-curator/SKILL.md)).
- **`.github/`** — CI workflows and Copilot instructions ([`.github/copilot-instructions.md`](./.github/copilot-instructions.md)).

---

## 3. Core Engineering Guardrails

1. **Local IPC & Single Active Strategy Slot:**
   - The backend operates locally over Wails IPC, not REST.
   - Preserve the single active strategy slot behavior in the cockpit; do not reintroduce multi-strategy fan-out unless explicitly requested.
   - Treat UI runtime settings as global execution settings.
2. **Watchlist, Market Data & Multi-Timeframe Compatibility:**
   - Watchlist items expose normalized `instrument_id` values in `SYMBOL.VENUE` form.
   - Market data stays subscribed for the full watchlist universe, while execution only runs for currently enabled strategy and ticker targets.
   - When changing rule evaluation, maintain compatibility with both flat market data maps and nested `symbol -> timeframe -> frame` bundles.
   - Respect TWS pacing limits for historical data block requests.
3. **Wails Bindings & Dry-Run Safety:**
   - Ensure Go struct fields intended for Svelte use standard `json:"property"` tags and regenerate bindings (`wails3 generate bindings`) when service signatures or models change.
   - Dry-run execution routes must **never** place live orders.
4. **UI & i18n Discipline:**
   - Keep the cockpit dense and operationally focused (single-line ticker rows, compact strategy summaries, minimal header waste).
   - Route all user-visible strings in `frontend/` through `frontend/src/lib/i18n/translations.ts` for both `en` and `fr`.

---

## 4. Quick Validation Matrix

```bash
# 1. Go Backend Unit Tests
go test -v ./backend/...
# or: task test:backend

# 2. Svelte Frontend Type-Check & Build
(cd frontend && npm run check && npm run build)

# 3. Wails Bindings & Full App Build
wails3 generate bindings
task build

# 4. OKF Hygiene & Markdown Allowlist Gate
python3 scripts/check-okf-hygiene.py
python3 -m unittest discover -s scripts/tests -p 'test_*.py'
```
