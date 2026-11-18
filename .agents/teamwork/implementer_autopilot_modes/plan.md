# Plan — Autopilot Modes

1.  Modify `cmd/autopilot/main.go`:
    *   Update `mode` flag to include `replay` and `paper-probe`.
    *   Add logic to initialize `bot.NewReplayRuntime` or `bot.NewIBKRClient` based on mode.
2.  In `replay` mode:
    *   Configure `engine` (with DryRun=true or controlled by flags).
    *   Initialize `LogSink` (simple implementation of `OrderSink`).
    *   Call `bot.NewReplayRuntime`.
3.  In `paper-probe` mode:
    *   Initialize `bot.NewIBKRClient(engine)`.
    *   Set `engine.AutopilotController.mode = bot.ModePaperLive`.
4.  Verify using existing tools (`task test:e2e` or similar, check `cmd/autopilot/scenarios.go` for helpers).
