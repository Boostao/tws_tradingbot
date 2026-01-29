from __future__ import annotations

from fastapi import APIRouter, HTTPException

from src.api.schemas import WatchlistChangeRequest, WatchlistUpdateRequest
from src.api.utils import load_watchlist, save_watchlist


router = APIRouter(tags=["watchlist"])


@router.get("/watchlist")
def get_watchlist():
    return {"symbols": load_watchlist()}


@router.put("/watchlist")
def replace_watchlist(payload: WatchlistUpdateRequest):
    symbols = [s.upper().strip() for s in payload.symbols if s.strip()]
    save_watchlist(symbols)
    return {"symbols": symbols}


@router.post("/watchlist/add")
def add_symbol(payload: WatchlistChangeRequest):
    symbol = payload.symbol.upper().strip()
    if not symbol:
        raise HTTPException(status_code=400, detail="symbol is required")
    symbols = load_watchlist()
    if symbol not in symbols:
        symbols.append(symbol)
        save_watchlist(symbols)
    return {"symbols": symbols}


@router.post("/watchlist/remove")
def remove_symbol(payload: WatchlistChangeRequest):
    symbol = payload.symbol.upper().strip()
    symbols = [s for s in load_watchlist() if s != symbol]
    save_watchlist(symbols)
    return {"symbols": symbols}
