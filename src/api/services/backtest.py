from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from typing import Dict, Optional
from uuid import uuid4
import threading
from concurrent.futures import ThreadPoolExecutor

from src.bot.backtest_runner import BacktestEngine, BacktestResult
from src.bot.strategy.rules.models import Strategy


@dataclass
class BacktestJob:
    job_id: str
    status: str
    started_at: Optional[str] = None
    finished_at: Optional[str] = None
    result: Optional[dict] = None
    error: Optional[str] = None


class BacktestManager:
    def __init__(self, max_workers: int = 2) -> None:
        self._executor = ThreadPoolExecutor(max_workers=max_workers)
        self._jobs: Dict[str, BacktestJob] = {}
        self._lock = threading.Lock()

    def start(self, strategy: Strategy, payload) -> BacktestJob:
        job_id = str(uuid4())
        job = BacktestJob(job_id=job_id, status="running", started_at=datetime.utcnow().isoformat())
        with self._lock:
            self._jobs[job_id] = job

        self._executor.submit(self._run_job, job_id, strategy, payload)
        return job

    def get(self, job_id: str) -> Optional[BacktestJob]:
        with self._lock:
            return self._jobs.get(job_id)

    def delete(self, job_id: str) -> bool:
        with self._lock:
            return self._jobs.pop(job_id, None) is not None

    def _run_job(self, job_id: str, strategy: Strategy, payload) -> None:
        try:
            engine = BacktestEngine(
                strategy=strategy,
                initial_capital=payload.initial_capital,
                use_tws_data=payload.use_tws_data,
                use_nautilus=payload.use_nautilus,
            )
            result = engine.run(
                tickers=payload.tickers,
                start_date=payload.start_date,
                end_date=payload.end_date,
                timeframe=payload.timeframe,
            )
            serialized = serialize_backtest_result(result)
            with self._lock:
                job = self._jobs[job_id]
                job.status = "completed"
                job.result = serialized
                job.finished_at = datetime.utcnow().isoformat()
        except Exception as exc:
            with self._lock:
                job = self._jobs[job_id]
                job.status = "failed"
                job.error = str(exc)
                job.finished_at = datetime.utcnow().isoformat()


def serialize_backtest_result(result: BacktestResult) -> dict:
    equity_records = []
    if not result.equity_curve.empty:
        equity_records = result.equity_curve.to_dict(orient="records")

    trades = [t.to_dict() for t in result.trades]
    metrics = result.metrics.to_dict()

    return {
        "equity_curve": equity_records,
        "trades": trades,
        "metrics": metrics,
        "start_date": result.start_date.isoformat() if result.start_date else None,
        "end_date": result.end_date.isoformat() if result.end_date else None,
        "initial_capital": result.initial_capital,
        "final_equity": result.final_equity,
        "tickers": result.tickers,
        "data_source": result.data_source,
    }


BACKTEST_MANAGER = BacktestManager()
