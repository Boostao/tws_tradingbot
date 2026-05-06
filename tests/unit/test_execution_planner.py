from __future__ import annotations

import pandas as pd

from src.api.utils import normalized_watchlist_instruments
from src.bot.execution import PositionSnapshot
from src.bot.execution import RuntimeExecutionConfig
from src.bot.execution import StrategyExecutionPlanner
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


def _strategy(*rules: Rule) -> Strategy:
    return Strategy(
        name="Planner Test",
        tickers=[],
        rules=list(rules),
    )


def test_normalized_watchlist_instruments_returns_unique_enabled_ids() -> None:
    groups = [
        {
            "id": "group-1",
            "name": "Manual",
            "source": "manual",
            "items": [
                {"symbol": "spy", "exchange": "arca", "enabled": True},
                {"symbol": "spy", "exchange": "arca", "enabled": True},
                {"symbol": "qqq", "exchange": "nasdaq", "enabled": False},
                {"symbol": "iwm", "exchange": "", "enabled": True},
            ],
        }
    ]

    assert normalized_watchlist_instruments(groups) == ["SPY.ARCA", "IWM.SMART"]


def test_planner_sizes_entry_from_fixed_notional_and_adds_brackets() -> None:
    planner = StrategyExecutionPlanner(
        _strategy(_price_rule(ActionType.BUY, ConditionType.GREATER_THAN, 100.0)),
        RuntimeExecutionConfig(
            fixed_notional=1000.0,
            bracket_enabled=True,
            stop_loss_pct=2.0,
            take_profit_pct=4.0,
        ),
    )

    orders = planner.plan_orders({"SPY.ARCA": _bars(99.0, 101.0, 125.0)})

    assert len(orders) == 1
    assert orders[0].instrument_id == "SPY.ARCA"
    assert orders[0].side == "buy"
    assert orders[0].quantity == 8
    assert orders[0].bracket is not None
    assert orders[0].bracket.stop_loss_price == 122.5
    assert orders[0].bracket.take_profit_price == 130.0


def test_planner_suppresses_duplicate_entry_when_position_open() -> None:
    planner = StrategyExecutionPlanner(
        _strategy(_price_rule(ActionType.BUY, ConditionType.GREATER_THAN, 100.0)),
        RuntimeExecutionConfig(fixed_notional=1000.0),
    )

    orders = planner.plan_orders(
        {"SPY.ARCA": _bars(101.0, 125.0)},
        positions={"SPY.ARCA": PositionSnapshot(instrument_id="SPY.ARCA", quantity=5, average_price=120.0)},
    )

    assert orders == []


def test_planner_emits_exit_and_allows_reentry_after_flat() -> None:
    exit_planner = StrategyExecutionPlanner(
        _strategy(_price_rule(ActionType.SELL, ConditionType.LESS_THAN, 100.0)),
        RuntimeExecutionConfig(fixed_notional=1000.0),
    )

    exit_orders = exit_planner.plan_orders(
        {"SPY.ARCA": _bars(110.0, 98.0)},
        positions={"SPY.ARCA": PositionSnapshot(instrument_id="SPY.ARCA", quantity=5, average_price=120.0)},
    )

    assert len(exit_orders) == 1
    assert exit_orders[0].side == "sell"
    assert exit_orders[0].quantity == 5
    assert exit_orders[0].bracket is None

    entry_planner = StrategyExecutionPlanner(
        _strategy(_price_rule(ActionType.BUY, ConditionType.GREATER_THAN, 100.0)),
        RuntimeExecutionConfig(fixed_notional=1000.0),
    )

    entry_orders = entry_planner.plan_orders(
        {"SPY.ARCA": _bars(99.0, 125.0)},
        positions={"SPY.ARCA": PositionSnapshot(instrument_id="SPY.ARCA", quantity=0, average_price=None)},
    )

    assert len(entry_orders) == 1
    assert entry_orders[0].side == "buy"
    assert entry_orders[0].quantity == 8


def test_planner_uses_runtime_tracked_tickers_without_mutating_strategy() -> None:
    strategy = Strategy(
        name="Planner Test",
        tickers=["AAPL"],
        rules=[_price_rule(ActionType.BUY, ConditionType.GREATER_THAN, 100.0)],
    )
    planner = StrategyExecutionPlanner(strategy, RuntimeExecutionConfig(fixed_notional=1000.0))

    subscriptions = planner.get_required_subscriptions(["SPY.ARCA"])
    orders = planner.plan_orders(
        {"SPY.ARCA": _bars(99.0, 101.0, 125.0)},
        tracked_tickers=["SPY.ARCA"],
    )

    assert strategy.tickers == ["AAPL"]
    assert ("SPY.ARCA", TimeframeUnit.M5) in subscriptions
    assert orders[0].instrument_id == "SPY.ARCA"