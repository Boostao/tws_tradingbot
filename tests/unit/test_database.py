"""Tests for the PostgreSQL database backend."""

import pytest
import os
from datetime import datetime
from uuid import uuid4

from src.config.database import DatabaseManager, get_database, reset_database_instance


@pytest.fixture
def temp_db():
    """Create a temporary database for testing."""
    db_url = os.getenv("TEST_DATABASE_URL") or os.getenv("DATABASE_URL")
    if not db_url:
        pytest.skip("TEST_DATABASE_URL or DATABASE_URL not set")

    schema = f"test_{uuid4().hex[:8]}"
    reset_database_instance()
    db = DatabaseManager(db_url=db_url, schema=schema)
    yield db
    db.drop_schema()
    reset_database_instance()


class TestDatabaseConfig:
    """Test configuration storage and retrieval."""
    
    def test_set_and_get_config(self, temp_db):
        """Test setting and getting a single config value."""
        temp_db.set_config("ib", "host", "192.168.1.100")
        result = temp_db.get_config("ib", "host")
        assert result == "192.168.1.100"
    
    def test_get_config_default(self, temp_db):
        """Test getting non-existent config returns default."""
        result = temp_db.get_config("nonexistent", "key", default="default_value")
        assert result == "default_value"
    
    def test_set_section_config(self, temp_db):
        """Test setting an entire config section."""
        config = {
            "host": "127.0.0.1",
            "port": 7497,
            "client_id": 1,
        }
        temp_db.set_section_config("ib", config)
        
        result = temp_db.get_section_config("ib")
        assert result == config
    
    def test_get_all_config(self, temp_db):
        """Test getting all config sections."""
        temp_db.set_section_config("ib", {"host": "localhost"})
        temp_db.set_section_config("app", {"log_level": "DEBUG"})
        
        all_config = temp_db.get_all_config()
        assert "ib" in all_config
        assert "app" in all_config
        assert all_config["ib"]["host"] == "localhost"
        assert all_config["app"]["log_level"] == "DEBUG"
    
    def test_delete_config(self, temp_db):
        """Test deleting config values."""
        temp_db.set_config("ib", "host", "localhost")
        temp_db.set_config("ib", "port", 7497)
        
        temp_db.delete_config("ib", "host")
        assert temp_db.get_config("ib", "host") is None
        assert temp_db.get_config("ib", "port") == 7497
        
        temp_db.delete_config("ib")  # Delete entire section
        assert temp_db.get_config("ib", "port") is None


class TestBotState:
    """Test bot state management."""
    
    def test_update_bot_state(self, temp_db):
        """Test updating bot state."""
        temp_db.update_bot_state(
            status="RUNNING",
            tws_connected=True,
            equity=10000.0,
            daily_pnl=150.0,
        )
        
        state = temp_db.get_bot_state()
        assert state["status"] == "RUNNING"
        assert state["tws_connected"] == True
        assert state["equity"] == 10000.0
        assert state["daily_pnl"] == 150.0
    
    def test_partial_state_update(self, temp_db):
        """Test that partial updates don't overwrite other fields."""
        temp_db.update_bot_state(status="RUNNING", equity=10000.0)
        temp_db.update_bot_state(daily_pnl=200.0)
        
        state = temp_db.get_bot_state()
        assert state["status"] == "RUNNING"
        assert state["equity"] == 10000.0
        assert state["daily_pnl"] == 200.0
    
    def test_reset_bot_state(self, temp_db):
        """Test resetting bot state to defaults."""
        temp_db.update_bot_state(status="RUNNING", equity=10000.0)
        temp_db.reset_bot_state()
        
        state = temp_db.get_bot_state()
        assert state["status"] == "STOPPED"
        assert state["equity"] == 0.0


class TestPositions:
    """Test position management."""
    
    def test_add_and_get_positions(self, temp_db):
        """Test adding and retrieving positions."""
        pos_id = temp_db.add_position(
            symbol="SPY",
            quantity=100,
            entry_price=450.0,
            current_price=455.0,
            unrealized_pnl=500.0,
        )
        
        positions = temp_db.get_positions()
        assert len(positions) == 1
        assert positions[0]["symbol"] == "SPY"
        assert positions[0]["quantity"] == 100
        assert positions[0]["entry_price"] == 450.0
    
    def test_update_position(self, temp_db):
        """Test updating a position."""
        pos_id = temp_db.add_position("SPY", 100, 450.0)
        temp_db.update_position(pos_id, current_price=460.0, unrealized_pnl=1000.0)
        
        position = temp_db.get_position_by_symbol("SPY")
        assert position["current_price"] == 460.0
        assert position["unrealized_pnl"] == 1000.0
    
    def test_delete_position(self, temp_db):
        """Test deleting a position."""
        pos_id = temp_db.add_position("SPY", 100, 450.0)
        temp_db.delete_position(pos_id)
        
        assert temp_db.get_positions() == []
    
    def test_clear_positions(self, temp_db):
        """Test clearing all positions."""
        temp_db.add_position("SPY", 100, 450.0)
        temp_db.add_position("QQQ", 50, 380.0)
        
        temp_db.clear_positions()
        assert temp_db.get_positions() == []


class TestOrders:
    """Test order management."""
    
    def test_add_and_get_orders(self, temp_db):
        """Test adding and retrieving orders."""
        temp_db.add_order(
            order_id="ORD-001",
            symbol="SPY",
            side="BUY",
            quantity=100,
            price=450.0,
            order_type="LIMIT",
        )
        
        orders = temp_db.get_orders()
        assert len(orders) == 1
        assert orders[0]["order_id"] == "ORD-001"
        assert orders[0]["symbol"] == "SPY"
        assert orders[0]["side"] == "BUY"
    
    def test_update_order(self, temp_db):
        """Test updating an order."""
        temp_db.add_order("ORD-001", "SPY", "BUY", 100)
        temp_db.update_order("ORD-001", status="FILLED", filled_quantity=100)
        
        orders = temp_db.get_orders()
        assert orders[0]["status"] == "FILLED"
        assert orders[0]["filled_quantity"] == 100
    
    def test_get_pending_orders(self, temp_db):
        """Test getting only pending orders."""
        temp_db.add_order("ORD-001", "SPY", "BUY", 100, status="PENDING")
        temp_db.add_order("ORD-002", "SPY", "SELL", 100, status="FILLED")
        
        pending = temp_db.get_pending_orders()
        assert len(pending) == 1
        assert pending[0]["order_id"] == "ORD-001"


class TestLogs:
    """Test log management."""
    
    def test_add_and_get_logs(self, temp_db):
        """Test adding and retrieving logs."""
        temp_db.add_log("INFO", "Test message 1")
        temp_db.add_log("WARNING", "Test warning")
        temp_db.add_log("ERROR", "Test error")
        
        logs = temp_db.get_recent_logs(10)
        assert len(logs) == 3
        assert logs[0]["level"] == "INFO"
        assert logs[2]["level"] == "ERROR"
    
    def test_log_limit(self, temp_db):
        """Test that log retrieval respects limit."""
        for i in range(10):
            temp_db.add_log("INFO", f"Message {i}")
        
        logs = temp_db.get_recent_logs(5)
        assert len(logs) == 5


class TestCommands:
    """Test bot command management."""
    
    def test_add_and_get_commands(self, temp_db):
        """Test adding and retrieving commands."""
        cmd_id = temp_db.add_command("START")
        
        commands = temp_db.get_pending_commands()
        assert len(commands) == 1
        assert commands[0]["command"] == "START"
    
    def test_command_with_payload(self, temp_db):
        """Test command with payload."""
        temp_db.add_command("EMERGENCY_STOP", {
            "cancel_orders": True,
            "flatten_positions": True,
        })
        
        commands = temp_db.get_pending_commands()
        assert commands[0]["payload"]["cancel_orders"] == True
    
    def test_mark_command_processed(self, temp_db):
        """Test marking command as processed."""
        cmd_id = temp_db.add_command("START")
        temp_db.mark_command_processed(cmd_id)
        
        commands = temp_db.get_pending_commands()
        assert len(commands) == 0


class TestTradeHistory:
    """Test trade history management."""
    
    def test_add_and_get_trades(self, temp_db):
        """Test adding and retrieving trade history."""
        temp_db.add_trade(
            symbol="SPY",
            side="BUY",
            quantity=100,
            price=450.0,
            pnl=0.0,
            strategy="EMA Crossover",
        )
        
        trades = temp_db.get_trade_history()
        assert len(trades) == 1
        assert trades[0]["symbol"] == "SPY"
        assert trades[0]["strategy"] == "EMA Crossover"
    
    def test_daily_stats(self, temp_db):
        """Test daily trading statistics."""
        # Add some trades
        temp_db.add_trade("SPY", "BUY", 100, 450.0, pnl=100.0)
        temp_db.add_trade("SPY", "SELL", 100, 455.0, pnl=500.0)
        temp_db.add_trade("QQQ", "BUY", 50, 380.0, pnl=-50.0)
        
        stats = temp_db.get_daily_stats()
        assert stats["total_trades"] == 3
        assert stats["winning_trades"] == 2
        assert stats["losing_trades"] == 1
        assert stats["total_pnl"] == 550.0
