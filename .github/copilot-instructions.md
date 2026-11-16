# GitHub Copilot Agent Instructions — `tws_traderbot`

**Start at [`AGENTS.md`](../AGENTS.md) first**, then load [`okf/AGENTS.md`](../okf/AGENTS.md) and [`okf/00_index.md`](../okf/00_index.md) when modifying architecture, schemas, or specifications.

## Core Guardrails

1. **Architecture & Stack (`Go 1.26` + `Wails v3` + `Svelte 5` + `SQLite`):**
   - Local desktop app operating over Wails IPC (not REST) with `modernc.org/sqlite` persistence and `scmhub/ibapi` (`IBWrapper` embedding `ibapi.Wrapper`).
   - Preserve the single active strategy slot in the cockpit, global runtime settings, `SYMBOL.VENUE` `instrument_id` normalization, full-watchlist market-data subscriptions with targeted execution, and compatibility with both flat market data maps and nested `symbol -> timeframe -> frame` bundles.
   - Ensure Go structs exposed over Wails use `json:"property"` tags and regenerate TypeScript bindings (`wails3 generate bindings`) when IPC contracts change.
   - Dry-run execution routes must **never** place live orders; respect TWS pacing for historical data requests.

2. **UI & i18n:**
   - Keep the cockpit dense and operationally focused (compact header, single-line ticker rows).
   - Route all new user-visible strings in `frontend/` through `frontend/src/lib/i18n/translations.ts` for both `en` and `fr`.

3. **Knowledge Hygiene (`okf/`):**
   - Keep `okf/` synchronized in place with architectural, schema, or strategy changes; never leave stray `.md` files outside the allowlist enforced by `scripts/check-okf-hygiene.py`.

4. **Validation Gates:**
   - **Backend:** `go test -v ./backend/...`
   - **Frontend:** `(cd frontend && npm run check && npm run build)`
   - **Full Build:** `task build`
   - **OKF Hygiene:** `python3 scripts/check-okf-hygiene.py && python3 -m unittest discover -s scripts/tests -p 'test_*.py'`
