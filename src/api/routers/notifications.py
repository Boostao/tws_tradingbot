from __future__ import annotations

from fastapi import APIRouter, HTTPException, Query

from src.api.schemas import (
    NotificationListResponse,
    NotificationTestRequest,
    NotificationTestResponse,
)
from src.config.settings import get_settings
from src.config.database import get_database
from src.utils.notifications import NotificationManager
from src.api.services.notifications import NOTIFICATION_STORE


router = APIRouter(tags=["notifications"])


@router.post("/notifications/test", response_model=NotificationTestResponse)
async def test_notification(payload: NotificationTestRequest) -> NotificationTestResponse:
    settings = get_settings(force_reload=True)

    if not settings.notifications.enabled:
        await NOTIFICATION_STORE.add_event(payload.message, level="warning", channel=payload.channel)
        return NotificationTestResponse(status="disabled")

    notifier = NotificationManager(settings)

    channel = (payload.channel or "").lower()
    if channel not in ("", "telegram", "discord"):
        raise HTTPException(status_code=400, detail="invalid channel")

    if channel == "telegram":
        notifier._send_telegram(payload.message)
        await NOTIFICATION_STORE.add_event(payload.message, channel="telegram")
        return NotificationTestResponse(status="telegram_sent")
    if channel == "discord":
        notifier._send_discord(payload.message)
        await NOTIFICATION_STORE.add_event(payload.message, channel="discord")
        return NotificationTestResponse(status="discord_sent")

    notifier._send_telegram(payload.message)
    notifier._send_discord(payload.message)
    await NOTIFICATION_STORE.add_event(payload.message)
    return NotificationTestResponse(status="sent")


@router.get("/notifications", response_model=NotificationListResponse)
def list_notifications(
    page: int = Query(1, ge=1),
    page_size: int = Query(25, ge=1, le=200),
) -> NotificationListResponse:
    settings = get_settings(force_reload=True)
    offset = (page - 1) * page_size

    if settings.database.enabled:
        db = get_database()
        events = db.get_notifications(limit=page_size, offset=offset)
        total = db.count_notifications()
    else:
        all_events = NOTIFICATION_STORE.list_events()
        total = len(all_events)
        events = all_events[offset : offset + page_size]

    return NotificationListResponse(
        events=events,
        total=total,
        page=page,
        page_size=page_size,
    )
