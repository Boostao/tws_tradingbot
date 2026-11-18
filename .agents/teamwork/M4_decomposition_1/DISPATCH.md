## 2026-11-18T19:19:29Z
Decompose the 'M4 Headless CLI, Replay & Paper-Probe' milestone based on the requirements in `ORIGINAL_REQUEST.md` (R4: Phase 3). 
Tasks include:
1. Extracting cockpit/slot resolution logic to `backend/`.
2. Creating `cmd/autopilot/main.go` and `cmd/autopilot/scenarios.go`.
3. Implementing `OrderSink` interface and `bot.NewReplayRuntime`.
4. Implementing `--mode=replay` with `simSink`.
5. Implementing `--mode=paper-probe` with `ValidatePaperSafety`.
6. Updating `Taskfile.yml` and `.gitignore`.
List milestones, dependencies, and feature inventory mapping.
