# GitHub Copilot Agent Instructions — TWS Trader Bot (Go Edition)

## Product Goal
Maintain a local TWS/Interactive Brokers trading bot with:
- Svelte UI embedded via Wails
- Golang Native backend replacing the old Python FastAPI
- SQLite-backed state (`system_config`)
- Rule-engine based strategy execution using concurrent Go native array math
- Direct Golang + `scmhub/ibapi` market-data and execution integration via TCP 7497

## Current State
- The backend operates locally over IPC (Wails), not REST.
- SQLite strictly types and persists state from UI preferences to Watchlists.
- The execution relies on `Engine.EvaluateTick()` which natively accepts arrays of floats against mapped Go strategy structs.
- PineScript ASTs have been deprecated in favor of native Go math (SMA, EMA, Crossover, Slope).
- TWS connection is handled by a custom `IBWrapper` embedding `ibapi.Wrapper`.

## Architecture Map
- Wails App Entry: `main.go`, `app.go`
- Go Backend API context: `backend/`
- SQLite DB logic: `backend/db/`
- Bot Execution Engine: `backend/bot/`
- Strategy Math Evaluator: `backend/strategy/`
- Svelte Frontend: `frontend/`

## Working Rules
- Preserve the single active strategy slot behavior in the cockpit.
- Treat the UI runtime settings as global execution settings.
- Wails IPC bindings (`wails generate module`) sync the Go structs to TypeScript seamlessly. Ensure struct fields intended for Svelte use standard JSON formatting `json:"property"`.
- Never suggest installing Python dependencies since the runtime is strictly Go + Node(build only).

## Validation
- Backend unit tests: `go test -v ./backend/...`
- Build verification: `wails build`
