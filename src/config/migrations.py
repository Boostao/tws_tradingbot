"""
Database Migration and Management Utilities.

Provides tools for migrating existing JSON-based state and config
to the PostgreSQL backend, and general database management commands.
"""

import json
import logging
from datetime import datetime
from pathlib import Path
from typing import Optional

logger = logging.getLogger(__name__)


def migrate_json_state_to_db(
    json_state_file: Optional[Path] = None,
    db_url: Optional[str] = None,
    schema: Optional[str] = None,
) -> bool:
    """
    Migrate existing JSON state file to PostgreSQL database.
    
    Args:
        json_state_file: Path to the JSON state file
        db_url: Database URL
        schema: Database schema name
        
    Returns:
        True if migration successful
    """
    from src.config.database import get_database
    from src.bot.state import (
        BotState, Position, Order,
        DEFAULT_STATE_FILE,
    )
    
    json_state_file = json_state_file or DEFAULT_STATE_FILE
    
    if not json_state_file.exists():
        logger.info(f"No JSON state file found at {json_state_file}")
        return True
    
    try:
        # Read existing JSON state
        with open(json_state_file, 'r') as f:
            data = json.load(f)
        
        state = BotState.from_dict(data)
        
        # Get database connection
        db = get_database(db_url=db_url, schema=schema or "public")
        
        # Migrate bot state
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
        
        # Migrate positions
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
        
        # Migrate orders
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
        
        # Migrate logs
        for log_entry in state.recent_logs:
            try:
                parts = log_entry.split("] ", 2)
                if len(parts) >= 2:
                    level = parts[1].strip("[")
                    message = parts[2] if len(parts) > 2 else ""
                    db.add_log(level, message)
            except Exception:
                db.add_log("INFO", log_entry)
        
        logger.info(f"Successfully migrated state from {json_state_file} to database")
        return True
        
    except Exception as e:
        logger.error(f"Failed to migrate JSON state to database: {e}")
        return False


def export_db_to_json(
    output_file: Path,
    db_url: Optional[str] = None,
    schema: Optional[str] = None,
) -> bool:
    """
    Export database state to JSON file.
    
    Args:
        output_file: Path to write JSON output
        db_url: Database URL
        schema: Database schema name
        
    Returns:
        True if export successful
    """
    from src.config.database import get_database
    
    try:
        db = get_database(db_url=db_url, schema=schema or "public")
        
        # Gather all data
        export_data = {
            "config": db.get_all_config(),
            "bot_state": db.get_bot_state(),
            "positions": db.get_positions(),
            "orders": db.get_orders(),
            "recent_logs": db.get_recent_logs(100),
            "trade_history": db.get_trade_history(1000),
            "exported_at": datetime.now().isoformat(),
        }
        
        # Write to file
        output_file.parent.mkdir(parents=True, exist_ok=True)
        with open(output_file, 'w') as f:
            json.dump(export_data, f, indent=2, default=str)
        
        logger.info(f"Successfully exported database to {output_file}")
        return True
        
    except Exception as e:
        logger.error(f"Failed to export database: {e}")
        return False


def reset_database(
    db_url: Optional[str] = None,
    schema: Optional[str] = None,
    confirm: bool = False,
) -> bool:
    """
    Reset the database to initial state.
    
    WARNING: This will delete all data!
    
    Args:
        db_url: Database URL
        schema: Database schema name
        confirm: Must be True to proceed
        
    Returns:
        True if reset successful
    """
    if not confirm:
        logger.error("Database reset requires confirm=True")
        return False
    
    from src.config.database import get_database, reset_database_instance
    
    try:
        db = get_database(db_url=db_url, schema=schema or "public")
        
        # Reset all tables
        db.reset_bot_state()
        db.clear_positions()
        db.clear_orders()
        db.clear_logs()
        db.clear_commands()
        
        with db.get_connection() as conn:
            conn.execute(
                """
                TRUNCATE config, positions, orders, logs, notifications,
                         bot_commands, trade_history RESTART IDENTITY
                """
            )
        
        logger.info("Database reset successfully")
        return True
        
    except Exception as e:
        logger.error(f"Failed to reset database: {e}")
        return False


def get_database_stats(db_url: Optional[str] = None, schema: Optional[str] = None) -> dict:
    """
    Get statistics about the database.
    
    Args:
        db_url: Database URL
        schema: Database schema name
        
    Returns:
        Dictionary with database statistics
    """
    from src.config.database import get_database
    
    try:
        db = get_database(db_url=db_url, schema=schema or "public")
        
        with db.get_connection() as conn:
            stats = {
                "config_entries": conn.execute("SELECT COUNT(*) FROM config").fetchone()[0],
                "positions": conn.execute("SELECT COUNT(*) FROM positions").fetchone()[0],
                "orders": conn.execute("SELECT COUNT(*) FROM orders").fetchone()[0],
                "logs": conn.execute("SELECT COUNT(*) FROM logs").fetchone()[0],
                "pending_commands": conn.execute("SELECT COUNT(*) FROM bot_commands WHERE processed = FALSE").fetchone()[0],
                "trades_total": conn.execute("SELECT COUNT(*) FROM trade_history").fetchone()[0],
                "database_name": conn.execute("SELECT current_database()").fetchone()[0],
                "database_schema": db.schema,
                "database_size_mb": conn.execute(
                    "SELECT pg_database_size(current_database())"
                ).fetchone()[0] / (1024 * 1024),
            }
            
            # Get bot state summary
            state = db.get_bot_state()
            stats["bot_status"] = state.get("status", "UNKNOWN")
            stats["last_update"] = state.get("last_update")
            
            return stats
            
    except Exception as e:
        logger.error(f"Failed to get database stats: {e}")
        return {"error": str(e)}


def verify_database_integrity(db_url: Optional[str] = None, schema: Optional[str] = None) -> dict:
    """
    Verify database integrity and check for issues.
    
    Args:
        db_url: Database URL
        schema: Database schema name
        
    Returns:
        Dictionary with integrity check results
    """
    from src.config.database import get_database
    
    results = {
        "status": "OK",
        "issues": [],
        "checks_passed": 0,
        "checks_failed": 0,
    }
    
    try:
        db = get_database(db_url=db_url, schema=schema or "public")
        
        with db.get_connection() as conn:
            # Check all expected tables exist
            expected_tables = [
                "config",
                "bot_state",
                "positions",
                "orders",
                "logs",
                "notifications",
                "bot_commands",
                "trade_history",
            ]
            
            existing_tables = [
                row[0] for row in 
                conn.execute(
                    "SELECT table_name FROM information_schema.tables WHERE table_schema = %s",
                    [db.schema],
                ).fetchall()
            ]
            
            for table in expected_tables:
                if table in existing_tables:
                    results["checks_passed"] += 1
                else:
                    results["issues"].append(f"Missing table: {table}")
                    results["checks_failed"] += 1
            
            # Check bot_state has exactly one row
            bot_state_count = conn.execute("SELECT COUNT(*) FROM bot_state").fetchone()[0]
            if bot_state_count == 1:
                results["checks_passed"] += 1
            else:
                results["issues"].append(f"bot_state should have 1 row, has {bot_state_count}")
                results["checks_failed"] += 1
            
            # Check for orphaned records
            # (positions/orders without valid state)
            
        if results["checks_failed"] > 0:
            results["status"] = "ISSUES_FOUND"
        
        return results
        
    except Exception as e:
        results["status"] = "ERROR"
        results["issues"].append(str(e))
        return results


if __name__ == "__main__":
    import argparse
    import sys
    
    logging.basicConfig(level=logging.INFO)
    
    parser = argparse.ArgumentParser(description="Database management utilities")
    parser.add_argument("command", choices=["migrate", "export", "reset", "stats", "verify"])
    parser.add_argument("--db-url", help="PostgreSQL URL")
    parser.add_argument("--schema", help="Database schema name")
    parser.add_argument("--json-file", type=Path, help="JSON file path (for migrate/export)")
    parser.add_argument("--confirm", action="store_true", help="Confirm destructive operations")
    
    args = parser.parse_args()
    
    if args.command == "migrate":
        success = migrate_json_state_to_db(args.json_file, args.db_url, args.schema)
        sys.exit(0 if success else 1)
    
    elif args.command == "export":
        if not args.json_file:
            args.json_file = Path("data/db_export.json")
        success = export_db_to_json(args.json_file, args.db_url, args.schema)
        sys.exit(0 if success else 1)
    
    elif args.command == "reset":
        success = reset_database(args.db_url, args.schema, confirm=args.confirm)
        sys.exit(0 if success else 1)
    
    elif args.command == "stats":
        stats = get_database_stats(args.db_url, args.schema)
        print(json.dumps(stats, indent=2, default=str))
    
    elif args.command == "verify":
        results = verify_database_integrity(args.db_url, args.schema)
        print(json.dumps(results, indent=2))
        sys.exit(0 if results["status"] == "OK" else 1)
