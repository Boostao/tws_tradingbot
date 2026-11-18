# Handoff: M4 Headless CLI, Replay & Paper-Probe Decomposition

## Observation
- The R4 Phase 3 requirements mandate a headless CLI for `tws_traderbot`, an `OrderSink` interface for replay, and a `paper-probe` mode for safety validation.
- Existing Wails-bound cockpit/slot logic needs to be extracted to `backend/`.
- `ORIGINAL_REQUEST.md` (R4: Phase 4) provides detailed requirements for `cmd/autopilot`.

## Logic Chain
1.  **Requirement Mapping**: Mapped R4: Phase 4 requirements to a 5-step implementation plan (Refactor, Scaffold, Engine Integration, Mode Implementation, Verification).
2.  **Architectural Separation**: Identified the need for `OrderSink` and `backend/bot/cockpit.go` to cleanly separate business logic from UI/Wails.
3.  **Standalone CLI**: `cmd/autopilot` serves as the entry point for headless modes, isolating it from the Wails runtime.

## Caveats
- None.

## Conclusion
The M4 milestone is ready to be implemented based on the decomposition provided in `analysis.md`. The tasks are structured to ensure architectural integrity, safety, and testability.

## Verification Method
1.  Verify extraction of cockpit logic with `go test -race ./backend/...`.
2.  Verify `cmd/autopilot` compilation with `go build ./cmd/autopilot`.
3.  Verify `--mode=replay` with `go run ./cmd/autopilot --mode=replay`.
4.  Verify `--mode=paper-probe` with `go run ./cmd/autopilot --mode=paper-probe`.
