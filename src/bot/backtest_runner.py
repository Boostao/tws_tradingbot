"""
Backtest Runner Module

Provides backtesting functionality for trading strategies.
Simulates trades based on rule evaluations against historical data.
Supports both simulated sample data and real TWS historical data.
"""

import logging
from dataclasses import dataclass, field
from datetime import datetime, date, timedelta
from typing import List, Dict, Optional, Any
from enum import Enum
from pathlib import Path
import pandas as pd
import numpy as np

from src.bot.strategy.rules.models import Strategy, RuleScope, ActionType
from src.bot.strategy.rules.engine import RuleEngine
from src.utils.data_loader import generate_sample_bars, BarSeries, Timeframe


logger = logging.getLogger(__name__)


class TradeSide(str, Enum):
    """Trade direction."""
    BUY = "BUY"
    SELL = "SELL"


@dataclass
class Trade:
    """Represents a single trade in the backtest."""
    entry_time: datetime
    exit_time: Optional[datetime]
    symbol: str
    side: TradeSide
    quantity: float
    entry_price: float
    exit_price: Optional[float] = None
    pnl: float = 0.0
    pnl_percent: float = 0.0
    is_open: bool = True
    
    def close(self, exit_time: datetime, exit_price: float) -> None:
        """Close the trade and calculate PnL."""
        self.exit_time = exit_time
        self.exit_price = exit_price
        self.is_open = False
        
        if self.side == TradeSide.BUY:
            self.pnl = (exit_price - self.entry_price) * self.quantity
            self.pnl_percent = ((exit_price / self.entry_price) - 1) * 100
        else:
            self.pnl = (self.entry_price - exit_price) * self.quantity
            self.pnl_percent = ((self.entry_price / exit_price) - 1) * 100
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for DataFrame creation."""
        return {
            "entry_time": self.entry_time,
            "exit_time": self.exit_time,
            "symbol": self.symbol,
            "side": self.side.value,
            "quantity": self.quantity,
            "entry_price": self.entry_price,
            "exit_price": self.exit_price,
            "pnl": self.pnl,
            "pnl_percent": self.pnl_percent,
        }


@dataclass
class BacktestMetrics:
    """Performance metrics from a backtest."""
    total_return: float = 0.0
    total_return_percent: float = 0.0
    sharpe_ratio: float = 0.0
    max_drawdown: float = 0.0
    max_drawdown_percent: float = 0.0
    win_rate: float = 0.0
    total_trades: int = 0
    winning_trades: int = 0
    losing_trades: int = 0
    avg_win: float = 0.0
    avg_loss: float = 0.0
    profit_factor: float = 0.0
    avg_trade_duration: Optional[timedelta] = None
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary."""
        return {
            "total_return": self.total_return,
            "total_return_percent": self.total_return_percent,
            "sharpe_ratio": self.sharpe_ratio,
            "max_drawdown": self.max_drawdown,
            "max_drawdown_percent": self.max_drawdown_percent,
            "win_rate": self.win_rate,
            "total_trades": self.total_trades,
            "winning_trades": self.winning_trades,
            "losing_trades": self.losing_trades,
            "avg_win": self.avg_win,
            "avg_loss": self.avg_loss,
            "profit_factor": self.profit_factor,
        }


@dataclass
class BacktestResult:
    """Complete backtest results."""
    equity_curve: pd.DataFrame = field(default_factory=pd.DataFrame)
    trades: List[Trade] = field(default_factory=list)
    metrics: BacktestMetrics = field(default_factory=BacktestMetrics)
    start_date: Optional[date] = None
    end_date: Optional[date] = None
    initial_capital: float = 10000.0
    final_equity: float = 10000.0
    tickers: List[str] = field(default_factory=list)
    data_source: str = "sample"  # "tws" or "sample"
    
    def get_trades_df(self) -> pd.DataFrame:
        """Get trades as a DataFrame."""
        if not self.trades:
            return pd.DataFrame(columns=[
                "entry_time", "exit_time", "symbol", "side", 
                "quantity", "entry_price", "exit_price", "pnl", "pnl_percent"
            ])
        return pd.DataFrame([t.to_dict() for t in self.trades])


try:
    from nautilus_trader.backtest.engine import BacktestEngine as NautilusEngine, BacktestEngineConfig
    from nautilus_trader.backtest.config import BacktestVenueConfig
    from nautilus_trader.config import TradingNodeConfig, LoggingConfig
    from nautilus_trader.model.currencies import USD
    from nautilus_trader.model.data import Bar, BarType, BarSpecification
    from nautilus_trader.model.enums import BarAggregation, PriceType
    from nautilus_trader.model.identifiers import InstrumentId, Venue
    from nautilus_trader.model.instruments import Instrument
    from nautilus_trader.model.objects import Money
    from nautilus_trader.test_kit.providers import TestInstrumentProvider
    # Import Nautilus integration from strategy
    from src.bot.strategy.base import (
        NautilusDynamicRuleStrategy,
        NautilusDynamicRuleStrategyConfig,
        TIMEFRAME_TO_AGGREGATION
    )
    NAUTILUS_BACKTEST_AVAILABLE = True
except ImportError as exc:
    NAUTILUS_BACKTEST_AVAILABLE = False
    logger.warning("Nautilus backtest import failed: %s", exc)
    logger.warning("Nautilus Trader not installed. Scalable backtesting backend unavailable.")

class BacktestEngine:
    """
    Engine for running backtests.
    
    Supports two modes:
    1. 'native': Custom lightweight engine for fast UI feedback (default)
    2. 'nautilus': Full Nautilus Trader engine for scalable/accurate testing
    """
    
    def __init__(
        self,
        strategy: Strategy,
        initial_capital: float = 10000.0,
        commission: float = 0.0,
        slippage: float = 0.0,
        use_tws_data: bool = True,
        use_nautilus: bool = False,
    ):
        """
        Initialize the backtest engine.
        
        Args:
            strategy: Trading strategy to backtest
            initial_capital: Starting capital in dollars
            commission: Commission per trade in dollars (default: 0)
            slippage: Slippage percentage (default: 0)
            use_tws_data: Try to fetch real data from TWS
            use_nautilus: Whether to use Nautilus Trader engine
        """
        self.strategy = strategy
        self.initial_capital = initial_capital
        self.commission = commission
        self.slippage = slippage
        self.use_tws_data = use_tws_data
        
        # Determine engine mode
        self.use_nautilus = use_nautilus and NAUTILUS_BACKTEST_AVAILABLE
        if use_nautilus and not NAUTILUS_BACKTEST_AVAILABLE:
            logger.warning("Nautilus requested but not available. Falling back to native engine.")
        
        self.rule_engine = RuleEngine(strategy)
        
        # State tracking (native mode)
        self._equity = initial_capital
        self._cash = initial_capital
        self._positions: Dict[str, Trade] = {}
        self._trades: List[Trade] = []
        self._equity_history: List[Dict[str, Any]] = []
        
        # Data source tracking
        self._data_source: str = "sample"

    def run(
        self,
        tickers: List[str],
        start_date: date,
        end_date: date,
        timeframe: str = "5m",
    ) -> BacktestResult:
        """Run the backtest using selected engine."""
        # Load data first (common to both engines)
        # Note: Nautilus needs data to be loaded before we can configure the engine
        # properly with instruments.
        
        # Optimization: If using Nautilus with 100+ tickers, 
        # consider lazy loading or streaming, but for now we load to check availability.
        
        if self.use_nautilus:
            return self._run_nautilus(tickers, start_date, end_date, timeframe)
        else:
            return self._run_native(tickers, start_date, end_date, timeframe)

    def _run_nautilus(
        self, 
        tickers: List[str], 
        start_date: date, 
        end_date: date, 
        timeframe: str
    ) -> BacktestResult:
        """Run backtest using Nautilus Trader engine."""
        logger.info("Starting Nautilus Trader backtest...")
        
        # 1. Load DataFrames first
        bar_data_dict = self._load_historical_data(tickers, start_date, end_date, timeframe)
        if not bar_data_dict:
            return self._create_empty_result(start_date, end_date, tickers)

        # 2. Configure Engine
        nautilus_config = BacktestEngineConfig(
            trader_id="BACKTESTER",
            logging=LoggingConfig(log_level="INFO", log_level_file="OFF")
        )
        engine = NautilusEngine(config=nautilus_config)

        # 3. Setup Venue
        # We simulate a "SIM" venue
        venue = Venue("SIM")
        engine.add_venue(
            venue=venue,
            config=BacktestVenueConfig(
                name="SIM",
                start_money=Money(self.initial_capital, USD),
            )
        )

        # 4. Add Instruments & Data
        instruments = []
        
        # Map timeframe to Nautilus BarAggregation
        from src.bot.strategy.rules.models import TimeframeUnit
        tf_unit_map = {
            "1m": TimeframeUnit.M1, "5m": TimeframeUnit.M5, 
            "15m": TimeframeUnit.M15, "30m": TimeframeUnit.M30,
            "1h": TimeframeUnit.H1, "4h": TimeframeUnit.H4, "1d": TimeframeUnit.D1
        }
        
        tf_enum = tf_unit_map.get(timeframe, TimeframeUnit.M5)
        agg_freq, agg_count = TIMEFRAME_TO_AGGREGATION.get(tf_enum, ("MINUTE", 5))
        
        nautilus_freq_map = {
            "MINUTE": BarAggregation.MINUTE,
            "HOUR": BarAggregation.HOUR,
            "DAY": BarAggregation.DAY
        }
        
        start_dt = pd.Timestamp(start_date).to_pydatetime()
        end_dt = pd.Timestamp(end_date).to_pydatetime() + timedelta(days=1)

        # Helper to convert DataFrame to Nautilus Bars
        def df_to_bars(df: pd.DataFrame, instrument: Instrument, bar_type: BarType) -> List[Bar]:
            bars = []
            for row in df.itertuples():
                # Ensure we have a valid timestamp
                ts = row.timestamp
                if isinstance(ts, str):
                    ts = pd.Timestamp(ts)
                
                # Convert to uint64 nanoseconds (Nautilus format)
                # Need to check msgspec/nautilus version specifics, usually it takes int ns
                ts_ns = uint64(ts.value)
                
                bar = Bar(
                    bar_type=bar_type,
                    open=Money(row.open, USD),
                    high=Money(row.high, USD),
                    low=Money(row.low, USD),
                    close=Money(row.close, USD),
                    volume=getattr(row, "volume", 0),
                    ts_event=ts_ns,
                    ts_init=ts_ns,
                )
                bars.append(bar)
            return bars

        try:
            # Need numpy uint64 for timestamps
            from numpy import uint64
            
            for ticker, df in bar_data_dict.items():
                if ticker == "VIX": continue # Skip VIX as tradeable instrument for now
                
                # Define instrument
                instrument_id = InstrumentId.from_str(f"{ticker}.SIM")
                instrument = TestInstrumentProvider.equity(
                    venue=venue,
                    symbol=ticker,
                )
                
                # Add instrument to engine
                engine.add_instrument(instrument)
                instruments.append(str(instrument_id))
                
                # Define Bar Type
                bar_type = BarType(
                    instrument_id=instrument_id,
                    bar_spec=BarSpecification(
                        step=agg_count,
                        freq=nautilus_freq_map[agg_freq],
                        price_type=PriceType.LAST,
                    ),
                    aggregation=BarAggregation.MINUTE, # Actually depends on spec
                )
                
                # Convert data and add to engine
                bars = df_to_bars(df, instrument, bar_type)
                engine.add_data(bars)
                
            # 5. Add Strategy
            config = NautilusDynamicRuleStrategyConfig(
                strategy_config_path="config/active_strategy.json",
                instruments=instruments,
                max_position_per_instrument=Decimal(str(self.initial_capital / max(1, len(instruments)))),
                use_equal_allocation=True,
            )
            
            # We need to temporarily save the current strategy to file so it loads correct rules
            # Or modify NautilusDynamicRuleStrategy to accept strategy object directly
            # For now, let's assume active_strategy.json is up to date or we write it
            
            engine.add_strategy(
                strategy_cls=NautilusDynamicRuleStrategy,
                config=config,
            )
            
            # 6. Run
            engine.run(start=start_dt, end=end_dt)
            
            # 7. Extract Results
            # Nautilus engine.trader.generated_orders / fills / positions
            # Map back to our Result format
            
            # This is complex mapping, for now we will log success and return empty
            # In a real implementation we would iterate engine.trader.fills
            
            logger.info("Nautilus backtest finished successfully")

            mapped = self._try_map_nautilus_results(engine, tickers, start_date, end_date)
            if mapped is not None:
                return mapped

            logger.warning("Nautilus result mapping not available; falling back to native engine results")
            self._data_source = "nautilus_fallback"
            return self._run_native(tickers, start_date, end_date, timeframe)

        except Exception as e:
            logger.error(f"Nautilus backtest failed: {e}", exc_info=True)
            logger.warning("Falling back to native engine")
            return self._run_native(tickers, start_date, end_date, timeframe)

    def _run_native(
        self,
        tickers: List[str],
        start_date: date,
        end_date: date,
        timeframe: str = "5m",
    ) -> BacktestResult:
        """
        Run the backtest using native lightweight engine.
        
        Args:
            tickers: List of ticker symbols to trade
            start_date: Start date for backtest
            end_date: End date for backtest
            timeframe: Bar timeframe (default: 5m)
            
        Returns:
            BacktestResult with equity curve, trades, and metrics
        """
        logger.info(f"Starting native backtest from {start_date} to {end_date}")
        logger.info(f"Tickers: {tickers}, Initial Capital: ${self.initial_capital:,.2f}")
        
        # Reset state
        self._reset_state()
        
        # Load historical data for all tickers
        bar_data = self._load_historical_data(tickers, start_date, end_date, timeframe)
        
        if not bar_data:
            logger.warning("No historical data available for backtest")
            return self._create_empty_result(start_date, end_date, tickers)
        
        # Get aligned timestamps (intersection of all tickers)
        timestamps = self._get_aligned_timestamps(bar_data)
        
        if len(timestamps) == 0:
            logger.warning("No overlapping timestamps in bar data")
            return self._create_empty_result(start_date, end_date, tickers)
        
        logger.info(f"Processing {len(timestamps)} bars")
        
        # Calculate position size (equal allocation)
        if not tickers:
             position_size = 0
        else:
             position_size = self.initial_capital / len(tickers)
        
        # Iterate through time
        for i, timestamp in enumerate(timestamps):
            current_time = pd.Timestamp(timestamp).to_pydatetime()
            
            # Build market data snapshot for this timestamp
            market_data = self._build_market_snapshot(bar_data, timestamp, i)
            
            # Get current prices for position marking
            current_prices = {
                ticker: bars.iloc[i]["close"] 
                for ticker, bars in bar_data.items() 
                if i < len(bars)
            }
            
            # Evaluate global rules first
            global_pass = self.rule_engine.evaluate_global_rules(
                market_data=market_data,
                vix_bars=market_data.get("VIX"),
                current_time=current_time
            )
            
            if global_pass:
                # Evaluate per-ticker rules and generate signals
                for ticker in tickers:
                    if ticker not in market_data:
                        continue
                    
                    ticker_bars = market_data[ticker]
                    if ticker_bars.empty:
                        continue
                    
                    actions = self.rule_engine.evaluate_ticker_rules(
                        ticker=ticker,
                        bars=ticker_bars,
                        vix_bars=market_data.get("VIX"),
                        current_time=current_time,
                        market_data=market_data,
                    )
                    
                    current_price = current_prices.get(ticker, 0)
                    
                    # Process signals
                    self._process_signals(
                        ticker, actions, current_time, current_price, position_size
                    )
            
            # Update equity with current positions
            self._update_equity(current_prices, current_time)
        
        # Close any remaining positions at end
        self._close_all_positions(timestamps[-1], bar_data)
        
        # Calculate final metrics
        result = self._create_result(start_date, end_date, tickers)
        
        logger.info(f"Backtest complete. Final equity: ${self._equity:,.2f}")
        logger.info(f"Total trades: {len(self._trades)}, Return: {result.metrics.total_return_percent:.2f}%")
        
        return result

    def _try_map_nautilus_results(
        self,
        engine: Any,
        tickers: List[str],
        start_date: date,
        end_date: date,
    ) -> Optional[BacktestResult]:
        """Best-effort mapping of Nautilus fills to BacktestResult."""
        try:
            trader = getattr(engine, "trader", None)
            if trader is None:
                return None

            fills = None
            for attr in ("fills", "generated_fills", "executed_fills", "orders_filled"):
                fills = getattr(trader, attr, None)
                if fills:
                    break

            if not fills:
                return None

            fills_list = list(fills)
            if not fills_list:
                return None

            cash = self.initial_capital
            positions: Dict[str, float] = {}
            last_price: Dict[str, float] = {}
            trades: List[Trade] = []
            open_trades: Dict[str, List[Trade]] = {}
            equity_points: List[Dict[str, Any]] = []

            def _fill_side(fill: Any) -> str:
                side_val = getattr(fill, "order_side", getattr(fill, "side", ""))
                if hasattr(side_val, "value"):
                    return str(side_val.value).upper()
                return str(side_val).upper()

            for fill in fills_list:
                instrument = getattr(fill, "instrument_id", None)
                instrument_str = str(instrument) if instrument is not None else ""
                symbol = instrument_str.split(".")[0] if instrument_str else ""

                qty = float(getattr(fill, "last_qty", getattr(fill, "quantity", 0)) or 0)
                px = float(getattr(fill, "last_px", getattr(fill, "price", 0)) or 0)

                ts_event = getattr(fill, "ts_event", getattr(fill, "timestamp", None))
                if ts_event is None:
                    ts = datetime.utcnow()
                else:
                    try:
                        ts = pd.to_datetime(ts_event, unit="ns", errors="coerce")
                        if pd.isna(ts):
                            ts = pd.to_datetime(ts_event, errors="coerce")
                        if not isinstance(ts, datetime):
                            ts = ts.to_pydatetime()
                    except Exception:
                        ts = datetime.utcnow()

                side = _fill_side(fill)
                if not symbol or qty <= 0 or px <= 0:
                    continue

                last_price[symbol] = px
                position_qty = positions.get(symbol, 0.0)

                if side == "BUY":
                    positions[symbol] = position_qty + qty
                    cash -= (qty * px) + self.commission

                    open_trades.setdefault(symbol, []).append(
                        Trade(
                            entry_time=ts,
                            exit_time=None,
                            symbol=symbol,
                            side=TradeSide.BUY,
                            quantity=qty,
                            entry_price=px,
                        )
                    )

                elif side == "SELL":
                    positions[symbol] = max(0.0, position_qty - qty)
                    cash += (qty * px) - self.commission

                    if symbol in open_trades and open_trades[symbol]:
                        remaining = qty
                        fifo_trades = open_trades[symbol]
                        while fifo_trades and remaining > 0:
                            trade = fifo_trades[0]
                            if trade.quantity <= remaining:
                                remaining -= trade.quantity
                                trade.close(ts, px)
                                trades.append(trade)
                                fifo_trades.pop(0)
                            else:
                                closed_trade = Trade(
                                    entry_time=trade.entry_time,
                                    exit_time=None,
                                    symbol=trade.symbol,
                                    side=trade.side,
                                    quantity=remaining,
                                    entry_price=trade.entry_price,
                                )
                                closed_trade.close(ts, px)
                                trades.append(closed_trade)
                                trade.quantity -= remaining
                                remaining = 0
                        if not open_trades[symbol]:
                            del open_trades[symbol]

                equity = cash
                for sym, pos_qty in positions.items():
                    equity += pos_qty * last_price.get(sym, 0.0)
                equity_points.append({"timestamp": ts, "equity": equity, "cash": cash})

            for sym, trade_list in list(open_trades.items()):
                last_px = last_price.get(sym)
                if last_px is None:
                    continue
                for trade in trade_list:
                    trade.close(trade.entry_time, last_px)
                    trades.append(trade)

            equity_df = pd.DataFrame(equity_points)
            if not equity_df.empty:
                equity_df["drawdown"] = equity_df["equity"].cummax() - equity_df["equity"]
                equity_df["drawdown_pct"] = equity_df["drawdown"] / equity_df["equity"].cummax() * 100

            self._trades = trades
            self._equity_history = equity_points
            self._equity = equity_df["equity"].iloc[-1] if not equity_df.empty else cash
            self._data_source = "nautilus_engine"

            metrics = self._calculate_metrics(equity_df)

            return BacktestResult(
                equity_curve=equity_df,
                trades=trades,
                metrics=metrics,
                start_date=start_date,
                end_date=end_date,
                initial_capital=self.initial_capital,
                final_equity=self._equity,
                tickers=tickers,
                data_source=self._data_source,
            )
        except Exception as e:
            logger.warning(f"Failed to map Nautilus results: {e}")
            return None
    
    def _reset_state(self) -> None:
        """Reset backtest state for a new run."""
        self._equity = self.initial_capital
        self._cash = self.initial_capital
        self._positions.clear()
        self._trades.clear()
        self._equity_history.clear()
        self._data_source = "sample"
    
    def _load_historical_data(
        self,
        tickers: List[str],
        start_date: date,
        end_date: date,
        timeframe: str
    ) -> Dict[str, pd.DataFrame]:
        """
        Load historical bar data for all tickers.
        
        Priority:
        1. TWS API (if connected and use_tws_data=True) - saves to cache
        2. Local historical cache (data/historical)
        3. Sample data directory (data/sample)
        4. Synthetic generation (last resort)
        
        Args:
            tickers: List of ticker symbols
            start_date: Start date for data
            end_date: End date for data
            timeframe: Bar timeframe string
            
        Returns:
            Dict mapping ticker to DataFrame with OHLCV data
        """
        bar_data = {}
        missing_tickers = []
        
        # 1. Try to use TWS data first if requested
        if self.use_tws_data:
            tws_data = self._load_tws_data(tickers, start_date, end_date, timeframe)
            if tws_data:
                self._data_source = "tws"
                logger.info(f"Using TWS historical data for {len(tws_data)} symbols")
                
                # Update bar_data with what we found
                bar_data.update(tws_data)
                
                # Check what's still missing
                missing_tickers = [t for t in tickers if t not in bar_data]
                if not missing_tickers:
                    return bar_data
            else:
                logger.warning("TWS data unavailable, falling back to cache/sample data")
                missing_tickers = list(tickers)
        else:
            missing_tickers = list(tickers)
            
        # 2 & 3. Try to load from cache/sample for missing tickers
        if missing_tickers:
            cached_data = self._load_cached_data(missing_tickers, start_date, end_date, timeframe)
            if cached_data:
                bar_data.update(cached_data)
                self._data_source = "cache" if not bar_data else "mixed"
                
                # Update missing list
                missing_tickers = [t for t in tickers if t not in bar_data]
        
        # 4. Generate synthetic data for any remaining tickers
        if missing_tickers:
            logger.warning(f"Generating synthetic data for {len(missing_tickers)} symbols: {missing_tickers}")
            synthetic_data = self._generate_sample_data(missing_tickers, start_date, end_date, timeframe)
            bar_data.update(synthetic_data)
            if self._data_source == "sample":
                self._data_source = "synthetic"
            elif self._data_source:
                self._data_source += "+synthetic"
        
        return bar_data

    def _get_cache_path(self, ticker: str, timeframe: str) -> Path:
        """Get path for cached data file."""
        # Use project root/data/historical
        project_root = Path(__file__).parent.parent.parent
        history_dir = project_root / "data" / "historical"
        history_dir.mkdir(parents=True, exist_ok=True)
        return history_dir / f"{ticker}_{timeframe}.csv"

    def _get_sample_path(self, ticker: str, timeframe: str) -> Path:
        """Get path for sample data file."""
        project_root = Path(__file__).parent.parent.parent
        # Try both naming conventions: 5m and 5min
        sample_dir = project_root / "data" / "sample"
        path1 = sample_dir / f"{ticker}_{timeframe}.csv"
        if path1.exists():
            return path1
            
        # Try mapping 5m -> 5min
        tf_map = {"5m": "5min", "1m": "1min", "1h": "1hour", "1d": "1day"}
        if timeframe in tf_map:
            path2 = sample_dir / f"{ticker}_{tf_map[timeframe]}.csv"
            if path2.exists():
                return path2
                
        return path1

    def _save_to_cache(self, ticker: str, timeframe: str, df: pd.DataFrame) -> None:
        """Save DataFrame to local cache CSV."""
        try:
            path = self._get_cache_path(ticker, timeframe)
            df.to_csv(path, index=False)
            logger.debug(f"Cached {ticker} data to {path}")
        except Exception as e:
            logger.warning(f"Failed to cache data for {ticker}: {e}")

    def _load_cached_data(
        self,
        tickers: List[str],
        start_date: date,
        end_date: date,
        timeframe: str
    ) -> Dict[str, pd.DataFrame]:
        """Load data from local CSV cache."""
        data = {}
        for ticker in tickers:
            # Check historical cache first
            path = self._get_cache_path(ticker, timeframe)
            if not path.exists():
                # Check sample directory
                path = self._get_sample_path(ticker, timeframe)
            
            if path.exists():
                try:
                    df = pd.read_csv(path, on_bad_lines="skip")
                    df = self._sanitize_bars_df(df)
                    
                    # Ensure timestamp column exists and is datetime
                    if "timestamp" in df.columns:
                        df["timestamp"] = pd.to_datetime(df["timestamp"], errors="coerce")
                        df = df.dropna(subset=["timestamp"])
                        
                        # Filter by date
                        df["date"] = df["timestamp"].dt.date
                        mask = (df["date"] >= start_date) & (df["date"] <= end_date)
                        filtered_df = df[mask]
                        
                        if not filtered_df.empty:
                            data[ticker] = filtered_df.drop(columns=["date"])
                            logger.info(f"Loaded {ticker} from cache: {path}")
                        else:
                            logger.debug(f"Cache for {ticker} exists but no data in date range")
                    else:
                        logger.warning(f"Invalid cache file format for {ticker}: {path}")
                        
                except Exception as e:
                    logger.warning(f"Error reading cache for {ticker}: {e}")
            
            # Special handling for VIX if requested and missing
            if ticker == "VIX" and "VIX" not in data:
                # Try to generate it or find it with different name
                pass
                
        return data

    def _sanitize_bars_df(self, df: pd.DataFrame) -> pd.DataFrame:
        """Coerce bar columns to numeric and drop invalid rows."""
        required_cols = ["open", "high", "low", "close"]
        if not isinstance(df, pd.DataFrame) or df.empty:
            return df

        for col in required_cols:
            if col in df.columns:
                df[col] = pd.to_numeric(df[col], errors="coerce")

        if "volume" in df.columns:
            df["volume"] = pd.to_numeric(df["volume"], errors="coerce").fillna(0)

        if all(col in df.columns for col in required_cols):
            df = df.dropna(subset=required_cols)

        return df

    def _load_tws_data(
        self,
        tickers: List[str],
        start_date: date,
        end_date: date,
        timeframe: str
    ) -> Optional[Dict[str, pd.DataFrame]]:
        """
        Load historical data from TWS.
        
        Returns:
            Dict of DataFrames or None if TWS unavailable
        """
        try:
            from src.bot.tws_data_provider import get_tws_provider
            
            provider = get_tws_provider()
            
            # Try to connect
            if not provider.connect(timeout=5.0):
                logger.warning("Could not connect to TWS for historical data")
                return None
            
            # Map timeframe to TWS bar size
            bar_size_map = {
                "1m": "1 min",
                "5m": "5 mins",
                "15m": "15 mins",
                "30m": "30 mins",
                "1h": "1 hour",
                "4h": "4 hours",
                "1d": "1 day",
            }
            bar_size = bar_size_map.get(timeframe, "5 mins")
            
            # Calculate duration
            days = (end_date - start_date).days
            if days <= 1:
                duration = "1 D"
            elif days <= 7:
                duration = "1 W"
            elif days <= 30:
                duration = "1 M"
            elif days <= 90:
                duration = "3 M"
            elif days <= 180:
                duration = "6 M"
            elif days <= 365:
                duration = "1 Y"
            else:
                duration = "2 Y"
            
            # End datetime
            end_dt = datetime.combine(end_date, datetime.max.time().replace(microsecond=0))
            
            bar_data = {}
            
            # Fetch data for each ticker
            for ticker in tickers:
                try:
                    df = provider.get_historical_data(
                        symbol=ticker,
                        duration=duration,
                        bar_size=bar_size,
                        end_datetime=end_dt,
                        what_to_show="TRADES" if ticker != "VIX" else "TRADES",
                        use_rth=True,
                        timeout=30.0,
                    )
                    
                    if not df.empty:
                        # Filter to date range
                        df["date"] = pd.to_datetime(df["timestamp"]).dt.date
                        df = df[(df["date"] >= start_date) & (df["date"] <= end_date)]
                        
                        if not df.empty:
                            bar_data[ticker] = df
                            logger.info(f"Loaded {len(df)} bars for {ticker} from TWS")
                            
                            # Save to cache for future use
                            self._save_to_cache(ticker, timeframe, df)
                    
                except Exception as e:
                    logger.warning(f"Failed to load TWS data for {ticker}: {e}")
            
            # Also try to get VIX if not already in list
            if "VIX" not in bar_data and "VIX" not in tickers:
                try:
                    vix_df = provider.get_historical_data(
                        symbol="VIX",
                        duration=duration,
                        bar_size=bar_size,
                        end_datetime=end_dt,
                        use_rth=True,
                        timeout=30.0,
                    )
                    if not vix_df.empty:
                        vix_df["date"] = pd.to_datetime(vix_df["timestamp"]).dt.date
                        vix_df = vix_df[(vix_df["date"] >= start_date) & (vix_df["date"] <= end_date)]
                        if not vix_df.empty:
                            bar_data["VIX"] = vix_df
                            # Cache VIX too
                            self._save_to_cache("VIX", timeframe, vix_df)
                except Exception as e:
                    logger.debug(f"Could not load VIX data: {e}")
            
            if bar_data:
                return bar_data
            
            return None
            
        except ImportError:
            logger.debug("TWS data provider not available")
            return None
        except Exception as e:
            logger.warning(f"Error loading TWS data: {e}")
            return None
    
    def _generate_sample_data(
        self,
        tickers: List[str],
        start_date: date,
        end_date: date,
        timeframe: str
    ) -> Dict[str, pd.DataFrame]:
        """
        Generate sample data for backtesting when TWS is unavailable.
        """
        tf_map = {
            "1m": Timeframe.MINUTE_1,
            "5m": Timeframe.MINUTE_5,
            "15m": Timeframe.MINUTE_15,
            "30m": Timeframe.MINUTE_30,
            "1h": Timeframe.HOUR_1,
            "4h": Timeframe.HOUR_4,
            "1d": Timeframe.DAY_1,
        }
        tf = tf_map.get(timeframe, Timeframe.MINUTE_5)
        
        # Calculate number of bars based on date range and timeframe
        days = (end_date - start_date).days
        tf_minutes = {
            "1m": 1, "5m": 5, "15m": 15, "30m": 30,
            "1h": 60, "4h": 240, "1d": 1440
        }
        minutes_per_bar = tf_minutes.get(timeframe, 5)
        trading_minutes_per_day = 390  # 6.5 hours for US market
        num_bars = max(100, int(days * trading_minutes_per_day / minutes_per_bar))
        
        bar_data = {}
        
        # Try to infer prices from cache index if available
        # This is strictly a fallback for when no real data is available
        
        for ticker in tickers:
            # Randomize base price slightly to avoid exact 100.0 everywhere
            # Hash ticker to get consistent price for same ticker
            seed = sum(ord(c) for c in ticker)
            base_price = 100.0 + (seed % 400)
            
            # Generate bar series
            bar_series = generate_sample_bars(
                symbol=ticker,
                timeframe=tf,
                num_bars=num_bars,
                base_price=base_price,
                volatility=0.015
            )
            
            # Convert to DataFrame
            df = pd.DataFrame({
                "timestamp": bar_series.timestamps,
                "open": bar_series.opens,
                "high": bar_series.highs,
                "low": bar_series.lows,
                "close": bar_series.closes,
                "volume": bar_series.volumes,
            })
            
            # Filter to date range (approximate, since we generated from end date backward)
            df["date"] = pd.to_datetime(df["timestamp"]).dt.date
            
            bar_data[ticker] = df
        
        # Also generate VIX data
        vix_series = generate_sample_bars(
            symbol="VIX",
            timeframe=tf,
            num_bars=num_bars,
            base_price=18.0,
            volatility=0.03
        )
        
        vix_df = pd.DataFrame({
            "timestamp": vix_series.timestamps,
            "open": vix_series.opens,
            "high": vix_series.highs,
            "low": vix_series.lows,
            "close": vix_series.closes,
            "volume": vix_series.volumes,
        })
        bar_data["VIX"] = vix_df
        
        logger.info(f"Generated sample data for {len(bar_data)} symbols")
        return bar_data
    
    def _get_aligned_timestamps(
        self,
        bar_data: Dict[str, pd.DataFrame]
    ) -> np.ndarray:
        """Get timestamps that exist across all tickers."""
        if not bar_data:
            return np.array([])
        
        # Get timestamps from first non-VIX ticker
        tickers = [t for t in bar_data.keys() if t != "VIX"]
        if not tickers:
            return np.array([])
        
        # Use first ticker's timestamps as base
        base_ticker = tickers[0]
        return bar_data[base_ticker]["timestamp"].values
    
    def _build_market_snapshot(
        self,
        bar_data: Dict[str, pd.DataFrame],
        timestamp: np.datetime64,
        bar_index: int
    ) -> Dict[str, pd.DataFrame]:
        """Build market data snapshot up to current bar."""
        snapshot = {}
        
        for ticker, df in bar_data.items():
            # Include all bars up to and including current
            if bar_index < len(df):
                snapshot[ticker] = df.iloc[:bar_index + 1].copy()
            else:
                snapshot[ticker] = df.copy()
        
        return snapshot
    
    def _process_signals(
        self,
        ticker: str,
        actions: List[str],
        current_time: datetime,
        current_price: float,
        position_size: float
    ) -> None:
        """Process trading signals for a ticker."""
        if not actions or current_price <= 0:
            return
        
        has_position = ticker in self._positions
        
        for action in actions:
            action_upper = action.upper()
            
            if action_upper == "BUY" and not has_position:
                # Open long position
                quantity = position_size / current_price
                trade = Trade(
                    entry_time=current_time,
                    exit_time=None,
                    symbol=ticker,
                    side=TradeSide.BUY,
                    quantity=quantity,
                    entry_price=current_price,
                )
                self._positions[ticker] = trade
                self._cash -= position_size + self.commission
                has_position = True
                logger.debug(f"BUY {ticker}: {quantity:.2f} @ ${current_price:.2f}")
                
            elif action_upper == "SELL" and has_position:
                # Close position
                trade = self._positions[ticker]
                trade.close(current_time, current_price)
                
                # Update cash
                position_value = trade.quantity * current_price
                self._cash += position_value - self.commission
                
                self._trades.append(trade)
                del self._positions[ticker]
                has_position = False
                logger.debug(f"SELL {ticker}: PnL ${trade.pnl:.2f}")
    
    def _update_equity(
        self,
        current_prices: Dict[str, float],
        current_time: datetime
    ) -> None:
        """Update equity based on current positions."""
        # Calculate position values
        position_value = 0.0
        for ticker, trade in self._positions.items():
            if ticker in current_prices:
                position_value += trade.quantity * current_prices[ticker]
        
        self._equity = self._cash + position_value
        
        # Record equity point
        self._equity_history.append({
            "timestamp": current_time,
            "equity": self._equity,
            "cash": self._cash,
            "position_value": position_value,
        })
    
    def _close_all_positions(
        self,
        final_timestamp: np.datetime64,
        bar_data: Dict[str, pd.DataFrame]
    ) -> None:
        """Close all remaining positions at end of backtest."""
        final_time = pd.Timestamp(final_timestamp).to_pydatetime()
        
        for ticker in list(self._positions.keys()):
            if ticker in bar_data:
                final_price = bar_data[ticker].iloc[-1]["close"]
                trade = self._positions[ticker]
                trade.close(final_time, final_price)
                
                position_value = trade.quantity * final_price
                self._cash += position_value - self.commission
                
                self._trades.append(trade)
                del self._positions[ticker]
    
    def _create_result(
        self,
        start_date: date,
        end_date: date,
        tickers: List[str]
    ) -> BacktestResult:
        """Create the final backtest result."""
        # Create equity curve DataFrame
        equity_df = pd.DataFrame(self._equity_history)
        if not equity_df.empty:
            equity_df["drawdown"] = equity_df["equity"].cummax() - equity_df["equity"]
            equity_df["drawdown_pct"] = (
                equity_df["drawdown"] / equity_df["equity"].cummax() * 100
            )
        
        # Calculate metrics
        metrics = self._calculate_metrics(equity_df)
        
        return BacktestResult(
            equity_curve=equity_df,
            trades=self._trades.copy(),
            metrics=metrics,
            start_date=start_date,
            end_date=end_date,
            initial_capital=self.initial_capital,
            final_equity=self._equity,
            tickers=tickers,
            data_source=self._data_source,
        )
    
    def _create_empty_result(
        self,
        start_date: date,
        end_date: date,
        tickers: List[str]
    ) -> BacktestResult:
        """Create an empty result when no data is available."""
        return BacktestResult(
            equity_curve=pd.DataFrame(columns=["timestamp", "equity", "drawdown"]),
            trades=[],
            metrics=BacktestMetrics(),
            start_date=start_date,
            end_date=end_date,
            initial_capital=self.initial_capital,
            final_equity=self.initial_capital,
            tickers=tickers,
        )
    
    def _calculate_metrics(self, equity_df: pd.DataFrame) -> BacktestMetrics:
        """Calculate performance metrics from equity curve and trades."""
        metrics = BacktestMetrics()
        
        if equity_df.empty or not self._trades:
            metrics.total_return = 0.0
            metrics.total_return_percent = 0.0
            return metrics
        
        # Basic return metrics
        metrics.total_return = self._equity - self.initial_capital
        metrics.total_return_percent = (
            (self._equity / self.initial_capital) - 1
        ) * 100
        
        # Trade statistics
        metrics.total_trades = len(self._trades)
        
        winning_trades = [t for t in self._trades if t.pnl > 0]
        losing_trades = [t for t in self._trades if t.pnl <= 0]
        
        metrics.winning_trades = len(winning_trades)
        metrics.losing_trades = len(losing_trades)
        
        if metrics.total_trades > 0:
            metrics.win_rate = (metrics.winning_trades / metrics.total_trades) * 100
        
        # Average win/loss
        if winning_trades:
            metrics.avg_win = np.mean([t.pnl for t in winning_trades])
        if losing_trades:
            metrics.avg_loss = abs(np.mean([t.pnl for t in losing_trades]))
        
        # Profit factor
        total_wins = sum(t.pnl for t in winning_trades) if winning_trades else 0
        total_losses = abs(sum(t.pnl for t in losing_trades)) if losing_trades else 0
        if total_losses > 0:
            metrics.profit_factor = total_wins / total_losses
        elif total_wins > 0:
            metrics.profit_factor = float('inf')
        
        # Max drawdown
        if "drawdown" in equity_df.columns:
            metrics.max_drawdown = equity_df["drawdown"].max()
            metrics.max_drawdown_percent = equity_df["drawdown_pct"].max()
        
        # Sharpe ratio (annualized, assuming 252 trading days)
        if len(equity_df) > 1:
            equity_df["returns"] = equity_df["equity"].pct_change()
            returns = equity_df["returns"].dropna()
            
            if len(returns) > 0 and returns.std() > 0:
                # Annualize based on bar frequency (approximate)
                trading_periods_per_year = 252 * 78  # 5-min bars
                mean_return = returns.mean()
                std_return = returns.std()
                metrics.sharpe_ratio = (
                    mean_return / std_return * np.sqrt(trading_periods_per_year)
                )
        
        # Average trade duration
        durations = []
        for trade in self._trades:
            if trade.exit_time and trade.entry_time:
                duration = trade.exit_time - trade.entry_time
                durations.append(duration)
        
        if durations:
            avg_seconds = np.mean([d.total_seconds() for d in durations])
            metrics.avg_trade_duration = timedelta(seconds=avg_seconds)
        
        return metrics
