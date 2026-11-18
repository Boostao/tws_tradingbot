# Analysis: M4 Headless CLI, Replay & Paper-Probe Decomposition

## Objective
Decompose the M4 milestone (Headless CLI, Replay & Paper-Probe) based on R4: Phase 4 requirements from `ORIGINAL_REQUEST.md`.

## Task Decomposition

### 1. Architectural Extraction (Backend Cockpit Logic)
- **Goal**: Move cockpit/slot resolution and `1/1` concurrency logic from `app.go` to `backend/`.
- **Target**: `backend/bot/cockpit.go` (new) or `backend/api/cockpit.go` (if existing API structure allows).
- **Rationale**: Decouple business logic from Wails runtime for headless operation.

### 2. Autopilot CLI Implementation (`cmd/autopilot`)
- **Goal**: Create headless entry point and scenario runner.
- **Files**:
    - `cmd/autopilot/main.go`: Entry point, flag parsing (`--mode`, `--db`, `--source-db`, `--host`, etc.), `os/signal` handling, logger setup.
    - `cmd/autopilot/scenarios.go`: Test scenarios, helper for setting up `bot.Engine` in headless mode.
- **Rationale**: Provide standalone CLI for automated testing and probes.

### 3. Simulation & Replay Engine (`bot.NewReplayRuntime`)
- **Goal**: Implement `OrderSink` for headless replay and simulation.
- **Files**:
    - `backend/bot/sink.go` (new `OrderSink` interface).
    - `backend/bot/replay.go` (implementation of `bot.NewReplayRuntime`).
- **Rationale**: Enable deterministic testing without TWS connection.

### 4. Mode Implementations
- **`--mode=replay`**: Configure `bot.NewReplayRuntime` with `simSink`, run against built-in scenarios or external cassette.
- **`--mode=paper-probe`**: Configure `bot.NewReplayRuntime` with real `IBKRClient` but wrapped with `ValidatePaperSafety`. Requires `7497`/`4002` + `DU` account validation.

### 5. Infrastructure Updates
- **`Taskfile.yml`**: Add `autopilot:replay` and `autopilot:probe` tasks.
- **`.gitignore`**: Add `build/autopilot/`.

## Dependencies
- `backend/api` / `backend/db` (for configuration and state loading).
- `third_party/ibapi` (for `paper-probe`).

## Feature Inventory Mapping
| Feature | Implementation Component |
|---|---|
| Headless Entry | `cmd/autopilot` |
| Cockpit Logic | `backend/bot/cockpit.go` |
| Replay Sink | `OrderSink` (interface) + `simSink` |
| Safety Probe | `ValidatePaperSafety` (in `ibkr.go` or dedicated file) |
| CLI Tasks | `Taskfile.yml` |

## Milestones & Logic Chain
1.  **Refactor**: Extract `backend/` logic (Dependencies: None).
2.  **CLI Scaffold**: Create `cmd/autopilot` (Dependencies: `backend/` extraction).
3.  **Engine Integration**: Add `OrderSink` and `NewReplayRuntime` (Dependencies: `cmd/autopilot` scaffold).
4.  **Mode Implementation**: Implement `--mode=replay` and `--mode=paper-probe` (Dependencies: `OrderSink`/`NewReplayRuntime`).
5.  **Test/Verification**: Run `replay` and `paper-probe` against test scenarios.
