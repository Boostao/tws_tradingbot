# TWS Trader Bot (PineScript Branch)

This branch is intentionally minimized to a single product target:

- Watchlist management
- Strategy rule builder
- Pine Script generation from enabled rules

All live runtime bot, IBAPI integration, backtesting runtime, and deployment stack were removed from this branch.

## What the app does

- Manage watchlist entries in `TICKER:MARKET` format
- Build/edit rule-based strategies and persist them to `config/active_strategy.json`
- Validate strategy rules
- Generate TradingView Pine Script from enabled rules via API

## API surface

Mounted routes:

- `GET /health`
- `GET /api/v1/watchlist`
- `PUT /api/v1/watchlist`
- `GET /api/v1/symbols`
- `GET /api/v1/strategy`
- `PUT /api/v1/strategy`
- `POST /api/v1/strategy/validate`
- `POST /api/v1/strategy/import`
- `POST /api/v1/strategy/import/file`
- `GET /api/v1/strategy/export`
- `GET /api/v1/strategy/pine-script`

## Run locally

### API

```bash
./run_api.sh
```

### Web UI

```bash
./run_web.sh
```

Open `http://localhost:5173`.

## Notes

- Pine Script output is generated deterministically from enabled rules.
- Unsupported indicators/constructs produce warnings in the generated output.
- This branch is not intended for trade execution or broker connectivity.
