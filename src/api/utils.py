from __future__ import annotations

import json
import os
from datetime import datetime, timezone
from dataclasses import asdict
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple
from uuid import uuid4

from src.bot.strategy.rules.models import Strategy
from src.config.settings import get_settings
import requests


def _reload_signal_path() -> Path:
    return Path(__file__).parent.parent.parent / "config" / ".reload_signal"


def _create_reload_signal() -> None:
    """Create reload signal file to trigger hot-reload in the bot."""
    signal_path = _reload_signal_path()
    signal_path.parent.mkdir(parents=True, exist_ok=True)
    signal_path.touch()
    # Note: The bot will remove this file after detecting it


_PROJECT_ROOT = Path(__file__).parent.parent.parent


def _resolve_path(path_value: str) -> Path:
    path = Path(path_value)
    return path if path.is_absolute() else (_PROJECT_ROOT / path)


def _watchlist_path() -> Path:
    settings = get_settings()
    return _resolve_path(settings.app.watchlist_path)


def _active_strategy_path() -> Path:
    settings = get_settings()
    return _resolve_path(settings.app.active_strategy_path)


def _symbol_cache_path() -> Path:
    settings = get_settings()
    return _resolve_path(settings.app.symbol_cache_path)


def load_watchlist() -> List[str]:
    symbols: List[str] = []
    watchlist_path = _watchlist_path()
    if watchlist_path.exists():
        with open(watchlist_path, "r") as f:
            for line in f:
                line = line.strip()
                if line and not line.startswith("#"):
                    symbols.append(line.upper())
    return symbols


def save_watchlist(symbols: List[str]) -> None:
    watchlist_path = _watchlist_path()
    watchlist_path.parent.mkdir(parents=True, exist_ok=True)
    with open(watchlist_path, "w") as f:
        f.write("# TWS Traderbot Watchlist\n")
        for symbol in symbols:
            if symbol.strip():
                f.write(f"{symbol.upper()}\n")
    _create_reload_signal()


def load_strategy() -> Strategy:
    strategy_path = _active_strategy_path()
    if strategy_path.exists():
        with open(strategy_path, "r") as f:
            data = json.load(f)
        return Strategy.model_validate(data)

    return Strategy(
        id=str(uuid4()),
        name="New Strategy",
        version="1.0.0",
        description="Created with API",
        rules=[],
    )


def save_strategy(strategy: Strategy) -> None:
    strategy_path = _active_strategy_path()
    strategy_path.parent.mkdir(parents=True, exist_ok=True)
    with open(strategy_path, "w") as f:
        json.dump(strategy.model_dump(mode="json"), f, indent=2, default=str)
    _create_reload_signal()


def _read_symbol_cache_file() -> Tuple[List[Dict[str, Any]], Optional[str], Optional[str]]:
    symbol_cache_path = _symbol_cache_path()
    if symbol_cache_path.exists():
        try:
            with open(symbol_cache_path, "r") as f:
                data = json.load(f)
            if isinstance(data, list):
                return data, "cache", None
            if isinstance(data, dict) and isinstance(data.get("symbols"), list):
                return data.get("symbols", []), data.get("source"), data.get("updated_at")
        except Exception:
            return [], None, None
    return [], None, None


def _write_symbol_cache(symbols: List[Dict[str, Any]], source: str) -> None:
    symbol_cache_path = _symbol_cache_path()
    symbol_cache_path.parent.mkdir(parents=True, exist_ok=True)
    payload = {
        "symbols": symbols,
        "source": source,
        "updated_at": datetime.now(timezone.utc).isoformat(),
    }
    with open(symbol_cache_path, "w") as f:
        json.dump(payload, f)


def _fetch_tradingview_scan(
    url: str,
    filters: List[Dict[str, Any]],
    range_end: int,
) -> List[Dict[str, Any]]:
    payload = {
        "filter": filters,
        "options": {"lang": "en"},
        "symbols": {"query": {"types": []}, "tickers": []},
        "columns": ["name", "description", "type", "subtype", "exchange"],
        "sort": {"sortBy": "name", "sortOrder": "asc"},
        "range": [0, range_end],
    }
    response = requests.post(url, json=payload, timeout=30)
    response.raise_for_status()
    data = response.json()
    symbols: List[Dict[str, Any]] = []
    for item in data.get("data", []):
        symbol_data = item.get("d", [])
        if len(symbol_data) >= 5:
            full_symbol = item.get("s", "")
            parts = full_symbol.split(":")
            symbol = parts[1] if len(parts) > 1 else parts[0]
            symbols.append(
                {
                    "symbol": symbol,
                    "name": symbol_data[1] or symbol_data[0] or symbol,
                    "exchange": symbol_data[4] or "US",
                    "type": symbol_data[2] or "stock",
                }
            )
    return symbols


def _fetch_symbols_from_tradingview() -> List[Dict[str, Any]]:
    stock_filters = [
        {"left": "type", "operation": "in_range", "right": ["stock", "dr", "fund"]},
        {
            "left": "subtype",
            "operation": "in_range",
            "right": [
                "common",
                "foreign-issuer",
                "",
                "etf",
                "etf,odd",
                "etf,otc",
                "etf,cfd",
            ],
        },
        {"left": "exchange", "operation": "in_range", "right": ["NYSE", "NASDAQ", "AMEX", "TSX", "TSXV"]},
        {"left": "is_primary", "operation": "equal", "right": True},
    ]
    canada_stock_filters = [
        {"left": "type", "operation": "in_range", "right": ["stock", "dr", "fund"]},
        {
            "left": "subtype",
            "operation": "in_range",
            "right": [
                "common",
                "foreign-issuer",
                "",
                "etf",
                "etf,odd",
                "etf,otc",
                "etf,cfd",
            ],
        },
        {"left": "exchange", "operation": "in_range", "right": ["TSX", "TSXV"]},
        {"left": "is_primary", "operation": "equal", "right": True},
    ]
    crypto_filters = [
        {"left": "type", "operation": "equal", "right": "crypto"},
    ]
    forex_filters = [
        {"left": "type", "operation": "equal", "right": "forex"},
    ]

    symbols: List[Dict[str, Any]] = []
    symbols.extend(
        _fetch_tradingview_scan(
            "https://scanner.tradingview.com/america/scan",
            stock_filters,
            36000,
        )
    )
    symbols.extend(
        _fetch_tradingview_scan(
            "https://scanner.tradingview.com/canada/scan",
            canada_stock_filters,
            12000,
        )
    )
    symbols.extend(
        _fetch_tradingview_scan(
            "https://scanner.tradingview.com/crypto/scan",
            crypto_filters,
            15000,
        )
    )
    symbols.extend(
        _fetch_tradingview_scan(
            "https://scanner.tradingview.com/forex/scan",
            forex_filters,
            12000,
        )
    )

    unique: Dict[str, Dict[str, Any]] = {}
    for item in symbols:
        key = f"{item.get('symbol', '')}:{item.get('exchange', '')}:{item.get('type', '')}".upper()
        if key and key not in unique:
            unique[key] = item
    return list(unique.values())


def get_symbol_cache(refresh: bool = False) -> Tuple[List[Dict[str, Any]], str, Optional[str]]:
    cached_symbols, cached_source, cached_updated_at = _read_symbol_cache_file()
    cache_ttl = int(os.getenv("SYMBOL_CACHE_TTL_SECONDS", "86400"))
    symbol_cache_path = _symbol_cache_path()
    is_stale = False
    if symbol_cache_path.exists():
        try:
            mtime = symbol_cache_path.stat().st_mtime
            is_stale = (datetime.now(timezone.utc).timestamp() - mtime) > cache_ttl
        except Exception:
            is_stale = True

    if refresh or is_stale or not cached_symbols:
        try:
            symbols = _fetch_symbols_from_tradingview()
            if len(symbols) > 1000:
                _write_symbol_cache(symbols, "tradingview")
                return symbols, "tradingview", datetime.now(timezone.utc).isoformat()
        except Exception:
            pass

    if cached_symbols:
        return cached_symbols, cached_source or "cache", cached_updated_at

    return [], "local", cached_updated_at


def get_redacted_settings() -> Dict[str, Any]:
    settings = get_settings(force_reload=True)
    data = asdict(settings)

    if "auth" in data:
        data["auth"]["password"] = "***"
    if "notifications" in data:
        telegram = data["notifications"].get("telegram", {})
        discord = data["notifications"].get("discord", {})
        if "bot_token" in telegram and telegram["bot_token"]:
            telegram["bot_token"] = "***"
        if "webhook_url" in discord and discord["webhook_url"]:
            discord["webhook_url"] = "***"
    return data
