# GitHub Copilot Agent Instructions — TWS Trader Bot

## ✅ Project Goal
Build and maintain a rule‑based trading bot with a Streamlit UI and Nautilus Trader integration. Focus on reliability, clear UX, and safe trading workflows (backtest before deploy).

## 📌 Current Project State (Jan 28, 2026)
- **Status:** UI functional, i18n complete, backtest mapping improved, VIX fallback/sanitization added.
- **Runtime:** Nautilus IB adapter supported; non‑Nautilus mode now supports live order execution via IB API.
- **Real-time data:** TWS market data subscriptions/snapshots supported.
- **Auth:** Optional UI login gate via `auth` config.
- **State:** DuckDB backend enabled by default (JSON is fallback).
- **Package manager:** `uv` with virtual env in `.venv/`.
- **Tests:** `uv run pytest tests/ -v` should pass (124 tests currently).

## 🗺️ Where Things Live (Quick Map)
- **UI Entry:** `src/ui/main.py`
- **UI Tabs:** `src/ui/tabs/monitoring.py`, `src/ui/tabs/strategy.py`
- **UI Components:** `src/ui/components/` (rule builder, charts, watchlist)
- **UI Theme/CSS:** `src/ui/styles.py`
- **i18n:** `src/ui/i18n.py` + `src/ui/translations.py`
- **Strategy Models & Rules:** `src/bot/strategy/rules/models.py`, `evaluator.py`, `indicators.py`
- **Strategy Validation:** `src/bot/strategy/validator.py`
- **Bot Runtime:** `src/bot/live_runner.py`
- **IB Adapter:** `src/bot/adapter.py`
- **State:** `src/bot/state.py` (DuckDB in `data/traderbot.duckdb`, JSON fallback)
- **Config:** `config/default.yaml`
- **Docs:** `docs/strategy_guide.md`
- **Sample Data:** `data/sample/`

## 🌍 i18n Requirements (IMPORTANT)
**All UI text must pass through the i18n layer.**
- Use `i18n.t("key")` for any user‑visible text.
- Never hardcode new UI strings in the Streamlit UI.
- When adding or changing UI text:
  1. Add/update the key in `src/ui/translations.py` for **both** `en` and `fr`.
  2. Use `i18n.t("your_key")` in UI code.
- If removing UI text, remove the related translation keys from **all** languages.
- If you must insert HTML in UI text, ensure it remains in translations (keep consistent across locales).

## 🔁 Common Agent Tasks
### Add a new indicator
1. `src/utils/indicators.py`
2. `src/bot/strategy/rules/indicators.py`
3. `src/bot/strategy/rules/models.py` (enum)
4. `src/ui/components/rule_builder.py` (dropdown)
5. `src/bot/strategy/validator.py`

### Add a new operator
1. `src/bot/strategy/rules/models.py` (enum)
2. `src/bot/strategy/rules/evaluator.py`
3. `src/ui/components/rule_builder.py` (UI)

### Modify bot state
1. `src/bot/state.py`
2. `src/bot/live_runner.py`
3. `src/ui/tabs/monitoring.py`

## 🧪 Run & Test
- UI: `./run_ui.sh`
- Bot: `./run_bot.sh`
- Tests: `uv run pytest tests/ -v`

## ⚠️ Known Notes
- VIX data is loaded via TWS when available, else sample CSV.
- VIX sample has a stray value; loader sanitizes it.
- Nautilus IB adapter uses `nautilus_ibapi`, with fallback to native IB adapter.
- UI launcher auto-picks a free port (8501–8510).
- Auth is disabled by default; enable in `config/default.yaml` or `AUTH_*` env vars.

## ✅ Working Standards
- Preserve existing architecture and patterns.
- Prefer minimal, targeted edits.
- Update tests when behavior changes.
- If UI text changes, update translations immediately.
