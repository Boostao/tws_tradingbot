from __future__ import annotations

import os

from fastapi import APIRouter

from src.api.schemas import DiagnosticsResponse, DiagnosticsRuntime, DiagnosticsStartup, DiagnosticsSymbols
from src.api.utils import get_symbol_cache_diagnostics
from src.bot.state import read_state
from src.config.settings import get_settings


router = APIRouter(tags=["diagnostics"])


def _mask_account(account: str | None) -> str:
    raw = str(account or "").strip()
    if not raw:
        return "unset"
    if len(raw) <= 4:
        return raw
    return f"{raw[:2]}***{raw[-2:]}"


@router.get("/diagnostics", response_model=DiagnosticsResponse)
def get_diagnostics() -> DiagnosticsResponse:
    settings = get_settings(force_reload=True)
    state = read_state()
    symbol_diagnostics = get_symbol_cache_diagnostics()
    return DiagnosticsResponse(
        startup=DiagnosticsStartup(
            environment=os.getenv("TRADING_BOT_ENV", "development"),
            trading_mode=settings.ib.trading_mode,
            ib_host=settings.ib.host,
            ib_port=settings.ib.port,
            client_id=settings.ib.client_id,
            account=_mask_account(settings.ib.account),
            log_level=settings.logging.level,
            log_file=settings.logging.file_path,
            watchlist_path=settings.app.watchlist_path,
            strategy_path=settings.app.active_strategy_path,
            symbol_cache_path=settings.app.symbol_cache_path,
        ),
        runtime=DiagnosticsRuntime(
            runner_active=state.runner_active,
            last_runtime_reload_at=state.last_runtime_reload_at,
            last_runtime_reload_reason=state.last_runtime_reload_reason,
            last_disconnect_at=state.last_disconnect_at,
            last_disconnect_reason=state.last_disconnect_reason or None,
        ),
        symbols=DiagnosticsSymbols(
            source=symbol_diagnostics.get("source"),
            last_checked_at=symbol_diagnostics.get("last_checked_at"),
            last_warning=symbol_diagnostics.get("last_warning"),
        ),
    )