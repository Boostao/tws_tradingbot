from __future__ import annotations

import asyncio
from fastapi import APIRouter, WebSocket, WebSocketDisconnect

from src.api.services.backtest import BACKTEST_MANAGER
from src.api.services.notifications import NOTIFICATION_STORE
from src.bot.state import read_state
from src.api.routers.state import get_state as get_state_payload


router = APIRouter()


@router.websocket("/ws/state")
async def ws_state(websocket: WebSocket):
    await websocket.accept()
    try:
        while True:
            state = get_state_payload()
            await websocket.send_json(state)
            await asyncio.sleep(1)
    except (WebSocketDisconnect, asyncio.CancelledError):
        return


@router.websocket("/ws/logs")
async def ws_logs(websocket: WebSocket):
    await websocket.accept()
    try:
        while True:
            state = read_state().to_dict()
            await websocket.send_json({"logs": state.get("recent_logs", [])})
            await asyncio.sleep(2)
    except (WebSocketDisconnect, asyncio.CancelledError):
        return


@router.websocket("/ws/backtest/{job_id}")
async def ws_backtest(websocket: WebSocket, job_id: str):
    await websocket.accept()
    try:
        while True:
            job = BACKTEST_MANAGER.get(job_id)
            if not job:
                await websocket.send_json({"status": "not_found"})
                await asyncio.sleep(1)
                continue

            payload = {
                "job_id": job.job_id,
                "status": job.status,
                "error": job.error,
                "started_at": job.started_at,
                "finished_at": job.finished_at,
            }
            await websocket.send_json(payload)

            if job.status in ("completed", "failed"):
                return
            await asyncio.sleep(1)
    except (WebSocketDisconnect, asyncio.CancelledError):
        return


@router.websocket("/ws/notifications")
async def ws_notifications(websocket: WebSocket):
    await websocket.accept()
    await NOTIFICATION_STORE.register(websocket)
    try:
        await websocket.send_json({"events": NOTIFICATION_STORE.list_events()})
        while True:
            await asyncio.sleep(5)
    except (WebSocketDisconnect, asyncio.CancelledError):
        return
    finally:
        await NOTIFICATION_STORE.unregister(websocket)
