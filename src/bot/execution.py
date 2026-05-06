from __future__ import annotations

from dataclasses import dataclass
from math import floor

import pandas as pd

from src.bot.instruments import split_instrument_id
from src.bot.strategy.rules.engine import RuleEngine
from src.bot.strategy.rules.market_data import get_market_frame
from src.bot.strategy.rules.models import ActionType, Strategy


@dataclass(frozen=True)
class PositionSnapshot:
    instrument_id: str
    quantity: int = 0
    average_price: float | None = None

    @property
    def is_flat(self) -> bool:
        return self.quantity == 0

    @property
    def is_long(self) -> bool:
        return self.quantity > 0

    @property
    def is_short(self) -> bool:
        return self.quantity < 0


@dataclass(frozen=True)
class BracketOrder:
    stop_loss_price: float
    take_profit_price: float


@dataclass(frozen=True)
class PlannedOrder:
    instrument_id: str
    symbol: str
    side: str
    quantity: int
    price: float
    reason: str
    bracket: BracketOrder | None = None


@dataclass(frozen=True)
class RuntimeExecutionConfig:
    fixed_notional: float
    bracket_enabled: bool = False
    stop_loss_pct: float = 0.0
    take_profit_pct: float = 0.0


def calculate_fixed_notional_quantity(fixed_notional: float, price: float) -> int:
    if fixed_notional <= 0 or price <= 0:
        return 0
    return floor(fixed_notional / price)


def build_entry_bracket(side: str, price: float, config: RuntimeExecutionConfig) -> BracketOrder | None:
    if not config.bracket_enabled:
        return None
    stop_loss_pct = max(config.stop_loss_pct, 0.0) / 100.0
    take_profit_pct = max(config.take_profit_pct, 0.0) / 100.0
    normalized_side = side.strip().upper()
    if normalized_side == "BUY":
        return BracketOrder(
            stop_loss_price=round(price * (1.0 - stop_loss_pct), 4),
            take_profit_price=round(price * (1.0 + take_profit_pct), 4),
        )
    return BracketOrder(
        stop_loss_price=round(price * (1.0 + stop_loss_pct), 4),
        take_profit_price=round(price * (1.0 - take_profit_pct), 4),
    )


class StrategyExecutionPlanner:
    def __init__(self, strategy: Strategy, config: RuntimeExecutionConfig):
        self._strategy = strategy
        self._config = config

    def get_required_subscriptions(self, tracked_tickers: list[str]) -> list[tuple[str, object]]:
        return RuleEngine(self._strategy).get_required_data_subscriptions(tickers=tracked_tickers)

    def plan_orders(
        self,
        market_data: dict[str, object],
        positions: dict[str, PositionSnapshot] | None = None,
        vix_data: pd.DataFrame | None = None,
        workspace_kind: str = "long",
        tracked_tickers: list[str] | None = None,
    ) -> list[PlannedOrder]:
        positions = positions or {}
        tracked_tickers = tracked_tickers or list(market_data.keys())
        engine = RuleEngine(self._strategy)
        signals = engine.evaluate_all(market_data=market_data, vix_bars=vix_data, tickers=tracked_tickers)
        orders: list[PlannedOrder] = []

        entry_action, exit_action = _workspace_actions(workspace_kind)

        for instrument_id in tracked_tickers:
            frame = get_market_frame(market_data, instrument_id)
            if frame is None or frame.empty:
                continue

            latest_price = _latest_close(frame)
            if latest_price <= 0:
                continue

            position = positions.get(instrument_id, PositionSnapshot(instrument_id=instrument_id))
            descriptor = split_instrument_id(instrument_id)
            actions = signals.get(instrument_id, [])

            if _has_action(actions, exit_action) and not position.is_flat:
                orders.append(
                    PlannedOrder(
                        instrument_id=instrument_id,
                        symbol=descriptor.symbol,
                        side=exit_action.value,
                        quantity=abs(position.quantity),
                        price=latest_price,
                        reason="exit_signal",
                    )
                )
                continue

            if not _has_action(actions, entry_action) or not position.is_flat:
                continue

            quantity = calculate_fixed_notional_quantity(self._config.fixed_notional, latest_price)
            if quantity <= 0:
                continue

            orders.append(
                PlannedOrder(
                    instrument_id=instrument_id,
                    symbol=descriptor.symbol,
                    side=entry_action.value,
                    quantity=quantity,
                    price=latest_price,
                    reason="entry_signal",
                    bracket=build_entry_bracket(entry_action.value, latest_price, self._config),
                )
            )

        return orders


def _workspace_actions(workspace_kind: str) -> tuple[ActionType, ActionType]:
    if workspace_kind.strip().lower() == "short":
        return ActionType.SELL, ActionType.BUY
    return ActionType.BUY, ActionType.SELL


def _has_action(actions: list[str], action: ActionType) -> bool:
    expected = action.value.upper()
    return any(str(candidate).upper() == expected for candidate in actions)


def _latest_close(frame: pd.DataFrame) -> float:
    value = frame["close"].iloc[-1]
    return float(value)