"""
DuckDB Database Backend for Configuration and State Management.

Provides a persistent database layer for storing configuration settings,
bot state, positions, orders, and trading history using DuckDB.
"""

import json
import logging
from contextlib import contextmanager
from dataclasses import asdict
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, Generator, List, Optional

import duckdb

try:
    import psycopg
except Exception:  # pragma: no cover - optional dependency
    psycopg = None

logger = logging.getLogger(__name__)

# Default database path
DEFAULT_DB_PATH = Path(__file__).parent.parent.parent / "data" / "traderbot.duckdb"


class DatabaseManager:
    """
    Manages DuckDB database connections and operations.
    
    Provides a centralized interface for storing and retrieving
    configuration, state, and trading data.
    """
    
    def __init__(self, db_path: Optional[Path] = None):
        """
        Initialize the database manager.
        
        Args:
            db_path: Path to the DuckDB database file. Defaults to data/traderbot.duckdb
        """
        self.db_path = db_path or DEFAULT_DB_PATH
        self.db_path.parent.mkdir(parents=True, exist_ok=True)
        self._connection: Optional[duckdb.DuckDBPyConnection] = None
        self._initialize_database()
    
    @contextmanager
    def get_connection(self) -> Generator[duckdb.DuckDBPyConnection, None, None]:
        """
        Get a database connection context manager.
        
        Yields:
            Active database connection
        """
        conn = duckdb.connect(str(self.db_path))
        try:
            yield conn
        finally:
            conn.close()
    
    def _initialize_database(self) -> None:
        """Initialize database schema if not exists."""
        with self.get_connection() as conn:
            # Configuration table - stores key-value pairs with JSON values
            conn.execute("""
                CREATE TABLE IF NOT EXISTS config (
                    section VARCHAR NOT NULL,
                    key VARCHAR NOT NULL,
                    value JSON NOT NULL,
                    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                    PRIMARY KEY (section, key)
                )
            """)
            
            # Bot state table - current state snapshot
            conn.execute("""
                CREATE TABLE IF NOT EXISTS bot_state (
                    id INTEGER PRIMARY KEY DEFAULT 1,
                    status VARCHAR DEFAULT 'STOPPED',
                    tws_connected BOOLEAN DEFAULT FALSE,
                    equity DOUBLE DEFAULT 0.0,
                    daily_pnl DOUBLE DEFAULT 0.0,
                    daily_pnl_percent DOUBLE DEFAULT 0.0,
                    total_pnl DOUBLE DEFAULT 0.0,
                    active_strategy VARCHAR DEFAULT '',
                    error_message VARCHAR DEFAULT '',
                    trades_today INTEGER DEFAULT 0,
                    win_rate_today DOUBLE DEFAULT 0.0,
                    last_update TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                    CHECK (id = 1)
                )
            """)
            
            # Positions table
            conn.execute("""
                CREATE TABLE IF NOT EXISTS positions (
                    id INTEGER PRIMARY KEY,
                    symbol VARCHAR NOT NULL,
                    quantity DOUBLE NOT NULL,
                    entry_price DOUBLE NOT NULL,
                    current_price DOUBLE DEFAULT 0.0,
                    unrealized_pnl DOUBLE DEFAULT 0.0,
                    entry_time TIMESTAMP,
                    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                )
            """)
            
            # Create sequence for positions if not exists
            conn.execute("""
                CREATE SEQUENCE IF NOT EXISTS positions_seq START 1
            """)
            
            # Orders table
            conn.execute("""
                CREATE TABLE IF NOT EXISTS orders (
                    order_id VARCHAR PRIMARY KEY,
                    symbol VARCHAR NOT NULL,
                    side VARCHAR NOT NULL,
                    quantity DOUBLE NOT NULL,
                    price DOUBLE,
                    status VARCHAR DEFAULT 'PENDING',
                    order_type VARCHAR DEFAULT 'MARKET',
                    submitted_time TIMESTAMP,
                    filled_quantity DOUBLE DEFAULT 0.0,
                    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                )
            """)
            
            # Recent logs table
            conn.execute("""
                CREATE TABLE IF NOT EXISTS logs (
                    id INTEGER PRIMARY KEY,
                    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                    level VARCHAR NOT NULL,
                    message TEXT NOT NULL
                )
            """)
            
            # Create sequence for logs if not exists
            conn.execute("""
                CREATE SEQUENCE IF NOT EXISTS logs_seq START 1
            """)
            
            # Bot commands table
            conn.execute("""
                CREATE TABLE IF NOT EXISTS bot_commands (
                    id INTEGER PRIMARY KEY,
                    command VARCHAR NOT NULL,
                    payload JSON,
                    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                    processed BOOLEAN DEFAULT FALSE
                )
            """)
            
            # Create sequence for commands if not exists
            conn.execute("""
                CREATE SEQUENCE IF NOT EXISTS commands_seq START 1
            """)
            
            # Trading history table
            conn.execute("""
                CREATE TABLE IF NOT EXISTS trade_history (
                    id INTEGER PRIMARY KEY,
                    symbol VARCHAR NOT NULL,
                    side VARCHAR NOT NULL,
                    quantity DOUBLE NOT NULL,
                    price DOUBLE NOT NULL,
                    pnl DOUBLE DEFAULT 0.0,
                    strategy VARCHAR,
                    executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                )
            """)
            
            # Create sequence for trade history if not exists
            conn.execute("""
                CREATE SEQUENCE IF NOT EXISTS trade_history_seq START 1
            """)
            
            # Initialize bot state if not exists
            conn.execute("""
                INSERT INTO bot_state (id) 
                SELECT 1 WHERE NOT EXISTS (SELECT 1 FROM bot_state WHERE id = 1)
            """)
            
            logger.info(f"Database initialized at {self.db_path}")


class PostgresDatabaseManager:
    """PostgreSQL database backend for configuration and state management."""

    def __init__(self, dsn: str):
        if psycopg is None:
            raise ImportError("psycopg is required for PostgreSQL support")
        self.dsn = dsn
        self._initialize_database()

    @contextmanager
    def get_connection(self) -> Generator[Any, None, None]:
        conn = psycopg.connect(self.dsn)
        conn.autocommit = True
        try:
            yield conn
        finally:
            conn.close()

    def _initialize_database(self) -> None:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    CREATE TABLE IF NOT EXISTS config (
                        section TEXT NOT NULL,
                        key TEXT NOT NULL,
                        value JSONB NOT NULL,
                        updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                        PRIMARY KEY (section, key)
                    )
                    """
                )
                cur.execute(
                    """
                    CREATE TABLE IF NOT EXISTS bot_state (
                        id INTEGER PRIMARY KEY,
                        status TEXT DEFAULT 'STOPPED',
                        tws_connected BOOLEAN DEFAULT FALSE,
                        equity DOUBLE PRECISION DEFAULT 0.0,
                        daily_pnl DOUBLE PRECISION DEFAULT 0.0,
                        daily_pnl_percent DOUBLE PRECISION DEFAULT 0.0,
                        total_pnl DOUBLE PRECISION DEFAULT 0.0,
                        active_strategy TEXT DEFAULT '',
                        error_message TEXT DEFAULT '',
                        trades_today INTEGER DEFAULT 0,
                        win_rate_today DOUBLE PRECISION DEFAULT 0.0,
                        last_update TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
                    )
                    """
                )
                cur.execute(
                    """
                    CREATE TABLE IF NOT EXISTS positions (
                        id BIGSERIAL PRIMARY KEY,
                        symbol TEXT NOT NULL,
                        quantity DOUBLE PRECISION NOT NULL,
                        entry_price DOUBLE PRECISION NOT NULL,
                        current_price DOUBLE PRECISION DEFAULT 0.0,
                        unrealized_pnl DOUBLE PRECISION DEFAULT 0.0,
                        entry_time TIMESTAMPTZ,
                        updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
                    )
                    """
                )
                cur.execute(
                    """
                    CREATE TABLE IF NOT EXISTS orders (
                        order_id TEXT PRIMARY KEY,
                        symbol TEXT NOT NULL,
                        side TEXT NOT NULL,
                        quantity DOUBLE PRECISION NOT NULL,
                        price DOUBLE PRECISION,
                        status TEXT DEFAULT 'PENDING',
                        order_type TEXT DEFAULT 'MARKET',
                        submitted_time TIMESTAMPTZ,
                        filled_quantity DOUBLE PRECISION DEFAULT 0.0,
                        updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
                    )
                    """
                )
                cur.execute(
                    """
                    CREATE TABLE IF NOT EXISTS logs (
                        id BIGSERIAL PRIMARY KEY,
                        timestamp TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                        level TEXT NOT NULL,
                        message TEXT NOT NULL
                    )
                    """
                )
                cur.execute(
                    """
                    CREATE TABLE IF NOT EXISTS bot_commands (
                        id BIGSERIAL PRIMARY KEY,
                        command TEXT NOT NULL,
                        payload JSONB,
                        created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                        processed BOOLEAN DEFAULT FALSE
                    )
                    """
                )
                cur.execute(
                    """
                    CREATE TABLE IF NOT EXISTS trade_history (
                        id BIGSERIAL PRIMARY KEY,
                        symbol TEXT NOT NULL,
                        side TEXT NOT NULL,
                        quantity DOUBLE PRECISION NOT NULL,
                        price DOUBLE PRECISION NOT NULL,
                        pnl DOUBLE PRECISION DEFAULT 0.0,
                        strategy TEXT,
                        executed_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
                    )
                    """
                )
                cur.execute(
                    """
                    INSERT INTO bot_state (id)
                    VALUES (1)
                    ON CONFLICT (id) DO NOTHING
                    """
                )

    def _json_value(self, value: Any) -> Any:
        if value is None:
            return None
        if isinstance(value, str):
            try:
                return json.loads(value)
            except Exception:
                return value
        return value

    # Configuration Methods
    def set_config(self, section: str, key: str, value: Any) -> None:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO config (section, key, value, updated_at)
                    VALUES (%s, %s, %s::jsonb, CURRENT_TIMESTAMP)
                    ON CONFLICT (section, key)
                    DO UPDATE SET value = EXCLUDED.value, updated_at = CURRENT_TIMESTAMP
                    """,
                    [section, key, json.dumps(value)],
                )

    def get_config(self, section: str, key: str, default: Any = None) -> Any:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    "SELECT value FROM config WHERE section = %s AND key = %s",
                    [section, key],
                )
                row = cur.fetchone()
                if row:
                    return self._json_value(row[0])
                return default

    def get_section_config(self, section: str) -> Dict[str, Any]:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute("SELECT key, value FROM config WHERE section = %s", [section])
                rows = cur.fetchall()
                return {row[0]: self._json_value(row[1]) for row in rows}

    def set_section_config(self, section: str, config: Dict[str, Any]) -> None:
        for key, value in config.items():
            self.set_config(section, key, value)

    def get_all_config(self) -> Dict[str, Dict[str, Any]]:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute("SELECT section, key, value FROM config")
                rows = cur.fetchall()
                result: Dict[str, Dict[str, Any]] = {}
                for section, key, value in rows:
                    result.setdefault(section, {})[key] = self._json_value(value)
                return result

    def delete_config(self, section: str, key: Optional[str] = None) -> None:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                if key is None:
                    cur.execute("DELETE FROM config WHERE section = %s", [section])
                else:
                    cur.execute("DELETE FROM config WHERE section = %s AND key = %s", [section, key])

    # Bot State
    def update_bot_state(self, **kwargs) -> None:
        updates = []
        params = []
        for key, value in kwargs.items():
            updates.append(f"{key} = %s")
            params.append(value)
        if updates:
            updates.append("last_update = CURRENT_TIMESTAMP")
            params.append(1)
            with self.get_connection() as conn:
                with conn.cursor() as cur:
                    cur.execute(
                        f"UPDATE bot_state SET {', '.join(updates)} WHERE id = %s",
                        params,
                    )

    def get_bot_state(self) -> Dict[str, Any]:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute("SELECT * FROM bot_state WHERE id = 1")
                row = cur.fetchone()
                if not row:
                    return {}
                columns = [desc[0] for desc in cur.description]
                return dict(zip(columns, row))

    def reset_bot_state(self) -> None:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    UPDATE bot_state SET
                        status = 'STOPPED',
                        tws_connected = FALSE,
                        equity = 0.0,
                        daily_pnl = 0.0,
                        daily_pnl_percent = 0.0,
                        total_pnl = 0.0,
                        active_strategy = '',
                        error_message = '',
                        trades_today = 0,
                        win_rate_today = 0.0,
                        last_update = CURRENT_TIMESTAMP
                    WHERE id = 1
                    """
                )

    # Positions
    def add_position(
        self,
        symbol: str,
        quantity: float,
        entry_price: float,
        current_price: float = 0.0,
        unrealized_pnl: float = 0.0,
        entry_time: Optional[datetime] = None,
    ) -> int:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO positions (symbol, quantity, entry_price, current_price, unrealized_pnl, entry_time, updated_at)
                    VALUES (%s, %s, %s, %s, %s, %s, CURRENT_TIMESTAMP)
                    RETURNING id
                    """,
                    [symbol, quantity, entry_price, current_price, unrealized_pnl, entry_time],
                )
                return cur.fetchone()[0]

    def update_position(
        self,
        position_id: int,
        current_price: Optional[float] = None,
        unrealized_pnl: Optional[float] = None,
        quantity: Optional[float] = None,
    ) -> None:
        updates = []
        params = []
        if current_price is not None:
            updates.append("current_price = %s")
            params.append(current_price)
        if unrealized_pnl is not None:
            updates.append("unrealized_pnl = %s")
            params.append(unrealized_pnl)
        if quantity is not None:
            updates.append("quantity = %s")
            params.append(quantity)
        if updates:
            updates.append("updated_at = CURRENT_TIMESTAMP")
            params.append(position_id)
            with self.get_connection() as conn:
                with conn.cursor() as cur:
                    cur.execute(
                        f"UPDATE positions SET {', '.join(updates)} WHERE id = %s",
                        params,
                    )

    def get_positions(self) -> List[Dict[str, Any]]:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    SELECT id, symbol, quantity, entry_price, current_price,
                           unrealized_pnl, entry_time, updated_at
                    FROM positions ORDER BY entry_time DESC
                    """
                )
                rows = cur.fetchall()
                results = []
                for row in rows:
                    results.append(
                        {
                            "id": row[0],
                            "symbol": row[1],
                            "quantity": row[2],
                            "entry_price": row[3],
                            "current_price": row[4],
                            "unrealized_pnl": row[5],
                            "entry_time": row[6].isoformat() if row[6] else None,
                            "updated_at": row[7].isoformat() if row[7] else None,
                        }
                    )
                return results

    def get_position_by_symbol(self, symbol: str) -> Optional[Dict[str, Any]]:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    SELECT id, symbol, quantity, entry_price, current_price,
                           unrealized_pnl, entry_time, updated_at
                    FROM positions WHERE symbol = %s
                    """,
                    [symbol],
                )
                row = cur.fetchone()
                if not row:
                    return None
                return {
                    "id": row[0],
                    "symbol": row[1],
                    "quantity": row[2],
                    "entry_price": row[3],
                    "current_price": row[4],
                    "unrealized_pnl": row[5],
                    "entry_time": row[6].isoformat() if row[6] else None,
                    "updated_at": row[7].isoformat() if row[7] else None,
                }

    def delete_position(self, position_id: int) -> None:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute("DELETE FROM positions WHERE id = %s", [position_id])

    def clear_positions(self) -> None:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute("DELETE FROM positions")

    # Orders
    def add_order(
        self,
        order_id: str,
        symbol: str,
        side: str,
        quantity: float,
        price: Optional[float] = None,
        status: str = "PENDING",
        order_type: str = "MARKET",
        submitted_time: Optional[datetime] = None,
    ) -> None:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO orders (order_id, symbol, side, quantity, price, status, order_type, submitted_time, filled_quantity, updated_at)
                    VALUES (%s, %s, %s, %s, %s, %s, %s, %s, 0.0, CURRENT_TIMESTAMP)
                    ON CONFLICT (order_id) DO NOTHING
                    """,
                    [order_id, symbol, side, quantity, price, status, order_type, submitted_time],
                )

    def update_order(
        self,
        order_id: str,
        status: Optional[str] = None,
        filled_quantity: Optional[float] = None,
        price: Optional[float] = None,
    ) -> None:
        updates = []
        params = []
        if status is not None:
            updates.append("status = %s")
            params.append(status)
        if filled_quantity is not None:
            updates.append("filled_quantity = %s")
            params.append(filled_quantity)
        if price is not None:
            updates.append("price = %s")
            params.append(price)
        if updates:
            updates.append("updated_at = CURRENT_TIMESTAMP")
            params.append(order_id)
            with self.get_connection() as conn:
                with conn.cursor() as cur:
                    cur.execute(
                        f"UPDATE orders SET {', '.join(updates)} WHERE order_id = %s",
                        params,
                    )

    def get_orders(self, status: Optional[str] = None) -> List[Dict[str, Any]]:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                if status:
                    cur.execute(
                        """
                        SELECT order_id, symbol, side, quantity, price, status, order_type,
                               submitted_time, filled_quantity, updated_at
                        FROM orders WHERE status = %s ORDER BY updated_at DESC
                        """,
                        [status],
                    )
                else:
                    cur.execute(
                        """
                        SELECT order_id, symbol, side, quantity, price, status, order_type,
                               submitted_time, filled_quantity, updated_at
                        FROM orders ORDER BY updated_at DESC
                        """
                    )
                rows = cur.fetchall()
                return [
                    {
                        "order_id": row[0],
                        "symbol": row[1],
                        "side": row[2],
                        "quantity": row[3],
                        "price": row[4],
                        "status": row[5],
                        "order_type": row[6],
                        "submitted_time": row[7].isoformat() if row[7] else None,
                        "filled_quantity": row[8],
                        "updated_at": row[9].isoformat() if row[9] else None,
                    }
                    for row in rows
                ]

    def get_pending_orders(self) -> List[Dict[str, Any]]:
        return self.get_orders(status="PENDING")

    def delete_order(self, order_id: str) -> None:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute("DELETE FROM orders WHERE order_id = %s", [order_id])

    def clear_orders(self) -> None:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute("DELETE FROM orders")

    # Logs
    def add_log(self, level: str, message: str) -> None:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    "INSERT INTO logs (level, message) VALUES (%s, %s)",
                    [level, message],
                )

    def get_recent_logs(self, limit: int = 50) -> List[Dict[str, Any]]:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    "SELECT id, timestamp, level, message FROM logs ORDER BY timestamp DESC LIMIT %s",
                    [limit],
                )
                rows = cur.fetchall()
                return [
                    {
                        "id": row[0],
                        "timestamp": row[1].isoformat() if row[1] else None,
                        "level": row[2],
                        "message": row[3],
                    }
                    for row in rows
                ]

    def clear_logs(self) -> None:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute("DELETE FROM logs")

    # Commands
    def add_command(self, command: str, payload: Optional[Dict[str, Any]] = None) -> int:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    "INSERT INTO bot_commands (command, payload) VALUES (%s, %s::jsonb) RETURNING id",
                    [command, json.dumps(payload) if payload else None],
                )
                return cur.fetchone()[0]

    def get_pending_commands(self) -> List[Dict[str, Any]]:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    "SELECT id, command, payload, created_at, processed FROM bot_commands WHERE processed = FALSE ORDER BY created_at ASC"
                )
                rows = cur.fetchall()
                return [
                    {
                        "id": row[0],
                        "command": row[1],
                        "payload": self._json_value(row[2]) if row[2] else None,
                        "created_at": row[3].isoformat() if row[3] else None,
                        "processed": row[4],
                    }
                    for row in rows
                ]

    def mark_command_processed(self, command_id: int) -> None:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute("UPDATE bot_commands SET processed = TRUE WHERE id = %s", [command_id])

    def clear_commands(self) -> None:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute("DELETE FROM bot_commands")

    # Trade History
    def add_trade(
        self,
        symbol: str,
        side: str,
        quantity: float,
        price: float,
        pnl: float = 0.0,
        strategy: Optional[str] = None,
    ) -> None:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO trade_history (symbol, side, quantity, price, pnl, strategy)
                    VALUES (%s, %s, %s, %s, %s, %s)
                    """,
                    [symbol, side, quantity, price, pnl, strategy],
                )

    def get_trade_history(
        self,
        limit: int = 100,
        symbol: Optional[str] = None,
    ) -> List[Dict[str, Any]]:
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                if symbol:
                    cur.execute(
                        """
                        SELECT id, symbol, side, quantity, price, pnl, strategy, executed_at
                        FROM trade_history WHERE symbol = %s
                        ORDER BY executed_at DESC LIMIT %s
                        """,
                        [symbol, limit],
                    )
                else:
                    cur.execute(
                        """
                        SELECT id, symbol, side, quantity, price, pnl, strategy, executed_at
                        FROM trade_history ORDER BY executed_at DESC LIMIT %s
                        """,
                        [limit],
                    )
                rows = cur.fetchall()
                return [
                    {
                        "id": row[0],
                        "symbol": row[1],
                        "side": row[2],
                        "quantity": row[3],
                        "price": row[4],
                        "pnl": row[5],
                        "strategy": row[6],
                        "executed_at": row[7].isoformat() if row[7] else None,
                    }
                    for row in rows
                ]

    def get_daily_stats(self, date: Optional[datetime] = None) -> Dict[str, Any]:
        date_str = (date or datetime.utcnow()).strftime("%Y-%m-%d")
        with self.get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    SELECT
                        COUNT(*) as total_trades,
                        SUM(CASE WHEN pnl > 0 THEN 1 ELSE 0 END) as winning_trades,
                        SUM(pnl) as total_pnl,
                        AVG(pnl) as avg_pnl,
                        MAX(pnl) as max_profit,
                        MIN(pnl) as max_loss
                    FROM trade_history
                    WHERE DATE(executed_at) = %s
                    """,
                    [date_str],
                )
                result = cur.fetchone()
                total_trades = result[0] or 0
                winning_trades = result[1] or 0
                return {
                    "date": date_str,
                    "total_trades": total_trades,
                    "winning_trades": winning_trades,
                    "losing_trades": total_trades - winning_trades,
                    "win_rate": (winning_trades / total_trades * 100) if total_trades > 0 else 0.0,
                    "total_pnl": result[2] or 0.0,
                    "avg_pnl": result[3] or 0.0,
                    "max_profit": result[4] or 0.0,
                    "max_loss": result[5] or 0.0,
                }
    
    # =========================================================================
    # Configuration Methods
    # =========================================================================
    
    def set_config(self, section: str, key: str, value: Any) -> None:
        """
        Set a configuration value.
        
        Args:
            section: Configuration section (e.g., 'ib', 'app', 'risk')
            key: Configuration key within the section
            value: Value to store (will be JSON serialized)
        """
        with self.get_connection() as conn:
            conn.execute("""
                INSERT OR REPLACE INTO config (section, key, value, updated_at)
                VALUES (?, ?, ?::JSON, CURRENT_TIMESTAMP)
            """, [section, key, json.dumps(value)])
    
    def get_config(self, section: str, key: str, default: Any = None) -> Any:
        """
        Get a configuration value.
        
        Args:
            section: Configuration section
            key: Configuration key
            default: Default value if not found
            
        Returns:
            The configuration value or default
        """
        with self.get_connection() as conn:
            result = conn.execute("""
                SELECT value FROM config WHERE section = ? AND key = ?
            """, [section, key]).fetchone()
            
            if result:
                return json.loads(result[0])
            return default
    
    def get_section_config(self, section: str) -> Dict[str, Any]:
        """
        Get all configuration values for a section.
        
        Args:
            section: Configuration section
            
        Returns:
            Dictionary of key-value pairs for the section
        """
        with self.get_connection() as conn:
            results = conn.execute("""
                SELECT key, value FROM config WHERE section = ?
            """, [section]).fetchall()
            
            return {row[0]: json.loads(row[1]) for row in results}
    
    def set_section_config(self, section: str, config: Dict[str, Any]) -> None:
        """
        Set all configuration values for a section.
        
        Args:
            section: Configuration section
            config: Dictionary of key-value pairs to store
        """
        with self.get_connection() as conn:
            # Delete existing section config
            conn.execute("DELETE FROM config WHERE section = ?", [section])
            
            # Insert new values
            for key, value in config.items():
                conn.execute("""
                    INSERT INTO config (section, key, value, updated_at)
                    VALUES (?, ?, ?::JSON, CURRENT_TIMESTAMP)
                """, [section, key, json.dumps(value)])
    
    def get_all_config(self) -> Dict[str, Dict[str, Any]]:
        """
        Get all configuration values grouped by section.
        
        Returns:
            Nested dictionary of section -> key -> value
        """
        with self.get_connection() as conn:
            results = conn.execute("""
                SELECT section, key, value FROM config ORDER BY section, key
            """).fetchall()
            
            config: Dict[str, Dict[str, Any]] = {}
            for section, key, value in results:
                if section not in config:
                    config[section] = {}
                config[section][key] = json.loads(value)
            
            return config
    
    def delete_config(self, section: str, key: Optional[str] = None) -> None:
        """
        Delete configuration value(s).
        
        Args:
            section: Configuration section
            key: Specific key to delete, or None to delete entire section
        """
        with self.get_connection() as conn:
            if key:
                conn.execute(
                    "DELETE FROM config WHERE section = ? AND key = ?",
                    [section, key]
                )
            else:
                conn.execute("DELETE FROM config WHERE section = ?", [section])
    
    # =========================================================================
    # Bot State Methods
    # =========================================================================
    
    def update_bot_state(
        self,
        status: Optional[str] = None,
        tws_connected: Optional[bool] = None,
        equity: Optional[float] = None,
        daily_pnl: Optional[float] = None,
        daily_pnl_percent: Optional[float] = None,
        total_pnl: Optional[float] = None,
        active_strategy: Optional[str] = None,
        error_message: Optional[str] = None,
        trades_today: Optional[int] = None,
        win_rate_today: Optional[float] = None,
    ) -> None:
        """
        Update bot state with provided values.
        
        Only non-None values will be updated.
        """
        updates = []
        params = []
        
        if status is not None:
            updates.append("status = ?")
            params.append(status)
        if tws_connected is not None:
            updates.append("tws_connected = ?")
            params.append(tws_connected)
        if equity is not None:
            updates.append("equity = ?")
            params.append(equity)
        if daily_pnl is not None:
            updates.append("daily_pnl = ?")
            params.append(daily_pnl)
        if daily_pnl_percent is not None:
            updates.append("daily_pnl_percent = ?")
            params.append(daily_pnl_percent)
        if total_pnl is not None:
            updates.append("total_pnl = ?")
            params.append(total_pnl)
        if active_strategy is not None:
            updates.append("active_strategy = ?")
            params.append(active_strategy)
        if error_message is not None:
            updates.append("error_message = ?")
            params.append(error_message)
        if trades_today is not None:
            updates.append("trades_today = ?")
            params.append(trades_today)
        if win_rate_today is not None:
            updates.append("win_rate_today = ?")
            params.append(win_rate_today)
        
        if updates:
            updates.append("last_update = CURRENT_TIMESTAMP")
            with self.get_connection() as conn:
                conn.execute(
                    f"UPDATE bot_state SET {', '.join(updates)} WHERE id = 1",
                    params
                )
    
    def get_bot_state(self) -> Dict[str, Any]:
        """
        Get current bot state.
        
        Returns:
            Dictionary with all bot state fields
        """
        with self.get_connection() as conn:
            result = conn.execute("""
                SELECT 
                    status, tws_connected, equity, daily_pnl, daily_pnl_percent,
                    total_pnl, active_strategy, error_message, trades_today,
                    win_rate_today, last_update
                FROM bot_state WHERE id = 1
            """).fetchone()
            
            if result:
                return {
                    "status": result[0],
                    "tws_connected": result[1],
                    "equity": result[2],
                    "daily_pnl": result[3],
                    "daily_pnl_percent": result[4],
                    "total_pnl": result[5],
                    "active_strategy": result[6],
                    "error_message": result[7],
                    "trades_today": result[8],
                    "win_rate_today": result[9],
                    "last_update": result[10].isoformat() if result[10] else None,
                }
            return {}
    
    def reset_bot_state(self) -> None:
        """Reset bot state to defaults."""
        with self.get_connection() as conn:
            conn.execute("""
                UPDATE bot_state SET
                    status = 'STOPPED',
                    tws_connected = FALSE,
                    equity = 0.0,
                    daily_pnl = 0.0,
                    daily_pnl_percent = 0.0,
                    total_pnl = 0.0,
                    active_strategy = '',
                    error_message = '',
                    trades_today = 0,
                    win_rate_today = 0.0,
                    last_update = CURRENT_TIMESTAMP
                WHERE id = 1
            """)
    
    # =========================================================================
    # Positions Methods
    # =========================================================================
    
    def add_position(
        self,
        symbol: str,
        quantity: float,
        entry_price: float,
        current_price: float = 0.0,
        unrealized_pnl: float = 0.0,
        entry_time: Optional[datetime] = None,
    ) -> int:
        """
        Add a new position.
        
        Returns:
            The position ID
        """
        with self.get_connection() as conn:
            result = conn.execute("""
                INSERT INTO positions (id, symbol, quantity, entry_price, current_price, unrealized_pnl, entry_time, updated_at)
                VALUES (nextval('positions_seq'), ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
                RETURNING id
            """, [symbol, quantity, entry_price, current_price, unrealized_pnl, entry_time]).fetchone()
            return result[0]
    
    def update_position(
        self,
        position_id: int,
        current_price: Optional[float] = None,
        unrealized_pnl: Optional[float] = None,
        quantity: Optional[float] = None,
    ) -> None:
        """Update an existing position."""
        updates = []
        params = []
        
        if current_price is not None:
            updates.append("current_price = ?")
            params.append(current_price)
        if unrealized_pnl is not None:
            updates.append("unrealized_pnl = ?")
            params.append(unrealized_pnl)
        if quantity is not None:
            updates.append("quantity = ?")
            params.append(quantity)
        
        if updates:
            updates.append("updated_at = CURRENT_TIMESTAMP")
            params.append(position_id)
            with self.get_connection() as conn:
                conn.execute(
                    f"UPDATE positions SET {', '.join(updates)} WHERE id = ?",
                    params
                )
    
    def get_positions(self) -> List[Dict[str, Any]]:
        """Get all open positions."""
        with self.get_connection() as conn:
            results = conn.execute("""
                SELECT id, symbol, quantity, entry_price, current_price, 
                       unrealized_pnl, entry_time, updated_at
                FROM positions ORDER BY entry_time DESC
            """).fetchall()
            
            return [
                {
                    "id": row[0],
                    "symbol": row[1],
                    "quantity": row[2],
                    "entry_price": row[3],
                    "current_price": row[4],
                    "unrealized_pnl": row[5],
                    "entry_time": row[6].isoformat() if row[6] else None,
                    "updated_at": row[7].isoformat() if row[7] else None,
                }
                for row in results
            ]
    
    def get_position_by_symbol(self, symbol: str) -> Optional[Dict[str, Any]]:
        """Get position by symbol."""
        with self.get_connection() as conn:
            result = conn.execute("""
                SELECT id, symbol, quantity, entry_price, current_price,
                       unrealized_pnl, entry_time, updated_at
                FROM positions WHERE symbol = ?
            """, [symbol]).fetchone()
            
            if result:
                return {
                    "id": result[0],
                    "symbol": result[1],
                    "quantity": result[2],
                    "entry_price": result[3],
                    "current_price": result[4],
                    "unrealized_pnl": result[5],
                    "entry_time": result[6].isoformat() if result[6] else None,
                    "updated_at": result[7].isoformat() if result[7] else None,
                }
            return None
    
    def delete_position(self, position_id: int) -> None:
        """Delete a position."""
        with self.get_connection() as conn:
            conn.execute("DELETE FROM positions WHERE id = ?", [position_id])
    
    def clear_positions(self) -> None:
        """Clear all positions."""
        with self.get_connection() as conn:
            conn.execute("DELETE FROM positions")
    
    # =========================================================================
    # Orders Methods
    # =========================================================================
    
    def add_order(
        self,
        order_id: str,
        symbol: str,
        side: str,
        quantity: float,
        price: Optional[float] = None,
        order_type: str = "MARKET",
        status: str = "PENDING",
        submitted_time: Optional[datetime] = None,
    ) -> None:
        """Add a new order."""
        with self.get_connection() as conn:
            conn.execute("""
                INSERT INTO orders (order_id, symbol, side, quantity, price, order_type, status, submitted_time, updated_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
            """, [order_id, symbol, side, quantity, price, order_type, status, submitted_time])
    
    def update_order(
        self,
        order_id: str,
        status: Optional[str] = None,
        filled_quantity: Optional[float] = None,
        price: Optional[float] = None,
    ) -> None:
        """Update an existing order."""
        updates = []
        params = []
        
        if status is not None:
            updates.append("status = ?")
            params.append(status)
        if filled_quantity is not None:
            updates.append("filled_quantity = ?")
            params.append(filled_quantity)
        if price is not None:
            updates.append("price = ?")
            params.append(price)
        
        if updates:
            updates.append("updated_at = CURRENT_TIMESTAMP")
            params.append(order_id)
            with self.get_connection() as conn:
                conn.execute(
                    f"UPDATE orders SET {', '.join(updates)} WHERE order_id = ?",
                    params
                )
    
    def get_orders(self, status: Optional[str] = None) -> List[Dict[str, Any]]:
        """Get orders, optionally filtered by status."""
        with self.get_connection() as conn:
            if status:
                results = conn.execute("""
                    SELECT order_id, symbol, side, quantity, price, status,
                           order_type, submitted_time, filled_quantity, updated_at
                    FROM orders WHERE status = ?
                    ORDER BY submitted_time DESC
                """, [status]).fetchall()
            else:
                results = conn.execute("""
                    SELECT order_id, symbol, side, quantity, price, status,
                           order_type, submitted_time, filled_quantity, updated_at
                    FROM orders ORDER BY submitted_time DESC
                """).fetchall()
            
            return [
                {
                    "order_id": row[0],
                    "symbol": row[1],
                    "side": row[2],
                    "quantity": row[3],
                    "price": row[4],
                    "status": row[5],
                    "order_type": row[6],
                    "submitted_time": row[7].isoformat() if row[7] else None,
                    "filled_quantity": row[8],
                    "updated_at": row[9].isoformat() if row[9] else None,
                }
                for row in results
            ]
    
    def get_pending_orders(self) -> List[Dict[str, Any]]:
        """Get pending and submitted orders."""
        with self.get_connection() as conn:
            results = conn.execute("""
                SELECT order_id, symbol, side, quantity, price, status,
                       order_type, submitted_time, filled_quantity, updated_at
                FROM orders WHERE status IN ('PENDING', 'SUBMITTED')
                ORDER BY submitted_time DESC
            """).fetchall()
            
            return [
                {
                    "order_id": row[0],
                    "symbol": row[1],
                    "side": row[2],
                    "quantity": row[3],
                    "price": row[4],
                    "status": row[5],
                    "order_type": row[6],
                    "submitted_time": row[7].isoformat() if row[7] else None,
                    "filled_quantity": row[8],
                    "updated_at": row[9].isoformat() if row[9] else None,
                }
                for row in results
            ]
    
    def delete_order(self, order_id: str) -> None:
        """Delete an order."""
        with self.get_connection() as conn:
            conn.execute("DELETE FROM orders WHERE order_id = ?", [order_id])
    
    def clear_orders(self) -> None:
        """Clear all orders."""
        with self.get_connection() as conn:
            conn.execute("DELETE FROM orders")
    
    # =========================================================================
    # Logs Methods
    # =========================================================================
    
    def add_log(self, level: str, message: str) -> None:
        """Add a log entry."""
        with self.get_connection() as conn:
            conn.execute("""
                INSERT INTO logs (id, level, message, timestamp)
                VALUES (nextval('logs_seq'), ?, ?, CURRENT_TIMESTAMP)
            """, [level, message])
            
            # Keep only last 500 logs
            conn.execute("""
                DELETE FROM logs WHERE id NOT IN (
                    SELECT id FROM logs ORDER BY timestamp DESC LIMIT 500
                )
            """)
    
    def get_recent_logs(self, limit: int = 50) -> List[Dict[str, Any]]:
        """Get recent log entries."""
        with self.get_connection() as conn:
            results = conn.execute("""
                SELECT id, timestamp, level, message
                FROM logs ORDER BY timestamp DESC LIMIT ?
            """, [limit]).fetchall()
            
            return [
                {
                    "id": row[0],
                    "timestamp": row[1].isoformat() if row[1] else None,
                    "level": row[2],
                    "message": row[3],
                }
                for row in reversed(results)  # Return in chronological order
            ]
    
    def clear_logs(self) -> None:
        """Clear all logs."""
        with self.get_connection() as conn:
            conn.execute("DELETE FROM logs")
    
    # =========================================================================
    # Bot Commands Methods
    # =========================================================================
    
    def add_command(self, command: str, payload: Optional[Dict[str, Any]] = None) -> int:
        """
        Add a bot command.
        
        Returns:
            The command ID
        """
        with self.get_connection() as conn:
            result = conn.execute("""
                INSERT INTO bot_commands (id, command, payload, created_at, processed)
                VALUES (nextval('commands_seq'), ?, ?::JSON, CURRENT_TIMESTAMP, FALSE)
                RETURNING id
            """, [command, json.dumps(payload) if payload else None]).fetchone()
            return result[0]
    
    def get_pending_commands(self) -> List[Dict[str, Any]]:
        """Get unprocessed commands."""
        with self.get_connection() as conn:
            results = conn.execute("""
                SELECT id, command, payload, created_at
                FROM bot_commands WHERE processed = FALSE
                ORDER BY created_at ASC
            """).fetchall()
            
            return [
                {
                    "id": row[0],
                    "command": row[1],
                    "payload": json.loads(row[2]) if row[2] else None,
                    "created_at": row[3].isoformat() if row[3] else None,
                }
                for row in results
            ]
    
    def mark_command_processed(self, command_id: int) -> None:
        """Mark a command as processed."""
        with self.get_connection() as conn:
            conn.execute(
                "UPDATE bot_commands SET processed = TRUE WHERE id = ?",
                [command_id]
            )
    
    def clear_commands(self) -> None:
        """Clear all commands."""
        with self.get_connection() as conn:
            conn.execute("DELETE FROM bot_commands")
    
    # =========================================================================
    # Trade History Methods
    # =========================================================================
    
    def add_trade(
        self,
        symbol: str,
        side: str,
        quantity: float,
        price: float,
        pnl: float = 0.0,
        strategy: Optional[str] = None,
        executed_at: Optional[datetime] = None,
    ) -> int:
        """
        Record a completed trade.
        
        Returns:
            The trade ID
        """
        with self.get_connection() as conn:
            result = conn.execute("""
                INSERT INTO trade_history (id, symbol, side, quantity, price, pnl, strategy, executed_at)
                VALUES (nextval('trade_history_seq'), ?, ?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP))
                RETURNING id
            """, [symbol, side, quantity, price, pnl, strategy, executed_at]).fetchone()
            return result[0]
    
    def get_trade_history(
        self,
        limit: int = 100,
        symbol: Optional[str] = None,
        start_date: Optional[datetime] = None,
        end_date: Optional[datetime] = None,
    ) -> List[Dict[str, Any]]:
        """Get trade history with optional filters."""
        with self.get_connection() as conn:
            query = "SELECT id, symbol, side, quantity, price, pnl, strategy, executed_at FROM trade_history WHERE 1=1"
            params = []
            
            if symbol:
                query += " AND symbol = ?"
                params.append(symbol)
            if start_date:
                query += " AND executed_at >= ?"
                params.append(start_date)
            if end_date:
                query += " AND executed_at <= ?"
                params.append(end_date)
            
            query += " ORDER BY executed_at DESC LIMIT ?"
            params.append(limit)
            
            results = conn.execute(query, params).fetchall()
            
            return [
                {
                    "id": row[0],
                    "symbol": row[1],
                    "side": row[2],
                    "quantity": row[3],
                    "price": row[4],
                    "pnl": row[5],
                    "strategy": row[6],
                    "executed_at": row[7].isoformat() if row[7] else None,
                }
                for row in results
            ]
    
    def get_daily_stats(self, date: Optional[datetime] = None) -> Dict[str, Any]:
        """Get trading statistics for a given day."""
        target_date = date or datetime.now()
        date_str = target_date.strftime("%Y-%m-%d")
        
        with self.get_connection() as conn:
            result = conn.execute("""
                SELECT 
                    COUNT(*) as total_trades,
                    SUM(CASE WHEN pnl > 0 THEN 1 ELSE 0 END) as winning_trades,
                    SUM(pnl) as total_pnl,
                    AVG(pnl) as avg_pnl,
                    MAX(pnl) as max_profit,
                    MIN(pnl) as max_loss
                FROM trade_history
                WHERE DATE(executed_at) = ?
            """, [date_str]).fetchone()
            
            total_trades = result[0] or 0
            winning_trades = result[1] or 0
            
            return {
                "date": date_str,
                "total_trades": total_trades,
                "winning_trades": winning_trades,
                "losing_trades": total_trades - winning_trades,
                "win_rate": (winning_trades / total_trades * 100) if total_trades > 0 else 0.0,
                "total_pnl": result[2] or 0.0,
                "avg_pnl": result[3] or 0.0,
                "max_profit": result[4] or 0.0,
                "max_loss": result[5] or 0.0,
            }


# Global database manager instance (lazy loaded)
_db_manager: Optional[DatabaseManager] = None


def get_database(db_path: Optional[Path] = None) -> DatabaseManager:
    """
    Get or create the global database manager instance.
    
    Args:
        db_path: Optional custom database path (only used on first call)
        
    Returns:
        DatabaseManager instance
    """
    global _db_manager
    if _db_manager is None:
        try:
            from src.config.settings import get_settings
            settings = get_settings()
            backend = settings.database.backend.lower()
            if backend == "postgres":
                if not settings.database.dsn:
                    raise ValueError("Postgres backend selected but database.dsn is empty")
                _db_manager = PostgresDatabaseManager(settings.database.dsn)
            else:
                _db_manager = DatabaseManager(db_path)
        except Exception as exc:
            logger.warning("Database backend fallback to DuckDB: %s", exc)
            _db_manager = DatabaseManager(db_path)
    return _db_manager


def reset_database_instance() -> None:
    """Reset the global database instance (useful for testing)."""
    global _db_manager
    _db_manager = None
