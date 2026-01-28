"""
Indicator Factory Module

Creates indicator series from bar data based on indicator configurations.
This module bridges the gap between JSON-defined indicators and actual calculations.
"""

import numpy as np
import pandas as pd
from typing import Optional, Union
from numpy.typing import NDArray

from src.bot.strategy.rules.models import (
    Indicator, 
    IndicatorType, 
    PriceSource,
    TimeframeUnit
)
from src.utils.indicators import (
    ema, sma, rsi, bollinger_bands, macd,
    stochastic, obv, williams_alligator
)


class IndicatorFactory:
    """
    Factory class for creating indicator series from bar data.
    
    Converts Indicator model definitions into actual numpy arrays of calculated values.
    """
    
    @staticmethod
    def get_price_series(bars: pd.DataFrame, source: PriceSource) -> NDArray[np.float64]:
        """
        Extract the appropriate price series from bar data.
        
        Args:
            bars: DataFrame with OHLCV columns
            source: Which price column to use
            
        Returns:
            Numpy array of price values
        """
        source_str = source.value if hasattr(source, 'value') else source
        
        if source_str == "close":
            return bars["close"].values.astype(np.float64)
        elif source_str == "open":
            return bars["open"].values.astype(np.float64)
        elif source_str == "high":
            return bars["high"].values.astype(np.float64)
        elif source_str == "low":
            return bars["low"].values.astype(np.float64)
        elif source_str == "volume":
            return bars["volume"].values.astype(np.float64)
        elif source_str == "hl2":
            return ((bars["high"] + bars["low"]) / 2).values.astype(np.float64)
        elif source_str == "hlc3":
            return ((bars["high"] + bars["low"] + bars["close"]) / 3).values.astype(np.float64)
        elif source_str == "ohlc4":
            return ((bars["open"] + bars["high"] + bars["low"] + bars["close"]) / 4).values.astype(np.float64)
        else:
            # Default to close
            return bars["close"].values.astype(np.float64)
    
    @classmethod
    def create_indicator_series(
        cls,
        indicator: Indicator,
        bars: pd.DataFrame,
        vix_bars: Optional[pd.DataFrame] = None
    ) -> NDArray[np.float64]:
        """
        Create an indicator series from bar data.
        
        Args:
            indicator: Indicator configuration
            bars: DataFrame with OHLCV data
            vix_bars: Optional DataFrame with VIX data (for VIX indicator type)
            
        Returns:
            Numpy array of indicator values
            
        Raises:
            ValueError: If indicator type is not supported or data is missing
        """
        indicator_type = indicator.type.value if hasattr(indicator.type, 'value') else indicator.type
        
        if indicator_type == "ema":
            if indicator.length is None:
                raise ValueError("EMA indicator requires a length parameter")
            price_data = cls.get_price_series(bars, indicator.source)
            return ema(price_data, indicator.length)
        
        elif indicator_type == "sma":
            if indicator.length is None:
                raise ValueError("SMA indicator requires a length parameter")
            price_data = cls.get_price_series(bars, indicator.source)
            return sma(price_data, indicator.length)
        
        elif indicator_type == "price":
            return cls.get_price_series(bars, indicator.source)
        
        elif indicator_type == "vix":
            if vix_bars is None or vix_bars.empty:
                raise ValueError("VIX indicator requires VIX bar data")
            # VIX is typically just the close price of VIX index
            return vix_bars["close"].values.astype(np.float64)
        
        elif indicator_type == "time":
            # Return timestamp as float (epoch) for time-based comparisons
            if "timestamp" in bars.columns:
                # Convert to datetime if needed, then to epoch
                ts = pd.to_datetime(bars["timestamp"])
                return ts.values.astype(np.float64)
            elif bars.index.dtype == 'datetime64[ns]':
                return bars.index.values.astype(np.float64)
            else:
                raise ValueError("Cannot extract time data from bars")
        
        elif indicator_type == "volume":
            return bars["volume"].values.astype(np.float64)
        
        elif indicator_type == "rsi":
            if indicator.length is None:
                raise ValueError("RSI indicator requires a length parameter")
            price_data = cls.get_price_series(bars, indicator.source)
            return rsi(price_data, indicator.length)
        
        elif indicator_type == "macd":
            price_data = cls.get_price_series(bars, indicator.source)
            fast_period = int(indicator.params.get("fast_period", 12))
            slow_period = int(indicator.params.get("slow_period", 26))
            signal_period = int(indicator.params.get("signal_period", 9))
            
            macd_line, signal_line, histogram = macd(
                price_data, 
                fast_period=fast_period,
                slow_period=slow_period,
                signal_period=signal_period
            )
            
            component = indicator.component or "macd"
            if component == "signal":
                return signal_line
            elif component == "histogram":
                return histogram
            else:
                return macd_line
        
        elif indicator_type == "bollinger":
            price_data = cls.get_price_series(bars, indicator.source)
            length = indicator.length or int(indicator.params.get("period", 20))
            std_dev = float(indicator.params.get("std_dev", 2.0))
            
            upper, middle, lower = bollinger_bands(price_data, period=length, std_dev=std_dev)
            
            # Apply offset if present
            offset = int(indicator.params.get("offset", 0))
            if offset != 0:
                upper = np.roll(upper, offset)
                middle = np.roll(middle, offset)
                lower = np.roll(lower, offset)
                if offset > 0:
                    upper[:offset] = np.nan
                    middle[:offset] = np.nan
                    lower[:offset] = np.nan
            
            component = indicator.component or "upper"
            if component == "lower":
                return lower
            elif component == "middle":
                return middle
            else:
                return upper
                
        elif indicator_type == "stochastic":
            high = bars["high"].values.astype(np.float64)
            low = bars["low"].values.astype(np.float64)
            close = bars["close"].values.astype(np.float64)
            
            k_period = int(indicator.params.get("k_period", 14))
            d_period = int(indicator.params.get("d_period", 3))
            smooth_k = int(indicator.params.get("smooth_k", 3))
            
            k_line, d_line = stochastic(
                high, low, close,
                k_period=k_period,
                d_period=d_period,
                smooth_k=smooth_k
            )
            
            component = indicator.component or "k"
            if component == "d":
                return d_line
            else:
                return k_line
        
        elif indicator_type == "obv":
            close = bars["close"].values.astype(np.float64)
            volume = bars["volume"].values.astype(np.float64)
            return obv(close, volume)
            
        elif indicator_type == "alligator":
            high = bars["high"].values.astype(np.float64)
            low = bars["low"].values.astype(np.float64)
            
            jaw_period = int(indicator.params.get("jaw_period", 13))
            teeth_period = int(indicator.params.get("teeth_period", 8))
            lips_period = int(indicator.params.get("lips_period", 5))
            jaw_shift = int(indicator.params.get("jaw_shift", 8))
            teeth_shift = int(indicator.params.get("teeth_shift", 5))
            lips_shift = int(indicator.params.get("lips_shift", 3))
            
            jaw, teeth, lips = williams_alligator(
                high, low,
                jaw_period=jaw_period,
                teeth_period=teeth_period,
                lips_period=lips_period,
                jaw_shift=jaw_shift,
                teeth_shift=teeth_shift,
                lips_shift=lips_shift
            )
            
            component = indicator.component or "jaw"
            if component == "teeth":
                return teeth
            elif component == "lips":
                return lips
            else:
                return jaw

        elif indicator_type == "dividend_yield":
            return np.zeros(len(bars))

        elif indicator_type == "pe_ratio":
            return np.zeros(len(bars))

        elif indicator_type == "relative_performance":
            return np.ones(len(bars))
        
        else:
            raise ValueError(f"Unsupported indicator type: {indicator_type}")
    
    @classmethod
    def get_indicator_key(cls, indicator: Indicator) -> str:
        """
        Generate a unique key for caching indicator values.
        
        Args:
            indicator: Indicator configuration
            
        Returns:
            String key uniquely identifying this indicator
        """
        indicator_type = indicator.type.value if hasattr(indicator.type, 'value') else indicator.type
        source_str = indicator.source.value if hasattr(indicator.source, 'value') else indicator.source
        timeframe_str = indicator.timeframe.value if hasattr(indicator.timeframe, 'value') else indicator.timeframe
        
        parts = [indicator_type, timeframe_str, source_str]
        
        if indicator.length:
            parts.append(str(indicator.length))
        
        if indicator.symbol:
            parts.append(indicator.symbol)
            
        if indicator.params:
            for k in sorted(indicator.params.keys()):
                parts.append(f"{k}={indicator.params[k]}")
        
        if indicator.component:
            parts.append(indicator.component)
        
        return "_".join(parts)


def create_indicator_series(
    indicator: Indicator,
    bars: pd.DataFrame,
    vix_bars: Optional[pd.DataFrame] = None
) -> NDArray[np.float64]:
    """
    Convenience function to create indicator series.
    
    Args:
        indicator: Indicator configuration
        bars: DataFrame with OHLCV data
        vix_bars: Optional DataFrame with VIX data
        
    Returns:
        Numpy array of indicator values
    """
    return IndicatorFactory.create_indicator_series(indicator, bars, vix_bars)
