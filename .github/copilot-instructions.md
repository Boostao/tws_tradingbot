# GitHub Copilot Agent Instructions — TWS Trader Bot

## Product Goal
Maintain a local TWS/Interactive Brokers trading bot with:
- Svelte split UI in `web/`
- FastAPI backend in `src/api/`
- File-backed live worker in `src/bot/live_runner.py`
- Rule-engine based strategy execution
- Direct Python + `ibapi` market-data and execution integration

## Current State
- Cockpit is the main control surface.
- Each workspace has exactly one active strategy slot.
- Watchlist state is stored as grouped JSON and exported to the legacy text watchlist for compatibility.
- Watchlist items expose normalized `instrument_id` values in `SYMBOL.VENUE` form.
- The live runner now fetches rule-required symbol/timeframe subscriptions, not just one dominant timeframe.
- The live runner reloads cockpit/watchlist/strategy state at the start of each execution cycle.
- Market data stays subscribed for the full watchlist universe, while execution only runs for currently enabled strategy and ticker targets.
- Direct TWS execution through `ibapi` is the primary live path.
- Runtime state includes recent logs, recent open orders, recent closed trades, and the last dry-run snapshot.

## Architecture Map
- UI routes: `web/src/routes/`
- Shared web API client: `web/src/lib/api.ts`
- Cockpit page: `web/src/routes/cockpit/+page.svelte`
- Shared app styles: `web/src/app.css`
- API app entry: `src/api/main.py`
- API routers: `src/api/routers/`
- Watchlist and cockpit persistence helpers: `src/api/utils.py`
- Rule models and engine: `src/bot/strategy/rules/`
- Market-data resolution helpers: `src/bot/strategy/rules/market_data.py`
- Live runtime: `src/bot/live_runner.py`
- Direct TWS provider: `src/bot/tws_data_provider.py`
- Runtime state and trade ledger: `src/bot/state.py`

## Working Rules
- Preserve the single active strategy slot behavior in the cockpit.
- Do not reintroduce multi-strategy fan-out unless the user explicitly asks for it.
- Treat the sidebar runtime settings as global execution settings.
- Prefer the direct TWS path for executable runtime work.
- When changing rule evaluation, maintain compatibility with both flat market data maps and nested `symbol -> timeframe -> frame` bundles.
- If you touch watchlist state, preserve both grouped JSON metadata and the enabled-symbol legacy text export.
- Keep API and web types aligned when runtime state changes.

## UI Guidance
- Keep the cockpit dense and operationally focused.
- Avoid wasting vertical space in the header area.
- Prefer single-line summaries for ticker rows and compact strategy summaries when possible.
- Any new user-visible strings in the Svelte UI must go through `web/src/lib/i18n/translations.ts` for both `en` and `fr`.

## Validation
- Backend unit tests: `pytest tests/unit -q`
- Focused runtime tests: `pytest tests/unit/test_runtime_control.py -q`
- Rule-engine tests: `pytest tests/unit/test_rule_engine.py -q`
- Frontend checks: `cd web && npm run check`
- Runner import check: `python -m src.bot.live_runner --help`

## Common Tasks
### Runtime changes
1. Update `src/bot/live_runner.py`
2. Update `src/bot/state.py` if persisted state changes
3. Update `src/api/routers/state.py` and `web/src/lib/api.ts` if the control plane changes
4. Add or update focused tests in `tests/unit/test_runtime_control.py`

### Rule-engine or data-flow changes
1. Update `src/bot/strategy/rules/conditions.py`, `engine.py`, or `market_data.py`
2. Keep `tests/unit/test_rule_engine.py` green
3. If the live runner depends on the change, add a focused runtime test too

### Cockpit layout changes
1. Update `web/src/routes/cockpit/+page.svelte`
2. Update `web/src/app.css`
3. Run `cd web && npm run check`

## Known Constraints
- The runtime is local and file-backed by design.
- TWS pacing still matters for historical requests.
- Multi-timeframe runtime support now exists in the execution path, but any UI/history tooling that assumes one flat frame per symbol should be treated carefully.
- The dry-run route should never place orders.
