# GitHub Copilot Agent Instructions — TWS Trader Bot

## ✅ Project Goal
Build and maintain a rule‑based trading bot with a SvelteKit UI and FastAPI backend plus Nautilus Trader integration. Focus on reliability, clear UX, and safe trading workflows (backtest before deploy).

## 📌 Current Project State (Jan 28, 2026)
- **Status:** SvelteKit UI functional; Streamlit UI removed.
- **Runtime:** Nautilus IB adapter supported; non‑Nautilus mode now supports live order execution via IB API.
- **Real-time data:** TWS market data subscriptions/snapshots supported.
- **Auth:** Optional UI login gate via `auth` config.
- **State:** DuckDB backend enabled by default (JSON is fallback).
- **Package manager:** `uv` with virtual env in `.venv/`.
- **Tests:** `uv run pytest tests/ -v` should pass (count may vary).

## 🧩 Recent Session Updates (Jan 28, 2026)
- **QuantStats report** integrated in UI charts and strategy tab (optional dependency).
- **Optuna optimizer** CLI added (`src/bot/optimizer.py`).
- **Notifications**: Telegram/Discord alerts + Telegram command polling.
- **ML signal indicator** with model loader support (ONNX/joblib).
- **Database**: Postgres backend option added; DuckDB remains default.
- **Docker**: `Dockerfile` uses Python 3.14-slim with `uv`; `docker-compose.yml` added.
- **Tests**: Shutdown/hanging test cleanup via session teardown.

## 🧩 Recent Session Updates (Jan 29, 2026)
- **Watchlist UI:** table view with inline SymbolSearch, X remove button, auto‑save on row changes, download TXT (one line per entry). Ticker ID column removed. ESC in search cancels to previous value, search auto‑focuses.
- **Watchlist file format:** entries now `TICKER:MARKET` per line. Import/export updated; watchlist API still stores strings. `src/bot/tws_data_provider.py` now tolerates `TICKER:MARKET` lines (uses ticker portion only for bot symbols).
- **Symbol search:** `SymbolSearch` supports autofocus/cancel; watchlist allows non‑stock instruments. Search results ordering favors exact/prefix matches in `src/api/routers/symbols.py`.
- **Symbol cache:** TradingView scan expanded to crypto + forex in `src/api/utils.py` (no hardcoded fallbacks). Name fallback uses description when name missing. Cache refresh via `/api/v1/symbols?refresh=true` or `get_symbol_cache(refresh=True)`.

## 🗺️ Where Things Live (Quick Map)
- **Web UI Routes:** `web/src/routes/`
- **Web API client:** `web/src/lib/api.ts`
- **Web WS client:** `web/src/lib/ws.ts`
- **Strategy Models & Rules:** `src/bot/strategy/rules/models.py`, `evaluator.py`, `indicators.py`
- **Strategy Validation:** `src/bot/strategy/validator.py`
- **Bot Runtime:** `src/bot/live_runner.py`
- **IB Adapter:** `src/bot/adapter.py`
- **State:** `src/bot/state.py` (DuckDB in `data/traderbot.duckdb`, JSON fallback)
- **Config:** `config/default.yaml`
- **Docs:** `docs/strategy_guide.md`
- **Sample Data:** `data/sample/`

## 🌍 i18n Requirements (IMPORTANT)
SvelteKit UI does not currently enforce an i18n layer. Keep UI text consistent and centralized where possible.

## 🔁 Common Agent Tasks
### Add a new indicator
1. `src/utils/indicators.py`
2. `src/bot/strategy/rules/indicators.py`
3. `src/bot/strategy/rules/models.py` (enum)
4. `web/src/routes/strategy/+page.svelte` (dropdown)
5. `src/bot/strategy/validator.py`

### Add a new operator
1. `src/bot/strategy/rules/models.py` (enum)
2. `src/bot/strategy/rules/evaluator.py`
3. `web/src/routes/strategy/+page.svelte` (UI)

### Modify bot state
1. `src/bot/state.py`
2. `src/bot/live_runner.py`
3. `web/src/routes/monitoring/+page.svelte`

## 🧪 Run & Test
- Web UI: `./run_web.sh`
- API: `./run_api.sh`
- Bot: `./run_bot.sh`
- Tests: `uv run pytest tests/ -v`

### Test Dependencies
- API/WebSocket integration tests require `httpx` (via FastAPI TestClient). Ensure it is installed in the dev environment before running tests.

## ⚠️ Known Notes
- VIX data is loaded via TWS when available, else sample CSV.
- VIX sample has a stray value; loader sanitizes it.
- Nautilus IB adapter uses `nautilus_ibapi`, with fallback to native IB adapter.
- UI launcher auto-picks a free port (8501–8510).
- Auth is disabled by default; enable in `config/default.yaml` or `AUTH_*` env vars.

## 🧠 Strategy System Notes
- **Condition operators:** `crosses_above`, `crosses_below`, `greater_than`, `less_than`, `equals`, `slope_above`, `slope_below`, `within_range`.
- **Cross-symbol indicators:** `indicator.symbol` can reference another symbol (e.g., `VIX`) and uses that symbol’s bars when available.
- **Tickers:** Strategy `tickers` are saved from UI selection and used by the live runner when present.
- **Position sizing:** Live runner uses `risk.max_position_size` if set; otherwise `strategy.initial_capital * risk.max_position_pct`.

## ✅ Working Standards
- Preserve existing architecture and patterns.
- Prefer minimal, targeted edits.
- Update tests when behavior changes.
- If UI text changes, update translations immediately.
- Update README.md when behavior, configuration, or user-facing features change.

## 📞 Support Contacts
- **Framework:** [Nautilus Trader Docs](https://nautilustrader.io/)
- **IB API:** [Interactive Brokers API Docs](https://interactivebrokers.github.io/tws-api/)
- **Svelte:** [Svelte Docs](https://svelte.dev/docs)
