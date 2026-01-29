from __future__ import annotations

from datetime import date
from typing import Any, Dict, List, Optional

from pydantic import BaseModel, Field


class BacktestRunRequest(BaseModel):
    tickers: List[str] = Field(..., min_length=1)
    start_date: date
    end_date: date
    timeframe: str = "5m"
    initial_capital: float = 10000.0
    use_tws_data: bool = True
    use_nautilus: bool = False


class BacktestRunResponse(BaseModel):
    job_id: str


class BacktestStatusResponse(BaseModel):
    job_id: str
    status: str
    error: Optional[str] = None
    started_at: Optional[str] = None
    finished_at: Optional[str] = None


class BacktestResultResponse(BaseModel):
    job_id: str
    status: str
    result: Optional[Dict[str, Any]] = None
    error: Optional[str] = None


class WatchlistUpdateRequest(BaseModel):
    symbols: List[str]


class WatchlistChangeRequest(BaseModel):
    symbol: str


class ConfigUpdateRequest(BaseModel):
    updates: Dict[str, Dict[str, Any]]


class TWSConnectionRequest(BaseModel):
    host: Optional[str] = None
    port: Optional[int] = None
    client_id: Optional[int] = None


class NotificationTestRequest(BaseModel):
    message: str = Field(..., min_length=1)
    channel: Optional[str] = Field(None, description="Optional channel: telegram or discord")


class NotificationTestResponse(BaseModel):
    status: str


class NotificationEvent(BaseModel):
    id: str
    message: str
    level: str = "info"
    channel: Optional[str] = None
    created_at: str


class NotificationListResponse(BaseModel):
    events: List[NotificationEvent]
    total: int
    page: int
    page_size: int
