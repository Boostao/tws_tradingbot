from __future__ import annotations

import argparse
import json
import logging
import signal
import socket
import time
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path

from src.api.utils import load_cockpit_state, load_strategy, load_strategy_preset, load_watchlist_state, normalized_watchlist_instruments
from src.bot.execution import PlannedOrder
from src.bot.execution import PositionSnapshot
from src.bot.execution import RuntimeExecutionConfig, StrategyExecutionPlanner
from src.bot.instruments import split_instrument_id
from src.bot.state import BotState, BotStatus, COMMAND_FILE, STOP_SIGNAL_FILE, TradeLedger, clear_stop_signals, read_state, read_trade_ledger, update_state, update_trade_ledger
from src.bot.strategy.rules.models import Strategy
from src.bot.strategy.rules.models import TimeframeUnit
from src.bot.tws_data_provider import IBAPI_AVAILABLE, TWSDataProvider
from src.config.settings import get_settings
from src.utils.logger import setup_logging


logger = logging.getLogger(__name__)

OPEN_ORDER_STATUSES = {"ApiPending", "PendingSubmit", "PreSubmitted", "Submitted"}
BUY_SIDES = {"BOT", "BUY"}
HEARTBEAT_PERSIST_INTERVAL_SECONDS = 5.0
TIMEFRAME_CONFIG = {
    TimeframeUnit.M1: ("1 min", "2 D", 30.0),
    TimeframeUnit.M5: ("5 mins", "5 D", 60.0),
    TimeframeUnit.M15: ("15 mins", "10 D", 120.0),
    TimeframeUnit.M30: ("30 mins", "20 D", 180.0),
    TimeframeUnit.H1: ("1 hour", "30 D", 300.0),
    TimeframeUnit.H4: ("4 hours", "90 D", 600.0),
    TimeframeUnit.D1: ("1 day", "1 Y", 1800.0),
}


@dataclass
class ActiveRuntimeContext:
    strategy: Strategy | None
    workspace_kind: str
    instrument_ids: list[str]
    active_instrument_ids: list[str] = field(default_factory=list)
    execution_enabled: bool = True

    def __post_init__(self) -> None:
        if not self.active_instrument_ids:
            self.active_instrument_ids = list(self.instrument_ids)


@dataclass(frozen=True)
class ExecutionConfig:
    subscriptions: list[tuple[str, TimeframeUnit]]
    poll_interval_seconds: float


class LiveTradingRunner:
    def __init__(self) -> None:
        self.settings = get_settings(force_reload=True)
        self._shutdown_requested = False
        self._runtime_context: ActiveRuntimeContext | None = None
        self._planner: StrategyExecutionPlanner | None = None
        self._tws_provider: TWSDataProvider | None = None
        self._execution_config: ExecutionConfig | None = None
        self._next_execution_at = 0.0
        self._last_heartbeat_persist_at: float | None = None
        self._last_heartbeat_strategy = ""
        signal.signal(signal.SIGINT, self._signal_handler)
        signal.signal(signal.SIGTERM, self._signal_handler)

    def _signal_handler(self, signum: int, _frame) -> None:
        logger.info("Received signal %s, shutting down", signum)
        self._shutdown_requested = True

    def _mask_account(self) -> str:
        raw = str(self.settings.ib.account or "").strip()
        if not raw:
            return "unset"
        if len(raw) <= 4:
            return raw
        return f"{raw[:2]}***{raw[-2:]}"

    def _log_bootstrap_summary(self, check_only: bool = False) -> None:
        logger.info(
            "Runner bootstrap | mode=%s ib_host=%s ib_port=%s client_id=%s account=%s log_file=%s watchlist=%s strategy=%s check_only=%s ibapi=%s",
            self.settings.ib.trading_mode,
            self.settings.ib.host,
            self.settings.ib.port,
            self.settings.ib.client_id,
            self._mask_account(),
            Path(self.settings.logging.file_path),
            self.settings.app.watchlist_path,
            self.settings.app.active_strategy_path,
            check_only,
            IBAPI_AVAILABLE,
        )

    def _log_runtime_context_summary(self, runtime_context: ActiveRuntimeContext) -> None:
        subscription_count = len(self._execution_config.subscriptions) if self._execution_config is not None else 0
        poll_interval = self._execution_config.poll_interval_seconds if self._execution_config is not None else 0.0
        logger.info(
            "Runtime context | strategy=%s workspace=%s execution_enabled=%s feed_instruments=%s active_instruments=%s subscriptions=%s poll_interval=%.1fs",
            runtime_context.strategy.name if runtime_context.strategy is not None else "unassigned",
            runtime_context.workspace_kind,
            runtime_context.execution_enabled,
            len(runtime_context.instrument_ids),
            len(runtime_context.active_instrument_ids),
            subscription_count,
            poll_interval,
        )

    def _active_strategy_name(self) -> str:
        if self._runtime_context is not None and self._runtime_context.strategy is not None:
            return self._runtime_context.strategy.name
        try:
            runtime_context = self._load_runtime_context()
            return runtime_context.strategy.name if runtime_context.strategy is not None else ""
        except ValueError:
            return ""

    def _load_runtime_context(self) -> ActiveRuntimeContext:
        cockpit = load_cockpit_state()
        active_workspace_id = cockpit.get("active_workspace_id")
        active_workspace = next(
            (workspace for workspace in cockpit.get("workspaces", []) if workspace.get("id") == active_workspace_id),
            cockpit.get("workspaces", [None])[0] if cockpit.get("workspaces") else None,
        )
        if not active_workspace:
            raise ValueError("No workspace is configured")

        watchlist_groups = load_watchlist_state().get("groups", [])
        instrument_ids = normalized_watchlist_instruments(watchlist_groups, include_disabled=True)
        if not instrument_ids:
            raise ValueError("No watchlist instruments are configured")
        active_instrument_ids = normalized_watchlist_instruments(watchlist_groups)

        slots = active_workspace.get("strategy_slots", [])
        slot = slots[0] if slots else {}
        strategy_id = str(slot.get("strategy_id") or "").strip()
        strategy: Strategy | None = None
        if strategy_id:
            strategy = load_strategy()
            if strategy.id != strategy_id:
                strategy = load_strategy_preset(strategy_id)

        execution_enabled = bool(
            cockpit.get("global_enabled", True)
            and active_workspace.get("enabled", True)
            and slot.get("enabled", True)
            and strategy is not None
            and active_instrument_ids
        )

        return ActiveRuntimeContext(
            strategy=strategy,
            workspace_kind=str(active_workspace.get("kind") or "long"),
            instrument_ids=instrument_ids,
            active_instrument_ids=active_instrument_ids,
            execution_enabled=execution_enabled,
        )

    def _build_planner(self, strategy: Strategy | None) -> StrategyExecutionPlanner | None:
        if strategy is None:
            return None
        self.settings = get_settings(force_reload=True)
        return StrategyExecutionPlanner(
            strategy,
            RuntimeExecutionConfig(
                fixed_notional=self.settings.runtime.fixed_notional,
                bracket_enabled=self.settings.runtime.bracket_enabled,
                stop_loss_pct=self.settings.runtime.stop_loss_pct,
                take_profit_pct=self.settings.runtime.take_profit_pct,
            ),
        )

    def _build_execution_config(self) -> ExecutionConfig:
        subscriptions = self._planner.get_required_subscriptions(self._runtime_context.instrument_ids) if self._planner and self._runtime_context else []
        unique_subscriptions: list[tuple[str, TimeframeUnit]] = []
        seen: set[tuple[str, str]] = set()
        for symbol, timeframe in subscriptions:
            key = (symbol, timeframe.value if hasattr(timeframe, "value") else str(timeframe))
            if key in seen:
                continue
            seen.add(key)
            unique_subscriptions.append((symbol, timeframe))
        if not unique_subscriptions:
            unique_subscriptions = [(instrument_id, TimeframeUnit.M5) for instrument_id in self._runtime_context.instrument_ids] if self._runtime_context else []
        smallest = min(
            (TIMEFRAME_CONFIG.get(timeframe, TIMEFRAME_CONFIG[TimeframeUnit.M5])[2] for _, timeframe in unique_subscriptions),
            default=TIMEFRAME_CONFIG[TimeframeUnit.M5][2],
        )
        return ExecutionConfig(
            subscriptions=unique_subscriptions,
            poll_interval_seconds=smallest,
        )

    def _reload_runtime_context(self) -> ActiveRuntimeContext:
        runtime_context = self._load_runtime_context()
        self._runtime_context = runtime_context
        self._planner = self._build_planner(runtime_context.strategy)
        self._execution_config = self._build_execution_config()
        return runtime_context

    def _create_provider(self) -> TWSDataProvider:
        provider_client_id = self.settings.ib.client_id + 100
        return TWSDataProvider(
            host=self.settings.ib.host,
            port=self.settings.ib.port,
            client_id=provider_client_id,
        )

    def _ensure_provider_connected(self) -> tuple[bool, str]:
        if not IBAPI_AVAILABLE:
            return False, "ibapi is not installed in the current environment"
        if self._tws_provider is None:
            self._tws_provider = self._create_provider()
        if self._tws_provider.is_connected() or self._tws_provider.connect(timeout=float(self.settings.ib.timeout)):
            return True, ""
        return False, "TWS provider connection failed"

    def _disconnect_provider(self) -> None:
        if self._tws_provider is not None:
            self._tws_provider.disconnect()
            self._tws_provider = None

    def _summary_value(self, summary: dict[str, dict], tag: str) -> float:
        for value in summary.values():
            if value.get("tag") == tag:
                try:
                    return float(value.get("value") or 0.0)
                except (TypeError, ValueError):
                    return 0.0
        return 0.0

    def _today_prefix(self) -> str:
        return datetime.now(timezone.utc).date().isoformat()

    def _append_recent_log(self, state: BotState, message: str) -> None:
        timestamp = datetime.now(timezone.utc).strftime("%H:%M:%S")
        state.recent_logs = [f"[{timestamp}] {message}", *state.recent_logs][:50]

    def _normalize_side(self, side: object) -> str:
        return str(side or "").strip().upper()

    def _parse_execution_timestamp(self, raw_value: object) -> str:
        raw = str(raw_value or "").strip()
        if not raw:
            return datetime.now(timezone.utc).isoformat()
        if "T" in raw:
            return raw
        clean = raw[:17]
        try:
            parsed = datetime.strptime(clean, "%Y%m%d-%H:%M:%S")
            return parsed.replace(tzinfo=timezone.utc).isoformat()
        except ValueError:
            return raw

    def _stamp_runtime_reload(self, state: BotState, reason: str) -> None:
        state.last_runtime_reload_at = datetime.now(timezone.utc).isoformat()
        state.last_runtime_reload_reason = reason

    def _stamp_disconnect(self, state: BotState, reason: str) -> None:
        state.last_disconnect_at = datetime.now(timezone.utc).isoformat()
        state.last_disconnect_reason = reason

    def _apply_execution_to_ledger(self, ledger: TradeLedger, execution: dict[str, object]) -> dict[str, object] | None:
        exec_id = str(execution.get("exec_id") or "").strip()
        if not exec_id or exec_id in ledger.processed_execution_ids:
            return None

        symbol = str(execution.get("symbol") or "").strip().upper()
        if not symbol:
            return None
        side = self._normalize_side(execution.get("side"))
        shares = abs(float(execution.get("shares") or 0.0))
        price = float(execution.get("price") or 0.0)
        occurred_at = self._parse_execution_timestamp(execution.get("time"))
        if shares <= 0 or price <= 0:
            ledger.processed_execution_ids.append(exec_id)
            return None

        direction = 1.0 if side in BUY_SIDES else -1.0
        delta = direction * shares
        current = ledger.open_positions.get(symbol, {"quantity": 0.0, "avg_price": 0.0})
        quantity = float(current.get("quantity") or 0.0)
        average_price = float(current.get("avg_price") or 0.0)
        trade_event: dict[str, object] | None = None

        if quantity == 0 or quantity * delta > 0:
            new_quantity = quantity + delta
            weighted_notional = (abs(quantity) * average_price) + (shares * price)
            ledger.open_positions[symbol] = {
                "quantity": new_quantity,
                "avg_price": weighted_notional / abs(new_quantity) if new_quantity else 0.0,
            }
        else:
            close_quantity = min(abs(quantity), shares)
            pnl = (price - average_price) * close_quantity * (1.0 if quantity > 0 else -1.0)
            trade_event = {
                "exec_id": exec_id,
                "symbol": symbol,
                "side": side,
                "quantity": close_quantity,
                "price": price,
                "closed_at": occurred_at,
                "pnl": round(pnl, 4),
                "win": pnl > 0,
            }
            ledger.closed_trades.append(trade_event)
            remaining = quantity + delta
            if remaining == 0:
                ledger.open_positions.pop(symbol, None)
            elif quantity * remaining < 0:
                ledger.open_positions[symbol] = {"quantity": remaining, "avg_price": price}
            else:
                ledger.open_positions[symbol] = {"quantity": remaining, "avg_price": average_price}

        ledger.processed_execution_ids.append(exec_id)
        return trade_event

    def _sync_execution_state(self, state: BotState) -> BotState:
        if self._tws_provider is None:
            return state
        ledger = read_trade_ledger()
        initial_processed_count = len(ledger.processed_execution_ids)
        initial_closed_count = len(ledger.closed_trades)
        initial_open_positions = dict(ledger.open_positions)
        since = datetime.now(timezone.utc).replace(hour=0, minute=0, second=0, microsecond=0)
        executions = self._tws_provider.get_executions(timeout=5.0, account=self.settings.ib.account, since=since)
        new_trade_events: list[dict[str, object]] = []
        for execution in executions:
            trade_event = self._apply_execution_to_ledger(ledger, execution)
            if trade_event is not None:
                new_trade_events.append(trade_event)

        if new_trade_events:
            for trade_event in new_trade_events:
                direction = trade_event.get("side")
                symbol = trade_event.get("symbol")
                quantity = trade_event.get("quantity")
                price = trade_event.get("price")
                pnl = float(trade_event.get("pnl") or 0.0)
                self._append_recent_log(state, f"Fill {direction} {quantity} {symbol} @ {price:.2f} | PnL {pnl:.2f}")

        if (
            len(ledger.processed_execution_ids) != initial_processed_count
            or len(ledger.closed_trades) != initial_closed_count
            or ledger.open_positions != initial_open_positions
        ):
            update_trade_ledger(ledger)
        today_prefix = self._today_prefix()
        closed_today = [trade for trade in ledger.closed_trades if str(trade.get("closed_at") or "").startswith(today_prefix)]
        wins = [trade for trade in closed_today if bool(trade.get("win"))]
        state.trades_today = len(closed_today)
        state.win_rate_today = (len(wins) / len(closed_today) * 100.0) if closed_today else 0.0
        state.recent_trades = list(reversed(ledger.closed_trades[-10:]))
        return state

    def _build_position_snapshots(
        self,
        positions: list[dict[str, object]],
        instrument_ids: list[str],
    ) -> dict[str, PositionSnapshot]:
        by_symbol = {str(position.get("symbol") or "").upper(): position for position in positions}
        snapshots: dict[str, PositionSnapshot] = {}
        for instrument_id in instrument_ids:
            descriptor = split_instrument_id(instrument_id)
            raw_position = by_symbol.get(descriptor.symbol, {})
            quantity = int(float(raw_position.get("position") or 0.0)) if raw_position else 0
            average_price = float(raw_position.get("avg_cost") or 0.0) if raw_position else 0.0
            snapshots[instrument_id] = PositionSnapshot(
                instrument_id=instrument_id,
                quantity=quantity,
                average_price=average_price or None,
            )
        return snapshots

    def _fetch_market_data(self) -> dict[str, object]:
        if self._execution_config is None or self._tws_provider is None:
            return {}
        market_data: dict[str, object] = {}
        for symbol_key, timeframe in self._execution_config.subscriptions:
            descriptor = split_instrument_id(symbol_key)
            bar_size, duration, _ = TIMEFRAME_CONFIG.get(timeframe, TIMEFRAME_CONFIG[TimeframeUnit.M5])
            frame = self._tws_provider.get_historical_data(
                symbol=descriptor.symbol,
                exchange=descriptor.venue,
                duration=duration,
                bar_size=bar_size,
                timeout=max(15.0, float(self.settings.ib.timeout) * 3.0),
            )
            if frame.empty:
                continue
            timeframe_key = timeframe.value if hasattr(timeframe, "value") else str(timeframe)
            for alias in {symbol_key, descriptor.symbol.upper()}:
                current = market_data.setdefault(alias, {})
                if isinstance(current, dict):
                    current[timeframe_key] = frame
            time.sleep(0.2)
        return market_data

    def _filter_planned_orders(
        self,
        planned_orders: list[PlannedOrder],
        open_orders: list[dict[str, object]],
    ) -> list[PlannedOrder]:
        active_pairs = {
            (str(order.get("symbol") or "").upper(), str(order.get("action") or "").upper())
            for order in open_orders
            if str(order.get("status") or "") in OPEN_ORDER_STATUSES
        }
        filtered: list[PlannedOrder] = []
        for order in planned_orders:
            symbol = split_instrument_id(order.instrument_id).symbol
            pair = (symbol, order.side.upper())
            if pair in active_pairs:
                logger.info("Skipping duplicate open order for %s %s", order.side.upper(), symbol)
                continue
            filtered.append(order)
        return filtered

    def _submit_planned_orders(self, planned_orders: list[PlannedOrder]) -> int:
        if self._tws_provider is None:
            return 0
        submitted = 0
        for order in planned_orders:
            descriptor = split_instrument_id(order.instrument_id)
            if order.bracket is not None:
                order_ids = self._tws_provider.place_bracket_order(
                    symbol=descriptor.symbol,
                    action=order.side,
                    quantity=order.quantity,
                    stop_loss_price=order.bracket.stop_loss_price,
                    take_profit_price=order.bracket.take_profit_price,
                    exchange=descriptor.venue,
                )
                if order_ids:
                    submitted += 1
                continue
            order_id = self._tws_provider.place_order(
                symbol=descriptor.symbol,
                action=order.side,
                quantity=order.quantity,
                exchange=descriptor.venue,
            )
            if order_id is not None:
                submitted += 1
        return submitted

    def _refresh_broker_state(self, state: BotState) -> tuple[BotState, dict[str, PositionSnapshot], list[dict[str, object]]]:
        if self._tws_provider is None or self._runtime_context is None:
            return state, {}, []
        positions = self._tws_provider.get_portfolio_positions(timeout=5.0, account=self.settings.ib.account)
        open_orders = self._tws_provider.get_open_orders(timeout=5.0)
        account_summary = self._tws_provider.get_account_summary(account=self.settings.ib.account, timeout=5.0)

        state.tws_connected = self._tws_provider.is_connected()
        state.open_positions_count = len([position for position in positions if float(position.get("position") or 0.0) != 0.0])
        state.pending_orders_count = len([order for order in open_orders if str(order.get("status") or "") in OPEN_ORDER_STATUSES])
        state.equity = self._summary_value(account_summary, "NetLiquidation")
        state.total_pnl = sum(
            float(position.get("unrealized_pnl") or 0.0) + float(position.get("realized_pnl") or 0.0)
            for position in positions
        )
        state.recent_orders = list(reversed(open_orders[-10:]))
        state = self._sync_execution_state(state)
        return state, self._build_position_snapshots(positions, self._runtime_context.instrument_ids), open_orders

    def _run_execution_cycle(self) -> None:
        state = read_state()
        try:
            runtime_context = self._reload_runtime_context()
            self._stamp_runtime_reload(state, "cycle")
            state, positions, open_orders = self._refresh_broker_state(state)
            market_data = self._fetch_market_data()
            if not market_data:
                state.error_message = "No market data returned from TWS for watchlist instruments"
                update_state(state)
                return
            if not runtime_context.execution_enabled or self._planner is None or not runtime_context.active_instrument_ids:
                state.active_strategy = runtime_context.strategy.name if runtime_context.strategy is not None else ""
                state.error_message = ""
                update_state(state)
                return
            planned_orders = self._planner.plan_orders(
                market_data=market_data,
                positions=positions,
                workspace_kind=runtime_context.workspace_kind,
                tracked_tickers=runtime_context.active_instrument_ids,
            )
            submitted = self._submit_planned_orders(self._filter_planned_orders(planned_orders, open_orders))
            state.pending_orders_count += submitted
            state.active_strategy = runtime_context.strategy.name if runtime_context.strategy is not None else ""
            state.error_message = ""
            update_state(state)
        except ValueError as exc:
            state.error_message = str(exc)
            update_state(state)
        except Exception as exc:
            logger.exception("Execution cycle failed")
            state.status = BotStatus.ERROR.value
            state.error_message = str(exc)
            update_state(state)

    def dry_run_once(self) -> dict[str, object]:
        connected, error_message = self._check_tws_connection()
        if not connected:
            raise RuntimeError(f"TWS connection required before dry run: {error_message}")
        self.settings = get_settings(force_reload=True)
        runtime_context = self._reload_runtime_context()
        provider_connected, provider_error = self._ensure_provider_connected()
        if not provider_connected:
            raise RuntimeError(provider_error)
        try:
            state, positions, open_orders = self._refresh_broker_state(read_state())
            self._stamp_runtime_reload(state, "dry_run")
            market_data = self._fetch_market_data()
            planned_orders: list[PlannedOrder] = []
            filtered_orders: list[PlannedOrder] = []
            if runtime_context.execution_enabled and self._planner is not None and runtime_context.active_instrument_ids:
                planned_orders = self._planner.plan_orders(
                    market_data=market_data,
                    positions=positions,
                    workspace_kind=runtime_context.workspace_kind,
                    tracked_tickers=runtime_context.active_instrument_ids,
                )
                filtered_orders = self._filter_planned_orders(planned_orders, open_orders)
            result = {
                "strategy": runtime_context.strategy.name if runtime_context.strategy is not None else "",
                "workspace_kind": runtime_context.workspace_kind,
                "subscriptions": [
                    {"symbol": symbol, "timeframe": timeframe.value if hasattr(timeframe, "value") else str(timeframe)}
                    for symbol, timeframe in self._execution_config.subscriptions
                ],
                "positions": {key: snapshot.__dict__ for key, snapshot in positions.items()},
                "open_orders": open_orders,
                "planned_orders": [
                    {
                        "instrument_id": order.instrument_id,
                        "side": order.side,
                        "quantity": order.quantity,
                        "price": order.price,
                        "reason": order.reason,
                        "bracket": order.bracket.__dict__ if order.bracket else None,
                    }
                    for order in filtered_orders
                ],
                "state": state.to_dict(),
                "generated_at": datetime.now(timezone.utc).isoformat(),
            }
            state.last_dry_run = result
            update_state(state)
            return result
        finally:
            self._disconnect_provider()

    def _heartbeat(self, state: BotState | None = None) -> BotState:
        current_state = state or read_state()
        current_state.last_heartbeat = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
        current_state.active_strategy = self._active_strategy_name()

        now = time.monotonic()
        should_persist = (
            self._last_heartbeat_persist_at is None
            or (now - self._last_heartbeat_persist_at) >= HEARTBEAT_PERSIST_INTERVAL_SECONDS
            or current_state.active_strategy != self._last_heartbeat_strategy
        )
        if not should_persist:
            return current_state

        self._last_heartbeat_persist_at = now
        self._last_heartbeat_strategy = current_state.active_strategy
        return update_state(current_state)

    def _check_tws_connection(self) -> tuple[bool, str]:
        self.settings = get_settings(force_reload=True)
        try:
            with socket.create_connection(
                (self.settings.ib.host, self.settings.ib.port),
                timeout=self.settings.ib.timeout,
            ):
                return True, ""
        except OSError as exc:
            return False, str(exc)

    def _read_command(self) -> dict | None:
        if not COMMAND_FILE.exists():
            return None
        try:
            with open(COMMAND_FILE, "r") as command_file:
                payload = json.load(command_file)
        except Exception:
            COMMAND_FILE.unlink(missing_ok=True)
            return None
        COMMAND_FILE.unlink(missing_ok=True)
        return payload

    def _handle_start(self) -> None:
        state = read_state()
        connected, error_message = self._check_tws_connection()
        if not connected:
            state.status = BotStatus.ERROR.value
            state.tws_connected = False
            state.error_message = f"TWS connection required before bot start: {error_message}"
            update_state(state)
            logger.error(state.error_message)
            return

        try:
            self.settings = get_settings(force_reload=True)
            runtime_context = self._reload_runtime_context()
        except ValueError as exc:
            state.status = BotStatus.ERROR.value
            state.tws_connected = True
            state.error_message = str(exc)
            update_state(state)
            logger.error(state.error_message)
            return

        provider_connected, provider_error = self._ensure_provider_connected()
        if not provider_connected:
            state.status = BotStatus.ERROR.value
            state.tws_connected = False
            state.error_message = provider_error
            update_state(state)
            logger.error(state.error_message)
            return

        state.status = BotStatus.RUNNING.value
        state.tws_connected = True
        state.error_message = ""
        state.active_strategy = runtime_context.strategy.name if runtime_context.strategy is not None else ""
        self._stamp_runtime_reload(state, "startup")
        update_state(state)
        self._log_runtime_context_summary(runtime_context)
        logger.info(
            "Bot started with strategy '%s' on %s feed instruments and %s active instruments (%s workspace)",
            state.active_strategy or "unassigned",
            len(runtime_context.instrument_ids),
            len(runtime_context.active_instrument_ids),
            runtime_context.workspace_kind,
        )
        self._next_execution_at = 0.0
        self._run_execution_cycle()

    def _handle_stop(self) -> None:
        state = read_state()
        state.status = BotStatus.STOPPED.value
        state.error_message = ""
        update_state(state)
        clear_stop_signals()
        self._runtime_context = None
        self._planner = None
        self._execution_config = None
        self._next_execution_at = 0.0
        self._disconnect_provider()
        logger.info("Bot stopped")

    def run(self) -> int:
        logger.info("Starting live runner control loop")
        state = read_state()
        state.status = BotStatus.STOPPED.value
        state.active_strategy = self._active_strategy_name()
        update_state(state)

        while not self._shutdown_requested:
            state = self._heartbeat()

            command = self._read_command()
            if command and command.get("command") == "start":
                self._handle_start()

            if STOP_SIGNAL_FILE.exists():
                self._handle_stop()

            if state.status == BotStatus.RUNNING.value:
                connected, error_message = self._check_tws_connection()
                if not connected:
                    state = read_state()
                    state.status = BotStatus.DISCONNECTED.value
                    state.tws_connected = False
                    state.error_message = f"Lost TWS connection: {error_message}"
                    self._stamp_disconnect(state, error_message)
                    update_state(state)
                    self._disconnect_provider()
                elif self._execution_config is not None and time.monotonic() >= self._next_execution_at:
                    self._run_execution_cycle()
                    self._next_execution_at = time.monotonic() + self._execution_config.poll_interval_seconds

            time.sleep(1.0)

        final_state = read_state()
        final_state.status = BotStatus.STOPPED.value
        update_state(final_state)
        self._disconnect_provider()
        return 0


def main() -> int:
    parser = argparse.ArgumentParser(description="Run the local TWS traderbot control loop")
    parser.add_argument("--check", action="store_true", help="Only verify whether TWS is reachable")
    args = parser.parse_args()

    setup_logging()
    runner = LiveTradingRunner()
    runner._log_bootstrap_summary(check_only=args.check)

    if args.check:
        connected, error_message = runner._check_tws_connection()
        if connected:
            logger.info("TWS reachable at %s:%s", runner.settings.ib.host, runner.settings.ib.port)
            return 0
        logger.error("TWS unreachable at %s:%s: %s", runner.settings.ib.host, runner.settings.ib.port, error_message)
        return 1

    return runner.run()


if __name__ == "__main__":
    raise SystemExit(main())