from __future__ import annotations

from datetime import datetime, timezone, timedelta
from fastapi import APIRouter, HTTPException

from src.bot.state import (
    read_state,
    update_state,
    write_start_command,
    write_stop_signal,
    write_emergency_stop,
    clear_stop_signals,
    BotStatus,
)
from src.bot.tws_data_provider import get_tws_provider, reset_tws_provider
from src.api.schemas import TWSConnectionRequest
from src.api.utils import load_strategy
from src.config.settings import update_setting


router = APIRouter(tags=["state"])


@router.get("/state")
def get_state():
    state = read_state().to_dict()
    
    # Check if runner is active based on last_heartbeat freshness
    # We use last_heartbeat to track the runner process separately from data updates
    runner_active = False
    last_heartbeat = state.get("last_heartbeat")
    
    if last_heartbeat:
        try:
            # Handle ISO formats with or without Z/offset
            last_dt = datetime.fromisoformat(last_heartbeat.replace("Z", "+00:00"))
            if last_dt.tzinfo is None:
                last_dt = last_dt.replace(tzinfo=timezone.utc)
            
            # Check if updated within last 15 seconds
            age = (datetime.now(timezone.utc) - last_dt).total_seconds()
            runner_active = age < 15
        except (ValueError, TypeError):
            pass
            
    state["runner_active"] = runner_active

    if not state.get("active_strategy"):
        try:
            strategy = load_strategy()
            if strategy and strategy.name:
                state["active_strategy"] = strategy.name
        except Exception:
            pass

    def _coerce_float(value: str | None) -> float | None:
        if value is None:
            return None
        try:
            return float(str(value).replace(",", ""))
        except (TypeError, ValueError):
            return None

    def _find_tag(summary: dict, tag: str) -> float | None:
        for item in summary.values():
            if item.get("tag") == tag:
                return _coerce_float(item.get("value"))
        return None

    def _extract_price(snapshot: dict | None, fallback: float) -> float:
        if not snapshot:
            return fallback
        for key in ("last", "close", "bid", "ask"):
            value = snapshot.get(key)
            if value is not None:
                try:
                    return float(value)
                except (TypeError, ValueError):
                    continue
        bid = snapshot.get("bid")
        ask = snapshot.get("ask")
        if bid is not None and ask is not None:
            try:
                return (float(bid) + float(ask)) / 2
            except (TypeError, ValueError):
                return fallback
        return fallback

    provider = get_tws_provider()
    tws_connected = bool(provider.is_connected())
    state["tws_connected"] = tws_connected

    if tws_connected:
        executions = provider.get_executions(
            timeout=3.0,
            since=datetime.now() - timedelta(days=7),
        )
        if not executions:
            executions = provider.get_executions(timeout=5.0, since=None)
        local_tz = datetime.now().astimezone().tzinfo

        def _normalize_side(raw: str) -> str:
            raw = (raw or "").upper()
            if raw in {"BOT", "BUY"}:
                return "BUY"
            if raw in {"SLD", "SELL"}:
                return "SELL"
            return ""

        def _parse_exec_time(value: object) -> datetime | None:
            if isinstance(value, datetime):
                return value.astimezone(local_tz) if value.tzinfo else value.replace(tzinfo=local_tz)
            if not value:
                return None
            text = str(value).strip()
            for fmt in ("%Y%m%d  %H:%M:%S", "%Y%m%d-%H:%M:%S", "%Y%m%d %H:%M:%S"):
                try:
                    parsed = datetime.strptime(text, fmt)
                    return parsed.replace(tzinfo=local_tz)
                except ValueError:
                    continue
            try:
                parsed = datetime.fromisoformat(text)
                return parsed.astimezone(local_tz) if parsed.tzinfo else parsed.replace(tzinfo=local_tz)
            except ValueError:
                return None

        exec_times: dict[tuple[str, str], datetime] = {}
        for exec_item in executions:
            symbol = (exec_item.get("symbol") or "").upper()
            side = _normalize_side(exec_item.get("side") or "")
            if not symbol or not side:
                continue
            exec_dt = _parse_exec_time(exec_item.get("time"))
            if not exec_dt:
                continue
            key = (symbol, side)
            prev = exec_times.get(key)
            if prev is None or exec_dt > prev:
                exec_times[key] = exec_dt

        portfolio_positions = provider.get_portfolio_positions(timeout=3.0)
        positions = provider.get_positions(timeout=3.0) if not portfolio_positions else []
        position_payload = []
        if portfolio_positions:
            for pos in portfolio_positions:
                qty = float(pos.get("position", 0) or 0)
                if qty == 0:
                    continue
                symbol = (pos.get("symbol") or "").upper()
                avg_cost = _coerce_float(pos.get("avg_cost")) or 0.0
                market_price = _coerce_float(pos.get("market_price")) or avg_cost
                unrealized = _coerce_float(pos.get("unrealized_pnl"))
                if unrealized is None:
                    unrealized = (market_price - avg_cost) * qty if avg_cost else 0.0
                entry_side = "BUY" if qty > 0 else "SELL"
                entry_dt = exec_times.get((symbol, entry_side))
                entry_time = entry_dt.isoformat() if entry_dt else pos.get("entry_time")
                position_payload.append(
                    {
                        "symbol": symbol,
                        "quantity": qty,
                        "entry_price": avg_cost,
                        "current_price": market_price,
                        "unrealized_pnl": unrealized,
                        "entry_time": entry_time,
                    }
                )
        else:
            for pos in positions:
                qty = float(pos.get("position", 0) or 0)
                if qty == 0:
                    continue
                symbol = (pos.get("symbol") or "").upper()
                avg_cost = _coerce_float(pos.get("avg_cost")) or 0.0
                snapshot = provider.get_market_data_snapshot(symbol, timeout=3.0)
                current_price = _extract_price(snapshot, avg_cost)
                if provider.has_market_data_permission_error(symbol):
                    delayed_snapshot = provider.get_market_data_snapshot(
                        symbol,
                        timeout=3.0,
                        market_data_type=3,
                    )
                    current_price = _extract_price(delayed_snapshot, current_price)
                unrealized = (current_price - avg_cost) * qty if avg_cost else 0.0
                position_payload.append(
                    {
                        "symbol": symbol,
                        "quantity": qty,
                        "entry_price": avg_cost,
                        "current_price": current_price,
                        "unrealized_pnl": unrealized,
                        "entry_time": None,
                    }
                )

        state["positions"] = position_payload
        state["open_positions_count"] = len(state["positions"])

        open_orders = provider.get_open_orders(timeout=3.0)
        state["orders"] = [
            {
                "order_id": order.get("order_id"),
                "symbol": order.get("symbol"),
                "side": order.get("action"),
                "quantity": order.get("quantity"),
                "price": order.get("price"),
                "status": order.get("status"),
                "order_type": order.get("order_type"),
                "submitted_time": order.get("submitted_time"),
                "filled_quantity": order.get("filled") or 0.0,
            }
            for order in open_orders
        ]
        state["pending_orders_count"] = len(state["orders"])

        summary = provider.get_account_summary(
            tags=(
                "NetLiquidation,TotalCashValue,GrossPositionValue,"
                "UnrealizedPnL,RealizedPnL,DailyPnL,PreviousDayEquityWithLoanValue,EquityWithLoanValue"
            )
        )
        net_liquidation = _find_tag(summary, "NetLiquidation")
        total_cash = _find_tag(summary, "TotalCashValue")
        realized = _find_tag(summary, "RealizedPnL")
        unrealized = _find_tag(summary, "UnrealizedPnL")
        daily_pnl = _find_tag(summary, "DailyPnL")
        prev_day_equity = _find_tag(summary, "PreviousDayEquityWithLoanValue")

        if net_liquidation is not None:
            state["equity"] = net_liquidation
        if daily_pnl is None and net_liquidation is not None and prev_day_equity:
            daily_pnl = net_liquidation - prev_day_equity
        if daily_pnl is not None:
            state["daily_pnl"] = daily_pnl
        if realized is not None or unrealized is not None:
            state["total_pnl"] = (realized or 0.0) + (unrealized or 0.0)
        else:
            unrealized_from_positions = sum(
                pos.get("unrealized_pnl", 0.0)
                for pos in state.get("positions", [])
                if pos.get("symbol") != "CASH"
            )
            if unrealized_from_positions:
                state["total_pnl"] = unrealized_from_positions

        equity = state.get("equity") or 0.0
        pct_base = prev_day_equity or equity
        if pct_base:
            state["daily_pnl_percent"] = (state.get("daily_pnl", 0.0) / pct_base) * 100

        if total_cash is not None:
            state["positions"].append(
                {
                    "symbol": "CASH",
                    "quantity": total_cash,
                    "entry_price": 1.0,
                    "current_price": 1.0,
                    "unrealized_pnl": 0.0,
                    "entry_time": None,
                }
            )
            state["open_positions_count"] = len(
                [pos for pos in state["positions"] if pos.get("symbol") != "CASH"]
            )

        state["last_update"] = datetime.now(timezone.utc).isoformat()

    return state


@router.get("/logs")
def get_logs():
    state = read_state()
    return {"logs": state.recent_logs}


@router.post("/bot/start")
def start_bot():
    # Check if runner is active before allowing start
    state = read_state()
    last_update = state.last_update
    runner_active = False
    if last_update:
        try:
            last_dt = datetime.fromisoformat(last_update.replace("Z", "+00:00"))
            if last_dt.tzinfo is None:
                last_dt = last_dt.replace(tzinfo=timezone.utc)
            age = (datetime.now(timezone.utc) - last_dt).total_seconds()
            runner_active = age < 15
        except Exception:
            pass
            
    if not runner_active:
        raise HTTPException(
            status_code=503, 
            detail="Bot runner process is not connected. Please run './run_bot.sh' in a terminal."
        )

    clear_stop_signals()
    write_start_command()
    try:
        state.status = BotStatus.STARTING.value
        update_state(state)
    except Exception:
        pass
    return {"status": "starting"}


@router.post("/bot/stop")
def stop_bot():
    # Only allow stop if runner is active
    state = read_state()
    last_update = state.last_update
    runner_active = False
    if last_update:
        try:
            last_dt = datetime.fromisoformat(last_update.replace("Z", "+00:00"))
            if last_dt.tzinfo is None:
                last_dt = last_dt.replace(tzinfo=timezone.utc)
            age = (datetime.now(timezone.utc) - last_dt).total_seconds()
            runner_active = age < 15
        except Exception:
            pass
            
    if not runner_active:
        raise HTTPException(
            status_code=503, 
            detail="Bot runner process is not connected. Please run './run_bot.sh' in a terminal."
        )

    write_stop_signal()
    try:
        state.status = BotStatus.STOPPING.value
        update_state(state)
    except Exception:
        pass
    return {"status": "stopping"}


@router.post("/bot/emergency_stop")
def emergency_stop():
    write_emergency_stop()
    return {"status": "emergency_stop"}


@router.post("/tws/connect")
def connect_tws(payload: TWSConnectionRequest):
    try:
        provider = get_tws_provider()
    except Exception as exc:
        raise HTTPException(status_code=500, detail=str(exc)) from exc

    if payload.host:
        provider.host = payload.host
        update_setting("ib", "host", payload.host)
    if payload.port:
        provider.port = int(payload.port)
        update_setting("ib", "port", int(payload.port))
    if payload.client_id:
        provider.client_id = int(payload.client_id)
        update_setting("ib", "client_id", int(payload.client_id))

    connected = False
    try:
        connected = provider.connect(timeout=3.0)
    except Exception as exc:
        state = read_state()
        state.tws_connected = False
        update_state(state)
        raise HTTPException(status_code=503, detail=str(exc)) from exc

    if not connected:
        state = read_state()
        state.tws_connected = False
        update_state(state)
        raise HTTPException(
            status_code=503,
            detail="Unable to connect to TWS. Is TWS running with API enabled?",
        )

    state = read_state()
    state.tws_connected = True
    update_state(state)
    return {"connected": True}


@router.post("/tws/disconnect")
def disconnect_tws():
    provider = get_tws_provider()
    provider.disconnect()
    reset_tws_provider()
    state = read_state()
    state.tws_connected = False
    update_state(state)
    return {"connected": False}


@router.post("/tws/reconnect")
def reconnect_tws():
    try:
        provider = get_tws_provider()
    except Exception as exc:
        raise HTTPException(status_code=500, detail=str(exc)) from exc

    connected = False
    try:
        connected = provider.reconnect(timeout=10.0)
    except Exception as exc:
        state = read_state()
        state.tws_connected = False
        update_state(state)
        raise HTTPException(status_code=503, detail=str(exc)) from exc

    if not connected:
        state = read_state()
        state.tws_connected = False
        update_state(state)
        raise HTTPException(
            status_code=503,
            detail="Unable to reconnect to TWS. Is TWS running with API enabled?",
        )

    state = read_state()
    state.tws_connected = True
    update_state(state)
    return {"connected": True}
