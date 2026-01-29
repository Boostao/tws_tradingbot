from __future__ import annotations

from fastapi import APIRouter, HTTPException

from src.api.schemas import (
    BacktestRunRequest,
    BacktestRunResponse,
    BacktestStatusResponse,
    BacktestResultResponse,
)
from src.api.services.backtest import BACKTEST_MANAGER
from src.api.utils import load_strategy


router = APIRouter(tags=["backtest"])


@router.post("/backtest/run", response_model=BacktestRunResponse)
def run_backtest(payload: BacktestRunRequest) -> BacktestRunResponse:
    if payload.start_date >= payload.end_date:
        raise HTTPException(status_code=400, detail="start_date must be before end_date")
    strategy = load_strategy()
    job = BACKTEST_MANAGER.start(strategy, payload)
    return BacktestRunResponse(job_id=job.job_id)


@router.get("/backtest/{job_id}", response_model=BacktestStatusResponse)
def get_backtest_status(job_id: str) -> BacktestStatusResponse:
    job = BACKTEST_MANAGER.get(job_id)
    if not job:
        raise HTTPException(status_code=404, detail="backtest job not found")
    return BacktestStatusResponse(
        job_id=job.job_id,
        status=job.status,
        error=job.error,
        started_at=job.started_at,
        finished_at=job.finished_at,
    )


@router.get("/backtest/{job_id}/results", response_model=BacktestResultResponse)
def get_backtest_results(job_id: str) -> BacktestResultResponse:
    job = BACKTEST_MANAGER.get(job_id)
    if not job:
        raise HTTPException(status_code=404, detail="backtest job not found")
    return BacktestResultResponse(
        job_id=job.job_id,
        status=job.status,
        result=job.result,
        error=job.error,
    )


@router.delete("/backtest/{job_id}")
def delete_backtest(job_id: str):
    deleted = BACKTEST_MANAGER.delete(job_id)
    if not deleted:
        raise HTTPException(status_code=404, detail="backtest job not found")
    return {"deleted": True}
