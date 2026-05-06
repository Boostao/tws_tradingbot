from __future__ import annotations

from fastapi import APIRouter, HTTPException
import requests

from src.api.schemas import (
    TradingViewWatchlistImportRequest,
    WatchlistChangeRequest,
    WatchlistResponse,
    WatchlistUpdateRequest,
)
from src.api.utils import (
    import_tradingview_watchlist,
    load_watchlist,
    load_watchlist_state,
    refresh_watchlist_feed,
    save_watchlist,
    save_watchlist_state,
)


router = APIRouter(tags=["watchlist"])


def _watchlist_response(state: dict | None = None) -> WatchlistResponse:
    return WatchlistResponse(symbols=load_watchlist(), **(state or load_watchlist_state()))


def _normalized_symbol(symbol: str) -> str:
    return symbol.upper().strip()


@router.get("/watchlist")
def get_watchlist():
    return _watchlist_response()


@router.put("/watchlist")
def replace_watchlist(payload: WatchlistUpdateRequest):
    if payload.groups is not None:
        state = save_watchlist_state(
            [group.model_dump(mode="json") for group in payload.groups],
            payload.feed.model_dump(mode="json") if payload.feed else None,
        )
        return _watchlist_response(state)

    symbols = [_normalized_symbol(s) for s in payload.symbols if s.strip()]
    save_watchlist(symbols)
    return _watchlist_response()


@router.post("/watchlist/import/tradingview")
def import_watchlist_from_tradingview(payload: TradingViewWatchlistImportRequest):
    try:
        state = import_tradingview_watchlist(payload.url.strip(), current_state=load_watchlist_state())
    except requests.RequestException as exc:
        raise HTTPException(status_code=502, detail=f"tradingview import failed: {exc}") from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return _watchlist_response(state)


@router.post("/watchlist/feed/refresh")
def refresh_watchlist():
    try:
        state = refresh_watchlist_feed()
    except requests.RequestException as exc:
        raise HTTPException(status_code=502, detail=f"watchlist refresh failed: {exc}") from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return _watchlist_response(state)


@router.post("/watchlist/add")
def add_symbol(payload: WatchlistChangeRequest):
    symbol = _normalized_symbol(payload.symbol)
    if not symbol:
        raise HTTPException(status_code=400, detail="symbol is required")
    symbols = load_watchlist()
    if symbol not in symbols:
        symbols.append(symbol)
        save_watchlist(symbols)
    return _watchlist_response()


@router.post("/watchlist/remove")
def remove_symbol(payload: WatchlistChangeRequest):
    symbol = _normalized_symbol(payload.symbol)
    symbols = [s for s in load_watchlist() if s != symbol]
    save_watchlist(symbols)
    return _watchlist_response()
