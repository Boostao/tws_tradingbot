from __future__ import annotations

from fastapi import APIRouter, HTTPException, UploadFile, File
from fastapi.responses import FileResponse
import json
import traceback

from src.api.schemas import StrategyLibraryEntry, StrategyLibrarySaveRequest
from src.api.utils import (
    apply_strategy_preset,
    delete_strategy_preset,
    list_strategy_presets,
    load_strategy,
    load_strategy_preset,
    save_strategy,
    save_strategy_preset,
    update_strategy_preset,
)
from src.bot.strategy.rules.models import Strategy
from src.bot.strategy.pine_script import strategy_to_pine_script
from src.bot.strategy.validator import validate_strategy
from src.utils.logger import get_logger


router = APIRouter(tags=["strategy"])
logger = get_logger(__name__)


@router.get("/strategy", response_model=Strategy)
def get_strategy() -> Strategy:
    try:
        return load_strategy()
    except Exception as e:
        logger.error(f"Error loading strategy: {e}")
        logger.error(traceback.format_exc())
        raise HTTPException(status_code=500, detail=f"Internal Server Error: {str(e)}")


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
    parsed_strategy = Strategy.model_validate(strategy_data)
    errors = validate_strategy(parsed_strategy)
    if errors:
        raise HTTPException(status_code=400, detail=f"Invalid strategy: {', '.join(errors)}")
    
    save_strategy(parsed_strategy)
    return parsed_strategy


@router.get("/strategy/export")
def export_strategy():
    strategy = load_strategy()
    # Create a temporary file
    temp_file = "/tmp/strategy_export.json"
    with open(temp_file, "w") as f:
        json.dump(strategy.model_dump(mode="json"), f, indent=2)
    return FileResponse(
        temp_file,
        media_type='application/json',
        filename=f"{strategy.name or 'strategy'}_{strategy.version or 'v1'}.json"
    )


@router.get("/strategy/pine-script")
def get_strategy_pine_script():
    strategy = load_strategy()
    result = strategy_to_pine_script(strategy)
    return {
        "script": result.script,
        "warnings": result.warnings,
    }


@router.get("/strategy/library", response_model=list[StrategyLibraryEntry])
def get_strategy_library() -> list[StrategyLibraryEntry]:
    return [StrategyLibraryEntry(**entry) for entry in list_strategy_presets()]


@router.post("/strategy/library/save", response_model=StrategyLibraryEntry)
def create_strategy_preset(payload: StrategyLibrarySaveRequest) -> StrategyLibraryEntry:
    strategy = Strategy.model_validate(payload.strategy)
    return StrategyLibraryEntry(**save_strategy_preset(strategy, payload.name))


@router.put("/strategy/library/{strategy_id}", response_model=StrategyLibraryEntry)
def update_saved_strategy(strategy_id: str, payload: StrategyLibrarySaveRequest) -> StrategyLibraryEntry:
    strategy = Strategy.model_validate(payload.strategy)
    try:
        return StrategyLibraryEntry(**update_strategy_preset(strategy_id, strategy, payload.name))
    except FileNotFoundError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc


@router.get("/strategy/library/{strategy_id}", response_model=Strategy)
def get_strategy_preset(strategy_id: str) -> Strategy:
    try:
        return load_strategy_preset(strategy_id)
    except FileNotFoundError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc


@router.post("/strategy/library/{strategy_id}/apply", response_model=Strategy)
def apply_saved_strategy(strategy_id: str) -> Strategy:
    try:
        return apply_strategy_preset(strategy_id)
    except FileNotFoundError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc


@router.delete("/strategy/library/{strategy_id}")
def remove_strategy_preset(strategy_id: str):
    try:
        delete_strategy_preset(strategy_id)
    except FileNotFoundError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return {"status": "deleted"}
