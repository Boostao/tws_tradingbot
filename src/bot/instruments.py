from __future__ import annotations

from dataclasses import dataclass


DEFAULT_VENUE = "SMART"


@dataclass(frozen=True)
class InstrumentDescriptor:
    symbol: str
    venue: str

    @property
    def instrument_id(self) -> str:
        return f"{self.symbol}.{self.venue}"


def normalize_symbol(symbol: str) -> str:
    return symbol.strip().upper().replace(" ", "-")


def normalize_venue(exchange: str | None) -> str:
    venue = (exchange or "").strip().upper()
    return venue or DEFAULT_VENUE


def normalize_instrument(symbol: str, exchange: str | None = None) -> InstrumentDescriptor:
    normalized_symbol = normalize_symbol(symbol)
    if not normalized_symbol:
        raise ValueError("symbol is required")
    if "." in normalized_symbol and not exchange:
        left, right = normalized_symbol.split(".", 1)
        return InstrumentDescriptor(symbol=normalize_symbol(left), venue=normalize_venue(right))
    return InstrumentDescriptor(symbol=normalized_symbol, venue=normalize_venue(exchange))


def normalize_instrument_id(symbol: str, exchange: str | None = None) -> str:
    return normalize_instrument(symbol, exchange).instrument_id


def split_instrument_id(instrument_id: str) -> InstrumentDescriptor:
    normalized = instrument_id.strip().upper()
    if not normalized:
        raise ValueError("instrument_id is required")
    if "." not in normalized:
        return InstrumentDescriptor(symbol=normalized, venue=DEFAULT_VENUE)
    symbol, venue = normalized.split(".", 1)
    return InstrumentDescriptor(symbol=normalize_symbol(symbol), venue=normalize_venue(venue))