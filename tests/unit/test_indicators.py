"""
Unit Tests for Technical Indicators

Tests for the indicators module to ensure correct calculations.
"""

import pytest
import numpy as np
from numpy.testing import assert_array_almost_equal, assert_almost_equal

import sys
from pathlib import Path

# Add src to path for imports - import indicators directly to avoid logger/config chain
sys.path.insert(0, str(Path(__file__).parent.parent.parent / "src" / "utils"))

from indicators import (
    sma,
    ema,
    slope,
    slope_series,
    crosses_above,
    crosses_below,
    rsi,
    bollinger_bands,
    macd,
    atr,
)


class TestSMA:
    """Tests for Simple Moving Average."""
    
    def test_sma_basic(self):
        """Test basic SMA calculation."""
        data = np.array([1.0, 2.0, 3.0, 4.0, 5.0])
        result = sma(data, 3)
        
        # First 2 values should be NaN
        assert np.isnan(result[0])
        assert np.isnan(result[1])
        
        # SMA values
        assert_almost_equal(result[2], 2.0)  # (1+2+3)/3
        assert_almost_equal(result[3], 3.0)  # (2+3+4)/3
        assert_almost_equal(result[4], 4.0)  # (3+4+5)/3
    
    def test_sma_insufficient_data(self):
        """Test SMA with insufficient data."""
        data = np.array([1.0, 2.0])
        result = sma(data, 5)
        
        # All should be NaN
        assert all(np.isnan(result))
    
    def test_sma_period_one(self):
        """Test SMA with period 1 (should equal input)."""
        data = np.array([10.0, 20.0, 30.0, 40.0])
        result = sma(data, 1)
        
        assert_array_almost_equal(result, data)
    
    def test_sma_constant_values(self):
        """Test SMA with constant values."""
        data = np.array([5.0, 5.0, 5.0, 5.0, 5.0])
        result = sma(data, 3)
        
        # SMA of constants should equal the constant
        assert_almost_equal(result[2], 5.0)
        assert_almost_equal(result[3], 5.0)
        assert_almost_equal(result[4], 5.0)


class TestEMA:
    """Tests for Exponential Moving Average."""
    
    def test_ema_basic(self):
        """Test basic EMA calculation."""
        data = np.array([1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0])
        result = ema(data, 5)
        
        # First 4 values should be NaN
        for i in range(4):
            assert np.isnan(result[i])
        
        # First EMA should be SMA of first 5 values
        assert_almost_equal(result[4], 3.0)  # (1+2+3+4+5)/5
        
        # Subsequent values should follow EMA formula
        alpha = 2.0 / 6  # 2 / (5 + 1)
        expected_5 = alpha * 6 + (1 - alpha) * 3.0
        assert_almost_equal(result[5], expected_5, decimal=5)
    
    def test_ema_insufficient_data(self):
        """Test EMA with insufficient data."""
        data = np.array([1.0, 2.0, 3.0])
        result = ema(data, 5)
        
        assert all(np.isnan(result))
    
    def test_ema_constant_values(self):
        """Test EMA with constant values."""
        data = np.array([10.0] * 20)
        result = ema(data, 5)
        
        # EMA of constants should equal the constant after warmup
        for i in range(4, 20):
            assert_almost_equal(result[i], 10.0, decimal=5)


class TestSlope:
    """Tests for slope calculation."""
    
    def test_slope_positive(self):
        """Test positive slope."""
        data = np.array([10.0, 12.0, 14.0, 16.0, 18.0])
        result = slope(data, 2)
        
        # (18 - 14) / 2 = 2.0
        assert_almost_equal(result, 2.0)
    
    def test_slope_negative(self):
        """Test negative slope."""
        data = np.array([20.0, 18.0, 16.0, 14.0, 12.0])
        result = slope(data, 2)
        
        # (12 - 16) / 2 = -2.0
        assert_almost_equal(result, -2.0)
    
    def test_slope_zero(self):
        """Test zero slope (flat)."""
        data = np.array([10.0, 10.0, 10.0, 10.0, 10.0])
        result = slope(data, 2)
        
        assert_almost_equal(result, 0.0)
    
    def test_slope_insufficient_data(self):
        """Test slope with insufficient data."""
        data = np.array([10.0])
        result = slope(data, 2)
        
        assert result == 0.0
    
    def test_slope_with_nan(self):
        """Test slope ignores NaN values."""
        data = np.array([np.nan, np.nan, 10.0, 12.0, 14.0])
        result = slope(data, 2)
        
        # Should use last 3 valid values: (14 - 10) / 2 = 2.0
        assert_almost_equal(result, 2.0)


class TestSlopeSeries:
    """Tests for slope series calculation."""
    
    def test_slope_series_basic(self):
        """Test slope series calculation."""
        data = np.array([10.0, 12.0, 14.0, 16.0, 18.0])
        result = slope_series(data, 1)
        
        # First value is NaN
        assert np.isnan(result[0])
        
        # All others should be 2.0 (constant slope)
        assert_almost_equal(result[1], 2.0)
        assert_almost_equal(result[2], 2.0)
        assert_almost_equal(result[3], 2.0)
        assert_almost_equal(result[4], 2.0)


class TestCrossovers:
    """Tests for crossover detection."""
    
    def test_crosses_above_true(self):
        """Test detecting cross above."""
        # Fast EMA crosses above slow EMA
        fast = np.array([8.0, 9.0, 11.0])   # Was below, now above
        slow = np.array([10.0, 10.0, 10.0])  # Constant
        
        assert crosses_above(fast, slow) == True
    
    def test_crosses_above_false_still_below(self):
        """Test no cross when still below."""
        fast = np.array([8.0, 8.5, 9.0])
        slow = np.array([10.0, 10.0, 10.0])
        
        assert crosses_above(fast, slow) == False
    
    def test_crosses_above_false_was_already_above(self):
        """Test no cross when already above."""
        fast = np.array([11.0, 12.0, 13.0])
        slow = np.array([10.0, 10.0, 10.0])
        
        assert crosses_above(fast, slow) == False
    
    def test_crosses_below_true(self):
        """Test detecting cross below."""
        fast = np.array([12.0, 11.0, 9.0])   # Was above, now below
        slow = np.array([10.0, 10.0, 10.0])  # Constant
        
        assert crosses_below(fast, slow) == True
    
    def test_crosses_below_false_still_above(self):
        """Test no cross when still above."""
        fast = np.array([12.0, 11.5, 11.0])
        slow = np.array([10.0, 10.0, 10.0])
        
        assert crosses_below(fast, slow) == False
    
    def test_crosses_below_false_was_already_below(self):
        """Test no cross when already below."""
        fast = np.array([8.0, 7.0, 6.0])
        slow = np.array([10.0, 10.0, 10.0])
        
        assert crosses_below(fast, slow) == False
    
    def test_crossover_insufficient_data(self):
        """Test crossover with insufficient data."""
        fast = np.array([10.0])
        slow = np.array([9.0])
        
        assert crosses_above(fast, slow) == False
        assert crosses_below(fast, slow) == False
    
    def test_crossover_with_nan(self):
        """Test crossover with NaN values returns False."""
        fast = np.array([np.nan, np.nan, 11.0])
        slow = np.array([10.0, 10.0, 10.0])
        
        # Can't detect crossover if we don't have previous values
        assert crosses_above(fast, slow) == False


class TestRSI:
    """Tests for RSI calculation."""
    
    def test_rsi_basic(self):
        """Test basic RSI calculation."""
        # Create trending data
        data = np.array([44.0, 44.34, 44.09, 44.15, 43.61, 
                        44.33, 44.83, 45.10, 45.42, 45.84,
                        46.08, 45.89, 46.03, 45.61, 46.28, 46.28])
        result = rsi(data, 14)
        
        # First 14 values should be NaN
        for i in range(14):
            assert np.isnan(result[i])
        
        # RSI should be between 0 and 100
        assert 0 <= result[14] <= 100
        assert 0 <= result[15] <= 100
    
    def test_rsi_all_gains(self):
        """Test RSI with all gains (should approach 100)."""
        # Steadily increasing prices
        data = np.arange(1.0, 50.0, 1.0)
        result = rsi(data, 14)
        
        # RSI should be close to 100 for strong uptrend
        assert result[-1] > 90
    
    def test_rsi_all_losses(self):
        """Test RSI with all losses (should approach 0)."""
        # Steadily decreasing prices
        data = np.arange(50.0, 1.0, -1.0)
        result = rsi(data, 14)
        
        # RSI should be close to 0 for strong downtrend
        assert result[-1] < 10


class TestBollingerBands:
    """Tests for Bollinger Bands calculation."""
    
    def test_bollinger_bands_basic(self):
        """Test basic Bollinger Bands calculation."""
        np.random.seed(42)
        data = 100 + np.random.randn(50) * 2
        
        upper, middle, lower = bollinger_bands(data, period=20, std_dev=2.0)
        
        # Middle band should equal SMA
        expected_middle = sma(data, 20)
        assert_array_almost_equal(middle, expected_middle)
        
        # Upper should be above middle, lower below
        valid_idx = ~np.isnan(middle)
        assert all(upper[valid_idx] > middle[valid_idx])
        assert all(lower[valid_idx] < middle[valid_idx])
    
    def test_bollinger_bands_width(self):
        """Test that bands width relates to volatility."""
        # Low volatility data
        low_vol = np.array([100.0] * 30)
        upper_lv, middle_lv, lower_lv = bollinger_bands(low_vol, 20, 2.0)
        
        # Bands should be tight (low std dev)
        assert_almost_equal(upper_lv[-1], middle_lv[-1], decimal=5)
        assert_almost_equal(lower_lv[-1], middle_lv[-1], decimal=5)


class TestMACD:
    """Tests for MACD calculation."""
    
    def test_macd_basic(self):
        """Test basic MACD calculation."""
        np.random.seed(42)
        data = 100 + np.cumsum(np.random.randn(50) * 0.5)
        
        macd_line, signal_line, histogram = macd(data, 12, 26, 9)
        
        # MACD line = Fast EMA - Slow EMA
        fast = ema(data, 12)
        slow = ema(data, 26)
        expected_macd = fast - slow
        assert_array_almost_equal(macd_line, expected_macd)
        
        # Histogram = MACD - Signal
        expected_hist = macd_line - signal_line
        assert_array_almost_equal(histogram, expected_hist)


class TestATR:
    """Tests for ATR calculation."""
    
    def test_atr_basic(self):
        """Test basic ATR calculation."""
        # Generate sample OHLC data
        np.random.seed(42)
        n = 30
        close = 100 + np.cumsum(np.random.randn(n) * 0.5)
        high = close + np.abs(np.random.randn(n) * 0.5)
        low = close - np.abs(np.random.randn(n) * 0.5)
        
        result = atr(high, low, close, period=14)
        
        # ATR should be positive after warmup period
        assert result[-1] > 0
        assert result[-5] > 0
    
    def test_atr_zero_volatility(self):
        """Test ATR with zero volatility."""
        n = 30
        close = np.array([100.0] * n)
        high = np.array([100.0] * n)
        low = np.array([100.0] * n)
        
        result = atr(high, low, close, period=14)
        
        # ATR should be 0 when there's no range
        assert_almost_equal(result[-1], 0.0, decimal=5)


class TestEdgeCases:
    """Tests for edge cases and error handling."""
    
    def test_empty_array(self):
        """Test functions with empty arrays."""
        empty = np.array([])
        
        assert len(sma(empty, 5)) == 0
        assert len(ema(empty, 5)) == 0
        assert slope(empty, 1) == 0.0
    
    def test_single_value(self):
        """Test functions with single value."""
        single = np.array([100.0])
        
        # Should return array with NaN
        assert len(sma(single, 5)) == 1
        assert np.isnan(sma(single, 5)[0])
    
    def test_negative_period(self):
        """Test functions handle edge case periods."""
        data = np.array([1.0, 2.0, 3.0, 4.0, 5.0])
        
        # Period 0 or negative should not crash
        # (behavior may vary, but shouldn't raise exception)
        try:
            sma(data, 0)
            ema(data, 0)
        except (ValueError, ZeroDivisionError):
            pass  # Expected behavior


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
