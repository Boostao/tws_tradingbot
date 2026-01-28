"""
Strategy Module

Contains the dynamic rule-based trading strategy implementation
for Nautilus Trader integration.

Main Components:
- DynamicRuleStrategy: Core strategy logic for rule evaluation
- DynamicRuleStrategyConfig: Configuration for strategy instances
- BarBuffer: Rolling buffer for OHLCV data storage
"""

from src.bot.strategy.base import (
    DynamicRuleStrategy,
    DynamicRuleStrategyConfig,
    BarBuffer,
    NAUTILUS_AVAILABLE,
)

__all__ = [
    "DynamicRuleStrategy",
    "DynamicRuleStrategyConfig",
    "BarBuffer",
    "NAUTILUS_AVAILABLE",
]
