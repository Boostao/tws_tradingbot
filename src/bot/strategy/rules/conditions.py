"""
Condition Evaluator Module

Evaluates trading rule conditions against market data.
This module handles the logic for each condition type (crossovers, comparisons, etc.)
"""

import numpy as np
import pandas as pd
from datetime import datetime, time
from typing import Dict, Optional, Any
from numpy.typing import NDArray

from src.bot.strategy.rules.models import (
    Condition,
    ConditionType,
    Indicator,
)
from src.bot.strategy.rules.indicators import IndicatorFactory
from src.utils.indicators import crosses_above, crosses_below, slope


class ConditionEvaluator:
    """
    Evaluates a single condition against market data.
    
    Supports various condition types including crossovers, comparisons,
    slope checks, and time-based conditions.
    """
    
    def __init__(self, condition: Condition):
        """
        Initialize the evaluator with a condition.
        
        Args:
            condition: The condition to evaluate
        """
        self.condition = condition
        self._indicator_factory = IndicatorFactory()
        self._market_data: Optional[Dict[str, pd.DataFrame]] = None
    
    def evaluate(
        self,
        bars: pd.DataFrame,
        vix_bars: Optional[pd.DataFrame] = None,
        current_time: Optional[datetime] = None,
        market_data: Optional[Dict[str, pd.DataFrame]] = None,
    ) -> bool:
        """
        Evaluate the condition against provided data.
        
        Args:
            bars: DataFrame with OHLCV bar data
            vix_bars: Optional DataFrame with VIX data for VIX conditions
            current_time: Current time for time-based conditions (defaults to now)
            
        Returns:
            True if the condition is satisfied, False otherwise
        """
        self._market_data = market_data
        condition_type = self._get_condition_type()
        
        if condition_type == "crosses_above":
            return self._evaluate_crosses_above(bars, vix_bars)
        
        elif condition_type == "crosses_below":
            return self._evaluate_crosses_below(bars, vix_bars)
        
        elif condition_type == "greater_than":
            return self._evaluate_greater_than(bars, vix_bars)
        
        elif condition_type == "less_than":
            return self._evaluate_less_than(bars, vix_bars)
        
        elif condition_type == "slope_above":
            return self._evaluate_slope_above(bars, vix_bars)
        
        elif condition_type == "slope_below":
            return self._evaluate_slope_below(bars, vix_bars)
        
        elif condition_type == "within_range":
            return self._evaluate_within_range(current_time)
        
        elif condition_type == "equals":
            return self._evaluate_equals(bars, vix_bars)
        
        else:
            raise ValueError(f"Unsupported condition type: {condition_type}")
    
    def _get_condition_type(self) -> str:
        """Get condition type as string."""
        return self.condition.type.value if hasattr(self.condition.type, 'value') else self.condition.type
    
    def _get_indicator_series(
        self,
        indicator: Indicator,
        bars: pd.DataFrame,
        vix_bars: Optional[pd.DataFrame] = None
    ) -> NDArray[np.float64]:
        """Get indicator series from bar data."""
        bars_for_indicator = bars

        if indicator.symbol and self._market_data:
            symbol_key = indicator.symbol
            if symbol_key in self._market_data:
                bars_for_indicator = self._market_data[symbol_key]
            else:
                # Try fallback keys
                symbol_upper = symbol_key.upper()
                if symbol_upper in self._market_data:
                    bars_for_indicator = self._market_data[symbol_upper]
                elif "." in symbol_key:
                    base_symbol = symbol_key.split(".")[0]
                    if base_symbol in self._market_data:
                        bars_for_indicator = self._market_data[base_symbol]
        return self._indicator_factory.create_indicator_series(indicator, bars_for_indicator, vix_bars)
    
    def _evaluate_crosses_above(
        self,
        bars: pd.DataFrame,
        vix_bars: Optional[pd.DataFrame] = None
    ) -> bool:
        """Evaluate CROSSES_ABOVE condition."""
        if self.condition.indicator_b is None:
            return False
        
        series_a = self._get_indicator_series(self.condition.indicator_a, bars, vix_bars)
        series_b = self._get_indicator_series(self.condition.indicator_b, bars, vix_bars)
        
        return crosses_above(series_a, series_b)
    
    def _evaluate_crosses_below(
        self,
        bars: pd.DataFrame,
        vix_bars: Optional[pd.DataFrame] = None
    ) -> bool:
        """Evaluate CROSSES_BELOW condition."""
        if self.condition.indicator_b is None:
            return False
        
        series_a = self._get_indicator_series(self.condition.indicator_a, bars, vix_bars)
        series_b = self._get_indicator_series(self.condition.indicator_b, bars, vix_bars)
        
        return crosses_below(series_a, series_b)
    
    def _evaluate_greater_than(
        self,
        bars: pd.DataFrame,
        vix_bars: Optional[pd.DataFrame] = None
    ) -> bool:
        """Evaluate GREATER_THAN condition."""
        series_a = self._get_indicator_series(self.condition.indicator_a, bars, vix_bars)
        
        # Get latest valid value from series_a
        valid_a = series_a[~np.isnan(series_a)]
        if len(valid_a) == 0:
            return False
        latest_a = valid_a[-1]
        
        if self.condition.indicator_b is not None:
            # Compare to another indicator
            series_b = self._get_indicator_series(self.condition.indicator_b, bars, vix_bars)
            valid_b = series_b[~np.isnan(series_b)]
            if len(valid_b) == 0:
                return False
            latest_b = valid_b[-1]
            return float(latest_a) > float(latest_b)
        
        elif self.condition.threshold is not None:
            # Compare to threshold
            return float(latest_a) > self.condition.threshold
        
        return False
    
    def _evaluate_less_than(
        self,
        bars: pd.DataFrame,
        vix_bars: Optional[pd.DataFrame] = None
    ) -> bool:
        """Evaluate LESS_THAN condition."""
        series_a = self._get_indicator_series(self.condition.indicator_a, bars, vix_bars)
        
        # Get latest valid value from series_a
        valid_a = series_a[~np.isnan(series_a)]
        if len(valid_a) == 0:
            return False
        latest_a = valid_a[-1]
        
        if self.condition.indicator_b is not None:
            # Compare to another indicator
            series_b = self._get_indicator_series(self.condition.indicator_b, bars, vix_bars)
            valid_b = series_b[~np.isnan(series_b)]
            if len(valid_b) == 0:
                return False
            latest_b = valid_b[-1]
            return float(latest_a) < float(latest_b)
        
        elif self.condition.threshold is not None:
            # Compare to threshold
            return float(latest_a) < self.condition.threshold
        
        return False
    
    def _evaluate_slope_above(
        self,
        bars: pd.DataFrame,
        vix_bars: Optional[pd.DataFrame] = None
    ) -> bool:
        """Evaluate SLOPE_ABOVE condition."""
        if self.condition.threshold is None:
            return False
        
        series_a = self._get_indicator_series(self.condition.indicator_a, bars, vix_bars)
        slope_value = slope(series_a, self.condition.lookback_periods)
        
        return slope_value > self.condition.threshold
    
    def _evaluate_slope_below(
        self,
        bars: pd.DataFrame,
        vix_bars: Optional[pd.DataFrame] = None
    ) -> bool:
        """Evaluate SLOPE_BELOW condition."""
        if self.condition.threshold is None:
            return False
        
        series_a = self._get_indicator_series(self.condition.indicator_a, bars, vix_bars)
        slope_value = slope(series_a, self.condition.lookback_periods)
        
        return slope_value < self.condition.threshold
    
    def _evaluate_within_range(
        self,
        current_time: Optional[datetime] = None
    ) -> bool:
        """
        Evaluate WITHIN_RANGE condition for time-based rules.
        
        Checks if current time is within the specified time range.
        """
        if self.condition.range_start is None or self.condition.range_end is None:
            return False
        
        if current_time is None:
            current_time = datetime.now()
        
        # Parse time strings (format: "HH:MM")
        try:
            start_parts = self.condition.range_start.split(":")
            end_parts = self.condition.range_end.split(":")
            
            start_time = time(int(start_parts[0]), int(start_parts[1]))
            end_time = time(int(end_parts[0]), int(end_parts[1]))
            
            current = current_time.time()
            
            # Handle ranges that cross midnight
            if start_time <= end_time:
                return start_time <= current <= end_time
            else:
                # Range crosses midnight (e.g., 22:00 - 06:00)
                return current >= start_time or current <= end_time
                
        except (ValueError, IndexError):
            return False
    
    def _evaluate_equals(
        self,
        bars: pd.DataFrame,
        vix_bars: Optional[pd.DataFrame] = None
    ) -> bool:
        """Evaluate EQUALS condition."""
        series_a = self._get_indicator_series(self.condition.indicator_a, bars, vix_bars)
        
        # Get latest valid value from series_a
        valid_a = series_a[~np.isnan(series_a)]
        if len(valid_a) == 0:
            return False
        latest_a = valid_a[-1]
        
        if self.condition.indicator_b is not None:
            # Compare to another indicator
            series_b = self._get_indicator_series(self.condition.indicator_b, bars, vix_bars)
            valid_b = series_b[~np.isnan(series_b)]
            if len(valid_b) == 0:
                return False
            latest_b = valid_b[-1]
            # Use approximate equality for floats
            return abs(float(latest_a) - float(latest_b)) < 1e-6
        
        elif self.condition.threshold is not None:
            # Compare to threshold
            return abs(float(latest_a) - self.condition.threshold) < 1e-6
        
        return False


def evaluate_condition(
    condition: Condition,
    bars: pd.DataFrame,
    vix_bars: Optional[pd.DataFrame] = None,
    current_time: Optional[datetime] = None
) -> bool:
    """
    Convenience function to evaluate a condition.
    
    Args:
        condition: The condition to evaluate
        bars: DataFrame with OHLCV bar data
        vix_bars: Optional DataFrame with VIX data
        current_time: Current time for time-based conditions
        
    Returns:
        True if condition is satisfied, False otherwise
    """
    evaluator = ConditionEvaluator(condition)
    return evaluator.evaluate(bars, vix_bars, current_time)
