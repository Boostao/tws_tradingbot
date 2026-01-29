from __future__ import annotations

import asyncio
from collections import deque
from dataclasses import dataclass
from datetime import datetime
from typing import Deque, List, Optional
from uuid import uuid4

from fastapi import WebSocket

from src.config.settings import get_settings
from src.config.database import get_database


@dataclass
class NotificationEvent:
    id: str
    message: str
    level: str = "info"
    channel: Optional[str] = None
    created_at: str = ""

    def to_dict(self) -> dict:
        return {
            "id": self.id,
            "message": self.message,
            "level": self.level,
            "channel": self.channel,
            "created_at": self.created_at,
        }


class NotificationStore:
    def __init__(self, maxlen: int = 200) -> None:
        self._events: Deque[NotificationEvent] = deque(maxlen=maxlen)
        self._connections: set[WebSocket] = set()
        self._lock = asyncio.Lock()
        self._load_from_db(maxlen)

    def _db_enabled(self) -> bool:
        try:
            settings = get_settings(force_reload=True)
            return settings.database.enabled
        except Exception:
            return False

    def _load_from_db(self, limit: int) -> None:
        if not self._db_enabled():
            return
        try:
            db = get_database()
            rows = db.get_notifications(limit, offset=0)
            for row in rows:
                self._events.append(
                    NotificationEvent(
                        id=str(row.get("id")),
                        message=row.get("message", ""),
                        level=row.get("level", "info"),
                        channel=row.get("channel"),
                        created_at=row.get("created_at", ""),
                    )
                )
        except Exception:
            return

    def list_events(self) -> List[dict]:
        return [event.to_dict() for event in list(self._events)]

    async def add_event(self, message: str, level: str = "info", channel: Optional[str] = None) -> NotificationEvent:
        event = NotificationEvent(
            id=str(uuid4()),
            message=message,
            level=level,
            channel=channel,
            created_at=datetime.utcnow().isoformat(),
        )
        self._events.append(event)
        if self._db_enabled():
            try:
                db = get_database()
                db.add_notification(level=level, message=message, channel=channel)
            except Exception:
                pass
        await self._broadcast({"event": event.to_dict()})
        return event

    async def register(self, websocket: WebSocket) -> None:
        async with self._lock:
            self._connections.add(websocket)

    async def unregister(self, websocket: WebSocket) -> None:
        async with self._lock:
            self._connections.discard(websocket)

    async def _broadcast(self, payload: dict) -> None:
        async with self._lock:
            connections = list(self._connections)

        for websocket in connections:
            try:
                await websocket.send_json(payload)
            except Exception:
                async with self._lock:
                    self._connections.discard(websocket)


NOTIFICATION_STORE = NotificationStore()
