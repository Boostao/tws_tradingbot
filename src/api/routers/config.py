from __future__ import annotations

from fastapi import APIRouter, HTTPException

from src.api.schemas import ConfigUpdateRequest
from src.config.settings import get_redacted_settings, update_settings


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

    update_settings(payload.updates)

    return get_redacted_settings()