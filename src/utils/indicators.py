"""
Technical Indicators Module

Pure Python/NumPy implementations of technical indicators for trading strategies.
These functions are designed to work with numpy arrays for efficient computation.
"""

import numpy as np
import pandas as pd
from typing import Optional, Tuple, Union
from numpy.typing import NDArray


def sma(data: NDArray[np.float64], period: int) -> NDArray[np.float64]:
    """
    Calculate Simple Moving Average.
    
    Args:
        data: 1D array of price data
        period: Number of periods for SMA calculation
        
    Returns:
        Array of SMA values. First (period-1) values will be NaN.
        
    Example:
        >>> prices = np.array([10, 11, 12, 13, 14, 15])
        >>> sma(prices, 3)
        array([nan, nan, 11., 12., 13., 14.])
    """
    if period <= 0:
        return np.full(len(data), np.nan)

    if len(data) < period:
        return np.full(len(data), np.nan)
    
    result = np.full(len(data), np.nan)
    
    # Use cumsum for efficient calculation
    cumsum = np.cumsum(data)
    result[period-1:] = (cumsum[period-1:] - np.concatenate([[0], cumsum[:-period]])) / period
    
    return result


def ema(data: NDArray[np.float64], period: int) -> NDArray[np.float64]:
    """
    Calculate Exponential Moving Average.
    
    Uses the standard EMA formula:
        EMA = alpha * price + (1 - alpha) * previous_EMA
        alpha = 2 / (period + 1)
    
    Args:
        data: 1D array of price data
        period: Number of periods for EMA calculation
        
    Returns:
        Array of EMA values. First (period-1) values will be NaN.
        
    Example:
        >>> prices = np.array([22.27, 22.19, 22.08, 22.17, 22.18, 22.13, 22.23, 22.43, 22.24, 22.29])
        >>> ema(prices, 10)  # First 9 values are NaN, 10th is the EMA
    """
    if period <= 0:
        return np.full(len(data), np.nan)

    if len(data) < period:
        return np.full(len(data), np.nan)
    
    result = np.full(len(data), np.nan)
    alpha = 2.0 / (period + 1)
    
    # Initialize with SMA of first 'period' values
    result[period - 1] = np.mean(data[:period])
    
    # Calculate EMA for remaining values
    for i in range(period, len(data)):
        result[i] = alpha * data[i] + (1 - alpha) * result[i - 1]
    
    return result


def slope(data: NDArray[np.float64], periods: int = 1) -> float:
    """
    Calculate the slope (rate of change) over N periods.
    
    Slope is calculated as: (current - previous) / periods
    This gives the average change per period.
    
    Args:
        data: 1D array of values (e.g., EMA values)
        periods: Number of periods to calculate slope over
        
    Returns:
        Slope value (rise/run). Returns 0.0 if insufficient data.
        
    Example:
        >>> values = np.array([10, 12, 15, 14, 16])
        >>> slope(values, 2)  # (16 - 14) / 2 = 1.0
        1.0
    """
    if len(data) < periods + 1:
        return 0.0
    
    # Filter out NaN values from the end
    valid_data = data[~np.isnan(data)]
    if len(valid_data) < periods + 1:
        return 0.0
    
    current = valid_data[-1]
    previous = valid_data[-(periods + 1)]
    
    return (current - previous) / periods


def slope_series(data: NDArray[np.float64], periods: int = 1) -> NDArray[np.float64]:
    """
    Calculate slope for each point in the series.
    
    Args:
        data: 1D array of values
        periods: Number of periods to calculate slope over
        
    Returns:
        Array of slope values. First 'periods' values will be NaN.
    """
    result = np.full(len(data), np.nan)
    
    if len(data) <= periods:
        return result
    
    result[periods:] = (data[periods:] - data[:-periods]) / periods
    return result


def crosses_above(series_a: NDArray[np.float64], series_b: NDArray[np.float64]) -> bool:
    """
    Check if series_a crossed above series_b in the last bar.
    
    A crossover occurs when:
        - Previous bar: A <= B
        - Current bar: A > B
    
    Args:
        series_a: First indicator series
        series_b: Second indicator series (or constant array)
        
    Returns:
        True if A crossed above B in the last bar, False otherwise.
        
    Example:
        >>> a = np.array([10, 11, 13])  # Rising
        >>> b = np.array([12, 12, 12])  # Constant
        >>> crosses_above(a, b)  # 11 < 12, then 13 > 12
        True
    """
    if len(series_a) < 2 or len(series_b) < 2:
        return False
    
    # Get last two valid values
    a_curr, a_prev = _get_last_two_valid(series_a)
    b_curr, b_prev = _get_last_two_valid(series_b)
    
    if any(np.isnan([a_curr, a_prev, b_curr, b_prev])):
        return False
    
    # Crossover: was below or equal, now above
    return a_prev <= b_prev and a_curr > b_curr


def crosses_below(series_a: NDArray[np.float64], series_b: NDArray[np.float64]) -> bool:
    """
    Check if series_a crossed below series_b in the last bar.
    
    A crossunder occurs when:
        - Previous bar: A >= B
        - Current bar: A < B
    
    Args:
        series_a: First indicator series
        series_b: Second indicator series (or constant array)
        
    Returns:
        True if A crossed below B in the last bar, False otherwise.
        
    Example:
        >>> a = np.array([13, 12, 10])  # Falling
        >>> b = np.array([11, 11, 11])  # Constant
        >>> crosses_below(a, b)  # 12 > 11, then 10 < 11
        True
    """
    if len(series_a) < 2 or len(series_b) < 2:
        return False
    
    # Get last two valid values
    a_curr, a_prev = _get_last_two_valid(series_a)
    b_curr, b_prev = _get_last_two_valid(series_b)
    
    if any(np.isnan([a_curr, a_prev, b_curr, b_prev])):
        return False
    
    # Crossunder: was above or equal, now below
    return a_prev >= b_prev and a_curr < b_curr


def _get_last_two_valid(data: NDArray[np.float64]) -> Tuple[float, float]:
    """Get the last two non-NaN values from an array."""
    valid_indices = np.where(~np.isnan(data))[0]
    if len(valid_indices) < 2:
        return np.nan, np.nan
    return data[valid_indices[-1]], data[valid_indices[-2]]


def rsi(data: NDArray[np.float64], period: int = 14) -> NDArray[np.float64]:
    """
    Calculate Relative Strength Index.
    
    RSI = 100 - (100 / (1 + RS))
    RS = Average Gain / Average Loss
    
    Args:
        data: 1D array of price data
        period: RSI period (default 14)
        
    Returns:
        Array of RSI values (0-100). First 'period' values will be NaN.
    """
    if len(data) < period + 1:
        return np.full(len(data), np.nan)
    
    # Calculate price changes
    delta = np.diff(data)
    
    # Separate gains and losses
    gains = np.where(delta > 0, delta, 0.0)
    losses = np.where(delta < 0, -delta, 0.0)
    
    result = np.full(len(data), np.nan)
    
    # First RSI uses simple average
    avg_gain = np.mean(gains[:period])
    avg_loss = np.mean(losses[:period])
    
    if avg_loss == 0:
        result[period] = 100.0
    else:
        rs = avg_gain / avg_loss
        result[period] = 100.0 - (100.0 / (1.0 + rs))
    
    # Subsequent RSI uses smoothed average
    for i in range(period, len(data) - 1):
        avg_gain = (avg_gain * (period - 1) + gains[i]) / period
        avg_loss = (avg_loss * (period - 1) + losses[i]) / period
        
        if avg_loss == 0:
            result[i + 1] = 100.0
        else:
            rs = avg_gain / avg_loss
            result[i + 1] = 100.0 - (100.0 / (1.0 + rs))
    
    return result


def bollinger_bands(
    data: NDArray[np.float64], 
    period: int = 20, 
    std_dev: float = 2.0
) -> Tuple[NDArray[np.float64], NDArray[np.float64], NDArray[np.float64]]:
    """
    Calculate Bollinger Bands.
    
    Args:
        data: 1D array of price data
        period: SMA period (default 20)
        std_dev: Number of standard deviations (default 2.0)
        
    Returns:
        Tuple of (upper_band, middle_band, lower_band) arrays
    """
    middle = sma(data, period)
    
    # Calculate rolling standard deviation
    std = np.full(len(data), np.nan)
    for i in range(period - 1, len(data)):
        std[i] = np.std(data[i - period + 1:i + 1], ddof=0)
    
    upper = middle + (std * std_dev)
    lower = middle - (std * std_dev)
    
    return upper, middle, lower


def macd(
    data: NDArray[np.float64],
    fast_period: int = 12,
    slow_period: int = 26,
    signal_period: int = 9
) -> Tuple[NDArray[np.float64], NDArray[np.float64], NDArray[np.float64]]:
    """
    Calculate MACD (Moving Average Convergence Divergence).
    
    Args:
        data: 1D array of price data
        fast_period: Fast EMA period (default 12)
        slow_period: Slow EMA period (default 26)
        signal_period: Signal line EMA period (default 9)
        
    Returns:
        Tuple of (macd_line, signal_line, histogram) arrays
    """
    fast_ema = ema(data, fast_period)
    slow_ema = ema(data, slow_period)
    
    macd_line = fast_ema - slow_ema
    
    # Signal line is EMA of MACD line
    signal_line = ema(macd_line, signal_period)
    
    histogram = macd_line - signal_line
    
    return macd_line, signal_line, histogram


def atr(
    high: NDArray[np.float64],
    low: NDArray[np.float64],
    close: NDArray[np.float64],
    period: int = 14
) -> NDArray[np.float64]:
    """
    Calculate Average True Range.
    
    Args:
        high: Array of high prices
        low: Array of low prices
        close: Array of close prices
        period: ATR period (default 14)
        
    Returns:
        Array of ATR values
    """
    # Calculate True Range
    tr1 = high - low
    tr2 = np.abs(high - np.roll(close, 1))
    tr3 = np.abs(low - np.roll(close, 1))
    
    # First TR is just high - low
    tr2[0] = tr1[0]
    tr3[0] = tr1[0]
    
    true_range = np.maximum(np.maximum(tr1, tr2), tr3)
    
    # ATR is smoothed average of TR
    return ema(true_range, period)


def stochastic(
    high: NDArray[np.float64],
    low: NDArray[np.float64],
    close: NDArray[np.float64],
    k_period: int = 14,
    d_period: int = 3,
    smooth_k: int = 3
) -> Tuple[NDArray[np.float64], NDArray[np.float64]]:
    """
    Calculate Stochastic Oscillator.
    
    Args:
        high: Array of high prices
        low: Array of low prices
        close: Array of close prices
        k_period: %K period (default 14)
        d_period: %D period (default 3)
        smooth_k: period for slowing %K (default 3)
        
    Returns:
        Tuple of (k_line, d_line) arrays
    """
    # Calculate Lowest Low and Highest High over k_period
    lowest_low = pd.Series(low).rolling(window=k_period).min().values
    highest_high = pd.Series(high).rolling(window=k_period).max().values
    
    # Calculate Raw %K
    # Avoid division by zero
    range_hl = highest_high - lowest_low
    range_hl = np.where(range_hl == 0, np.nan, range_hl)
    
    raw_k = 100 * ((close - lowest_low) / range_hl)
    
    # Smooth %K
    k_line = sma(raw_k, smooth_k) if smooth_k > 1 else raw_k
    
    # %D is SMA of %K
    d_line = sma(k_line, d_period)
    
    return k_line, d_line


def obv(
    close: NDArray[np.float64],
    volume: NDArray[np.float64]
) -> NDArray[np.float64]:
    """
    Calculate On-Balance Volume (OBV).
    
    Args:
        close: Array of close prices
        volume: Array of volume data
        
    Returns:
        Array of OBV values
    """
    if len(close) == 0:
        return np.array([])
        
    obv_values = np.zeros(len(close))
    obv_values[0] = volume[0]
    
    # Determine sign of price change
    # 1 if close > prev, -1 if close < prev, 0 if equal
    price_change = np.diff(close, prepend=close[0])
    direction = np.zeros(len(close))
    direction[price_change > 0] = 1
    direction[price_change < 0] = -1
    
    # Calculate cumulative sum of directed volume
    obv_values = np.cumsum(direction * volume)
    
    # Adjust first value to be just the volume (convention varies, but cumsum starts at 0 + dir*vol)
    # Usually OBV starts at an arbitrary number or 0.
    return obv_values


def smma(data: NDArray[np.float64], period: int) -> NDArray[np.float64]:
    """
    Calculate Smoothed Moving Average (SMMA).
    Used for Williams Alligator.
    SMMA_i = (Sum_1 + ... + Sum_period) / n
    SMMA_i = (SMMA_{i-1} * (n-1) + Price_i) / n
    
    Args:
        data: Array of price data
        period: SMMA period
        
    Returns:
        Array of SMMA values
    """
    if len(data) < period:
        return np.full(len(data), np.nan)
        
    result = np.full(len(data), np.nan)
    
    # First value is SMA
    result[period-1] = np.mean(data[:period])
    
    # Subsequent values
    for i in range(period, len(data)):
        result[i] = (result[i-1] * (period - 1) + data[i]) / period
        
    return result


def williams_alligator(
    high: NDArray[np.float64],
    low: NDArray[np.float64],
    jaw_period: int = 13,
    teeth_period: int = 8,
    lips_period: int = 5,
    jaw_shift: int = 8,
    teeth_shift: int = 5,
    lips_shift: int = 3
) -> Tuple[NDArray[np.float64], NDArray[np.float64], NDArray[np.float64]]:
    """
    Calculate Williams Alligator.
    
    Args:
        high: Array of high prices
        low: Array of low prices
        jaw_period: Period for Jaw (blue line)
        teeth_period: Period for Teeth (red line)
        lips_period: Period for Lips (green line)
        
    Returns:
        Tuple of (jaw, teeth, lips) arrays, shifted into the future.
        Note: Shift usually means plotting it forward. For algorithmic usage,
        finding the value at time T means looking at the unshifted value.
        However, standard definition says value at T is the SMMA from T-shift.
        
        So: Current Jaw value = SMMA(median_price)[now - shift]
    """
    median_price = (high + low) / 2
    
    # Calculate SMMAs
    jaw_smma = smma(median_price, jaw_period)
    teeth_smma = smma(median_price, teeth_period)
    lips_smma = smma(median_price, lips_period)
    
    # Shift arrays
    # A positive shift mean the value calculated at index i is plotted at i + shift.
    # So the value valid for trading at index i (now) is the one that was plotted there,
    # which came from i - shift.
    
    jaw = np.roll(jaw_smma, jaw_shift)
    jaw[:jaw_shift] = np.nan
    
    teeth = np.roll(teeth_smma, teeth_shift)
    teeth[:teeth_shift] = np.nan
    
    lips = np.roll(lips_smma, lips_shift)
    lips[:lips_shift] = np.nan
    
    return jaw, teeth, lips






