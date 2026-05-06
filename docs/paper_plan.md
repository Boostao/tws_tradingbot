# Paper Plan - 2026-05-06

Prepared on 2026-05-05 for tomorrow's paper session.

## Session Objective

- Validate the long-side paper workflow on the active `Achat` / `workspace-long` cockpit workspace.
- Run the current `example-ema-crossover-strategy` (`EMA Crossover with VIX Filter`) with a controlled symbol universe instead of the full watchlist.
- Confirm clean TWS connectivity, clean runtime reloads, sensible dry-run output, and stable diagnostics through the session.

## Current Starting State

- Trading mode: `paper`
- TWS endpoint: `127.0.0.1:7497`
- Client ID: `1`
- Active cockpit workspace: `workspace-long`
- Enabled strategy: `EMA Crossover with VIX Filter`
- Runtime sizing: `fixed_notional: 10000`
- Brackets: `disabled`
- Strategy logic: 5 minute EMA 9/21 crossover entries gated by declining VIX slope and market-hours filtering

## Focus Universe For Tomorrow

The runtime executes on enabled watchlist symbols, not just the strategy's original ticker list. For tomorrow, do not run against the full current watchlist.

Enable only these instruments before the dry run:

- `VIX.TVC`
- `SPY.AMEX`
- `QQQ.NASDAQ`
- `AAPL.NASDAQ`
- `MSFT.NASDAQ`
- `GOOGL.NASDAQ`

Disable every other watchlist symbol before starting the bot.

If you want a smaller opening test, start with only:

- `VIX.TVC`
- `SPY.AMEX`
- `QQQ.NASDAQ`

Then expand to the full focus set only after the first clean hour.

## Timeline (ET)

### 08:45 - 09:00 Preflight

1. Start TWS paper and confirm API access is enabled on port `7497`.
2. Set `LOG_LEVEL=DEBUG` for the session if you want full diagnostics detail.
3. From `tws_traderbot/`, run:

```bash
./run_api.sh
./run_web.sh
./run_bot.sh --check
```

4. Open the cockpit UI and confirm the app loads cleanly.

### 09:00 - 09:20 Cockpit Prep

1. Confirm `workspace-long` is the active workspace.
2. Confirm only one strategy slot is enabled globally.
3. Trim the watchlist to the focus universe above.
4. Verify the Diagnostics card shows:
   - TWS endpoint `127.0.0.1:7497`
   - client ID `1`
   - expected log level
   - no unresolved symbol warning

### 09:20 - 09:29 Dry-Run Gate

Run a dry run from the UI.

Do not start the bot unless all of these are true:

- the strategy name is correct,
- subscriptions are limited to the intended universe,
- planned orders, if any, only reference the focus symbols,
- no runtime error is shown,
- the Diagnostics panel remains current.

### 09:30 - 10:30 Opening Session

1. Start the bot only if the dry run is clean.
2. Avoid changing strategy or workspace settings during the first few cycles.
3. Watch these fields closely:
   - `status`
   - `last runtime reload`
   - `last broker disconnect`
   - symbol warning state

### 10:30 - 15:30 Steady-State Monitor

1. Let the runner operate without unnecessary UI churn.
2. If you must change the watchlist, do it deliberately and verify the next cycle reflects the change.
3. Check `logs/trading_bot.log` if diagnostics drift from what the cockpit shows.

### 15:45 - 16:05 Controlled Shutdown

1. Stop the bot from the UI.
2. Confirm the worker returns to `STOPPED`.
3. Review recent orders, recent trades, and the log file.
4. Capture anything unexpected before the next session.

## Go / No-Go Criteria

### Go

Proceed only if all of the following are true:

- TWS connects successfully.
- The cockpit shows only one enabled strategy/workspace combination.
- The watchlist is trimmed to the intended focus universe.
- Dry run output only references intended symbols.
- Diagnostics update normally and do not show repeated disconnects.

### No-Go

Do not proceed if any of the following occur:

- TWS cannot connect or drops repeatedly before the open.
- VIX data is missing.
- Dry run includes unexpected symbols.
- `last runtime reload` stops advancing after startup.
- The bot enters `ERROR` or `DISCONNECTED` before the session is stable.

## Success Criteria

Tomorrow counts as a successful paper session if you get all of the following:

- one clean startup,
- one clean dry run,
- at least several stable execution cycles without repeated disconnects,
- no unexpected duplicate order behavior,
- diagnostics and logs that agree with each other.

It is acceptable if the bot produces few or no trades, as long as the runtime behavior is clean and observable.

## Evidence To Capture

- one screenshot of the Diagnostics panel before starting the bot,
- one screenshot or note from the dry-run output,
- end-of-day notes covering:
  - whether the session was go or no-go,
  - any disconnect reason,
  - any symbol warning,
  - any manual changes made during the session.

## Allowed Manual Intervention

Allowed tomorrow:

- connect or disconnect TWS,
- stop the bot,
- enable or disable watchlist symbols,
- abort the session if diagnostics go stale.

Avoid tomorrow unless you are explicitly aborting the test:

- changing the active strategy mid-session,
- switching workspaces during the open,
- widening the watchlist back to the full symbol set.