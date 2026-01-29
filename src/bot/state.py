"""
Bot State Management Module

Provides shared state mechanism for communication between the trading bot
and the UI. Supports both JSON file-based state (legacy) and
DuckDB database backend for persistence.

The bot writes state updates after each cycle, and the UI reads
the state to display real-time information.
"""

import json
import logging
import fcntl
import os
from dataclasses import dataclass, field, asdict
from datetime import datetime
from enum import Enum
from pathlib import Path
from typing import List, Optional, Dict, Any


logger = logging.getLogger(__name__)


# Default state file path
DEFAULT_STATE_FILE = Path(__file__).parent.parent.parent / "config" / ".bot_state.json"
COMMAND_FILE = Path(__file__).parent.parent.parent / "config" / ".bot_command"
STOP_SIGNAL_FILE = Path(__file__).parent.parent.parent / "config" / ".stop_signal"
EMERGENCY_STOP_FILE = Path(__file__).parent.parent.parent / "config" / ".emergency_stop"


class BotStatus(str, Enum):
    """Bot status enumeration."""
    STOPPED = "STOPPED"
    RUNNING = "RUNNING"
    STARTING = "STARTING"
    STOPPING = "STOPPING"
    ERROR = "ERROR"
    DISCONNECTED = "DISCONNECTED"


class OrderSide(str, Enum):
    """Order side enumeration."""
    BUY = "BUY"
    SELL = "SELL"


class OrderStatus(str, Enum):
    """Order status enumeration."""
    PENDING = "PENDING"
    SUBMITTED = "SUBMITTED"
    FILLED = "FILLED"
    PARTIALLY_FILLED = "PARTIALLY_FILLED"
    CANCELLED = "CANCELLED"
    REJECTED = "REJECTED"


@dataclass
class Position:
    """Represents an open position."""
    symbol: str
    quantity: float
    entry_price: float
    current_price: float
    unrealized_pnl: float
    entry_time: Optional[str] = None
    
    @property
    def pnl_percent(self) -> float:
        """Calculate PnL as percentage."""
        if self.entry_price == 0:
            return 0.0
        return ((self.current_price - self.entry_price) / self.entry_price) * 100
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary."""
        return {
            "symbol": self.symbol,
            "quantity": self.quantity,
            "entry_price": self.entry_price,
            "current_price": self.current_price,
            "unrealized_pnl": self.unrealized_pnl,
            "entry_time": self.entry_time,
        }
    
    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Position":
        """Create from dictionary."""
        return cls(
            symbol=data.get("symbol", ""),
            quantity=data.get("quantity", 0.0),
            entry_price=data.get("entry_price", 0.0),
            current_price=data.get("current_price", 0.0),
            unrealized_pnl=data.get("unrealized_pnl", 0.0),
            entry_time=data.get("entry_time"),
        )


@dataclass
class Order:
    """Represents an order."""
    order_id: str
    symbol: str
    side: str  # BUY or SELL
    quantity: float
    price: Optional[float]  # None for market orders
    status: str
    order_type: str = "MARKET"  # MARKET, LIMIT, STOP
    submitted_time: Optional[str] = None
    filled_quantity: float = 0.0
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary."""
        return {
            "order_id": self.order_id,
            "symbol": self.symbol,
            "side": self.side,
            "quantity": self.quantity,
            "price": self.price,
            "status": self.status,
            "order_type": self.order_type,
            "submitted_time": self.submitted_time,
            "filled_quantity": self.filled_quantity,
        }
    
    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Order":
        """Create from dictionary."""
        return cls(
            order_id=data.get("order_id", ""),
            symbol=data.get("symbol", ""),
            side=data.get("side", "BUY"),
            quantity=data.get("quantity", 0.0),
            price=data.get("price"),
            status=data.get("status", "PENDING"),
            order_type=data.get("order_type", "MARKET"),
            submitted_time=data.get("submitted_time"),
            filled_quantity=data.get("filled_quantity", 0.0),
        )


@dataclass
class BotState:
    """
    Complete bot state for UI communication.
    
    This class holds all information that the UI needs to display
    the current state of the trading bot.
    """
    status: str = BotStatus.STOPPED.value
    tws_connected: bool = False
    positions: List[Position] = field(default_factory=list)
    orders: List[Order] = field(default_factory=list)
    equity: float = 0.0
    daily_pnl: float = 0.0
    daily_pnl_percent: float = 0.0
    total_pnl: float = 0.0
    last_update: Optional[str] = None
    recent_logs: List[str] = field(default_factory=list)
    active_strategy: str = ""
    error_message: str = ""
    
    # Additional metrics
    open_positions_count: int = 0
    pending_orders_count: int = 0
    trades_today: int = 0
    win_rate_today: float = 0.0
    
    def __post_init__(self):
        """Update computed fields."""
        self.open_positions_count = len(self.positions)
        self.pending_orders_count = len([o for o in self.orders if o.status in ("PENDING", "SUBMITTED")])
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for JSON serialization."""
        return {
            "status": self.status,
            "tws_connected": self.tws_connected,
            "positions": [p.to_dict() if isinstance(p, Position) else p for p in self.positions],
            "orders": [o.to_dict() if isinstance(o, Order) else o for o in self.orders],
            "equity": self.equity,
            "daily_pnl": self.daily_pnl,
            "daily_pnl_percent": self.daily_pnl_percent,
            "total_pnl": self.total_pnl,
            "last_update": self.last_update,
            "recent_logs": self.recent_logs[-50:],  # Keep only last 50 logs
            "active_strategy": self.active_strategy,
            "error_message": self.error_message,
            "open_positions_count": self.open_positions_count,
            "pending_orders_count": self.pending_orders_count,
            "trades_today": self.trades_today,
            "win_rate_today": self.win_rate_today,
        }
    
    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "BotState":
        """Create from dictionary."""
        positions = [
            Position.from_dict(p) if isinstance(p, dict) else p 
            for p in data.get("positions", [])
        ]
        orders = [
            Order.from_dict(o) if isinstance(o, dict) else o 
            for o in data.get("orders", [])
        ]
        
        return cls(
            status=data.get("status", BotStatus.STOPPED.value),
            tws_connected=data.get("tws_connected", False),
            positions=positions,
            orders=orders,
            equity=data.get("equity", 0.0),
            daily_pnl=data.get("daily_pnl", 0.0),
            daily_pnl_percent=data.get("daily_pnl_percent", 0.0),
            total_pnl=data.get("total_pnl", 0.0),
            last_update=data.get("last_update"),
            recent_logs=data.get("recent_logs", []),
            active_strategy=data.get("active_strategy", ""),
            error_message=data.get("error_message", ""),
            trades_today=data.get("trades_today", 0),
            win_rate_today=data.get("win_rate_today", 0.0),
        )
    
    def add_log(self, message: str, level: str = "INFO") -> None:
        """Add a log entry."""
        timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        log_entry = f"[{timestamp}] [{level}] {message}"
        self.recent_logs.append(log_entry)
        # Keep only last 50 entries
        if len(self.recent_logs) > 50:
            self.recent_logs = self.recent_logs[-50:]


# =============================================================================
# Database-backed State Functions
# =============================================================================

def _use_database() -> bool:
    """Check if database backend should be used for state."""
    try:
        from src.config.settings import get_settings
        settings = get_settings()
        return settings.database.enabled and settings.database.use_db_for_state
    except Exception:
        return False


def _get_database():
    """Get the database manager instance."""
    from src.config.database import get_database
    from src.config.settings import get_settings
    settings = get_settings()
    
    db_path = Path(settings.database.path)
    if not db_path.is_absolute():
        db_path = Path(__file__).parent.parent.parent / db_path
    
    return get_database(db_path)


def update_state_db(state: BotState) -> bool:
    """
    Update bot state in DuckDB database.
    
    Args:
        state: The current bot state
        
    Returns:
        True if update successful, False otherwise
    """
    try:
        db = _get_database()
        
        # Update main bot state
        db.update_bot_state(
            status=state.status,
            tws_connected=state.tws_connected,
            equity=state.equity,
            daily_pnl=state.daily_pnl,
            daily_pnl_percent=state.daily_pnl_percent,
            total_pnl=state.total_pnl,
            active_strategy=state.active_strategy,
            error_message=state.error_message,
            trades_today=state.trades_today,
            win_rate_today=state.win_rate_today,
        )
        
        # Sync positions - clear and re-add
        db.clear_positions()
        for pos in state.positions:
            if isinstance(pos, Position):
                entry_time = datetime.fromisoformat(pos.entry_time) if pos.entry_time else None
                db.add_position(
                    symbol=pos.symbol,
                    quantity=pos.quantity,
                    entry_price=pos.entry_price,
                    current_price=pos.current_price,
                    unrealized_pnl=pos.unrealized_pnl,
                    entry_time=entry_time,
                )
        
        # Sync orders - clear and re-add
        db.clear_orders()
        for order in state.orders:
            if isinstance(order, Order):
                submitted_time = datetime.fromisoformat(order.submitted_time) if order.submitted_time else None
                db.add_order(
                    order_id=order.order_id,
                    symbol=order.symbol,
                    side=order.side,
                    quantity=order.quantity,
                    price=order.price,
                    order_type=order.order_type,
                    status=order.status,
                    submitted_time=submitted_time,
                )
                if order.filled_quantity > 0:
                    db.update_order(order.order_id, filled_quantity=order.filled_quantity)
        
        # Add recent logs
        for log_entry in state.recent_logs[-10:]:  # Only add recent ones to avoid duplicates
            # Parse log entry format: "[timestamp] [level] message"
            try:
                parts = log_entry.split("] ", 2)
                if len(parts) >= 2:
                    level = parts[1].strip("[")
                    message = parts[2] if len(parts) > 2 else ""
                    db.add_log(level, message)
            except Exception:
                db.add_log("INFO", log_entry)
        
        return True
        
    except Exception as e:
        logger.error(f"Failed to update state in database: {e}")
        return False


def read_state_db() -> BotState:
    """
    Read bot state from DuckDB database.
    
    Returns:
        Current bot state (or default state if database unavailable)
    """
    try:
        db = _get_database()
        
        # Get main state
        state_dict = db.get_bot_state()
        
        # Get positions
        positions_data = db.get_positions()
        positions = [
            Position(
                symbol=p["symbol"],
                quantity=p["quantity"],
                entry_price=p["entry_price"],
                current_price=p["current_price"],
                unrealized_pnl=p["unrealized_pnl"],
                entry_time=p["entry_time"],
            )
            for p in positions_data
        ]
        
        # Get orders
        orders_data = db.get_orders()
        orders = [
            Order(
                order_id=o["order_id"],
                symbol=o["symbol"],
                side=o["side"],
                quantity=o["quantity"],
                price=o["price"],
                status=o["status"],
                order_type=o["order_type"],
                submitted_time=o["submitted_time"],
                filled_quantity=o["filled_quantity"],
            )
            for o in orders_data
        ]
        
        # Get recent logs
        logs_data = db.get_recent_logs(50)
        recent_logs = [
            f"[{log['timestamp']}] [{log['level']}] {log['message']}"
            for log in logs_data
        ]
        
        return BotState(
            status=state_dict.get("status", BotStatus.STOPPED.value),
            tws_connected=state_dict.get("tws_connected", False),
            positions=positions,
            orders=orders,
            equity=state_dict.get("equity", 0.0),
            daily_pnl=state_dict.get("daily_pnl", 0.0),
            daily_pnl_percent=state_dict.get("daily_pnl_percent", 0.0),
            total_pnl=state_dict.get("total_pnl", 0.0),
            last_update=state_dict.get("last_update"),
            recent_logs=recent_logs,
            active_strategy=state_dict.get("active_strategy", ""),
            error_message=state_dict.get("error_message", ""),
            trades_today=state_dict.get("trades_today", 0),
            win_rate_today=state_dict.get("win_rate_today", 0.0),
        )
        
    except Exception as e:
        logger.error(f"Failed to read state from database: {e}")
        return BotState()


def update_state(state: BotState, state_file: Path = DEFAULT_STATE_FILE) -> bool:
    """
    Update the bot state (called by bot).
    
    Uses DuckDB database if enabled, otherwise falls back to JSON file
    with file locking to prevent race conditions.
    
    Args:
        state: The current bot state
        state_file: Path to the state file (used for JSON fallback)
        
    Returns:
        True if update successful, False otherwise
    """
    # Try database first if enabled
    if _use_database():
        if update_state_db(state):
            return True
        logger.warning("Database update failed, falling back to JSON file")
    
    # JSON file fallback
    try:
        # Ensure directory exists
        state_file.parent.mkdir(parents=True, exist_ok=True)
        
        # Update timestamp
        state.last_update = datetime.now().isoformat()
        
        # Write with file locking
        with open(state_file, 'w') as f:
            fcntl.flock(f.fileno(), fcntl.LOCK_EX)
            try:
                json.dump(state.to_dict(), f, indent=2)
            finally:
                fcntl.flock(f.fileno(), fcntl.LOCK_UN)
        
        return True
        
    except Exception as e:
        logger.error(f"Failed to update state: {e}")
        return False


def read_state(state_file: Path = DEFAULT_STATE_FILE) -> BotState:
    """
    Read the bot state (called by UI).
    
    Uses DuckDB database if enabled, otherwise falls back to JSON file
    with file locking to prevent race conditions.
    
    Args:
        state_file: Path to the state file (used for JSON fallback)
        
    Returns:
        Current bot state (or default state if file doesn't exist)
    """
    # Try database first if enabled
    if _use_database():
        try:
            return read_state_db()
        except Exception as e:
            logger.warning(f"Database read failed, falling back to JSON file: {e}")
    
    # JSON file fallback
    try:
        if not state_file.exists():
            return BotState()
        
        with open(state_file, 'r') as f:
            fcntl.flock(f.fileno(), fcntl.LOCK_SH)
            try:
                data = json.load(f)
            finally:
                fcntl.flock(f.fileno(), fcntl.LOCK_UN)
        
        return BotState.from_dict(data)
        
    except json.JSONDecodeError as e:
        logger.warning(f"Invalid state file, returning default: {e}")
        return BotState()
    except Exception as e:
        logger.error(f"Failed to read state: {e}")
        return BotState()


def clear_state(state_file: Path = DEFAULT_STATE_FILE) -> bool:
    """
    Clear the state (reset to default).
    
    Clears both database and JSON file state.
    
    Args:
        state_file: Path to the state file
        
    Returns:
        True if successful
    """
    success = True
    
    # Clear database state if enabled
    if _use_database():
        try:
            db = _get_database()
            db.reset_bot_state()
            db.clear_positions()
            db.clear_orders()
            db.clear_logs()
        except Exception as e:
            logger.error(f"Failed to clear database state: {e}")
            success = False
    
    # Clear JSON file
    try:
        if state_file.exists():
            state_file.unlink()
        return success
    except Exception as e:
        logger.error(f"Failed to clear state: {e}")
        return False


# =============================================================================
# Bot Control Signals (Database-backed when enabled)
# =============================================================================

def write_start_command() -> bool:
    """
    Write start command for the bot.
    
    Returns:
        True if successful
    """
    # Use database if enabled
    if _use_database():
        try:
            db = _get_database()
            db.add_command("START")
            return True
        except Exception as e:
            logger.warning(f"Failed to write start command to database: {e}")
    
    # JSON file fallback
    try:
        COMMAND_FILE.parent.mkdir(parents=True, exist_ok=True)
        with open(COMMAND_FILE, 'w') as f:
            json.dump({"command": "START", "timestamp": datetime.now().isoformat()}, f)
        return True
    except Exception as e:
        logger.error(f"Failed to write start command: {e}")
        return False


def write_stop_signal() -> bool:
    """
    Write stop signal for the bot.
    
    Returns:
        True if successful
    """
    # Use database if enabled
    if _use_database():
        try:
            db = _get_database()
            db.add_command("STOP")
            return True
        except Exception as e:
            logger.warning(f"Failed to write stop signal to database: {e}")
    
    # File-based fallback
    try:
        STOP_SIGNAL_FILE.parent.mkdir(parents=True, exist_ok=True)
        STOP_SIGNAL_FILE.touch()
        return True
    except Exception as e:
        logger.error(f"Failed to write stop signal: {e}")
        return False


def write_emergency_stop() -> bool:
    """
    Write emergency stop signal (cancel all orders + flatten positions).
    
    Returns:
        True if successful
    """
    # Use database if enabled
    if _use_database():
        try:
            db = _get_database()
            db.add_command("EMERGENCY_STOP", {
                "cancel_orders": True,
                "flatten_positions": True,
            })
            return True
        except Exception as e:
            logger.warning(f"Failed to write emergency stop to database: {e}")
    
    # File-based fallback
    try:
        EMERGENCY_STOP_FILE.parent.mkdir(parents=True, exist_ok=True)
        with open(EMERGENCY_STOP_FILE, 'w') as f:
            json.dump({
                "command": "EMERGENCY_STOP",
                "timestamp": datetime.now().isoformat(),
                "cancel_orders": True,
                "flatten_positions": True,
            }, f)
        return True
    except Exception as e:
        logger.error(f"Failed to write emergency stop: {e}")
        return False


def check_stop_signal() -> bool:
    """
    Check if stop signal exists (called by bot).
    
    Returns:
        True if stop signal is present
    """
    # Check database first if enabled
    if _use_database():
        try:
            db = _get_database()
            commands = db.get_pending_commands()
            for cmd in commands:
                if cmd["command"] == "STOP":
                    return True
        except Exception:
            pass
    
    return STOP_SIGNAL_FILE.exists()


def check_emergency_stop() -> Optional[Dict[str, Any]]:
    """
    Check if emergency stop signal exists (called by bot).
    
    Returns:
        Emergency stop data if present, None otherwise
    """
    # Check database first if enabled
    if _use_database():
        try:
            db = _get_database()
            commands = db.get_pending_commands()
            for cmd in commands:
                if cmd["command"] == "EMERGENCY_STOP":
                    return cmd.get("payload", {
                        "cancel_orders": True,
                        "flatten_positions": True,
                    })
        except Exception:
            pass
    
    # File-based fallback
    try:
        if EMERGENCY_STOP_FILE.exists():
            with open(EMERGENCY_STOP_FILE, 'r') as f:
                return json.load(f)
    except Exception:
        pass
    return None


def clear_stop_signals() -> None:
    """Clear all stop signal files and database commands."""
    # Clear database commands if enabled
    if _use_database():
        try:
            db = _get_database()
            # Mark all pending commands as processed
            for cmd in db.get_pending_commands():
                db.mark_command_processed(cmd["id"])
        except Exception as e:
            logger.warning(f"Failed to clear database commands: {e}")
    
    # Clear files
    try:
        if STOP_SIGNAL_FILE.exists():
            STOP_SIGNAL_FILE.unlink()
        if EMERGENCY_STOP_FILE.exists():
            EMERGENCY_STOP_FILE.unlink()
        if COMMAND_FILE.exists():
            COMMAND_FILE.unlink()
    except Exception as e:
        logger.warning(f"Failed to clear stop signals: {e}")


def get_state_file_age() -> Optional[float]:
    """
    Get the age of the state file in seconds.
    
    Returns:
        Age in seconds, or None if file doesn't exist
    """
    try:
        if DEFAULT_STATE_FILE.exists():
            mtime = DEFAULT_STATE_FILE.stat().st_mtime
            return (datetime.now().timestamp() - mtime)
    except Exception:
        pass
    return None


class StateLogger(logging.Handler):
    """
    Custom logging handler that writes logs to the bot state.
    
    This allows the UI to display recent log messages.
    """
    
    def __init__(self, max_logs: int = 50):
        super().__init__()
        self.max_logs = max_logs
        self._logs: List[str] = []
    
    def emit(self, record: logging.LogRecord) -> None:
        """Emit a log record."""
        try:
            msg = self.format(record)
            self._logs.append(msg)
            if len(self._logs) > self.max_logs:
                self._logs = self._logs[-self.max_logs:]
        except Exception:
            self.handleError(record)
    
    def get_logs(self) -> List[str]:
        """Get recent logs."""
        return self._logs.copy()
    
    def clear_logs(self) -> None:
        """Clear logs."""
        self._logs = []


# Global state logger instance
_state_logger: Optional[StateLogger] = None


def get_state_logger() -> StateLogger:
    """Get or create the state logger."""
    global _state_logger
    if _state_logger is None:
        _state_logger = StateLogger()
        _state_logger.setFormatter(
            logging.Formatter("[%(asctime)s] [%(levelname)s] %(message)s", 
                            datefmt="%Y-%m-%d %H:%M:%S")
        )
        # Add to trading_bot logger
        trading_logger = logging.getLogger("trading_bot")
        trading_logger.addHandler(_state_logger)
    return _state_logger
