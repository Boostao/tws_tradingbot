"""
Dynamic Rule Strategy Module

Implements a Nautilus Trader strategy that executes trading rules
defined in JSON configuration files.

This strategy:
1. Loads rules from a JSON strategy file
2. Subscribes to required market data based on rules
3. Evaluates global filter rules on each bar
4. Evaluates per-ticker signal rules
5. Generates BUY/SELL orders based on rule results
"""

import logging
import threading
from datetime import datetime
from decimal import Decimal
from pathlib import Path
from typing import Dict, List, Optional, Any
from dataclasses import dataclass, field

import pandas as pd
import numpy as np

# Nautilus Trader imports - wrapped in try/except for environments where not installed
try:
    from nautilus_trader.config import StrategyConfig
    from nautilus_trader.trading.strategy import Strategy
    from nautilus_trader.model.data import Bar, BarType, BarSpecification, QuoteTick, TradeTick
    from nautilus_trader.model.enums import OrderSide, TimeInForce, BarAggregation
    from nautilus_trader.model.identifiers import InstrumentId, Venue
    from nautilus_trader.model.instruments import Instrument
    from nautilus_trader.model.orders import MarketOrder
    from nautilus_trader.model.events import OrderFilled, PositionChanged
    from nautilus_trader.model.position import Position
    from nautilus_trader.model.objects import Quantity
    NAUTILUS_AVAILABLE = True
except ImportError:
    NAUTILUS_AVAILABLE = False
    # Create stub classes for development/testing without Nautilus
    class StrategyConfig:
        pass
    class Strategy:
        pass

from src.bot.strategy.rules.models import Strategy as StrategyModel, TimeframeUnit
from src.bot.strategy.rules.engine import RuleEngine
from src.bot.strategy.rules.serialization import load_strategy
from src.bot.state import (
    BotState,
    BotStatus,
    Position as StatePosition,
    Order as StateOrder,
    update_state,
    check_stop_signal,
    check_emergency_stop,
    clear_stop_signals,
    get_state_logger,
)
from src.utils.logger import get_log_buffer
from src.bot.tws_data_provider import TWSDataProvider


logger = logging.getLogger(__name__)


# Timeframe mapping from our models to Nautilus BarAggregation
TIMEFRAME_TO_AGGREGATION = {
    TimeframeUnit.M1: ("MINUTE", 1),
    TimeframeUnit.M5: ("MINUTE", 5),
    TimeframeUnit.M15: ("MINUTE", 15),
    TimeframeUnit.M30: ("MINUTE", 30),
    TimeframeUnit.H1: ("HOUR", 1),
    TimeframeUnit.H4: ("HOUR", 4),
    TimeframeUnit.D1: ("DAY", 1),
}


@dataclass
class DynamicRuleStrategyConfig:
    """
    Configuration for the DynamicRuleStrategy.
    
    Attributes:
        strategy_id: Unique identifier for this strategy instance
        strategy_config_path: Path to the JSON strategy configuration file
        instruments: List of instrument IDs to trade (e.g., ["AAPL.NASDAQ", "SPY.ARCA"])
        base_currency: Base currency for the strategy
        max_position_per_instrument: Maximum position size per instrument (in quote currency)
        use_equal_allocation: If True, allocate capital equally among instruments
    """
    strategy_id: str = "dynamic_rule_strategy"
    strategy_config_path: str = "config/active_strategy.json"
    instruments: List[str] = field(default_factory=list)
    base_currency: str = "USD"
    max_position_per_instrument: float = 10000.0
    use_equal_allocation: bool = True
    
    def __post_init__(self):
        if not self.instruments:
            self.instruments = ["SPY.ARCA", "QQQ.NASDAQ"]


class BarBuffer:
    """
    Rolling buffer for storing bar data with efficient numpy operations.
    
    Maintains a fixed-size buffer of OHLCV data for indicator calculations.
    """
    
    def __init__(self, max_size: int = 500):
        """
        Initialize the bar buffer.
        
        Args:
            max_size: Maximum number of bars to store
        """
        self.max_size = max_size
        self._timestamps: List[datetime] = []
        self._opens: List[float] = []
        self._highs: List[float] = []
        self._lows: List[float] = []
        self._closes: List[float] = []
        self._volumes: List[float] = []
    
    def append(self, bar: Any) -> None:
        """
        Append a new bar to the buffer.
        
        Args:
            bar: Nautilus Bar object (or dict for testing)
        """
        if isinstance(bar, dict):
            # For testing without Nautilus
            self._timestamps.append(bar.get("timestamp", datetime.now()))
            self._opens.append(float(bar.get("open", 0)))
            self._highs.append(float(bar.get("high", 0)))
            self._lows.append(float(bar.get("low", 0)))
            self._closes.append(float(bar.get("close", 0)))
            self._volumes.append(float(bar.get("volume", 0)))
        else:
            # Nautilus Bar object
            self._timestamps.append(bar.ts_event)
            self._opens.append(float(bar.open))
            self._highs.append(float(bar.high))
            self._lows.append(float(bar.low))
            self._closes.append(float(bar.close))
            self._volumes.append(float(bar.volume))
        
        # Trim to max size
        if len(self._timestamps) > self.max_size:
            self._timestamps = self._timestamps[-self.max_size:]
            self._opens = self._opens[-self.max_size:]
            self._highs = self._highs[-self.max_size:]
            self._lows = self._lows[-self.max_size:]
            self._closes = self._closes[-self.max_size:]
            self._volumes = self._volumes[-self.max_size:]
    
    def to_dataframe(self) -> pd.DataFrame:
        """
        Convert buffer to pandas DataFrame.
        
        Returns:
            DataFrame with OHLCV columns
        """
        if not self._timestamps:
            return pd.DataFrame(columns=["timestamp", "open", "high", "low", "close", "volume"])
        
        return pd.DataFrame({
            "timestamp": self._timestamps,
            "open": self._opens,
            "high": self._highs,
            "low": self._lows,
            "close": self._closes,
            "volume": self._volumes,
        })
    
    def __len__(self) -> int:
        return len(self._timestamps)
    
    @property
    def closes(self) -> np.ndarray:
        """Get closing prices as numpy array."""
        return np.array(self._closes, dtype=np.float64)
    
    @property
    def is_ready(self) -> bool:
        """Check if buffer has enough data for indicator calculations."""
        return len(self._timestamps) >= 50  # Minimum bars for most indicators


class DynamicRuleStrategy:
    """
    A dynamic trading strategy that executes rules defined in JSON configuration.
    
    This strategy:
    1. Loads trading rules from a JSON file (see src/bot/strategy/rules/models.py)
    2. Subscribes to required bar data for each instrument
    3. On each bar update:
       - Evaluates global filter rules (VIX slope, market hours, etc.)
       - If global rules pass, evaluates per-ticker signal rules
       - Generates BUY/SELL orders based on signals
    4. Uses equal allocation sizing (1/N where N = number of instruments)
    
    For Nautilus Trader integration, this class would extend Strategy:
    
        class DynamicRuleStrategy(Strategy):
            ...
    
    Current implementation provides the core logic that can be adapted
    to Nautilus Trader's event-driven architecture.
    """
    
    def __init__(self, config: DynamicRuleStrategyConfig):
        """
        Initialize the dynamic rule strategy.
        
        Args:
            config: Strategy configuration
        """
        self.config = config
        self.strategy_id = config.strategy_id
        
        # Load strategy rules from JSON
        self.strategy_model: Optional[StrategyModel] = None
        self.rule_engine: Optional[RuleEngine] = None
        self._load_strategy()
        
        # Bar data buffers per instrument
        self._bar_buffers: Dict[str, BarBuffer] = {}
        
        # VIX buffer for global rules
        self._vix_buffer: BarBuffer = BarBuffer(max_size=200)
        
        # Position tracking
        self._positions: Dict[str, float] = {}  # instrument -> quantity
        self._entry_prices: Dict[str, float] = {}  # instrument -> entry price
        self._equity: float = config.max_position_per_instrument * len(config.instruments)
        self._initial_equity: float = self._equity
        self._daily_pnl: float = 0.0
        self._total_pnl: float = 0.0
        self._trades_today: int = 0
        self._wins_today: int = 0
        
        # Pending orders tracking
        self._pending_orders: Dict[str, StateOrder] = {}  # order_id -> Order
        
        # State management
        self._bot_state: Optional[BotState] = None
        self._state_update_counter: int = 0
        self._state_update_interval: int = 5  # Update state every N bars
        self._state_update_interval_seconds: int = 60
        self._last_state_update_time: datetime = datetime.now()
        self._state_refresh_stop = threading.Event()
        self._state_refresh_thread: Optional[threading.Thread] = None
        
        # State
        self._is_running: bool = False
        self._last_evaluation_time: Optional[datetime] = None
        self._tws_connected: bool = False
        self._tws_provider: Optional[TWSDataProvider] = None
        self._nautilus_strategy: Optional[Any] = None
        self._nautilus_cache_override: Optional[Any] = None

        # Order handlers (for Nautilus integration)
        self._submit_buy_handler: Optional[Any] = None
        self._submit_sell_handler: Optional[Any] = None
        self._cancel_order_handler: Optional[Any] = None

        # Reload signal path (shared with UI)
        self._reload_signal_path: Path = Path(__file__).parent.parent.parent / "config" / ".reload_signal"
        
        logger.info(f"DynamicRuleStrategy initialized: {self.strategy_id}")
        if self.strategy_model:
            logger.info(f"Loaded strategy: {self.strategy_model.name} with {len(self.strategy_model.rules)} rules")
    
    def _load_strategy(self) -> None:
        """Load strategy from JSON configuration file."""
        try:
            strategy_path = Path(self.config.strategy_config_path)
            if strategy_path.exists():
                self.strategy_model = load_strategy(strategy_path)
                self.rule_engine = RuleEngine(self.strategy_model)
                logger.info(f"Loaded strategy from {strategy_path}")
            else:
                logger.warning(f"Strategy file not found: {strategy_path}")
        except Exception as e:
            logger.error(f"Error loading strategy: {e}")
            raise
    
    def reload_strategy(self) -> bool:
        """
        Reload strategy from disk (for hot-reloading).
        
        Returns:
            True if reload successful, False otherwise
        """
        try:
            self._load_strategy()
            logger.info("Strategy reloaded successfully")
            return True
        except Exception as e:
            logger.error(f"Failed to reload strategy: {e}")
            return False

    def set_order_handlers(
        self,
        submit_buy: Optional[Any] = None,
        submit_sell: Optional[Any] = None,
        cancel_order: Optional[Any] = None,
    ) -> None:
        """Inject order submission/cancellation handlers (Nautilus integration)."""
        self._submit_buy_handler = submit_buy
        self._submit_sell_handler = submit_sell
        self._cancel_order_handler = cancel_order

    def _check_reload_signal(self) -> None:
        """Check for reload signal file and reload strategy if present."""
        try:
            if self._reload_signal_path.exists():
                logger.info("Reload signal detected - reloading strategy")
                try:
                    self._reload_signal_path.unlink()
                except OSError as e:
                    logger.warning(f"Could not remove reload signal file: {e}")
                self.reload_strategy()
        except Exception as e:
            logger.warning(f"Error checking reload signal: {e}")

    def _start_state_refresh_thread(self) -> None:
        if self._state_refresh_thread and self._state_refresh_thread.is_alive():
            return

        self._state_refresh_stop.clear()

        def _run() -> None:
            while not self._state_refresh_stop.wait(1.0):
                if not self._is_running:
                    continue
                elapsed = (datetime.now() - self._last_state_update_time).total_seconds()
                if elapsed >= self._state_update_interval_seconds:
                    self._update_bot_state()

        self._state_refresh_thread = threading.Thread(target=_run, daemon=True)
        self._state_refresh_thread.start()

    def _stop_state_refresh_thread(self) -> None:
        self._state_refresh_stop.set()
        if self._state_refresh_thread and self._state_refresh_thread.is_alive():
            self._state_refresh_thread.join(timeout=2.0)
    
    # =========================================================================
    # Nautilus Trader Strategy Lifecycle Methods
    # =========================================================================
    
    def on_start(self) -> None:
        """
        Called when the strategy is started.
        
        Subscribes to required bar data for all instruments and timeframes
        defined in the strategy rules.
        
        In Nautilus Trader, this would subscribe to bar types:
        
            self.subscribe_bars(bar_type)
        """
        logger.info(f"Strategy {self.strategy_id} starting...")
        self._is_running = True
        self._start_state_refresh_thread()
        
        # Initialize bar buffers for each instrument
        for instrument_id in self.config.instruments:
            self._bar_buffers[instrument_id] = BarBuffer()
            self._positions[instrument_id] = 0.0
            self._entry_prices[instrument_id] = 0.0
        
        # Get required data subscriptions from rule engine
        if self.rule_engine:
            subscriptions = self.rule_engine.get_required_data_subscriptions()
            logger.info(f"Required data subscriptions: {subscriptions}")
            
            # TODO: In Nautilus Trader, subscribe to each bar type:
            # for symbol, timeframe in subscriptions:
            #     bar_type = self._create_bar_type(symbol, timeframe)
            #     self.subscribe_bars(bar_type)
        
        # Update bot state
        self._update_bot_state(BotStatus.RUNNING)
        
        logger.info(f"Strategy {self.strategy_id} started with {len(self.config.instruments)} instruments")
    
    def on_stop(self) -> None:
        """
        Called when the strategy is stopped.
        
        Performs cleanup and logs final state.
        """
        logger.info(f"Strategy {self.strategy_id} stopping...")
        self._is_running = False
        
        # Log final positions
        for instrument_id, qty in self._positions.items():
            if qty != 0:
                logger.info(f"Final position {instrument_id}: {qty}")
        
        # Update bot state
        self._update_bot_state(BotStatus.STOPPED)

        self._stop_state_refresh_thread()
        
        # Clear stop signals
        clear_stop_signals()
        
        logger.info(f"Strategy {self.strategy_id} stopped")
    
    def on_bar(self, bar: Any) -> None:
        """
        Called when a new bar is received.
        
        This is the main event handler that:
        1. Checks for stop signals
        2. Updates the bar buffer for the instrument
        3. Evaluates global rules (filters)
        4. If global rules pass, evaluates per-ticker rules
        5. Generates orders based on signals
        6. Updates bot state periodically
        
        Args:
            bar: Nautilus Bar object (or dict for testing)
        """
        if not self._is_running:
            return

        # Hot-reload strategy if signaled
        self._check_reload_signal()
        
        # Check for stop signals
        if check_stop_signal():
            logger.info("Stop signal received - stopping strategy")
            self.on_stop()
            return
        
        # Check for emergency stop
        emergency = check_emergency_stop()
        if emergency:
            logger.warning("EMERGENCY STOP triggered - cancelling orders and flattening positions")
            self._handle_emergency_stop()
            return
        
        # Get instrument ID from bar
        if isinstance(bar, dict):
            instrument_id = bar.get("instrument_id", "UNKNOWN")
        else:
            instrument_id = str(bar.bar_type.instrument_id)
        
        # Update bar buffer
        symbol = instrument_id.split(".")[0]
        if instrument_id in self._bar_buffers:
            self._bar_buffers[instrument_id].append(bar)
        if symbol == "VIX":
            self._vix_buffer.append(bar)
        
        # Only evaluate rules if we have enough data
        if not self._has_sufficient_data():
            return
        
        current_time = datetime.now()
        self._last_evaluation_time = current_time
        
        # Build market data for rule evaluation
        market_data = self._build_market_data()
        vix_data = self._vix_buffer.to_dataframe() if len(self._vix_buffer) > 0 else market_data.get("VIX")
        
        # Evaluate global rules (filters)
        if self.rule_engine:
            global_pass = self.rule_engine.evaluate_global_rules(
                market_data=market_data,
                vix_bars=vix_data,
                current_time=current_time
            )
            
            if not global_pass:
                logger.debug("Global rules failed - skipping signal evaluation")
                # Still update state periodically
                self._maybe_update_state()
                return
            
            # Evaluate per-ticker rules
            for instrument_id in self.config.instruments:
                if instrument_id not in self._bar_buffers:
                    continue
                
                ticker_bars = self._bar_buffers[instrument_id].to_dataframe()
                if ticker_bars.empty:
                    continue
                
                # Extract ticker symbol (e.g., "SPY" from "SPY.ARCA")
                ticker = instrument_id.split(".")[0]
                
                actions = self.rule_engine.evaluate_ticker_rules(
                    ticker=ticker,
                    bars=ticker_bars,
                    vix_bars=vix_data,
                    current_time=current_time,
                    market_data=market_data,
                )
                
                # Process signals
                self._process_signals(instrument_id, actions)
        
        # Update bot state periodically
        self._maybe_update_state()
    
    def on_order_filled(self, event: Any) -> None:
        """
        Called when an order is filled.
        
        Updates position tracking and logs the fill.
        
        Args:
            event: Nautilus OrderFilled event (or dict for testing)
        """
        if isinstance(event, dict):
            instrument_id = event.get("instrument_id", "UNKNOWN")
            side = event.get("side", "BUY")
            quantity = event.get("quantity", 0)
            price = event.get("price", 0)
        else:
            instrument_id = str(event.instrument_id)
            side_value = event.order_side.value if hasattr(event.order_side, "value") else str(event.order_side)
            side = str(side_value).upper().replace("ORDER_SIDE.", "")
            quantity = float(event.last_qty)
            price = float(event.last_px)
        
        # Track entry price for new positions
        old_position = self._positions.get(instrument_id, 0)
        
        # Update position
        if side == "BUY":
            if old_position == 0:
                # New position - track entry price
                self._entry_prices[instrument_id] = price
            self._positions[instrument_id] = old_position + quantity
        else:
            # Calculate realized PnL for closed position
            if old_position > 0 and self._entry_prices.get(instrument_id, 0) > 0:
                entry_price = self._entry_prices[instrument_id]
                realized_pnl = (price - entry_price) * min(quantity, old_position)
                self._daily_pnl += realized_pnl
                self._total_pnl += realized_pnl
                self._trades_today += 1
                if realized_pnl > 0:
                    self._wins_today += 1
                logger.info(f"Trade closed: {instrument_id} PnL=${realized_pnl:+.2f}")
            
            self._positions[instrument_id] = old_position - quantity
            
            # Clear entry price if position is closed
            if self._positions[instrument_id] <= 0:
                self._entry_prices[instrument_id] = 0.0
        
        logger.info(
            f"Order filled: {side} {quantity} {instrument_id} @ ${price:.2f}. "
            f"Position: {self._positions.get(instrument_id, 0)}"
        )
        
        # Update state immediately after fill
        self._update_bot_state()
    
    def on_position_changed(self, event: Any) -> None:
        """
        Called when a position changes.
        
        Args:
            event: Nautilus PositionChanged event (or dict for testing)
        """
        if isinstance(event, dict):
            instrument_id = event.get("instrument_id", "UNKNOWN")
            quantity = event.get("quantity", 0)
        else:
            instrument_id = str(event.instrument_id)
            quantity = float(event.quantity)
        
        self._positions[instrument_id] = quantity
        logger.debug(f"Position changed: {instrument_id} = {quantity}")
    
    # =========================================================================
    # Signal Processing
    # =========================================================================
    
    def _process_signals(self, instrument_id: str, actions: List[str]) -> None:
        """
        Process trading signals for an instrument.
        
        Args:
            instrument_id: The instrument to trade
            actions: List of action strings ("BUY", "SELL")
        """
        if not actions:
            return
        
        current_position = self._positions.get(instrument_id, 0)
        
        for action in actions:
            action_upper = action.upper()
            
            if action_upper == "BUY" and current_position == 0:
                # Generate buy order
                quantity = self._calculate_position_size(instrument_id)
                self._submit_buy_order(instrument_id, quantity)
                logger.info(f"BUY signal: {instrument_id}, quantity={quantity}")
                
            elif action_upper == "SELL" and current_position > 0:
                # Generate sell order to close position
                self._submit_sell_order(instrument_id, current_position)
                logger.info(f"SELL signal: {instrument_id}, quantity={current_position}")
    
    def _calculate_position_size(self, instrument_id: str) -> float:
        """
        Calculate position size using equal allocation.
        
        Uses 1/N allocation where N = number of instruments.
        
        Args:
            instrument_id: The instrument to size
            
        Returns:
            Quantity to trade
        """
        if self.config.use_equal_allocation:
            # Equal allocation across instruments
            allocation_per_instrument = (
                self.config.max_position_per_instrument / len(self.config.instruments)
            )
        else:
            allocation_per_instrument = self.config.max_position_per_instrument
        
        # Get current price from buffer
        if instrument_id in self._bar_buffers and len(self._bar_buffers[instrument_id]) > 0:
            current_price = self._bar_buffers[instrument_id].closes[-1]
            if current_price > 0:
                return allocation_per_instrument / current_price
        
        return 0.0
    
    def _submit_buy_order(self, instrument_id: str, quantity: float) -> None:
        """
        Submit a market buy order.
        
        In Nautilus Trader, this would create and submit an order:
        
            order = self.order_factory.market(
                instrument_id=InstrumentId.from_str(instrument_id),
                order_side=OrderSide.BUY,
                quantity=Quantity.from_int(int(quantity)),
            )
            self.submit_order(order)
        
        Args:
            instrument_id: The instrument to buy
            quantity: Quantity to buy
        """
        if self._submit_buy_handler:
            order_id = self._submit_buy_handler(instrument_id, quantity)
            if order_id is not None:
                order = StateOrder(
                    order_id=str(order_id),
                    symbol=instrument_id.split(".")[0],
                    side="BUY",
                    quantity=float(quantity),
                    price=None,
                    status="SUBMITTED",
                    order_type="MARKET",
                    submitted_time=datetime.now().isoformat(),
                )
                self._pending_orders[order.order_id] = order
                self._update_bot_state()
            else:
                logger.warning("BUY order submission failed")
            return

        logger.info(f"SUBMIT BUY ORDER: {quantity} {instrument_id}")

        # Simulate the fill immediately (for testing)
        self.on_order_filled({
            "instrument_id": instrument_id,
            "side": "BUY",
            "quantity": quantity,
            "price": self._bar_buffers[instrument_id].closes[-1] if instrument_id in self._bar_buffers else 0
        })
    
    def _submit_sell_order(self, instrument_id: str, quantity: float) -> None:
        """
        Submit a market sell order.
        
        In Nautilus Trader, this would create and submit an order:
        
            order = self.order_factory.market(
                instrument_id=InstrumentId.from_str(instrument_id),
                order_side=OrderSide.SELL,
                quantity=Quantity.from_int(int(quantity)),
            )
            self.submit_order(order)
        
        Args:
            instrument_id: The instrument to sell
            quantity: Quantity to sell
        """
        if self._submit_sell_handler:
            order_id = self._submit_sell_handler(instrument_id, quantity)
            if order_id is not None:
                order = StateOrder(
                    order_id=str(order_id),
                    symbol=instrument_id.split(".")[0],
                    side="SELL",
                    quantity=float(quantity),
                    price=None,
                    status="SUBMITTED",
                    order_type="MARKET",
                    submitted_time=datetime.now().isoformat(),
                )
                self._pending_orders[order.order_id] = order
                self._update_bot_state()
            else:
                logger.warning("SELL order submission failed")
            return

        logger.info(f"SUBMIT SELL ORDER: {quantity} {instrument_id}")

        # Simulate the fill immediately (for testing)
        self.on_order_filled({
            "instrument_id": instrument_id,
            "side": "SELL",
            "quantity": quantity,
            "price": self._bar_buffers[instrument_id].closes[-1] if instrument_id in self._bar_buffers else 0
        })
    
    # =========================================================================
    # Helpers
    # =========================================================================
    
    def _has_sufficient_data(self) -> bool:
        """Check if we have enough bar data for indicator calculations."""
        for instrument_id in self.config.instruments:
            if instrument_id not in self._bar_buffers:
                return False
            if not self._bar_buffers[instrument_id].is_ready:
                return False
        return True
    
    def _build_market_data(self) -> Dict[str, pd.DataFrame]:
        """Build market data dict for rule evaluation."""
        market_data = {}
        for instrument_id, buffer in self._bar_buffers.items():
            df = buffer.to_dataframe()
            market_data[instrument_id] = df
            symbol = instrument_id.split(".")[0]
            if symbol and symbol not in market_data:
                market_data[symbol] = df
        if len(self._vix_buffer) > 0:
            vix_df = self._vix_buffer.to_dataframe()
            market_data.setdefault("VIX", vix_df)
            market_data.setdefault("VIX.CBOE", vix_df)
        return market_data
    
    def get_position(self, instrument_id: str) -> float:
        """Get current position for an instrument."""
        return self._positions.get(instrument_id, 0.0)
    
    def get_all_positions(self) -> Dict[str, float]:
        """Get all current positions."""
        return self._positions.copy()
    
    @property
    def is_running(self) -> bool:
        """Check if strategy is running."""
        return self._is_running
    
    # =========================================================================
    # State Management
    # =========================================================================
    
    def _maybe_update_state(self) -> None:
        """Update state if enough time has passed since last update."""
        self._state_update_counter += 1
        if self._state_update_counter >= self._state_update_interval:
            self._state_update_counter = 0
            self._update_bot_state()
            self._last_state_update_time = datetime.now()
    
    def _update_bot_state(self, status: Optional[BotStatus] = None) -> None:
        """
        Update the shared bot state for UI display.
        
        Args:
            status: Optional status override
        """
        try:
            def _coerce_float(value: Optional[str]) -> Optional[float]:
                if value is None:
                    return None
                try:
                    return float(value)
                except (TypeError, ValueError):
                    try:
                        return float(str(value).replace(",", ""))
                    except (TypeError, ValueError):
                        return None

            def _extract_price(snapshot: dict | None, fallback: float) -> float:
                if not snapshot:
                    return fallback
                for key in ("last", "close", "bid", "ask"):
                    value = snapshot.get(key)
                    if value is not None:
                        try:
                            return float(value)
                        except (TypeError, ValueError):
                            continue
                bid = snapshot.get("bid")
                ask = snapshot.get("ask")
                if bid is not None and ask is not None:
                    try:
                        return (float(bid) + float(ask)) / 2
                    except (TypeError, ValueError):
                        return fallback
                return fallback

            def _read_account_value(account: object, attr: str) -> Optional[float]:
                value = getattr(account, attr, None)
                if callable(value):
                    try:
                        return _coerce_float(value())
                    except Exception:
                        return None
                return _coerce_float(value)

            def _first_account_value(account: object, attrs: list[str]) -> Optional[float]:
                for attr in attrs:
                    value = _read_account_value(account, attr)
                    if value is not None:
                        return value
                return None

            positions = []
            equity_override: Optional[float] = None
            daily_pnl_override: Optional[float] = None
            total_pnl_override: Optional[float] = None
            tws_connected = self._tws_connected
            orders = list(self._pending_orders.values())
            use_nautilus_cache = False
            has_nautilus_strategy = self._nautilus_strategy is not None

            if (
                self._tws_provider
                and not has_nautilus_strategy
                and (self._tws_provider.is_connected() or self._tws_provider.connect(timeout=3.0))
            ):
                tws_connected = True
                tws_positions = self._tws_provider.get_positions(timeout=3.0)
                for pos in tws_positions:
                    qty = float(pos.get("position", 0) or 0)
                    if qty == 0:
                        continue
                    avg_cost = _coerce_float(pos.get("avg_cost")) or 0.0
                    symbol = (pos.get("symbol") or "").upper()
                    snapshot = self._tws_provider.get_market_data_snapshot(symbol, timeout=3.0)
                    current_price = _extract_price(snapshot, avg_cost)
                    unrealized = (current_price - avg_cost) * qty if avg_cost else 0.0
                    positions.append(
                        StatePosition(
                            symbol=symbol,
                            quantity=qty,
                            entry_price=avg_cost,
                            current_price=current_price,
                            unrealized_pnl=unrealized,
                            entry_time=None,
                        )
                    )

                tws_orders = self._tws_provider.get_open_orders(timeout=3.0)
                orders = [
                    StateOrder(
                        order_id=str(order.get("order_id")),
                        symbol=order.get("symbol") or "",
                        side=(order.get("action") or "").upper(),
                        quantity=float(order.get("quantity") or 0),
                        price=order.get("price"),
                        status=order.get("status") or "PENDING",
                        order_type=order.get("order_type") or "MARKET",
                        submitted_time=None,
                        filled_quantity=float(order.get("filled") or 0),
                    )
                    for order in tws_orders
                ]

                summary = self._tws_provider.get_account_summary(
                    tags="NetLiquidation,TotalCashValue,GrossPositionValue,UnrealizedPnL,RealizedPnL,DailyPnL"
                )

                def _find_tag(tag: str) -> Optional[float]:
                    for item in summary.values():
                        if item.get("tag") == tag:
                            return _coerce_float(item.get("value"))
                    return None

                net_liquidation = _find_tag("NetLiquidation")
                realized = _find_tag("RealizedPnL")
                unrealized = _find_tag("UnrealizedPnL")
                daily_pnl = _find_tag("DailyPnL")

                if net_liquidation is not None:
                    equity_override = net_liquidation
                if daily_pnl is not None:
                    daily_pnl_override = daily_pnl
                if realized is not None or unrealized is not None:
                    total_pnl_override = (realized or 0.0) + (unrealized or 0.0)
                if not positions and has_nautilus_strategy:
                    use_nautilus_cache = True
            elif has_nautilus_strategy:
                use_nautilus_cache = True

            if use_nautilus_cache:
                cache = self._nautilus_cache_override or self._nautilus_strategy.cache
                try:
                    open_position_ids = cache.position_open_ids()
                    logger.info(
                        "State refresh: Nautilus cache open positions=%d (strategy_id=%s)",
                        len(open_position_ids),
                        self.strategy_id,
                    )
                    try:
                        account_count = len(cache.accounts())
                        open_count = cache.positions_open_count()
                        logger.info(
                            "State refresh: Nautilus cache accounts=%d open_positions_count=%d",
                            account_count,
                            open_count,
                        )
                    except Exception:
                        pass
                except Exception as exc:
                    logger.info("State refresh: Nautilus cache lookup failed: %s", exc)
                positions = []
                cached_positions = cache.positions_open()
                if not cached_positions:
                    cached_positions = cache.positions()
                for pos in cached_positions:
                    qty = _coerce_float(pos.signed_qty) or 0.0
                    if qty == 0:
                        continue
                    instrument_id = str(pos.instrument_id)
                    symbol = instrument_id.split(".")[0]
                    entry_price = _coerce_float(pos.avg_px_open) or 0.0
                    fallback_price = entry_price
                    if instrument_id in self._bar_buffers and len(self._bar_buffers[instrument_id]) > 0:
                        fallback_price = self._bar_buffers[instrument_id].closes[-1]
                    current_price = fallback_price
                    try:
                        if cache.has_quote_ticks(pos.instrument_id):
                            quote = cache.quote_tick(pos.instrument_id)
                            bid = _coerce_float(quote.bid_price) if quote else None
                            ask = _coerce_float(quote.ask_price) if quote else None
                            if bid is not None and ask is not None:
                                current_price = (bid + ask) / 2
                            elif bid is not None:
                                current_price = bid
                            elif ask is not None:
                                current_price = ask
                        elif cache.has_trade_ticks(pos.instrument_id):
                            trade = cache.trade_tick(pos.instrument_id)
                            trade_price = _coerce_float(trade.price) if trade else None
                            if trade_price is not None:
                                current_price = trade_price
                    except Exception:
                        current_price = fallback_price

                    try:
                        unrealized = float(pos.unrealized_pnl(current_price))
                    except Exception:
                        unrealized = (current_price - entry_price) * qty if entry_price else 0.0

                    positions.append(
                        StatePosition(
                            symbol=symbol,
                            quantity=qty,
                            entry_price=entry_price,
                            current_price=current_price,
                            unrealized_pnl=unrealized,
                            entry_time=None,
                        )
                    )

                orders = []
                for order in cache.orders_open():
                    instrument_id = str(order.instrument_id)
                    orders.append(
                        StateOrder(
                            order_id=str(order.client_order_id),
                            symbol=instrument_id.split(".")[0],
                            side=(order.side_string or "").upper(),
                            quantity=_coerce_float(order.quantity) or 0.0,
                            price=None,
                            status=order.status_string or "PENDING",
                            order_type=order.type_string or "MARKET",
                            submitted_time=None,
                            filled_quantity=_coerce_float(order.filled_qty) or 0.0,
                        )
                    )

                if equity_override is None:
                    account = None
                    try:
                        if cached_positions:
                            venue_value = getattr(cached_positions[0].instrument_id, "venue", None)
                            if venue_value is not None:
                                account = cache.account_for_venue(venue_value)
                        if account is None:
                            accounts = cache.accounts()
                            if accounts:
                                account = accounts[0]
                        if account is not None:
                            equity_override = _first_account_value(
                                account,
                                [
                                    "balance_total",
                                    "equity",
                                    "net_liquidation",
                                    "equity_with_loan_value",
                                    "balance",
                                ],
                            )
                            if daily_pnl_override is None:
                                daily_pnl_override = _first_account_value(
                                    account,
                                    [
                                        "pnl_daily",
                                        "daily_pnl",
                                        "pnl_day",
                                    ],
                                )
                            if total_pnl_override is None:
                                total_pnl_override = _first_account_value(
                                    account,
                                    [
                                        "pnl_total",
                                        "total_pnl",
                                    ],
                                )
                                if total_pnl_override is None:
                                    realized = _first_account_value(
                                        account,
                                        [
                                            "pnl_realized",
                                            "realized_pnl",
                                        ],
                                    )
                                    unrealized = _first_account_value(
                                        account,
                                        [
                                            "pnl_unrealized",
                                            "unrealized_pnl",
                                        ],
                                    )
                                    if realized is not None or unrealized is not None:
                                        total_pnl_override = (realized or 0.0) + (unrealized or 0.0)
                    except Exception:
                        pass
            else:
                for instrument_id, qty in self._positions.items():
                    if qty != 0:
                        entry_price = self._entry_prices.get(instrument_id, 0)
                        current_price = 0.0
                        if instrument_id in self._bar_buffers and len(self._bar_buffers[instrument_id]) > 0:
                            current_price = self._bar_buffers[instrument_id].closes[-1]
                        
                        unrealized_pnl = (current_price - entry_price) * qty if entry_price > 0 else 0
                        
                        positions.append(StatePosition(
                            symbol=instrument_id.split(".")[0],
                            quantity=qty,
                            entry_price=entry_price,
                            current_price=current_price,
                            unrealized_pnl=unrealized_pnl,
                            entry_time=datetime.now().isoformat(),
                        ))
            
            unrealized_total = sum(p.unrealized_pnl for p in positions)
            current_equity = equity_override if equity_override is not None else (
                self._initial_equity + self._total_pnl + unrealized_total
            )
            
            # Calculate win rate
            win_rate = (self._wins_today / self._trades_today * 100) if self._trades_today > 0 else 0.0
            
            # Get recent logs
            recent_logs = get_log_buffer()
            
            # Build state
            daily_pnl = daily_pnl_override if daily_pnl_override is not None else self._daily_pnl
            total_pnl = total_pnl_override if total_pnl_override is not None else self._total_pnl

            state = BotState(
                status=status.value if status else (BotStatus.RUNNING.value if self._is_running else BotStatus.STOPPED.value),
                tws_connected=tws_connected,
                positions=positions,
                orders=orders,
                equity=current_equity,
                daily_pnl=daily_pnl,
                daily_pnl_percent=(daily_pnl / current_equity * 100) if current_equity else 0,
                total_pnl=total_pnl,
                recent_logs=recent_logs[-50:],
                active_strategy=self.strategy_model.name if self.strategy_model else "",
                trades_today=self._trades_today,
                win_rate_today=win_rate,
            )
            
            update_state(state)
            self._last_state_update_time = datetime.now()
            
        except Exception as e:
            logger.error(f"Failed to update bot state: {e}")
    
    def _handle_emergency_stop(self) -> None:
        """
        Handle emergency stop signal.
        
        Cancels all pending orders and flattens all positions.
        """
        logger.warning("Executing EMERGENCY STOP")
        
        # Cancel all pending orders
        for order_id in list(self._pending_orders.keys()):
            logger.info(f"Cancelling order: {order_id}")
            if self._cancel_order_handler:
                try:
                    self._cancel_order_handler(order_id)
                except Exception as e:
                    logger.warning(f"Failed to cancel order {order_id}: {e}")
            del self._pending_orders[order_id]
        
        # Flatten all positions
        for instrument_id, qty in list(self._positions.items()):
            if qty > 0:
                logger.info(f"Flattening position: {instrument_id} qty={qty}")
                self._submit_sell_order(instrument_id, qty)
            elif qty < 0:
                logger.info(f"Covering short: {instrument_id} qty={abs(qty)}")
                self._submit_buy_order(instrument_id, abs(qty))
        
        # Update state
        self._update_bot_state(BotStatus.STOPPED)
        
        # Stop the strategy
        self._is_running = False
        
        # Clear the emergency stop signal
        clear_stop_signals()
        
        logger.warning("EMERGENCY STOP complete")
    
    def set_tws_connected(self, connected: bool) -> None:
        """Set TWS connection status."""
        self._tws_connected = connected
        if connected:
            logger.info("TWS connected")
        else:
            logger.warning("TWS disconnected")
        self._update_bot_state()

    def set_tws_provider(self, provider: Optional[TWSDataProvider]) -> None:
        """Attach TWS provider for state updates."""
        self._tws_provider = provider

    def set_nautilus_strategy(self, strategy: Optional[Any]) -> None:
        """Attach the Nautilus strategy for cache-backed state updates."""
        self._nautilus_strategy = strategy

    def set_nautilus_cache_override(self, cache: Optional[Any]) -> None:
        """Override the Nautilus cache source (e.g., node-level cache)."""
        self._nautilus_cache_override = cache


# =============================================================================
# Nautilus Trader Strategy Implementation
# =============================================================================

if NAUTILUS_AVAILABLE:
    import msgspec
    
    class NautilusDynamicRuleStrategyConfig(StrategyConfig):
        """
        Nautilus Trader configuration class for DynamicRuleStrategy.
        
        This extends StrategyConfig to integrate with Nautilus's configuration system.
        """
        strategy_config_path: str = "config/active_strategy.json"
        instruments: List[str] = msgspec.field(default_factory=lambda: ["SPY.ARCA"])
        max_position_per_instrument: Decimal = Decimal("10000")
        use_equal_allocation: bool = True
    
    
    class NautilusDynamicRuleStrategy(Strategy):
        """
        Nautilus Trader implementation of the dynamic rule strategy.
        
        This class extends Nautilus's Strategy base class and uses the
        DynamicRuleStrategy logic for rule evaluation.
        
        Usage:
            config = NautilusDynamicRuleStrategyConfig(
                strategy_config_path="src/config/active_strategy.json",
                instruments=["SPY.ARCA", "QQQ.NASDAQ"],
            )
            strategy = NautilusDynamicRuleStrategy(config)
            node.trader.add_strategy(strategy)
        """
        
        def __init__(self, config: NautilusDynamicRuleStrategyConfig):
            """Initialize the Nautilus strategy."""
            super().__init__(config)
            
            # Create internal strategy with our config
            internal_config = DynamicRuleStrategyConfig(
                strategy_id=str(config.id),
                strategy_config_path=config.strategy_config_path,
                instruments=config.instruments,
                max_position_per_instrument=float(config.max_position_per_instrument),
                use_equal_allocation=config.use_equal_allocation,
            )
            self._strategy = DynamicRuleStrategy(internal_config)
            self._strategy.set_order_handlers(
                submit_buy=self._submit_buy,
                submit_sell=self._submit_sell,
                cancel_order=self._cancel_order,
            )
            self._strategy.set_nautilus_strategy(self)
        
        def on_start(self) -> None:
            """Handle strategy start."""
            self._strategy.on_start()
            
            # Subscribe to bar data for required symbols/timeframes
            subscriptions = []
            if self._strategy.rule_engine:
                subscriptions = self._strategy.rule_engine.get_required_data_subscriptions()

            if not subscriptions:
                subscriptions = [(instrument_id, TimeframeUnit.M5) for instrument_id in self._strategy.config.instruments]

            default_venue = "ARCA"
            if self._strategy.config.instruments:
                default_venue = self._strategy.config.instruments[0].split(".")[-1]

            for symbol, timeframe in subscriptions:
                instrument_str = symbol
                if "." not in symbol:
                    instrument_str = f"{symbol}.{default_venue}"

                instrument_id = InstrumentId.from_str(instrument_str)

                agg_unit, step = TIMEFRAME_TO_AGGREGATION.get(timeframe, ("MINUTE", 5))
                aggregation = BarAggregation.MINUTE if agg_unit == "MINUTE" else BarAggregation.HOUR if agg_unit == "HOUR" else BarAggregation.DAY

                bar_type = BarType(
                    instrument_id=instrument_id,
                    bar_spec=BarSpecification(
                        step=step,
                        aggregation=aggregation,
                    ),
                )
                self.subscribe_bars(bar_type)
                self.log.info(f"Subscribed to {bar_type}")
        
        def on_stop(self) -> None:
            """Handle strategy stop."""
            self._strategy.on_stop()
        
        def on_bar(self, bar: Bar) -> None:
            """Handle new bar data."""
            self._strategy.on_bar(bar)
        
        def on_order_filled(self, event: OrderFilled) -> None:
            """Handle order filled event."""
            self._strategy.on_order_filled(event)

        def set_tws_provider(self, provider: Optional[TWSDataProvider]) -> None:
            """Attach TWS provider for state updates."""
            self._strategy.set_tws_provider(provider)

        def refresh_state(self) -> None:
            """Force a state refresh using the internal strategy cache."""
            self._strategy._update_bot_state(BotStatus.RUNNING)

        def set_cache_override(self, cache: Optional[Any]) -> None:
            """Override the cache used for state refresh (node cache preferred)."""
            self._strategy.set_nautilus_cache_override(cache)

        def _submit_buy(self, instrument_id: str, quantity: float) -> None:
            self._submit_market_order(instrument_id, OrderSide.BUY, quantity)

        def _submit_sell(self, instrument_id: str, quantity: float) -> None:
            self._submit_market_order(instrument_id, OrderSide.SELL, quantity)

        def _submit_market_order(self, instrument_id: str, side: OrderSide, quantity: float) -> None:
            qty_int = max(1, int(quantity))
            instrument = InstrumentId.from_str(instrument_id)
            order = self.order_factory.market(
                instrument_id=instrument,
                order_side=side,
                quantity=Quantity.from_int(qty_int),
                time_in_force=TimeInForce.DAY,
            )
            self.submit_order(order)
            self.log.info(f"Submitted {side} order: {qty_int} {instrument_id}")

        def _cancel_order(self, order_id: str) -> None:
            try:
                self.cancel_order(order_id)
            except Exception as e:
                self.log.warning(f"Cancel order failed for {order_id}: {e}")
        
        def on_position_changed(self, event: PositionChanged) -> None:
            """Handle position change event."""
            self._strategy.on_position_changed(event)
