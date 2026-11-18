## 2026-11-18T18:12:49Z
Refactor `backend/bot/ibkr.go` to replace the legacy callback evaluation mechanism with a snapshot-based approach for market data. Ensure `HistoricalDataEnd` does not perform evaluation directly, but instead coordinates with a snapshot mechanism or updated evaluation trigger to comply with the requirement that signals should only be evaluated on closed-bar transitions. Ensure compliance with the design specified in ORIGINAL_REQUEST.md (Phase 1, R2).


## 2026-11-18T18:29:05Z
Implement the `PlaceAutopilotOrder` chokepoint in a new file `backend/bot/autopilot_orders.go`. This function must be the sole caller of `PlaceOrder` and `CancelOrder`. It must check `autopilotExecMode`, verify `ValidatePaperSafety`, set `OrderRef` to `"AUTOPILOT:" + runID`, enforce `max_qty_per_order`, notional caps, `max_orders_per_run`, and `max_open_positions`. Fail closed on missing/stale prices or invalid equity/sizing. Ensure it is compatible with existing infrastructure.

## 2026-11-18T19:20:28Z
Extract cockpit/slot resolution logic from `app.go` into `backend/bot/cockpit.go` to support headless operation. Ensure `Engine` and other backend components remain decoupled from Wails-specific logic. Verify the extraction with `go test -race ./backend/...`.
## 2026-11-18T20:22:43Z
Update `backend/bot/runner_test.go` to use the new evaluation trigger mechanism.
Since `EvaluateTick` evaluation has moved to a "snapshot mechanism" in the `SubscriptionManager` and is no longer triggered in `HistoricalDataEnd`, modify `TestRunner_HeadlessMockFlow` to manually trigger `engine.EvaluateTick()` after pushing synthetic bars to ensure rule evaluation occurs.

Working directory: /home/bruno/Work/Active project/cobalt/tws_traderbot
