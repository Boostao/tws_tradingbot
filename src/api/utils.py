from __future__ import annotations

import json
import logging
import os
import re
from datetime import datetime, timezone
from html import unescape
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple
from uuid import uuid4

from src.bot.instruments import normalize_instrument_id
from src.bot.strategy.rules.models import Strategy
from src.bot.strategy.rules.serialization import list_strategies as list_strategy_files
from src.bot.strategy.rules.serialization import load_strategy as load_strategy_file
from src.bot.strategy.rules.serialization import save_strategy as save_strategy_file
from src.config.settings import get_settings
import requests


logger = logging.getLogger(__name__)

_symbol_cache_diagnostics: Dict[str, Optional[str]] = {
    "last_warning": None,
    "last_checked_at": None,
    "source": None,
}


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


def _watchlist_state_path() -> Path:
    watchlist_path = _watchlist_path()
    if watchlist_path.suffix:
        return watchlist_path.with_suffix(".json")
    return watchlist_path.with_name(f"{watchlist_path.name}.json")


def _active_strategy_path() -> Path:
    settings = get_settings()
    return _resolve_path(settings.app.active_strategy_path)


def _symbol_cache_path() -> Path:
    settings = get_settings()
    return _resolve_path(settings.app.symbol_cache_path)


def _strategies_dir_path() -> Path:
    settings = get_settings()
    return _resolve_path(settings.app.strategies_dir)


def _cockpit_state_path() -> Path:
    return _active_strategy_path().with_name("cockpit.json")


def _utc_now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def _update_symbol_cache_diagnostics(source: Optional[str], warning: Optional[str]) -> None:
    _symbol_cache_diagnostics["source"] = source
    _symbol_cache_diagnostics["last_warning"] = warning
    _symbol_cache_diagnostics["last_checked_at"] = _utc_now_iso()


def get_symbol_cache_diagnostics() -> Dict[str, Optional[str]]:
    return dict(_symbol_cache_diagnostics)


def _watchlist_entry_key(symbol: str, exchange: str = "") -> str:
    symbol_part = symbol.strip().upper()
    exchange_part = exchange.strip().upper()
    return f"{symbol_part}:{exchange_part}" if exchange_part else symbol_part


def _split_watchlist_entry(value: str) -> Tuple[str, str]:
    raw = value.strip()
    if not raw:
        return "", ""
    if ":" not in raw:
        return raw.upper(), ""
    symbol, exchange = raw.split(":", 1)
    return symbol.strip().upper(), exchange.strip().upper()


def _tradingview_symbol_to_item(value: str) -> Optional[Dict[str, Any]]:
    raw = value.strip()
    if not raw or raw.startswith("###"):
        return None
    if ":" not in raw:
        symbol, exchange = raw, ""
    else:
        exchange, symbol = raw.split(":", 1)
    symbol = symbol.strip().upper()
    exchange = exchange.strip().upper()
    if not symbol:
        return None
    return {
        "symbol": symbol,
        "exchange": exchange,
        "name": "",
        "enabled": True,
    }


def _item_to_export_symbol(item: Dict[str, Any]) -> str:
    return _watchlist_entry_key(item.get("symbol", ""), item.get("exchange", ""))


def _default_watchlist_state() -> Dict[str, Any]:
    return {
        "groups": [
            {
                "id": "manual",
                "name": "Manual",
                "source": "manual",
                "items": [],
            }
        ],
        "feed": None,
        "updated_at": None,
    }


def _slugify(value: str, fallback: str) -> str:
    slug = re.sub(r"[^a-z0-9]+", "-", value.strip().lower()).strip("-")
    return slug or fallback


def _normalize_watchlist_item(item: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    symbol = str(item.get("symbol", "")).strip().upper()
    exchange = str(item.get("exchange", "")).strip().upper()
    name = str(item.get("name", "")).strip()
    enabled = bool(item.get("enabled", True))
    if not symbol:
        return None
    return {
        "symbol": symbol,
        "exchange": exchange,
        "name": name,
        "enabled": enabled,
        "instrument_id": normalize_instrument_id(symbol, exchange),
    }


def _normalize_watchlist_group(group: Dict[str, Any], fallback_index: int) -> Optional[Dict[str, Any]]:
    name = str(group.get("name", "")).strip() or f"Group {fallback_index + 1}"
    source = str(group.get("source", "manual")).strip() or "manual"
    group_id = str(group.get("id", "")).strip() or _slugify(name, f"group-{fallback_index + 1}")
    items = []
    seen = set()
    for raw_item in group.get("items", []):
        if not isinstance(raw_item, dict):
            continue
        item = _normalize_watchlist_item(raw_item)
        if not item:
            continue
        item_key = _watchlist_entry_key(item["symbol"], item["exchange"])
        if item_key in seen:
            continue
        seen.add(item_key)
        items.append(item)
    return {
        "id": group_id,
        "name": name,
        "source": source,
        "items": items,
    }


def _legacy_symbols_to_groups(symbols: List[str]) -> List[Dict[str, Any]]:
    items = []
    seen = set()
    for entry in symbols:
        symbol, exchange = _split_watchlist_entry(entry)
        if not symbol:
            continue
        item_key = _watchlist_entry_key(symbol, exchange)
        if item_key in seen:
            continue
        seen.add(item_key)
        items.append({"symbol": symbol, "exchange": exchange, "name": "", "enabled": True})
    return [{"id": "manual", "name": "Manual", "source": "manual", "items": items}]


def _normalize_watchlist_state(payload: Dict[str, Any]) -> Dict[str, Any]:
    groups_payload = payload.get("groups")
    if not isinstance(groups_payload, list):
        groups_payload = _legacy_symbols_to_groups(payload.get("symbols", []))
    groups = []
    seen_group_ids = set()
    for index, raw_group in enumerate(groups_payload):
        if not isinstance(raw_group, dict):
            continue
        group = _normalize_watchlist_group(raw_group, index)
        if not group:
            continue
        original_group_id = group["id"]
        suffix = 2
        while group["id"] in seen_group_ids:
            group["id"] = f"{original_group_id}-{suffix}"
            suffix += 1
        seen_group_ids.add(group["id"])
        groups.append(group)

    if not groups:
        groups = _default_watchlist_state()["groups"]

    feed = payload.get("feed")
    if not isinstance(feed, dict):
        feed = None
    elif not feed.get("provider") or not feed.get("url"):
        feed = None
    else:
        feed = {
            "provider": str(feed.get("provider", "")).strip(),
            "url": str(feed.get("url", "")).strip(),
            "title": str(feed.get("title") or "").strip() or None,
            "external_id": str(feed.get("external_id") or "").strip() or None,
            "last_refreshed_at": str(feed.get("last_refreshed_at") or "").strip() or None,
        }

    updated_at = str(payload.get("updated_at") or "").strip() or None
    return {"groups": groups, "feed": feed, "updated_at": updated_at}


def _active_watchlist_symbols_from_groups(groups: List[Dict[str, Any]]) -> List[str]:
    active = []
    seen = set()
    for group in groups:
        for item in group.get("items", []):
            if not item.get("enabled", True):
                continue
            symbol = _item_to_export_symbol(item)
            if not symbol or symbol in seen:
                continue
            seen.add(symbol)
            active.append(symbol)
    return active


def normalized_watchlist_instruments(groups: List[Dict[str, Any]], include_disabled: bool = False) -> List[str]:
    instruments: List[str] = []
    seen = set()
    for group in groups:
        for raw_item in group.get("items", []):
            if not isinstance(raw_item, dict):
                continue
            item = _normalize_watchlist_item(raw_item)
            if not item:
                continue
            if not include_disabled and not item.get("enabled", True):
                continue
            instrument_id = str(item.get("instrument_id") or "").strip().upper()
            if not instrument_id or instrument_id in seen:
                continue
            seen.add(instrument_id)
            instruments.append(instrument_id)
    return instruments


def _copy_groups(groups: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
    return [json.loads(json.dumps(group)) for group in groups]


def _write_watchlist_symbols(symbols: List[str]) -> None:
    watchlist_path = _watchlist_path()
    watchlist_path.parent.mkdir(parents=True, exist_ok=True)
    with open(watchlist_path, "w") as f:
        for symbol in symbols:
            cleaned = symbol.strip().upper()
            if cleaned:
                f.write(f"{cleaned}\n")


def _write_watchlist_state(state: Dict[str, Any]) -> None:
    watchlist_state_path = _watchlist_state_path()
    watchlist_state_path.parent.mkdir(parents=True, exist_ok=True)
    with open(watchlist_state_path, "w") as f:
        json.dump(state, f, indent=2)


def _read_legacy_watchlist_symbols() -> List[str]:
    symbols: List[str] = []
    watchlist_path = _watchlist_path()
    if watchlist_path.exists():
        with open(watchlist_path, "r") as f:
            for line in f:
                line = line.strip()
                if line and not line.startswith("#"):
                    symbols.append(line.upper())
    return symbols


def load_watchlist() -> List[str]:
    state = load_watchlist_state()
    return _active_watchlist_symbols_from_groups(state["groups"])


def save_watchlist(symbols: List[str]) -> None:
    state = {
        "groups": _legacy_symbols_to_groups(symbols),
        "feed": None,
        "updated_at": _utc_now_iso(),
    }
    _write_watchlist_state(state)
    _write_watchlist_symbols(_active_watchlist_symbols_from_groups(state["groups"]))
    _create_reload_signal()


def load_watchlist_state() -> Dict[str, Any]:
    watchlist_state_path = _watchlist_state_path()
    if watchlist_state_path.exists():
        try:
            with open(watchlist_state_path, "r") as f:
                return _normalize_watchlist_state(json.load(f))
        except (OSError, json.JSONDecodeError, TypeError, ValueError) as exc:
            logger.warning("Failed to read watchlist state file %s: %s", watchlist_state_path, exc)

    legacy_symbols = _read_legacy_watchlist_symbols()
    if legacy_symbols:
        return _normalize_watchlist_state({"symbols": legacy_symbols, "updated_at": None})

    return _default_watchlist_state()


def save_watchlist_state(groups: List[Dict[str, Any]], feed: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
    state = _normalize_watchlist_state({
        "groups": groups,
        "feed": feed,
        "updated_at": _utc_now_iso(),
    })
    _write_watchlist_state(state)
    _write_watchlist_symbols(_active_watchlist_symbols_from_groups(state["groups"]))
    _create_reload_signal()
    return state


def _extract_tradingview_payload(html: str) -> Dict[str, Any]:
    match = re.search(
        r'<script type="application/prs\.init-data\+json">\s*(\{.*?\})\s*</script>',
        html,
        re.DOTALL,
    )
    if not match:
        raise ValueError("TradingView watchlist payload not found")
    return json.loads(unescape(match.group(1)))


def _tradingview_groups_from_symbols(symbols: List[str], fallback_name: str) -> List[Dict[str, Any]]:
    groups: List[Dict[str, Any]] = []
    current_group: Optional[Dict[str, Any]] = None
    seen_in_group: set[str] = set()

    def start_group(name: str, index: int) -> Dict[str, Any]:
        group = {
            "id": _slugify(name, f"feed-group-{index + 1}"),
            "name": name,
            "source": "tradingview",
            "items": [],
        }
        groups.append(group)
        return group

    for raw_entry in symbols:
        entry = raw_entry.strip()
        if not entry:
            continue
        if entry.startswith("###"):
            group_name = entry.lstrip("#").strip() or fallback_name
            current_group = start_group(group_name, len(groups))
            seen_in_group = set()
            continue
        if current_group is None:
            current_group = start_group(fallback_name, len(groups))
            seen_in_group = set()
        item = _tradingview_symbol_to_item(entry)
        if not item:
            continue
        item_key = _watchlist_entry_key(item["symbol"], item["exchange"])
        if item_key in seen_in_group:
            continue
        seen_in_group.add(item_key)
        current_group["items"].append(item)

    return [group for group in groups if group["items"]]


def _watchlist_name_lookup(groups: List[Dict[str, Any]]) -> Dict[str, str]:
    names: Dict[str, str] = {}
    for group in groups:
        for item in group.get("items", []):
            key = _watchlist_entry_key(item.get("symbol", ""), item.get("exchange", ""))
            name = str(item.get("name") or "").strip()
            if key and name:
                names[key] = name
    return names


def _symbol_metadata_lookup() -> Tuple[Dict[str, str], Dict[str, str]]:
    by_symbol_exchange: Dict[str, str] = {}
    by_symbol_only: Dict[str, str] = {}
    try:
        symbols, _, _, _ = get_symbol_cache(refresh=False)
    except (OSError, ValueError, TypeError) as exc:
        logger.warning("Failed to build symbol metadata lookup: %s", exc)
        return by_symbol_exchange, by_symbol_only

    for item in symbols:
        symbol = str(item.get("symbol") or "").strip().upper()
        exchange = str(item.get("exchange") or "").strip().upper()
        name = str(item.get("name") or "").strip()
        if not symbol or not name:
            continue
        if exchange:
            by_symbol_exchange[_watchlist_entry_key(symbol, exchange)] = name
        by_symbol_only.setdefault(symbol, name)
    return by_symbol_exchange, by_symbol_only


def _enrich_imported_groups_with_names(
    imported_groups: List[Dict[str, Any]],
    current_groups: Optional[List[Dict[str, Any]]] = None,
) -> List[Dict[str, Any]]:
    current_names = _watchlist_name_lookup(current_groups or [])
    names_by_symbol_exchange, names_by_symbol_only = _symbol_metadata_lookup()

    for group in imported_groups:
        for item in group.get("items", []):
            symbol = str(item.get("symbol") or "").strip().upper()
            exchange = str(item.get("exchange") or "").strip().upper()
            if not symbol:
                continue
            item_key = _watchlist_entry_key(symbol, exchange)
            fallback_key = _watchlist_entry_key(symbol)
            item["name"] = (
                names_by_symbol_exchange.get(item_key)
                or names_by_symbol_only.get(symbol)
                or current_names.get(item_key)
                or current_names.get(fallback_key)
                or str(item.get("name") or "").strip()
            )
    return imported_groups


def _merge_watchlist_enabled_state(
    imported_groups: List[Dict[str, Any]], current_groups: List[Dict[str, Any]]
) -> List[Dict[str, Any]]:
    enabled_by_symbol = {}
    for group in current_groups:
        for item in group.get("items", []):
            enabled_by_symbol[_watchlist_entry_key(item.get("symbol", ""), item.get("exchange", ""))] = bool(
                item.get("enabled", True)
            )

    for group in imported_groups:
        for item in group.get("items", []):
            item_key = _watchlist_entry_key(item.get("symbol", ""), item.get("exchange", ""))
            if item_key in enabled_by_symbol:
                item["enabled"] = enabled_by_symbol[item_key]
    return imported_groups


def import_tradingview_watchlist(url: str, current_state: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
    response = requests.get(
        url,
        timeout=30,
        headers={"User-Agent": "Mozilla/5.0", "Accept": "text/html,application/xhtml+xml"},
    )
    response.raise_for_status()
    payload = _extract_tradingview_payload(response.text)
    shared_watchlist = payload.get("sharedWatchlist", {})
    watchlist = shared_watchlist.get("list", {})
    if not isinstance(watchlist, dict):
        raise ValueError("TradingView watchlist payload is invalid")

    title = str(watchlist.get("name") or "TradingView Import").strip() or "TradingView Import"
    symbol_list = watchlist.get("symbols", [])
    if not isinstance(symbol_list, list):
        raise ValueError("TradingView watchlist symbols are missing")

    groups = _tradingview_groups_from_symbols(symbol_list, title)
    groups = _enrich_imported_groups_with_names(groups, current_state.get("groups", []) if current_state else None)
    if current_state:
        groups = _merge_watchlist_enabled_state(groups, current_state.get("groups", []))

    if not groups:
        groups = [{"id": "manual", "name": title, "source": "tradingview", "items": []}]

    return save_watchlist_state(
        groups,
        feed={
            "provider": "tradingview",
            "url": url,
            "title": title,
            "external_id": str(watchlist.get("id") or "").strip() or None,
            "last_refreshed_at": _utc_now_iso(),
        },
    )


def refresh_watchlist_feed() -> Dict[str, Any]:
    current_state = load_watchlist_state()
    feed = current_state.get("feed") or {}
    provider = str(feed.get("provider") or "").strip().lower()
    url = str(feed.get("url") or "").strip()
    if provider != "tradingview" or not url:
        raise ValueError("No TradingView feed is configured")
    return import_tradingview_watchlist(url, current_state=current_state)


def _strategy_to_summary(strategy: Strategy, source: str) -> Dict[str, Any]:
    return {
        "id": strategy.id,
        "name": strategy.name,
        "rule_count": len(strategy.rules),
        "enabled_rule_count": len([rule for rule in strategy.rules if rule.enabled]),
        "source": source,
    }


def list_strategy_library() -> List[Dict[str, Any]]:
    library: List[Dict[str, Any]] = []
    seen = set()

    active_strategy = load_strategy()
    active_summary = _strategy_to_summary(active_strategy, "active")
    library.append(active_summary)
    seen.add(active_strategy.id)

    for _, _, file_path in list_strategy_files(_strategies_dir_path()):
        try:
            strategy = load_strategy_file(file_path)
        except (OSError, json.JSONDecodeError, TypeError, ValueError) as exc:
            logger.warning("Failed to load strategy library file %s: %s", file_path, exc)
            continue
        if strategy.id in seen:
            continue
        library.append(_strategy_to_summary(strategy, "library"))
        seen.add(strategy.id)

    return sorted(library, key=lambda item: (item["source"] != "active", item["name"].lower()))


def _strategy_library_entry(strategy: Strategy) -> Dict[str, Any]:
    return {
        "id": strategy.id,
        "name": strategy.name,
        "rule_count": len(strategy.rules),
        "enabled_rule_count": len([rule for rule in strategy.rules if rule.enabled]),
        "updated_at": strategy.updated_at.isoformat() if getattr(strategy, "updated_at", None) else None,
    }


def list_strategy_presets() -> List[Dict[str, Any]]:
    presets: List[Dict[str, Any]] = []
    for _, _, file_path in list_strategy_files(_strategies_dir_path()):
        try:
            strategy = load_strategy_file(file_path)
        except (OSError, json.JSONDecodeError, TypeError, ValueError) as exc:
            logger.warning("Failed to load strategy preset file %s: %s", file_path, exc)
            continue
        presets.append(_strategy_library_entry(strategy))
    return sorted(presets, key=lambda item: item["name"].lower())


def _find_strategy_preset_path(strategy_id: str) -> Optional[Path]:
    for _, current_strategy_id, file_path in list_strategy_files(_strategies_dir_path()):
        if current_strategy_id == strategy_id:
            return file_path
    return None


def load_strategy_preset(strategy_id: str) -> Strategy:
    preset_path = _find_strategy_preset_path(strategy_id)
    if preset_path is None:
        raise FileNotFoundError(f"Strategy preset not found: {strategy_id}")
    return load_strategy_file(preset_path)


def save_strategy_preset(strategy: Strategy, name: Optional[str] = None) -> Dict[str, Any]:
    preset = strategy.model_copy(deep=True)
    preset.id = str(uuid4())
    preset.name = (name or preset.name or "Strategy Preset").strip()
    preset.updated_at = datetime.now(timezone.utc)
    preset.created_at = datetime.now(timezone.utc)
    preset_path = _strategies_dir_path() / f"{preset.id}.json"
    save_strategy_file(preset, preset_path)
    return _strategy_library_entry(preset)


def update_strategy_preset(strategy_id: str, strategy: Strategy, name: Optional[str] = None) -> Dict[str, Any]:
    preset_path = _find_strategy_preset_path(strategy_id)
    if preset_path is None:
        raise FileNotFoundError(f"Strategy preset not found: {strategy_id}")

    existing_preset = load_strategy_file(preset_path)
    preset = strategy.model_copy(deep=True)
    preset.id = strategy_id
    preset.name = (name or preset.name or existing_preset.name or "Strategy Preset").strip()
    preset.created_at = existing_preset.created_at
    save_strategy_file(preset, preset_path)
    return _strategy_library_entry(preset)


def delete_strategy_preset(strategy_id: str) -> None:
    preset_path = _find_strategy_preset_path(strategy_id)
    if preset_path is None:
        raise FileNotFoundError(f"Strategy preset not found: {strategy_id}")
    preset_path.unlink(missing_ok=False)


def apply_strategy_preset(strategy_id: str) -> Strategy:
    preset = load_strategy_preset(strategy_id)
    save_strategy(preset)
    return preset


def _default_strategy_slots(strategy_library: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
    default_strategy_id = strategy_library[0]["id"] if strategy_library else None
    return [
        {
            "id": "slot-1",
            "label": "Active Strategy",
            "strategy_id": default_strategy_id,
            "enabled": bool(default_strategy_id),
        }
    ]


def _normalize_single_strategy_slot(
    slots_payload: List[Dict[str, Any]],
    valid_strategy_ids: set[str],
) -> List[Dict[str, Any]]:
    slots = [
        _normalize_strategy_slot(slot, index, valid_strategy_ids)
        for index, slot in enumerate(slots_payload)
        if isinstance(slot, dict)
    ]

    if not slots:
        return _default_strategy_slots([{"id": next(iter(valid_strategy_ids), None)}] if valid_strategy_ids else [])

    selected_slot = next(
        (slot for slot in slots if slot["enabled"] and slot["strategy_id"]),
        next((slot for slot in slots if slot["strategy_id"]), slots[0]),
    )

    selected_slot = {
        "id": "slot-1",
        "label": "Active Strategy",
        "strategy_id": selected_slot["strategy_id"],
        "enabled": bool(selected_slot["strategy_id"]),
    }
    return [selected_slot]


def _default_cockpit_state() -> Dict[str, Any]:
    strategy_library = list_strategy_library()
    workspaces = [
        {
            "id": "workspace-long",
            "name": "Achat",
            "kind": "long",
            "enabled": True,
            "strategy_slots": _default_strategy_slots(strategy_library),
        },
        {
            "id": "workspace-short",
            "name": "Short",
            "kind": "short",
            "enabled": False,
            "strategy_slots": _default_strategy_slots(strategy_library),
        },
        {
            "id": "workspace-swing",
            "name": "Swing",
            "kind": "swing",
            "enabled": False,
            "strategy_slots": _default_strategy_slots(strategy_library),
        },
    ]
    return {
        "global_enabled": True,
        "active_workspace_id": workspaces[0]["id"],
        "workspaces": workspaces,
        "updated_at": None,
    }


def _normalize_strategy_slot(slot: Dict[str, Any], fallback_index: int, valid_strategy_ids: set[str]) -> Dict[str, Any]:
    strategy_id = str(slot.get("strategy_id") or "").strip() or None
    if strategy_id and strategy_id not in valid_strategy_ids:
        strategy_id = None
    return {
        "id": str(slot.get("id") or f"slot-{fallback_index + 1}").strip() or f"slot-{fallback_index + 1}",
        "label": str(slot.get("label") or f"Strategy {fallback_index + 1}").strip() or f"Strategy {fallback_index + 1}",
        "strategy_id": strategy_id,
        "enabled": bool(slot.get("enabled", True)),
    }


def _normalize_cockpit_workspace(
    workspace: Dict[str, Any],
    fallback_index: int,
    valid_strategy_ids: set[str],
) -> Dict[str, Any]:
    workspace_id = str(workspace.get("id") or f"workspace-{fallback_index + 1}").strip() or f"workspace-{fallback_index + 1}"
    name = str(workspace.get("name") or f"Workspace {fallback_index + 1}").strip() or f"Workspace {fallback_index + 1}"
    kind = str(workspace.get("kind") or "custom").strip() or "custom"
    slots_payload = workspace.get("strategy_slots") if isinstance(workspace.get("strategy_slots"), list) else []
    slots = _normalize_single_strategy_slot(slots_payload, valid_strategy_ids)

    return {
        "id": workspace_id,
        "name": name,
        "kind": kind,
        "enabled": bool(workspace.get("enabled", True)),
        "strategy_slots": slots,
    }


def _workspace_has_active_strategy(workspace: Dict[str, Any]) -> bool:
    slots = workspace.get("strategy_slots") if isinstance(workspace.get("strategy_slots"), list) else []
    if not slots:
        return False
    slot = slots[0]
    return bool(slot.get("enabled") and slot.get("strategy_id"))


def _enforce_single_active_strategy_workspace(
    workspaces: List[Dict[str, Any]],
    requested_active_workspace_id: str,
) -> str:
    selected_workspace_id = ""

    requested_workspace = next((workspace for workspace in workspaces if workspace["id"] == requested_active_workspace_id), None)
    if requested_workspace and _workspace_has_active_strategy(requested_workspace):
        selected_workspace_id = requested_workspace["id"]
    else:
        selected_workspace_id = next(
            (workspace["id"] for workspace in workspaces if _workspace_has_active_strategy(workspace)),
            requested_active_workspace_id,
        )

    for workspace in workspaces:
        slots = workspace.get("strategy_slots") if isinstance(workspace.get("strategy_slots"), list) else []
        if not slots:
            continue
        slot = slots[0]
        slot["enabled"] = bool(
            workspace["id"] == selected_workspace_id and slot.get("strategy_id") and slot.get("enabled")
        )

    return selected_workspace_id


def _normalize_cockpit_state(payload: Dict[str, Any]) -> Dict[str, Any]:
    watchlist_state = load_watchlist_state()
    strategy_library = list_strategy_library()
    valid_strategy_ids = {item["id"] for item in strategy_library}

    workspaces_payload = payload.get("workspaces") if isinstance(payload.get("workspaces"), list) else []
    workspaces = []
    seen_workspace_ids = set()
    for index, raw_workspace in enumerate(workspaces_payload):
        if not isinstance(raw_workspace, dict):
            continue
        workspace = _normalize_cockpit_workspace(raw_workspace, index, valid_strategy_ids)
        original_workspace_id = workspace["id"]
        suffix = 2
        while workspace["id"] in seen_workspace_ids:
            workspace["id"] = f"{original_workspace_id}-{suffix}"
            suffix += 1
        seen_workspace_ids.add(workspace["id"])
        workspaces.append(workspace)

    if not workspaces:
        default_state = _default_cockpit_state()
        workspaces = default_state["workspaces"]

    active_workspace_id = str(payload.get("active_workspace_id") or "").strip() or workspaces[0]["id"]
    if active_workspace_id not in {workspace["id"] for workspace in workspaces}:
        active_workspace_id = workspaces[0]["id"]
    active_workspace_id = _enforce_single_active_strategy_workspace(workspaces, active_workspace_id)

    updated_at = str(payload.get("updated_at") or "").strip() or None
    return {
        "global_enabled": bool(payload.get("global_enabled", True)),
        "active_workspace_id": active_workspace_id,
        "workspaces": workspaces,
        "strategy_library": strategy_library,
        "feed": watchlist_state.get("feed"),
        "updated_at": updated_at,
    }


def load_cockpit_state() -> Dict[str, Any]:
    cockpit_path = _cockpit_state_path()
    if cockpit_path.exists():
        try:
            with open(cockpit_path, "r") as f:
                return _normalize_cockpit_state(json.load(f))
        except (OSError, json.JSONDecodeError, TypeError, ValueError) as exc:
            logger.warning("Failed to read cockpit state file %s: %s", cockpit_path, exc)
    return _normalize_cockpit_state(_default_cockpit_state())


def save_cockpit_state(payload: Dict[str, Any]) -> Dict[str, Any]:
    normalized = _normalize_cockpit_state({**payload, "updated_at": _utc_now_iso()})
    cockpit_path = _cockpit_state_path()
    cockpit_path.parent.mkdir(parents=True, exist_ok=True)
    persisted = {
        "global_enabled": normalized["global_enabled"],
        "active_workspace_id": normalized["active_workspace_id"],
        "workspaces": normalized["workspaces"],
        "updated_at": normalized["updated_at"],
    }
    with open(cockpit_path, "w") as f:
        json.dump(persisted, f, indent=2)
    return normalized


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


def _read_symbol_cache_file() -> Tuple[List[Dict[str, Any]], Optional[str], Optional[str], Optional[str]]:
    symbol_cache_path = _symbol_cache_path()
    if symbol_cache_path.exists():
        try:
            with open(symbol_cache_path, "r") as f:
                data = json.load(f)
            if isinstance(data, list):
                return data, "cache", None, None
            if isinstance(data, dict) and isinstance(data.get("symbols"), list):
                return data.get("symbols", []), data.get("source"), data.get("updated_at"), None
            logger.warning("Symbol cache file has unexpected payload shape: %s", symbol_cache_path)
            return [], None, None, "invalid_symbol_cache_payload"
        except (OSError, json.JSONDecodeError, TypeError, ValueError) as exc:
            logger.warning("Failed to read symbol cache file %s: %s", symbol_cache_path, exc)
            return [], None, None, "symbol_cache_read_failed"
    return [], None, None, None


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


def get_symbol_cache(refresh: bool = False) -> Tuple[List[Dict[str, Any]], str, Optional[str], Optional[str]]:
    cached_symbols, cached_source, cached_updated_at, cache_warning = _read_symbol_cache_file()
    cache_ttl = int(os.getenv("SYMBOL_CACHE_TTL_SECONDS", "86400"))
    symbol_cache_path = _symbol_cache_path()
    is_stale = False
    if symbol_cache_path.exists():
        try:
            mtime = symbol_cache_path.stat().st_mtime
            is_stale = (datetime.now(timezone.utc).timestamp() - mtime) > cache_ttl
        except OSError as exc:
            logger.warning("Failed to inspect symbol cache file %s: %s", symbol_cache_path, exc)
            is_stale = True

    if refresh or is_stale or not cached_symbols:
        try:
            symbols = _fetch_symbols_from_tradingview()
            if len(symbols) > 1000:
                _write_symbol_cache(symbols, "tradingview")
                _update_symbol_cache_diagnostics("tradingview", None)
                return symbols, "tradingview", datetime.now(timezone.utc).isoformat(), None
            logger.warning("TradingView symbol refresh returned too few symbols: %s", len(symbols))
            if cache_warning is None:
                cache_warning = "symbol_refresh_incomplete"
        except (requests.RequestException, OSError, ValueError, TypeError) as exc:
            logger.warning("TradingView symbol refresh failed: %s", exc)
            if cache_warning is None:
                cache_warning = "symbol_refresh_failed"

    if cached_symbols:
        _update_symbol_cache_diagnostics(cached_source or "cache", cache_warning)
        return cached_symbols, cached_source or "cache", cached_updated_at, cache_warning

    _update_symbol_cache_diagnostics("local", cache_warning)
    return [], "local", cached_updated_at, cache_warning

