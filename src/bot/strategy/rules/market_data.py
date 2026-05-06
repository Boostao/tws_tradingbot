from __future__ import annotations

from typing import Any

import pandas as pd

from src.bot.strategy.rules.models import TimeframeUnit


TIMEFRAME_SECONDS = {
    TimeframeUnit.M1.value: 60,
    TimeframeUnit.M5.value: 300,
    TimeframeUnit.M15.value: 900,
    TimeframeUnit.M30.value: 1800,
    TimeframeUnit.H1.value: 3600,
    TimeframeUnit.H4.value: 14400,
    TimeframeUnit.D1.value: 86400,
}


def timeframe_key(timeframe: TimeframeUnit | str | None) -> str | None:
    if timeframe is None:
        return None
    return timeframe.value if hasattr(timeframe, "value") else str(timeframe)


def market_data_keys(symbol_key: str | None) -> list[str]:
    if not symbol_key:
        return []
    raw = str(symbol_key).strip()
    if not raw:
        return []
    keys = [raw, raw.upper()]
    if "." in raw:
        keys.append(raw.split(".", 1)[0].upper())
    seen: set[str] = set()
    ordered: list[str] = []
    for key in keys:
        normalized = key.strip()
        if not normalized or normalized in seen:
            continue
        seen.add(normalized)
        ordered.append(normalized)
    return ordered


def get_market_frame(
    market_data: dict[str, Any] | None,
    symbol_key: str | None,
    timeframe: TimeframeUnit | str | None = None,
) -> pd.DataFrame | None:
    if not market_data:
        return None
    requested_timeframe = timeframe_key(timeframe)
    for candidate in market_data_keys(symbol_key):
        entry = market_data.get(candidate)
        frame = frame_from_entry(entry, requested_timeframe)
        if frame is not None:
            return frame
    return None


def frame_from_entry(entry: Any, requested_timeframe: str | None = None) -> pd.DataFrame | None:
    if isinstance(entry, pd.DataFrame):
        return entry
    if not isinstance(entry, dict):
        return None
    if requested_timeframe:
        direct = entry.get(requested_timeframe)
        if isinstance(direct, pd.DataFrame):
            return direct
    best_frame: pd.DataFrame | None = None
    best_rank = 10**12
    for key, value in entry.items():
        if not isinstance(value, pd.DataFrame):
            continue
        rank = TIMEFRAME_SECONDS.get(str(key), 10**12)
        if best_frame is None or rank < best_rank:
            best_frame = value
            best_rank = rank
    return best_frame


def first_available_frame(market_data: dict[str, Any] | None) -> pd.DataFrame:
    if not market_data:
        return pd.DataFrame()
    for entry in market_data.values():
        frame = frame_from_entry(entry)
        if frame is not None:
            return frame
    return pd.DataFrame()