from __future__ import annotations

from fastapi import APIRouter, Query

from src.api.utils import get_symbol_cache


router = APIRouter(tags=["symbols"])


@router.get("/symbols")
def get_symbols(
    q: str | None = Query(default=None, min_length=1),
    symbol_type: str | None = Query(default=None, alias="type"),
    exchange: str | None = None,
    refresh: bool = False,
    limit: int | None = None,
):
    symbols, source, updated_at = get_symbol_cache(refresh=refresh)
    filtered = symbols
    if q:
        needle = q.lower()
        filtered = [
            item
            for item in filtered
            if needle in str(item.get("symbol", "")).lower()
            or needle in str(item.get("name", "")).lower()
        ]
        def sort_key(item: dict):
            symbol = str(item.get("symbol", "")).lower()
            name = str(item.get("name", "")).lower()
            exact = symbol == needle
            starts = symbol.startswith(needle)
            name_starts = name.startswith(needle)
            return (
                0 if exact else 1,
                0 if starts else 1,
                0 if name_starts else 1,
                len(symbol) if symbol else 9999,
                symbol,
            )
        filtered = sorted(filtered, key=sort_key)
    if symbol_type:
        filtered = [
            item
            for item in filtered
            if str(item.get("type", "")).lower() == symbol_type.lower()
        ]
    if exchange:
        filtered = [
            item
            for item in filtered
            if str(item.get("exchange", "")).lower() == exchange.lower()
        ]
    if limit and limit > 0:
        filtered = filtered[:limit]
    return {"symbols": filtered, "source": source, "updated_at": updated_at}
