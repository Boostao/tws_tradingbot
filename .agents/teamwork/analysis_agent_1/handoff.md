# Handoff Report: Investigation of `TestRunner_HeadlessMockFlow` failure

## Observation
- `TestRunner_HeadlessMockFlow` (in `backend/bot/runner_test.go`) pushes synthetic bar data to `IBWrapper` but fails to trigger order execution, leaving `nextOrderID` at the initialized value of 500.
- `IBWrapper.HistoricalDataEnd` in `backend/bot/ibkr.go` contains the comment: `// Evaluation moved to snapshot mechanism`.
- `EvaluateTick` (the main rule evaluation function in `backend/bot/engine.go`) is not called within `TestRunner_HeadlessMockFlow`.
- The build currently fails due to unrelated issues (`bypass_test.go:22:8: undefined: ibapi.SetConnected`).

## Logic Chain
1. `TestRunner_HeadlessMockFlow` pushes data, then expects `nextOrderID` to increment.
2. `nextOrderID` increments only if `ExecuteAction` is called.
3. `ExecuteAction` is only called if `EvaluateTick` identifies a trigger.
4. `EvaluateTick` is not called in `runner_test.go`.
5. Historically, evaluation was triggered in `HistoricalDataEnd` (now disabled).
6. Therefore, the test relies on an outdated, removed mechanism to trigger evaluation and is failing because it does not use the current mechanism.

## Caveats
- Build failure (`bypass_test.go`) prevented running the test suite to confirm the specific error message *during* this session (the error was provided in the prompt).
- I assumed the "snapshot mechanism" mentioned in `ibkr.go` implies a different way to trigger `EvaluateTick` that `runner_test.go` is not currently using.

## Conclusion
`TestRunner_HeadlessMockFlow` is failing because it is outdated. The engine's rule evaluation logic has been moved out of `HistoricalDataEnd`, and `runner_test.go` has not been updated to use the new evaluation trigger (e.g., calling `EvaluateTick` or utilizing the `SubscriptionManager`).

## Verification Method
1. Resolve the build issues in `backend/bot/bypass_test.go` (`undefined: ibapi.SetConnected`).
2. Run `go test -v ./backend/bot -run TestRunner_HeadlessMockFlow`.
3. Update `TestRunner_HeadlessMockFlow` to use the current evaluation trigger (e.g., call `eng.EvaluateTick` directly or simulate a snapshot transition).
