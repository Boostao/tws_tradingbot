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
    symbols: List[str] = Field(default_factory=list)
    groups: Optional[List["WatchlistGroup"]] = None
    feed: Optional["WatchlistFeed"] = None


class WatchlistChangeRequest(BaseModel):
    symbol: str


class WatchlistItem(BaseModel):
    symbol: str = Field(..., min_length=1)
    exchange: str = ""
    name: str = ""
    enabled: bool = True
    instrument_id: Optional[str] = None


class WatchlistGroup(BaseModel):
    id: str = Field(..., min_length=1)
    name: str = Field(..., min_length=1)
    source: str = "manual"
    items: List[WatchlistItem] = Field(default_factory=list)


class WatchlistFeed(BaseModel):
    provider: str = Field(..., min_length=1)
    url: str = Field(..., min_length=1)
    title: Optional[str] = None
    external_id: Optional[str] = None
    last_refreshed_at: Optional[str] = None


class TradingViewWatchlistImportRequest(BaseModel):
    url: str = Field(..., min_length=1)


class WatchlistResponse(BaseModel):
    symbols: List[str] = Field(default_factory=list)
    groups: List[WatchlistGroup] = Field(default_factory=list)
    feed: Optional[WatchlistFeed] = None
    updated_at: Optional[str] = None


class CockpitStrategySummary(BaseModel):
    id: str = Field(..., min_length=1)
    name: str = Field(..., min_length=1)
    rule_count: int = 0
    enabled_rule_count: int = 0
    source: str = "library"


class CockpitStrategySlot(BaseModel):
    id: str = Field(..., min_length=1)
    label: str = Field(..., min_length=1)
    strategy_id: Optional[str] = None
    enabled: bool = True


class CockpitWorkspace(BaseModel):
    id: str = Field(..., min_length=1)
    name: str = Field(..., min_length=1)
    kind: str = "long"
    enabled: bool = True
    strategy_slots: List[CockpitStrategySlot] = Field(default_factory=list)


class CockpitStateUpdateRequest(BaseModel):
    global_enabled: bool = True
    active_workspace_id: Optional[str] = None
    workspaces: List[CockpitWorkspace] = Field(default_factory=list)


class CockpitStateResponse(BaseModel):
    global_enabled: bool = True
    active_workspace_id: Optional[str] = None
    workspaces: List[CockpitWorkspace] = Field(default_factory=list)
    strategy_library: List[CockpitStrategySummary] = Field(default_factory=list)
    feed: Optional[WatchlistFeed] = None
    updated_at: Optional[str] = None


class StrategyLibraryEntry(BaseModel):
    id: str = Field(..., min_length=1)
    name: str = Field(..., min_length=1)
    rule_count: int = 0
    enabled_rule_count: int = 0
    updated_at: Optional[str] = None


class StrategyLibrarySaveRequest(BaseModel):
    strategy: Dict[str, Any]
    name: Optional[str] = None


class ConfigUpdateRequest(BaseModel):
    updates: Dict[str, Dict[str, Any]]


class TWSConnectionRequest(BaseModel):
    host: Optional[str] = None
    port: Optional[int] = None
    client_id: Optional[int] = None


class BotStateIngestRequest(BaseModel):
    state: Dict[str, Any]


class BotCommand(BaseModel):
    id: int
    command: str
    payload: Optional[Dict[str, Any]] = None
    created_at: Optional[str] = None


class BotCommandResponse(BaseModel):
    commands: List[BotCommand]


class DiagnosticsStartup(BaseModel):
    environment: str
    trading_mode: str
    ib_host: str
    ib_port: int
    client_id: int
    account: str
    log_level: str
    log_file: str
    watchlist_path: str
    strategy_path: str
    symbol_cache_path: str


class DiagnosticsRuntime(BaseModel):
    runner_active: bool = False
    last_runtime_reload_at: Optional[str] = None
    last_runtime_reload_reason: Optional[str] = None
    last_disconnect_at: Optional[str] = None
    last_disconnect_reason: Optional[str] = None


class DiagnosticsSymbols(BaseModel):
    source: Optional[str] = None
    last_checked_at: Optional[str] = None
    last_warning: Optional[str] = None


class DiagnosticsResponse(BaseModel):
    startup: DiagnosticsStartup
    runtime: DiagnosticsRuntime
    symbols: DiagnosticsSymbols


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
