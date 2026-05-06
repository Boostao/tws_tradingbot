from __future__ import annotations

import json
import logging
from dataclasses import dataclass, field
from datetime import datetime, timezone
from enum import Enum
from pathlib import Path
from typing import Any, Dict, List, Optional


STATE_FILE = Path(__file__).parent.parent.parent / "config" / ".bot_state.json"
LEDGER_FILE = Path(__file__).parent.parent.parent / "config" / ".bot_ledger.json"
COMMAND_FILE = Path(__file__).parent.parent.parent / "config" / ".bot_command"
STOP_SIGNAL_FILE = Path(__file__).parent.parent.parent / "config" / ".stop_signal"
EMERGENCY_STOP_FILE = Path(__file__).parent.parent.parent / "config" / ".emergency_stop"

MAX_RECENT_LOGS = 50
MAX_RECENT_ORDERS = 25
MAX_RECENT_TRADES = 25
MAX_PROCESSED_EXECUTION_IDS = 500
MAX_CLOSED_TRADES = 100


logger = logging.getLogger(__name__)


class BotStatus(str, Enum):
    STOPPED = "STOPPED"
    RUNNING = "RUNNING"
    STARTING = "STARTING"
    STOPPING = "STOPPING"
    ERROR = "ERROR"
    DISCONNECTED = "DISCONNECTED"


@dataclass
class BotState:
    status: str = BotStatus.STOPPED.value
    tws_connected: bool = False
    equity: float = 0.0
    daily_pnl: float = 0.0
    daily_pnl_percent: float = 0.0
    total_pnl: float = 0.0
    active_strategy: str = ""
    open_positions_count: int = 0
    pending_orders_count: int = 0
    trades_today: int = 0
    win_rate_today: float = 0.0
    last_update: Optional[str] = None
    last_heartbeat: Optional[str] = None
    recent_logs: List[str] = field(default_factory=list)
    recent_orders: List[Dict[str, Any]] = field(default_factory=list)
    recent_trades: List[Dict[str, Any]] = field(default_factory=list)
    last_dry_run: Optional[Dict[str, Any]] = None
    last_runtime_reload_at: Optional[str] = None
    last_runtime_reload_reason: Optional[str] = None
    last_disconnect_at: Optional[str] = None
    last_disconnect_reason: str = ""
    error_message: str = ""

    def prune(self) -> "BotState":
        self.recent_logs = self.recent_logs[-MAX_RECENT_LOGS:]
        self.recent_orders = self.recent_orders[-MAX_RECENT_ORDERS:]
        self.recent_trades = self.recent_trades[-MAX_RECENT_TRADES:]
        return self

    @property
    def runner_active(self) -> bool:
        return self.status in {
            BotStatus.RUNNING.value,
            BotStatus.STARTING.value,
            BotStatus.STOPPING.value,
        }

    def to_dict(self) -> Dict[str, Any]:
        self.prune()
        return {
            "status": self.status,
            "tws_connected": self.tws_connected,
            "equity": self.equity,
            "daily_pnl": self.daily_pnl,
            "daily_pnl_percent": self.daily_pnl_percent,
            "total_pnl": self.total_pnl,
            "active_strategy": self.active_strategy,
            "open_positions_count": self.open_positions_count,
            "pending_orders_count": self.pending_orders_count,
            "trades_today": self.trades_today,
            "win_rate_today": self.win_rate_today,
            "last_update": self.last_update,
            "last_heartbeat": self.last_heartbeat,
            "recent_logs": self.recent_logs,
            "recent_orders": self.recent_orders,
            "recent_trades": self.recent_trades,
            "last_dry_run": self.last_dry_run,
            "last_runtime_reload_at": self.last_runtime_reload_at,
            "last_runtime_reload_reason": self.last_runtime_reload_reason,
            "last_disconnect_at": self.last_disconnect_at,
            "last_disconnect_reason": self.last_disconnect_reason,
            "error_message": self.error_message,
            "runner_active": self.runner_active,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "BotState":
        return cls(
            status=data.get("status", BotStatus.STOPPED.value),
            tws_connected=bool(data.get("tws_connected", False)),
            equity=float(data.get("equity", 0.0) or 0.0),
            daily_pnl=float(data.get("daily_pnl", 0.0) or 0.0),
            daily_pnl_percent=float(data.get("daily_pnl_percent", 0.0) or 0.0),
            total_pnl=float(data.get("total_pnl", 0.0) or 0.0),
            active_strategy=str(data.get("active_strategy", "") or ""),
            open_positions_count=int(data.get("open_positions_count", 0) or 0),
            pending_orders_count=int(data.get("pending_orders_count", 0) or 0),
            trades_today=int(data.get("trades_today", 0) or 0),
            win_rate_today=float(data.get("win_rate_today", 0.0) or 0.0),
            last_update=data.get("last_update"),
            last_heartbeat=data.get("last_heartbeat"),
            recent_logs=list(data.get("recent_logs", []) or []),
            recent_orders=list(data.get("recent_orders", []) or []),
            recent_trades=list(data.get("recent_trades", []) or []),
            last_dry_run=data.get("last_dry_run"),
            last_runtime_reload_at=data.get("last_runtime_reload_at"),
            last_runtime_reload_reason=data.get("last_runtime_reload_reason"),
            last_disconnect_at=data.get("last_disconnect_at"),
            last_disconnect_reason=str(data.get("last_disconnect_reason", "") or ""),
            error_message=str(data.get("error_message", "") or ""),
        )


@dataclass
class TradeLedger:
    processed_execution_ids: List[str] = field(default_factory=list)
    open_positions: Dict[str, Dict[str, Any]] = field(default_factory=dict)
    closed_trades: List[Dict[str, Any]] = field(default_factory=list)

    def prune(self) -> "TradeLedger":
        self.processed_execution_ids = self.processed_execution_ids[-MAX_PROCESSED_EXECUTION_IDS:]
        self.closed_trades = self.closed_trades[-MAX_CLOSED_TRADES:]
        return self

    def to_dict(self) -> Dict[str, Any]:
        self.prune()
        return {
            "processed_execution_ids": self.processed_execution_ids,
            "open_positions": self.open_positions,
            "closed_trades": self.closed_trades,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "TradeLedger":
        return cls(
            processed_execution_ids=list(data.get("processed_execution_ids", []) or []),
            open_positions=dict(data.get("open_positions", {}) or {}),
            closed_trades=list(data.get("closed_trades", []) or []),
        )


def _ensure_parent(path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)


def read_state() -> BotState:
    if not STATE_FILE.exists():
        return BotState()
    try:
        with open(STATE_FILE, "r") as state_file:
            return BotState.from_dict(json.load(state_file))
    except (OSError, json.JSONDecodeError, TypeError, ValueError) as exc:
        logger.warning("Failed to read bot state file %s: %s", STATE_FILE, exc)
        return BotState(status=BotStatus.ERROR.value, error_message="Could not read bot state")


def update_state(state: BotState) -> BotState:
    state.prune()
    state.last_update = datetime.now(timezone.utc).isoformat()
    _ensure_parent(STATE_FILE)
    with open(STATE_FILE, "w") as state_file:
        json.dump(state.to_dict(), state_file, indent=2)
    return state


def read_trade_ledger() -> TradeLedger:
    if not LEDGER_FILE.exists():
        return TradeLedger()
    try:
        with open(LEDGER_FILE, "r") as ledger_file:
            return TradeLedger.from_dict(json.load(ledger_file))
    except (OSError, json.JSONDecodeError, TypeError, ValueError) as exc:
        logger.warning("Failed to read trade ledger file %s: %s", LEDGER_FILE, exc)
        return TradeLedger()


def update_trade_ledger(ledger: TradeLedger) -> TradeLedger:
    ledger.prune()
    _ensure_parent(LEDGER_FILE)
    with open(LEDGER_FILE, "w") as ledger_file:
        json.dump(ledger.to_dict(), ledger_file, indent=2)
    return ledger


def write_start_command(payload: Optional[Dict[str, Any]] = None) -> None:
    _ensure_parent(COMMAND_FILE)
    with open(COMMAND_FILE, "w") as command_file:
        json.dump({"command": "start", "payload": payload or {}}, command_file)


def write_stop_signal() -> None:
    _ensure_parent(STOP_SIGNAL_FILE)
    STOP_SIGNAL_FILE.touch()


def write_emergency_stop() -> None:
    _ensure_parent(EMERGENCY_STOP_FILE)
    EMERGENCY_STOP_FILE.touch()


def clear_stop_signals() -> None:
    STOP_SIGNAL_FILE.unlink(missing_ok=True)
    EMERGENCY_STOP_FILE.unlink(missing_ok=True)