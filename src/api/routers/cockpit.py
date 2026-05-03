from __future__ import annotations

from fastapi import APIRouter

from src.api.schemas import CockpitStateResponse, CockpitStateUpdateRequest
from src.api.utils import load_cockpit_state, save_cockpit_state


router = APIRouter(tags=["cockpit"])


@router.get("/cockpit", response_model=CockpitStateResponse)
def get_cockpit_state() -> CockpitStateResponse:
    return CockpitStateResponse(**load_cockpit_state())


@router.put("/cockpit", response_model=CockpitStateResponse)
def update_cockpit_state(payload: CockpitStateUpdateRequest) -> CockpitStateResponse:
    return CockpitStateResponse(**save_cockpit_state(payload.model_dump(mode="json")))