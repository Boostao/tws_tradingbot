# Paper Soak Runbook

This runbook is the operator path for a paper-trading soak session on the local TWS trader bot.

## Goal

Prove that the local stack can:

- connect cleanly to TWS paper,
- reload cockpit, strategy, and watchlist state without restart,
- fetch market data for the configured feed universe,
- plan orders without unexpected duplicates,
- survive disconnects with actionable diagnostics,
- shut down cleanly without leaving the operator blind.

## Preflight

Before market hours, verify all of the following:

1. TWS paper is running and logged in.
2. TWS API access is enabled and the socket port matches the bot configuration.
3. `./run_bot.sh --check` succeeds.
4. `LOG_LEVEL=DEBUG` is set if you want full soak diagnostics.
5. The cockpit has exactly one intended active workspace/strategy combination.
6. The watchlist contains the intended symbols and exchanges.
7. The active strategy has already passed dry-run and unit-test validation.

## Launch Sequence

Start the stack in separate terminals from `tws_traderbot/`:

```bash
./run_api.sh
./run_web.sh
./run_bot.sh --check
./run_bot.sh
```

Open `http://localhost:5173`.

## Operator Flow

1. Confirm the sidebar shows TWS disconnected, then use the UI connect action.
2. Open Cockpit and verify the Diagnostics card shows the expected endpoint, client id, log level, and paths.
3. Run a dry run and confirm:
   - the active strategy is correct,
   - subscriptions are plausible,
   - planned orders are sensible,
   - no unexpected runtime error appears.
4. Start the bot only after the dry run is clean.
5. Watch the Runtime and Diagnostics cards for at least the first few execution cycles.

## During The Soak

Keep these checks active during the session:

1. `status` stays `RUNNING` unless you intentionally stop the worker.
2. `last runtime reload` advances over time.
3. `last broker disconnect` stays empty unless a real disconnect occurs.
4. Symbol warnings remain empty, or if populated they are understood and logged.
5. The feed universe and active ticker behavior match your cockpit toggles.
6. `logs/trading_bot.log` contains startup summaries, runtime context summaries, and any disconnect reason.

## If Something Breaks

Use this order of operations:

1. Do not restart everything immediately. Read the Cockpit Diagnostics panel first.
2. Check `logs/trading_bot.log` for the latest startup, reload, and disconnect lines.
3. If TWS is unreachable, fix TWS first and only then reconnect from the UI.
4. If strategy/watchlist state looks wrong, use Cockpit reload before restarting the worker.
5. If the bot is unhealthy, stop it from the UI, correct the state, then rerun a dry run before restarting.

## Clean Shutdown

At the end of the session:

1. Stop the bot from the UI.
2. Confirm the worker returns to `STOPPED`.
3. Review recent trades, recent orders, and the log file.
4. Capture any disconnect reasons, symbol warnings, or unexpected reload behavior before the next session.