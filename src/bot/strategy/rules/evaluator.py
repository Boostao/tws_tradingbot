"""
Rule History Evaluator Module

Evaluates trading rules across historical bar data to determine when
conditions were TRUE. Used primarily for visualization in the UI.
"""

import numpy as np
import pandas as pd
from typing import Optional
from datetime import datetime

from src.bot.strategy.rules.models import (
    Rule,
    Condition,
    ConditionType,
    Indicator,
)
from src.bot.strategy.rules.indicators import IndicatorFactory
from src.utils.indicators import crosses_above, crosses_below, slope_series


def evaluate_rule_history(
    rule: Rule,
    bars: pd.DataFrame,
    vix_bars: Optional[pd.DataFrame] = None
) -> pd.Series:
    """
    Evaluate a rule across historical bar data.
    
    Returns a boolean Series indicating True/False for each bar,
    showing when the rule's condition was satisfied.
    
    Args:
        rule: The Rule object to evaluate
        bars: DataFrame with OHLCV bar data
        vix_bars: Optional DataFrame with VIX data for VIX-based rules
        
    Returns:
        pd.Series of boolean values (True where condition was met)
    """
    if bars is None or len(bars) == 0:
        return pd.Series(dtype=bool)
    
    condition = rule.condition
    return evaluate_condition_history(condition, bars, vix_bars)


def evaluate_condition_history(
    condition: Condition,
    bars: pd.DataFrame,
    vix_bars: Optional[pd.DataFrame] = None
) -> pd.Series:
    """
    Evaluate a condition across all bars in the dataset.
    
    Args:
        condition: The Condition to evaluate
        bars: DataFrame with OHLCV bar data
        vix_bars: Optional VIX data
        
    Returns:
        pd.Series of boolean values
    """
    condition_type = condition.type.value if hasattr(condition.type, 'value') else condition.type
    
    if condition_type == "crosses_above":
        return _evaluate_crosses_above_history(condition, bars, vix_bars)
    
    elif condition_type == "crosses_below":
        return _evaluate_crosses_below_history(condition, bars, vix_bars)
    
    elif condition_type == "greater_than":
        return _evaluate_greater_than_history(condition, bars, vix_bars)
    
    elif condition_type == "less_than":
        return _evaluate_less_than_history(condition, bars, vix_bars)
    
    elif condition_type == "slope_above":
        return _evaluate_slope_above_history(condition, bars, vix_bars)
    
    elif condition_type == "slope_below":
        return _evaluate_slope_below_history(condition, bars, vix_bars)
    
    elif condition_type == "within_range":
        return _evaluate_within_range_history(condition, bars)
    
    else:
        # Unsupported condition type
        return pd.Series([False] * len(bars), index=bars.index)


def _get_indicator_series(
    indicator: Indicator,
    bars: pd.DataFrame,
    vix_bars: Optional[pd.DataFrame] = None
) -> np.ndarray:
    """Get indicator series from bar data."""
    return IndicatorFactory.create_indicator_series(indicator, bars, vix_bars)


def _evaluate_crosses_above_history(
    condition: Condition,
    bars: pd.DataFrame,
    vix_bars: Optional[pd.DataFrame] = None
) -> pd.Series:
    """Evaluate CROSSES_ABOVE across all bars."""
    if condition.indicator_b is None:
        return pd.Series([False] * len(bars), index=bars.index)
    
    series_a = _get_indicator_series(condition.indicator_a, bars, vix_bars)
    series_b = _get_indicator_series(condition.indicator_b, bars, vix_bars)
    
    # Create result array
    result = np.zeros(len(bars), dtype=bool)
    
    # For each bar, check if a crossover occurred
    for i in range(1, len(bars)):
        # Check if series_a crossed above series_b at bar i
        # (was below or equal, now above)
        if not np.isnan(series_a[i]) and not np.isnan(series_b[i]):
            if not np.isnan(series_a[i-1]) and not np.isnan(series_b[i-1]):
                was_below = series_a[i-1] <= series_b[i-1]
                is_above = series_a[i] > series_b[i]
                result[i] = was_below and is_above
    
    return pd.Series(result, index=bars.index)


def _evaluate_crosses_below_history(
    condition: Condition,
    bars: pd.DataFrame,
    vix_bars: Optional[pd.DataFrame] = None
) -> pd.Series:
    """Evaluate CROSSES_BELOW across all bars."""
    if condition.indicator_b is None:
        return pd.Series([False] * len(bars), index=bars.index)
    
    series_a = _get_indicator_series(condition.indicator_a, bars, vix_bars)
    series_b = _get_indicator_series(condition.indicator_b, bars, vix_bars)
    
    # Create result array
    result = np.zeros(len(bars), dtype=bool)
    
    # For each bar, check if a crossover occurred
    for i in range(1, len(bars)):
        if not np.isnan(series_a[i]) and not np.isnan(series_b[i]):
            if not np.isnan(series_a[i-1]) and not np.isnan(series_b[i-1]):
                was_above = series_a[i-1] >= series_b[i-1]
                is_below = series_a[i] < series_b[i]
                result[i] = was_above and is_below
    
    return pd.Series(result, index=bars.index)


def _evaluate_greater_than_history(
    condition: Condition,
    bars: pd.DataFrame,
    vix_bars: Optional[pd.DataFrame] = None
) -> pd.Series:
    """Evaluate GREATER_THAN across all bars."""
    series_a = _get_indicator_series(condition.indicator_a, bars, vix_bars)
    
    if condition.indicator_b is not None:
        series_b = _get_indicator_series(condition.indicator_b, bars, vix_bars)
        result = series_a > series_b
    elif condition.threshold is not None:
        result = series_a > condition.threshold
    else:
        return pd.Series([False] * len(bars), index=bars.index)
    
    # Handle NaN values
    result = np.where(np.isnan(series_a), False, result)
    
    return pd.Series(result, index=bars.index)


def _evaluate_less_than_history(
    condition: Condition,
    bars: pd.DataFrame,
    vix_bars: Optional[pd.DataFrame] = None
) -> pd.Series:
    """Evaluate LESS_THAN across all bars."""
    series_a = _get_indicator_series(condition.indicator_a, bars, vix_bars)
    
    if condition.indicator_b is not None:
        series_b = _get_indicator_series(condition.indicator_b, bars, vix_bars)
        result = series_a < series_b
    elif condition.threshold is not None:
        result = series_a < condition.threshold
    else:
        return pd.Series([False] * len(bars), index=bars.index)
    
    # Handle NaN values
    result = np.where(np.isnan(series_a), False, result)
    
    return pd.Series(result, index=bars.index)


def _evaluate_slope_above_history(
    condition: Condition,
    bars: pd.DataFrame,
    vix_bars: Optional[pd.DataFrame] = None
) -> pd.Series:
    """Evaluate SLOPE_ABOVE across all bars."""
    if condition.threshold is None:
        return pd.Series([False] * len(bars), index=bars.index)
    
    series_a = _get_indicator_series(condition.indicator_a, bars, vix_bars)
    slopes = slope_series(series_a, condition.lookback_periods)
    
    result = slopes > condition.threshold
    result = np.where(np.isnan(slopes), False, result)
    
    return pd.Series(result, index=bars.index)


def _evaluate_slope_below_history(
    condition: Condition,
    bars: pd.DataFrame,
    vix_bars: Optional[pd.DataFrame] = None
) -> pd.Series:
    """Evaluate SLOPE_BELOW across all bars."""
    if condition.threshold is None:
        return pd.Series([False] * len(bars), index=bars.index)
    
    series_a = _get_indicator_series(condition.indicator_a, bars, vix_bars)
    slopes = slope_series(series_a, condition.lookback_periods)
    
    result = slopes < condition.threshold
    result = np.where(np.isnan(slopes), False, result)
    
    return pd.Series(result, index=bars.index)


def _evaluate_within_range_history(
    condition: Condition,
    bars: pd.DataFrame
) -> pd.Series:
    """Evaluate WITHIN_RANGE (time-based) across all bars."""
    if condition.range_start is None or condition.range_end is None:
        return pd.Series([False] * len(bars), index=bars.index)
    
    # Parse range times
    try:
        start_parts = condition.range_start.split(":")
        end_parts = condition.range_end.split(":")
        start_time = int(start_parts[0]) * 60 + int(start_parts[1])
        end_time = int(end_parts[0]) * 60 + int(end_parts[1])
    except (ValueError, IndexError):
        return pd.Series([False] * len(bars), index=bars.index)
    
    result = []
    
    # Check if bars have a timestamp/datetime column
    if 'timestamp' in bars.columns:
        timestamps = pd.to_datetime(bars['timestamp'])
    elif bars.index.dtype == 'datetime64[ns]' or isinstance(bars.index, pd.DatetimeIndex):
        timestamps = bars.index
    else:
        # No datetime info available
        return pd.Series([False] * len(bars), index=bars.index)
    
    for ts in timestamps:
        bar_minutes = ts.hour * 60 + ts.minute
        in_range = start_time <= bar_minutes <= end_time
        result.append(in_range)
    
    return pd.Series(result, index=bars.index)


def get_last_true_info(
    rule: Rule,
    bars: pd.DataFrame,
    vix_bars: Optional[pd.DataFrame] = None
) -> dict:
    """
    Get information about when the rule was last TRUE.
    
    Args:
        rule: The Rule to evaluate
        bars: DataFrame with OHLCV data
        vix_bars: Optional VIX data
        
    Returns:
        Dict with keys:
            - 'last_true_idx': Index of last TRUE bar (or None)
            - 'last_true_datetime': Datetime of last TRUE bar (or None)
            - 'bars_ago': Number of bars since last TRUE (or None)
            - 'total_true_count': Total number of TRUE occurrences
    """
    history = evaluate_rule_history(rule, bars, vix_bars)
    
    true_indices = history[history].index.tolist()
    
    if not true_indices:
        return {
            'last_true_idx': None,
            'last_true_datetime': None,
            'bars_ago': None,
            'total_true_count': 0,
        }
    
    last_true_idx = true_indices[-1]
    
    # Calculate bars ago
    if isinstance(bars.index, pd.RangeIndex):
        bars_ago = len(bars) - 1 - bars.index.get_loc(last_true_idx)
    else:
        bars_ago = len(bars) - 1 - bars.index.tolist().index(last_true_idx)
    
    # Get datetime if available
    last_true_datetime = None
    if 'timestamp' in bars.columns:
        last_true_datetime = bars.loc[last_true_idx, 'timestamp']
    elif isinstance(bars.index, pd.DatetimeIndex):
        last_true_datetime = last_true_idx
    
    return {
        'last_true_idx': last_true_idx,
        'last_true_datetime': last_true_datetime,
        'bars_ago': bars_ago,
        'total_true_count': len(true_indices),
    }
