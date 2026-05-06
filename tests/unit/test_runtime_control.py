from __future__ import annotations

import importlib
import json
from types import SimpleNamespace

import pandas as pd
import requests
import yaml

from src.bot.execution import RuntimeExecutionConfig, StrategyExecutionPlanner
from src.bot.strategy.rules.models import ActionType
from src.bot.strategy.rules.models import Condition
from src.bot.strategy.rules.models import ConditionType
from src.bot.strategy.rules.models import Indicator
from src.bot.strategy.rules.models import IndicatorType
from src.bot.strategy.rules.models import PriceSource
from src.bot.strategy.rules.models import Rule
from src.bot.strategy.rules.models import RuleScope
from src.bot.strategy.rules.models import Strategy
from src.bot.strategy.rules.models import TimeframeUnit


def _bars(*closes: float) -> pd.DataFrame:
    return pd.DataFrame(
        {
            "open": list(closes),
            "high": list(closes),
            "low": list(closes),
            "close": list(closes),
            "volume": [1000.0 for _ in closes],
        }
    )


def _price_rule(action: ActionType, condition_type: ConditionType, threshold: float) -> Rule:
    return Rule(
        name=f"{action.value}-{condition_type.value}",
        scope=RuleScope.PER_TICKER,
        action=action,
        condition=Condition(
            type=condition_type,
            indicator_a=Indicator(
                type=IndicatorType.PRICE,
                timeframe=TimeframeUnit.M5,
                source=PriceSource.CLOSE,
            ),
            threshold=threshold,
        ),
    )


class FakeProvider:
    def __init__(self, bars: pd.DataFrame, open_orders: list[dict] | None = None, positions: list[dict] | None = None):
        self._bars = bars
        self._open_orders = open_orders or []
        self._positions = positions or []
        self.submitted_orders: list[dict] = []
        self.historical_requests: list[dict] = []

    def is_connected(self) -> bool:
        return True

    def connect(self, timeout: float = 10.0) -> bool:
        return True

    def disconnect(self) -> None:
        return None

    def get_portfolio_positions(self, timeout: float = 10.0, account: str = "") -> list[dict]:
        return list(self._positions)

    def get_open_orders(self, timeout: float = 5.0) -> list[dict]:
        return list(self._open_orders)

    def get_account_summary(self, account: str = "", tags: str = "", timeout: float = 10.0) -> dict:
        return {
            "DU123_NetLiquidation": {
                "account": "DU123",
                "tag": "NetLiquidation",
                "value": "25000",
                "currency": "USD",
            }
        }

    def get_executions(self, timeout: float = 10.0, account: str = "", since=None) -> list[dict]:
        return []

    def get_historical_data(
        self,
        symbol: str,
        exchange: str = "SMART",
        duration: str = "5 D",
        bar_size: str = "5 mins",
        timeout: float = 60.0,
        **_: object,
    ) -> pd.DataFrame:
        self.historical_requests.append(
            {"symbol": symbol, "exchange": exchange, "duration": duration, "bar_size": bar_size, "timeout": timeout}
        )
        assert duration
        assert bar_size
        assert timeout
        return self._bars.copy()

    def place_order(self, symbol: str, action: str, quantity: int, exchange: str = "SMART", **_: object) -> int:
        self.submitted_orders.append(
            {"symbol": symbol, "action": action, "quantity": quantity, "exchange": exchange}
        )
        return len(self.submitted_orders)

    def place_bracket_order(self, **_: object) -> list[int]:
        raise AssertionError("Bracket path should not be used in this test")


def test_update_setting_persists_to_default_yaml(monkeypatch, tmp_path) -> None:
    config_dir = tmp_path / "config"
    config_dir.mkdir()
    (config_dir / "default.yaml").write_text(
        yaml.safe_dump(
            {
                "ib": {"host": "127.0.0.1", "port": 7497, "client_id": 1, "account": "", "timeout": 5, "trading_mode": "paper"},
                "runtime": {"fixed_notional": 10000.0, "bracket_enabled": False, "stop_loss_pct": 2.0, "take_profit_pct": 4.0},
            },
            sort_keys=False,
        )
    )

    monkeypatch.setenv("TRADERBOT_CONFIG_DIR", str(config_dir))

    from src.config import settings as settings_module

    settings_module._settings = None
    settings_module.update_setting("runtime", "fixed_notional", 25000.0)

    saved = yaml.safe_load((config_dir / "default.yaml").read_text())
    assert saved["runtime"]["fixed_notional"] == 25000.0
    assert settings_module.get_redacted_settings()["runtime"]["fixed_notional"] == 25000.0


def test_update_settings_batches_persistence(monkeypatch, tmp_path) -> None:
    config_dir = tmp_path / "config"
    config_dir.mkdir()
    (config_dir / "default.yaml").write_text(
        yaml.safe_dump(
            {
                "ib": {"host": "127.0.0.1", "port": 7497, "client_id": 1, "account": "", "timeout": 5, "trading_mode": "paper"},
                "runtime": {"fixed_notional": 10000.0, "bracket_enabled": False, "stop_loss_pct": 2.0, "take_profit_pct": 4.0},
            },
            sort_keys=False,
        )
    )

    monkeypatch.setenv("TRADERBOT_CONFIG_DIR", str(config_dir))

    from src.config import settings as settings_module

    settings_module._settings = None
    dump_calls = 0
    original_safe_dump = settings_module.yaml.safe_dump

    def counted_safe_dump(*args, **kwargs):
        nonlocal dump_calls
        dump_calls += 1
        return original_safe_dump(*args, **kwargs)

    monkeypatch.setattr(settings_module.yaml, "safe_dump", counted_safe_dump)

    settings_module.update_settings(
        {
            "ib": {"host": "10.0.0.5", "port": 4002, "client_id": 9},
            "runtime": {"fixed_notional": 5000.0, "bracket_enabled": True},
        }
    )

    saved = yaml.safe_load((config_dir / "default.yaml").read_text())
    assert dump_calls == 1
    assert saved["ib"]["host"] == "10.0.0.5"
    assert saved["ib"]["port"] == 4002
    assert saved["ib"]["client_id"] == 9
    assert saved["runtime"]["fixed_notional"] == 5000.0
    assert saved["runtime"]["bracket_enabled"] is True


def test_log_level_env_updates_logging_config(monkeypatch, tmp_path) -> None:
    config_dir = tmp_path / "config"
    config_dir.mkdir()
    (config_dir / "default.yaml").write_text(
        yaml.safe_dump(
            {
                "app": {"log_level": "INFO"},
                "logging": {
                    "level": "INFO",
                    "console": {"enabled": True, "level": "INFO"},
                    "file": {"enabled": True, "path": "logs/trading_bot.log", "backup_count": 5},
                },
            },
            sort_keys=False,
        )
    )

    monkeypatch.setenv("TRADERBOT_CONFIG_DIR", str(config_dir))
    monkeypatch.setenv("LOG_LEVEL", "DEBUG")

    from src.config import settings as settings_module

    settings_module._settings = None
    settings = settings_module.get_settings(force_reload=True)

    assert settings.app.log_level == "DEBUG"
    assert settings.logging.level == "DEBUG"
    assert settings.logging.console_level == "DEBUG"


def test_get_state_uses_cockpit_strategy_name(monkeypatch) -> None:
    from src.api.routers import state as state_router
    from src.bot.state import BotState

    monkeypatch.setattr(state_router, "read_state", lambda: BotState())
    monkeypatch.setattr(
        state_router,
        "load_cockpit_state",
        lambda: {
            "active_workspace_id": "workspace-long",
            "strategy_library": [{"id": "strategy-a", "name": "Strategy A"}],
            "workspaces": [{"id": "workspace-long", "strategy_slots": [{"strategy_id": "strategy-a"}]}],
        },
    )

    payload = state_router.get_state()

    assert payload["active_strategy"] == "Strategy A"
    assert payload["status"] == "STOPPED"


def test_get_state_refreshes_active_strategy_when_runner_not_running(monkeypatch) -> None:
    from src.api.routers import state as state_router
    from src.bot.state import BotState

    monkeypatch.setattr(
        state_router,
        "read_state",
        lambda: BotState(status="STOPPED", active_strategy="Old Strategy"),
    )
    monkeypatch.setattr(
        state_router,
        "load_cockpit_state",
        lambda: {
            "active_workspace_id": "workspace-long",
            "strategy_library": [{"id": "strategy-a", "name": "Strategy A"}],
            "workspaces": [{"id": "workspace-long", "strategy_slots": [{"strategy_id": "strategy-a"}]}],
        },
    )

    payload = state_router.get_state()

    assert payload["active_strategy"] == "Strategy A"


def test_live_runner_loads_active_runtime_context(monkeypatch) -> None:
    from src.bot.live_runner import LiveTradingRunner
    from src.bot.strategy.rules.models import Strategy

    runner = LiveTradingRunner()

    monkeypatch.setattr(
        runner,
        "_check_tws_connection",
        lambda: (True, ""),
    )
    monkeypatch.setattr(
        "src.bot.live_runner.load_cockpit_state",
        lambda: {
            "global_enabled": True,
            "active_workspace_id": "workspace-long",
            "workspaces": [
                {
                    "id": "workspace-long",
                    "kind": "long",
                    "enabled": True,
                    "strategy_slots": [{"strategy_id": "strategy-a"}],
                }
            ],
        },
    )
    monkeypatch.setattr(
        "src.bot.live_runner.load_strategy",
        lambda: Strategy(name="Strategy A", id="strategy-a", tickers=[], rules=[]),
    )
    monkeypatch.setattr(
        "src.bot.live_runner.load_watchlist_state",
        lambda: {"groups": [{"items": [{"symbol": "spy", "exchange": "arca", "enabled": True}]}]},
    )

    context = runner._load_runtime_context()

    assert context.strategy.name == "Strategy A"
    assert context.workspace_kind == "long"
    assert context.instrument_ids == ["SPY.ARCA"]
    assert context.active_instrument_ids == ["SPY.ARCA"]
    assert context.execution_enabled is True


def test_live_runner_rejects_empty_enabled_watchlist(monkeypatch) -> None:
    from src.bot.live_runner import LiveTradingRunner
    from src.bot.strategy.rules.models import Strategy

    runner = LiveTradingRunner()

    monkeypatch.setattr(
        "src.bot.live_runner.load_cockpit_state",
        lambda: {
            "global_enabled": True,
            "active_workspace_id": "workspace-long",
            "workspaces": [
                {
                    "id": "workspace-long",
                    "kind": "long",
                    "enabled": True,
                    "strategy_slots": [{"strategy_id": "strategy-a"}],
                }
            ],
        },
    )
    monkeypatch.setattr(
        "src.bot.live_runner.load_strategy",
        lambda: Strategy(name="Strategy A", id="strategy-a", tickers=[], rules=[]),
    )
    monkeypatch.setattr("src.bot.live_runner.load_watchlist_state", lambda: {"groups": []})

    try:
        runner._load_runtime_context()
    except ValueError as exc:
        assert str(exc) == "No watchlist instruments are configured"
    else:
        raise AssertionError("Expected runtime context loading to fail")


def test_live_runner_keeps_disabled_tickers_in_feed_universe(monkeypatch) -> None:
    from src.bot.live_runner import LiveTradingRunner
    from src.bot.strategy.rules.models import Strategy

    runner = LiveTradingRunner()

    monkeypatch.setattr(
        "src.bot.live_runner.load_cockpit_state",
        lambda: {
            "global_enabled": True,
            "active_workspace_id": "workspace-long",
            "workspaces": [
                {
                    "id": "workspace-long",
                    "kind": "long",
                    "enabled": True,
                    "strategy_slots": [{"strategy_id": "strategy-a", "enabled": True}],
                }
            ],
        },
    )
    monkeypatch.setattr(
        "src.bot.live_runner.load_strategy",
        lambda: Strategy(name="Strategy A", id="strategy-a", tickers=[], rules=[]),
    )
    monkeypatch.setattr(
        "src.bot.live_runner.load_watchlist_state",
        lambda: {
            "groups": [
                {
                    "items": [
                        {"symbol": "spy", "exchange": "arca", "enabled": True},
                        {"symbol": "qqq", "exchange": "nasdaq", "enabled": False},
                    ]
                }
            ]
        },
    )

    context = runner._load_runtime_context()

    assert context.instrument_ids == ["SPY.ARCA", "QQQ.NASDAQ"]
    assert context.active_instrument_ids == ["SPY.ARCA"]
    assert context.execution_enabled is True


def test_live_runner_execution_cycle_submits_tws_order(monkeypatch) -> None:
    from src.bot.live_runner import ActiveRuntimeContext
    from src.bot.live_runner import ExecutionConfig
    from src.bot.live_runner import LiveTradingRunner
    from src.bot.state import BotState

    runner = LiveTradingRunner()
    strategy = Strategy(name="Strategy A", id="strategy-a", tickers=[], rules=[_price_rule(ActionType.BUY, ConditionType.GREATER_THAN, 100.0)])
    runner._runtime_context = ActiveRuntimeContext(strategy=strategy, workspace_kind="long", instrument_ids=["SPY.ARCA"], active_instrument_ids=["SPY.ARCA"])
    runner._planner = StrategyExecutionPlanner(strategy, RuntimeExecutionConfig(fixed_notional=1000.0))
    runner._execution_config = ExecutionConfig(subscriptions=[("SPY.ARCA", TimeframeUnit.M5)], poll_interval_seconds=60.0)
    runner._tws_provider = FakeProvider(_bars(99.0, 125.0))

    state = BotState(status="RUNNING", tws_connected=True)
    monkeypatch.setattr("src.bot.live_runner.read_state", lambda: state)
    monkeypatch.setattr("src.bot.live_runner.update_state", lambda current: current)
    monkeypatch.setattr(runner, "_reload_runtime_context", lambda: runner._runtime_context)

    runner._run_execution_cycle()

    assert runner._tws_provider.submitted_orders == [
        {"symbol": "SPY", "action": "buy", "quantity": 8, "exchange": "ARCA"}
    ]
    assert state.equity == 25000.0
    assert state.pending_orders_count == 1
    assert state.open_positions_count == 0
    assert state.error_message == ""
    assert state.last_runtime_reload_reason == "cycle"
    assert state.last_runtime_reload_at is not None


def test_live_runner_execution_cycle_skips_duplicate_open_order(monkeypatch) -> None:
    from src.bot.live_runner import ActiveRuntimeContext
    from src.bot.live_runner import ExecutionConfig
    from src.bot.live_runner import LiveTradingRunner
    from src.bot.state import BotState

    runner = LiveTradingRunner()
    strategy = Strategy(name="Strategy A", id="strategy-a", tickers=[], rules=[_price_rule(ActionType.BUY, ConditionType.GREATER_THAN, 100.0)])
    runner._runtime_context = ActiveRuntimeContext(strategy=strategy, workspace_kind="long", instrument_ids=["SPY.ARCA"], active_instrument_ids=["SPY.ARCA"])
    runner._planner = StrategyExecutionPlanner(strategy, RuntimeExecutionConfig(fixed_notional=1000.0))
    runner._execution_config = ExecutionConfig(subscriptions=[("SPY.ARCA", TimeframeUnit.M5)], poll_interval_seconds=60.0)
    runner._tws_provider = FakeProvider(
        _bars(99.0, 125.0),
        open_orders=[{"symbol": "SPY", "action": "BUY", "status": "Submitted"}],
    )

    state = BotState(status="RUNNING", tws_connected=True)
    monkeypatch.setattr("src.bot.live_runner.read_state", lambda: state)
    monkeypatch.setattr("src.bot.live_runner.update_state", lambda current: current)
    monkeypatch.setattr(runner, "_reload_runtime_context", lambda: runner._runtime_context)

    runner._run_execution_cycle()

    assert runner._tws_provider.submitted_orders == []
    assert state.pending_orders_count == 1


def test_live_runner_dry_run_returns_planned_orders(monkeypatch) -> None:
    from src.bot.live_runner import ActiveRuntimeContext
    from src.bot.live_runner import ExecutionConfig
    from src.bot.live_runner import LiveTradingRunner
    from src.bot.state import BotState

    runner = LiveTradingRunner()
    strategy = Strategy(name="Strategy A", id="strategy-a", tickers=[], rules=[_price_rule(ActionType.BUY, ConditionType.GREATER_THAN, 100.0)])
    runner._runtime_context = ActiveRuntimeContext(strategy=strategy, workspace_kind="long", instrument_ids=["SPY.ARCA"], active_instrument_ids=["SPY.ARCA"])
    runner._planner = StrategyExecutionPlanner(strategy, RuntimeExecutionConfig(fixed_notional=1000.0))
    runner._execution_config = ExecutionConfig(subscriptions=[("SPY.ARCA", TimeframeUnit.M5)], poll_interval_seconds=60.0)
    runner._tws_provider = FakeProvider(_bars(99.0, 125.0))

    monkeypatch.setattr(runner, "_check_tws_connection", lambda: (True, ""))
    monkeypatch.setattr(runner, "_reload_runtime_context", lambda: runner._runtime_context)
    monkeypatch.setattr(runner, "_ensure_provider_connected", lambda: (True, ""))
    monkeypatch.setattr(runner, "_disconnect_provider", lambda: None)
    monkeypatch.setattr("src.bot.live_runner.read_state", lambda: BotState(status="STOPPED", tws_connected=True))
    monkeypatch.setattr("src.bot.live_runner.update_state", lambda current: current)

    result = runner.dry_run_once()

    assert result["strategy"] == "Strategy A"
    assert result["workspace_kind"] == "long"
    assert result["planned_orders"]
    assert result["planned_orders"][0]["instrument_id"] == "SPY.ARCA"
    assert result["state"]["last_runtime_reload_reason"] == "dry_run"
    assert result["state"]["last_runtime_reload_at"] is not None


def test_live_runner_skips_disabled_ticker_execution_but_keeps_feed(monkeypatch) -> None:
    from src.bot.live_runner import ActiveRuntimeContext
    from src.bot.live_runner import ExecutionConfig
    from src.bot.live_runner import LiveTradingRunner
    from src.bot.state import BotState

    runner = LiveTradingRunner()
    strategy = Strategy(name="Strategy A", id="strategy-a", tickers=[], rules=[_price_rule(ActionType.BUY, ConditionType.GREATER_THAN, 100.0)])
    runner._runtime_context = ActiveRuntimeContext(
        strategy=strategy,
        workspace_kind="long",
        instrument_ids=["SPY.ARCA", "QQQ.NASDAQ"],
        active_instrument_ids=["SPY.ARCA"],
        execution_enabled=True,
    )
    runner._planner = StrategyExecutionPlanner(strategy, RuntimeExecutionConfig(fixed_notional=1000.0))
    runner._execution_config = ExecutionConfig(
        subscriptions=[("SPY.ARCA", TimeframeUnit.M5), ("QQQ.NASDAQ", TimeframeUnit.M5)],
        poll_interval_seconds=60.0,
    )
    runner._tws_provider = FakeProvider(_bars(99.0, 125.0))

    state = BotState(status="RUNNING", tws_connected=True)
    monkeypatch.setattr("src.bot.live_runner.read_state", lambda: state)
    monkeypatch.setattr("src.bot.live_runner.update_state", lambda current: current)
    monkeypatch.setattr(runner, "_reload_runtime_context", lambda: runner._runtime_context)

    runner._run_execution_cycle()

    assert runner._tws_provider.submitted_orders == [
        {"symbol": "SPY", "action": "buy", "quantity": 8, "exchange": "ARCA"}
    ]
    assert [request["symbol"] for request in runner._tws_provider.historical_requests] == ["SPY", "QQQ"]


def test_live_runner_heartbeat_throttles_state_persistence(monkeypatch) -> None:
    from src.bot.live_runner import LiveTradingRunner
    from src.bot.state import BotState

    runner = LiveTradingRunner()
    state = BotState(status="STOPPED", active_strategy="Strategy A")
    persisted: list[BotState] = []
    monotonic_values = iter([0.0, 1.0, 5.1])

    monkeypatch.setattr(runner, "_active_strategy_name", lambda: "Strategy A")
    monkeypatch.setattr("src.bot.live_runner.read_state", lambda: state)
    monkeypatch.setattr("src.bot.live_runner.update_state", lambda current: persisted.append(current) or current)
    monkeypatch.setattr("src.bot.live_runner.time.monotonic", lambda: next(monotonic_values))

    runner._heartbeat()
    runner._heartbeat()
    runner._heartbeat()

    assert len(persisted) == 2


def test_live_runner_skips_trade_ledger_write_without_new_executions(monkeypatch) -> None:
    from src.bot.live_runner import ActiveRuntimeContext
    from src.bot.live_runner import LiveTradingRunner
    from src.bot.state import BotState
    from src.bot.state import TradeLedger

    runner = LiveTradingRunner()
    runner._runtime_context = ActiveRuntimeContext(strategy=None, workspace_kind="long", instrument_ids=["SPY.ARCA"])
    runner._tws_provider = FakeProvider(_bars(99.0, 125.0))

    writes: list[TradeLedger] = []
    monkeypatch.setattr("src.bot.live_runner.read_trade_ledger", lambda: TradeLedger())
    monkeypatch.setattr("src.bot.live_runner.update_trade_ledger", lambda ledger: writes.append(ledger) or ledger)

    state = runner._sync_execution_state(BotState(status="RUNNING", tws_connected=True))

    assert writes == []
    assert state.trades_today == 0


def test_get_symbol_cache_logs_warning_for_invalid_cache(monkeypatch, tmp_path, caplog) -> None:
    from src.api import utils as api_utils

    cache_path = tmp_path / "symbol_cache.json"
    cache_path.write_text("{not-json", encoding="utf-8")
    monkeypatch.setattr(api_utils, "_symbol_cache_path", lambda: cache_path)
    monkeypatch.setattr(api_utils, "_fetch_symbols_from_tradingview", lambda: [])

    with caplog.at_level("WARNING"):
        symbols, source, updated_at, warning = api_utils.get_symbol_cache(refresh=False)

    assert symbols == []
    assert source == "local"
    assert updated_at is None
    assert warning == "symbol_cache_read_failed"
    assert "Failed to read symbol cache file" in caplog.text


def test_get_symbol_cache_falls_back_to_cached_symbols_when_refresh_fails(monkeypatch, tmp_path, caplog) -> None:
    from src.api import utils as api_utils

    cache_path = tmp_path / "symbol_cache.json"
    cache_path.write_text(
        json.dumps(
            {
                "symbols": [{"symbol": "SPY", "exchange": "ARCA", "type": "stock"}],
                "source": "cache",
                "updated_at": "2024-01-01T00:00:00+00:00",
            }
        ),
        encoding="utf-8",
    )
    monkeypatch.setattr(api_utils, "_symbol_cache_path", lambda: cache_path)

    def raise_refresh_error():
        raise requests.RequestException("network down")

    monkeypatch.setattr(api_utils, "_fetch_symbols_from_tradingview", raise_refresh_error)

    with caplog.at_level("WARNING"):
        symbols, source, updated_at, warning = api_utils.get_symbol_cache(refresh=True)

    assert symbols == [{"symbol": "SPY", "exchange": "ARCA", "type": "stock"}]
    assert source == "cache"
    assert updated_at == "2024-01-01T00:00:00+00:00"
    assert warning == "symbol_refresh_failed"
    assert "TradingView symbol refresh failed" in caplog.text
    assert api_utils.get_symbol_cache_diagnostics()["last_warning"] == "symbol_refresh_failed"


def test_bot_state_runner_active_tracks_runtime_status() -> None:
    from src.bot.state import BotState
    from src.bot.state import BotStatus

    assert BotState(status=BotStatus.STOPPED.value).to_dict()["runner_active"] is False
    assert BotState(status=BotStatus.ERROR.value).to_dict()["runner_active"] is False
    assert BotState(status=BotStatus.RUNNING.value).to_dict()["runner_active"] is True
    assert BotState(status=BotStatus.STARTING.value).to_dict()["runner_active"] is True
    assert BotState(status=BotStatus.STOPPING.value).to_dict()["runner_active"] is True


def test_read_state_logs_warning_for_corrupt_state_file(monkeypatch, tmp_path, caplog) -> None:
    from src.bot import state as bot_state

    state_path = tmp_path / "bot_state.json"
    state_path.write_text("{bad-json", encoding="utf-8")
    monkeypatch.setattr(bot_state, "STATE_FILE", state_path)

    with caplog.at_level("WARNING"):
        state = bot_state.read_state()

    assert state.status == bot_state.BotStatus.ERROR.value
    assert state.error_message == "Could not read bot state"
    assert "Failed to read bot state file" in caplog.text


def test_read_trade_ledger_logs_warning_for_corrupt_ledger(monkeypatch, tmp_path, caplog) -> None:
    from src.bot import state as bot_state

    ledger_path = tmp_path / "bot_ledger.json"
    ledger_path.write_text("{bad-json", encoding="utf-8")
    monkeypatch.setattr(bot_state, "LEDGER_FILE", ledger_path)

    with caplog.at_level("WARNING"):
        ledger = bot_state.read_trade_ledger()

    assert ledger.processed_execution_ids == []
    assert ledger.closed_trades == []
    assert "Failed to read trade ledger file" in caplog.text


def test_load_watchlist_state_logs_warning_for_corrupt_file(monkeypatch, tmp_path, caplog) -> None:
    from src.api import utils as api_utils

    watchlist_path = tmp_path / "watchlist.json"
    watchlist_path.write_text("{bad-json", encoding="utf-8")
    monkeypatch.setattr(api_utils, "_watchlist_state_path", lambda: watchlist_path)
    monkeypatch.setattr(api_utils, "_read_legacy_watchlist_symbols", lambda: [])

    with caplog.at_level("WARNING"):
        state = api_utils.load_watchlist_state()

    assert len(state["groups"]) == 1
    assert state["groups"][0]["items"] == []
    assert "Failed to read watchlist state file" in caplog.text


def test_load_cockpit_state_logs_warning_for_corrupt_file(monkeypatch, tmp_path, caplog) -> None:
    from src.api import utils as api_utils

    cockpit_path = tmp_path / "cockpit.json"
    cockpit_path.write_text("{bad-json", encoding="utf-8")
    monkeypatch.setattr(api_utils, "_cockpit_state_path", lambda: cockpit_path)
    monkeypatch.setattr(api_utils, "load_watchlist_state", lambda: {"groups": [], "feed": None, "updated_at": None})
    monkeypatch.setattr(
        api_utils,
        "list_strategy_library",
        lambda: [{"id": "strategy-a", "name": "Strategy A", "rule_count": 1, "enabled_rule_count": 1, "source": "active"}],
    )

    with caplog.at_level("WARNING"):
        state = api_utils.load_cockpit_state()

    assert state["workspaces"]
    assert "Failed to read cockpit state file" in caplog.text


def test_disconnect_tws_records_manual_disconnect_reason(monkeypatch) -> None:
    from src.api.routers import state as state_router
    from src.bot.state import BotState
    from src.bot.state import BotStatus

    state = BotState(status=BotStatus.RUNNING.value, tws_connected=True)
    monkeypatch.setattr(state_router, "read_state", lambda: state)
    monkeypatch.setattr(state_router, "update_state", lambda current: current)

    payload = state_router.disconnect_tws()

    assert payload["status"] == "disconnected"
    assert state.status == BotStatus.DISCONNECTED.value
    assert state.last_disconnect_reason == "Manual disconnect requested"
    assert state.last_disconnect_at is not None


def test_diagnostics_router_returns_runtime_and_startup_snapshot(monkeypatch) -> None:
    from src.api.routers import diagnostics as diagnostics_router
    from src.bot.state import BotState
    from src.bot.state import BotStatus

    monkeypatch.setenv("TRADING_BOT_ENV", "production")
    monkeypatch.setattr(
        diagnostics_router,
        "get_settings",
        lambda force_reload=False: SimpleNamespace(
            ib=SimpleNamespace(host="127.0.0.1", port=7497, client_id=7, account="DU123456", trading_mode="paper"),
            logging=SimpleNamespace(level="DEBUG", file_path="logs/trading_bot.log"),
            app=SimpleNamespace(
                watchlist_path="config/watchlist.txt",
                active_strategy_path="config/active_strategy.json",
                symbol_cache_path="data/symbol_cache.json",
            ),
        ),
    )
    monkeypatch.setattr(
        diagnostics_router,
        "read_state",
        lambda: BotState(
            status=BotStatus.RUNNING.value,
            last_runtime_reload_at="2026-05-05T10:00:00+00:00",
            last_runtime_reload_reason="cycle",
            last_disconnect_at="2026-05-05T09:30:00+00:00",
            last_disconnect_reason="Lost TWS connection",
        ),
    )
    monkeypatch.setattr(
        diagnostics_router,
        "get_symbol_cache_diagnostics",
        lambda: {"source": "cache", "last_checked_at": "2026-05-05T09:45:00+00:00", "last_warning": "symbol_refresh_failed"},
    )

    payload = diagnostics_router.get_diagnostics()

    assert payload.startup.environment == "production"
    assert payload.startup.account == "DU***56"
    assert payload.runtime.runner_active is True
    assert payload.runtime.last_runtime_reload_reason == "cycle"
    assert payload.runtime.last_disconnect_reason == "Lost TWS connection"
    assert payload.symbols.last_warning == "symbol_refresh_failed"