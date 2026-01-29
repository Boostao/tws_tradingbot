from __future__ import annotations

from fastapi import APIRouter, HTTPException

from dataclasses import asdict

from src.api.schemas import ConfigUpdateRequest
from src.api.utils import get_redacted_settings
from src.config.settings import get_settings, update_setting


router = APIRouter(tags=["config"])


@router.get("/config")
def get_config():
    return get_redacted_settings()


@router.put("/config")
def update_config(payload: ConfigUpdateRequest):
    if not payload.updates:
        raise HTTPException(status_code=400, detail="updates payload is empty")

    for section, values in payload.updates.items():
        if not isinstance(values, dict):
            raise HTTPException(status_code=400, detail=f"invalid section: {section}")
        if section == "notifications":
            settings = get_settings(force_reload=True)
            current = asdict(settings.notifications)
            merged = {**current}

            for key, value in values.items():
                if isinstance(value, dict) and isinstance(merged.get(key), dict):
                    merged[key] = {**merged[key], **value}
                else:
                    merged[key] = value

            telegram = merged.get("telegram", {})
            if telegram.get("bot_token") in (None, "", "***"):
                telegram["bot_token"] = current.get("telegram", {}).get("bot_token", "")
            merged["telegram"] = telegram

            discord = merged.get("discord", {})
            if discord.get("webhook_url") in (None, "", "***"):
                discord["webhook_url"] = current.get("discord", {}).get("webhook_url", "")
            merged["discord"] = discord

            update_setting("notifications", "enabled", merged.get("enabled", False))
            update_setting("notifications", "telegram", merged.get("telegram", {}))
            update_setting("notifications", "discord", merged.get("discord", {}))
            continue

        for key, value in values.items():
            update_setting(section, key, value)

    return get_redacted_settings()
