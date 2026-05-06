from __future__ import annotations

import socket
from datetime import datetime, timezone

from fastapi import APIRouter, HTTPException

from src.api.schemas import BotStateIngestRequest, TWSConnectionRequest
from src.api.utils import load_cockpit_state
from src.bot.live_runner import LiveTradingRunner
from src.bot.state import BotState, BotStatus, clear_stop_signals, read_state, update_state, write_emergency_stop, write_start_command, write_stop_signal
from src.config.settings import get_settings, update_settings


router = APIRouter(tags=["state"])


def _active_strategy_name() -> str:
    cockpit = load_cockpit_state()
    active_workspace_id = cockpit.get("active_workspace_id")
    strategy_library = {item["id"]: item["name"] for item in cockpit.get("strategy_library", [])}
    for workspace in cockpit.get("workspaces", []):
        if workspace.get("id") != active_workspace_id:
            continue
        slots = workspace.get("strategy_slots", [])
        if not slots:
            return ""
        strategy_id = slots[0].get("strategy_id")
        return strategy_library.get(strategy_id, "") if strategy_id else ""
    return ""


def _check_tws_connection(host: str, port: int, timeout: int) -> None:
    with socket.create_connection((host, port), timeout=timeout):
        return


@router.get("/state")
def get_state():
    state = read_state()
    if state.status != BotStatus.RUNNING.value or not state.active_strategy:
        state.active_strategy = _active_strategy_name()
    return state.to_dict()


@router.post("/state/ingest")
def ingest_state(payload: BotStateIngestRequest):
    state = BotState.from_dict(payload.state)
    update_state(state)
    return {"status": "ok"}


@router.post("/bot/start")
def start_bot():
    clear_stop_signals()
    write_start_command()
    state = read_state()
    state.status = BotStatus.STARTING.value
    state.active_strategy = _active_strategy_name()
    update_state(state)
    return {"status": "starting"}


@router.post("/bot/stop")
def stop_bot():
    write_stop_signal()
    state = read_state()
    state.status = BotStatus.STOPPING.value
    update_state(state)
    return {"status": "stopping"}


@router.post("/bot/emergency_stop")
def emergency_stop():
    write_emergency_stop()
    state = read_state()
    state.status = BotStatus.ERROR.value
    state.error_message = "Emergency stop requested"
    update_state(state)
    return {"status": "emergency_stop"}


@router.post("/bot/dry_run")
def dry_run_bot():
    runner = LiveTradingRunner()
    try:
        return runner.dry_run_once()
    except RuntimeError as exc:
        raise HTTPException(status_code=503, detail=str(exc)) from exc


@router.post("/tws/connect")
def connect_tws(payload: TWSConnectionRequest):
    settings = get_settings(force_reload=True)
    host = payload.host or settings.ib.host
    port = int(payload.port or settings.ib.port)
    client_id = int(payload.client_id or settings.ib.client_id)

    try:
        _check_tws_connection(host, port, settings.ib.timeout)
    except OSError as exc:
        state = read_state()
        state.tws_connected = False
        state.error_message = str(exc)
        update_state(state)
        raise HTTPException(status_code=503, detail=f"Could not connect to TWS at {host}:{port}: {exc}") from exc

    update_settings({"ib": {"host": host, "port": port, "client_id": client_id}})

    state = read_state()
    state.tws_connected = True
    state.error_message = ""
    if state.status == BotStatus.DISCONNECTED.value:
        state.status = BotStatus.STOPPED.value
    update_state(state)
    return {"status": "connected", "host": host, "port": port, "client_id": client_id}


@router.post("/tws/disconnect")
def disconnect_tws():
    state = read_state()
    state.tws_connected = False
    state.error_message = ""
    state.last_disconnect_at = datetime.now(timezone.utc).isoformat()
    state.last_disconnect_reason = "Manual disconnect requested"
    if state.status == BotStatus.RUNNING.value:
        state.status = BotStatus.DISCONNECTED.value
    update_state(state)
    return {"status": "disconnected"}