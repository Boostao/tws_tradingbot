"""
Data Loader Module

Stub implementation for fetching historical market data.
This will be integrated with Nautilus Trader's data providers.
"""

import numpy as np
from typing import Optional, Dict, Any, List
from dataclasses import dataclass
from datetime import datetime, timedelta
from enum import Enum


class Timeframe(Enum):
    """Supported timeframes for historical data."""
    SECOND_1 = "1s"
    SECOND_5 = "5s"
    SECOND_15 = "15s"
    SECOND_30 = "30s"
    MINUTE_1 = "1m"
    MINUTE_5 = "5m"
    MINUTE_15 = "15m"
    MINUTE_30 = "30m"
    HOUR_1 = "1h"
    HOUR_4 = "4h"
    DAY_1 = "1d"
    WEEK_1 = "1w"
    MONTH_1 = "1M"


@dataclass
class Bar:
    """OHLCV bar data."""
    timestamp: datetime
    open: float
    high: float
    low: float
    close: float
    volume: int
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary."""
        return {
            "timestamp": self.timestamp.isoformat(),
            "open": self.open,
            "high": self.high,
            "low": self.low,
            "close": self.close,
            "volume": self.volume
        }


@dataclass
class BarSeries:
    """Collection of OHLCV bars with numpy arrays for efficient computation."""
    symbol: str
    timeframe: Timeframe
    timestamps: np.ndarray  # datetime64
    opens: np.ndarray       # float64
    highs: np.ndarray       # float64
    lows: np.ndarray        # float64
    closes: np.ndarray      # float64
    volumes: np.ndarray     # int64
    
    def __len__(self) -> int:
        return len(self.closes)
    
    @property
    def last_bar(self) -> Optional[Bar]:
        """Get the most recent bar."""
        if len(self) == 0:
            return None
        return Bar(
            timestamp=self.timestamps[-1].astype(datetime),
            open=float(self.opens[-1]),
            high=float(self.highs[-1]),
            low=float(self.lows[-1]),
            close=float(self.closes[-1]),
            volume=int(self.volumes[-1])
        )
    
    @classmethod
    def from_bars(cls, symbol: str, timeframe: Timeframe, bars: List[Bar]) -> "BarSeries":
        """Create BarSeries from list of Bar objects."""
        if not bars:
            return cls(
                symbol=symbol,
                timeframe=timeframe,
                timestamps=np.array([], dtype="datetime64[ns]"),
                opens=np.array([], dtype=np.float64),
                highs=np.array([], dtype=np.float64),
                lows=np.array([], dtype=np.float64),
                closes=np.array([], dtype=np.float64),
                volumes=np.array([], dtype=np.int64)
            )
        
        return cls(
            symbol=symbol,
            timeframe=timeframe,
            timestamps=np.array([b.timestamp for b in bars], dtype="datetime64[ns]"),
            opens=np.array([b.open for b in bars], dtype=np.float64),
            highs=np.array([b.high for b in bars], dtype=np.float64),
            lows=np.array([b.low for b in bars], dtype=np.float64),
            closes=np.array([b.close for b in bars], dtype=np.float64),
            volumes=np.array([b.volume for b in bars], dtype=np.int64)
        )


class DataLoader:
    """
    Abstract data loader for fetching historical market data.
    
    This is a stub implementation. In production, this will be replaced
    with Nautilus Trader's data providers for live/backtest data.
    """
    
    def __init__(self, config: Optional[Dict[str, Any]] = None):
        """
        Initialize the data loader.
        
        Args:
            config: Optional configuration dictionary
        """
        self.config = config or {}
        self._cache: Dict[str, BarSeries] = {}
    
    async def fetch_historical_bars(
        self,
        symbol: str,
        timeframe: Timeframe,
        start: Optional[datetime] = None,
        end: Optional[datetime] = None,
        limit: int = 1000
    ) -> BarSeries:
        """
        Fetch historical OHLCV bars for a symbol.
        
        Args:
            symbol: Ticker symbol (e.g., "AAPL", "SPY")
            timeframe: Bar timeframe
            start: Start datetime (optional)
            end: End datetime (optional, defaults to now)
            limit: Maximum number of bars to return
            
        Returns:
            BarSeries with historical data
            
        Note:
            This is a stub implementation that returns empty data.
            Real implementation will use Nautilus Trader data providers.
        """
        # Stub: Return empty BarSeries
        # TODO: Integrate with Nautilus Trader data providers
        return BarSeries(
            symbol=symbol,
            timeframe=timeframe,
            timestamps=np.array([], dtype="datetime64[ns]"),
            opens=np.array([], dtype=np.float64),
            highs=np.array([], dtype=np.float64),
            lows=np.array([], dtype=np.float64),
            closes=np.array([], dtype=np.float64),
            volumes=np.array([], dtype=np.int64)
        )
    
    async def fetch_latest_bar(
        self,
        symbol: str,
        timeframe: Timeframe
    ) -> Optional[Bar]:
        """
        Fetch the most recent bar for a symbol.
        
        Args:
            symbol: Ticker symbol
            timeframe: Bar timeframe
            
        Returns:
            Most recent Bar or None
        """
        bars = await self.fetch_historical_bars(symbol, timeframe, limit=1)
        return bars.last_bar
    
    async def fetch_vix(self) -> Optional[float]:
        """
        Fetch the current VIX value.
        
        Returns:
            Current VIX value or None if unavailable
            
        Note:
            Stub implementation. Real implementation will fetch from market data.
        """
        try:
            from src.bot.tws_data_provider import get_tws_provider
            import asyncio

            def _fetch_latest_vix() -> Optional[float]:
                provider = get_tws_provider()
                if not provider or not provider.connect():
                    return None
                df = provider.get_historical_data(
                    symbol="VIX",
                    duration="1 D",
                    bar_size="5 mins",
                    what_to_show="TRADES",
                    use_rth=True,
                    timeout=15.0,
                )
                if df is None or df.empty:
                    return None
                return float(df.iloc[-1]["close"])

            return await asyncio.to_thread(_fetch_latest_vix)
        except Exception:
            return None
    
    def clear_cache(self) -> None:
        """Clear the data cache."""
        self._cache.clear()
    
    def get_cache_key(self, symbol: str, timeframe: Timeframe) -> str:
        """Generate a cache key for symbol/timeframe combination."""
        return f"{symbol}_{timeframe.value}"


# Convenience functions

async def fetch_historical_bars(
    symbol: str,
    timeframe: str = "1m",
    limit: int = 1000
) -> BarSeries:
    """
    Convenience function to fetch historical bars.
    
    Args:
        symbol: Ticker symbol
        timeframe: Timeframe string (e.g., "1m", "5m", "1h", "1d")
        limit: Maximum number of bars
        
    Returns:
        BarSeries with historical data
    """
    tf_map = {
        "1s": Timeframe.SECOND_1,
        "5s": Timeframe.SECOND_5,
        "15s": Timeframe.SECOND_15,
        "30s": Timeframe.SECOND_30,
        "1m": Timeframe.MINUTE_1,
        "5m": Timeframe.MINUTE_5,
        "15m": Timeframe.MINUTE_15,
        "30m": Timeframe.MINUTE_30,
        "1h": Timeframe.HOUR_1,
        "4h": Timeframe.HOUR_4,
        "1d": Timeframe.DAY_1,
        "1w": Timeframe.WEEK_1,
        "1M": Timeframe.MONTH_1,
    }
    
    tf = tf_map.get(timeframe, Timeframe.MINUTE_1)
    loader = DataLoader()
    return await loader.fetch_historical_bars(symbol, tf, limit=limit)


def generate_sample_bars(
    symbol: str,
    timeframe: Timeframe = Timeframe.MINUTE_1,
    num_bars: int = 100,
    base_price: float = 100.0,
    volatility: float = 0.02
) -> BarSeries:
    """
    Generate sample bar data for testing.
    
    Args:
        symbol: Ticker symbol
        timeframe: Bar timeframe
        num_bars: Number of bars to generate
        base_price: Starting price
        volatility: Price volatility (standard deviation)
        
    Returns:
        BarSeries with generated sample data
    """
    np.random.seed(42)  # For reproducibility
    
    # Generate timestamps
    end_time = datetime.now()
    
    # Map timeframe to timedelta
    tf_deltas = {
        Timeframe.SECOND_1: timedelta(seconds=1),
        Timeframe.SECOND_5: timedelta(seconds=5),
        Timeframe.SECOND_15: timedelta(seconds=15),
        Timeframe.SECOND_30: timedelta(seconds=30),
        Timeframe.MINUTE_1: timedelta(minutes=1),
        Timeframe.MINUTE_5: timedelta(minutes=5),
        Timeframe.MINUTE_15: timedelta(minutes=15),
        Timeframe.MINUTE_30: timedelta(minutes=30),
        Timeframe.HOUR_1: timedelta(hours=1),
        Timeframe.HOUR_4: timedelta(hours=4),
        Timeframe.DAY_1: timedelta(days=1),
        Timeframe.WEEK_1: timedelta(weeks=1),
        Timeframe.MONTH_1: timedelta(days=30),
    }
    
    delta = tf_deltas.get(timeframe, timedelta(minutes=1))
    timestamps = [end_time - delta * (num_bars - i - 1) for i in range(num_bars)]
    
    # Generate price data with random walk
    returns = np.random.normal(0, volatility, num_bars)
    prices = base_price * np.exp(np.cumsum(returns))
    
    # Generate OHLC from prices
    opens = prices.copy()
    closes = np.roll(prices, -1)
    closes[-1] = prices[-1] * (1 + np.random.normal(0, volatility / 2))
    
    highs = np.maximum(opens, closes) * (1 + np.abs(np.random.normal(0, volatility / 2, num_bars)))
    lows = np.minimum(opens, closes) * (1 - np.abs(np.random.normal(0, volatility / 2, num_bars)))
    
    # Generate volume
    base_volume = 100000
    volumes = np.random.poisson(base_volume, num_bars)
    
    return BarSeries(
        symbol=symbol,
        timeframe=timeframe,
        timestamps=np.array(timestamps, dtype="datetime64[ns]"),
        opens=opens.astype(np.float64),
        highs=highs.astype(np.float64),
        lows=lows.astype(np.float64),
        closes=closes.astype(np.float64),
        volumes=volumes.astype(np.int64)
    )
