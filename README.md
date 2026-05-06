# TWS Trader Bot

TWS Trader Bot is a local Interactive Brokers/TWS trading application with a split Svelte UI, a FastAPI control plane, and a file-backed live runner built directly on Python plus `ibapi`.

## Current product shape

- Split cockpit UI for workspace, strategy, and ticker control.
- Exactly one active strategy slot per workspace.
- Watchlist management with grouped JSON state plus legacy text export.
- Rule-based strategy editing and validation.
- Live TWS execution loop using `ibapi` for historical bars, position/order snapshots, executions, and order submission.
- Sidebar runtime controls for TWS connection, fixed notional sizing, and global bracket settings.

## Runtime model

- The live worker runs separately via `python -m src.bot.live_runner`.
- The worker resolves the active cockpit workspace, the single active strategy, and enabled watchlist instruments before it starts trading.
- Watchlist items are normalized to `instrument_id` values in `SYMBOL.VENUE` form. `SMART` is the default venue when no exchange is provided.
- The worker now fetches all rule-required symbol/timeframe subscriptions instead of one dominant timeframe.
- The worker reloads cockpit/watchlist/strategy state at the start of each execution cycle so strategy, rule, and ticker enablement changes are picked up without restarting the process.
- Market data is still requested for all watchlist instruments, while order evaluation and submission run only for the currently enabled strategy and enabled tickers.
- A dry-run API route is available to preview a full planning cycle without placing orders.

## Important limitations

- Multi-timeframe support is now explicit in the data pipeline, but it still depends on the current rule evaluator and indicator factory. The runtime fetches per-timeframe data bundles correctly; strategy design should still stay disciplined until broader historical/UI tooling uses the same bundle model everywhere.
- The live runner is file-backed and process-local. It is designed for a local TWS workflow, not clustered deployment.
- The system is intentionally built around the direct TWS/IBAPI path rather than an external trading engine abstraction.

## Key API routes

- `GET /health`
- `GET /api/v1/watchlist`
- `PUT /api/v1/watchlist`
- `GET /api/v1/cockpit`
- `PUT /api/v1/cockpit`
- `GET /api/v1/strategy`
- `PUT /api/v1/strategy`
- `POST /api/v1/strategy/validate`
- `GET /api/v1/config`
- `PUT /api/v1/config`
- `GET /api/v1/state`
- `POST /api/v1/bot/start`
- `POST /api/v1/bot/stop`
- `POST /api/v1/bot/emergency_stop`
- `POST /api/v1/bot/dry_run`
- `POST /api/v1/tws/connect`
- `POST /api/v1/tws/disconnect`

## Local development

### API

```bash
./run_api.sh
```

### Web UI

```bash
./run_web.sh
```

### Worker

```bash
./run_bot.sh
```

Open `http://localhost:5173`.

## Dependencies

- Base app dependencies are declared in `pyproject.toml`.
- Optional extras:
	- `live`: installs `ibapi`

## Validation

Backend:

```bash
pytest tests/unit -q
```

Frontend:

```bash
cd web && npm run check
```
