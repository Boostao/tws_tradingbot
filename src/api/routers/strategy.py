from __future__ import annotations

from fastapi import APIRouter, HTTPException, UploadFile, File
from fastapi.responses import FileResponse
import json
import os

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


@router.post("/strategy/import/file", response_model=Strategy)
async def import_strategy_file(file: UploadFile = File(...)) -> Strategy:
    if not file.filename.endswith('.json'):
        raise HTTPException(status_code=400, detail="File must be a JSON file")
    
    content = await file.read()
    try:
        strategy_data = json.loads(content.decode('utf-8'))
    except json.JSONDecodeError:
        raise HTTPException(status_code=400, detail="Invalid JSON file")
    
    # Validate it's a strategy
    errors = validate_strategy(strategy_data)
    if errors:
        raise HTTPException(status_code=400, detail=f"Invalid strategy: {', '.join(errors)}")
    
    save_strategy(strategy_data)
    return strategy_data


@router.get("/strategy/export")
def export_strategy():
    strategy = load_strategy()
    # Create a temporary file
    temp_file = "/tmp/strategy_export.json"
    with open(temp_file, "w") as f:
        json.dump(strategy, f, indent=2)
    return FileResponse(
        temp_file,
        media_type='application/json',
        filename=f"{strategy.get('name', 'strategy')}_{strategy.get('version', 'v1')}.json"
    )
