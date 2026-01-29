from __future__ import annotations

from fastapi import APIRouter, HTTPException

from src.api.utils import load_strategy, save_strategy
from src.bot.strategy.rules.models import Strategy
from src.bot.strategy.validator import validate_strategy


router = APIRouter(tags=["strategy"])


@router.get("/strategy", response_model=Strategy)
def get_strategy() -> Strategy:
    return load_strategy()


@router.put("/strategy", response_model=Strategy)
def update_strategy(strategy: Strategy) -> Strategy:
    save_strategy(strategy)
    return strategy


@router.post("/strategy/validate")
def validate_strategy_endpoint(strategy: Strategy):
    errors = validate_strategy(strategy)
    return {
        "valid": len(errors) == 0,
        "errors": errors,
    }


@router.post("/strategy/import", response_model=Strategy)
def import_strategy(strategy: Strategy) -> Strategy:
    save_strategy(strategy)
    return strategy


@router.get("/strategy/export", response_model=Strategy)
def export_strategy() -> Strategy:
    return load_strategy()
