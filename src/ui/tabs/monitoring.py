"""
Live Trading monitoring tab for the Streamlit UI.

Displays real-time trading bot status, positions, orders, and logs.
Auto-refreshes every 5 seconds to show latest state.
"""

import streamlit as st
import pandas as pd
import time
from datetime import datetime
from typing import Optional

from ..styles import COLORS, status_badge, format_currency
from src.bot.state import (
    BotState,
    BotStatus,
    Position,
    Order,
    read_state,
    write_start_command,
    write_stop_signal,
    write_emergency_stop,
    clear_stop_signals,
    get_state_file_age,
)
from src.bot.tws_data_provider import get_tws_provider


# Global i18n instance
i18n = None

# Auto-refresh interval in seconds
AUTO_REFRESH_INTERVAL = 5
DEFAULT_ACCOUNT_REFRESH_INTERVAL = 15


def get_bot_state() -> BotState:
    """Get current bot state. Can be used by other modules."""
    return read_state()


def render_monitoring_tab() -> None:
    """Render the Live Trading monitoring tab content."""
    global i18n
    i18n = st.session_state['i18n']
    
    # Initialize session state for refresh
    if "last_refresh" not in st.session_state:
        st.session_state.last_refresh = time.time()
    
    # Read current bot state
    state = read_state()
    
    # Auto-refresh toggle
    col_refresh, col_status = st.columns([3, 1])
    with col_refresh:
        auto_refresh = st.toggle(i18n.t("auto_refresh"), value=True, key="auto_refresh_toggle")
    with col_status:
        if state.last_update:
            try:
                last_update = datetime.fromisoformat(state.last_update)
                time_ago = (datetime.now() - last_update).total_seconds()
                if time_ago < 60:
                    st.caption(i18n.t("updated_seconds_ago", seconds=int(time_ago)))
                else:
                    st.caption(i18n.t("updated_minutes_ago", minutes=int(time_ago/60)))
            except:
                st.caption(i18n.t("last_update_na"))
        else:
            st.caption(i18n.t("no_updates_yet"))
    
    # Auto-refresh logic
    if auto_refresh:
        current_time = time.time()
        if current_time - st.session_state.last_refresh >= AUTO_REFRESH_INTERVAL:
            st.session_state.last_refresh = current_time
            st.rerun()
    
    # Connection Status Section
    _render_connection_status(state)

    # TWS Account Snapshot
    _render_tws_account_snapshot()
    
    st.divider()
    
    # Key Metrics Row
    _render_metrics_row(state)
    
    st.divider()
    
    # Positions and Orders in two columns
    col1, col2 = st.columns(2)
    
    with col1:
        _render_positions_table(state)
    
    with col2:
        _render_orders_table(state)
    
    st.divider()
    
    # Bot Controls
    _render_bot_controls(state)
    
    st.divider()
    
    # Logs Section
    _render_logs_section(state)


def _render_connection_status(state: BotState) -> None:
    """Render the connection status section."""
    col1, col2, col3 = st.columns([2, 2, 2])
    
    with col1:
        st.markdown(f"#### {i18n.t('connection_status')}")
        # Determine connection status based on state age
        state_age = get_state_file_age()
        if state_age is not None and state_age < 30:  # State updated in last 30s
            is_connected = True
            status_text = i18n.t("connected")
        else:
            is_connected = False
            status_text = i18n.t("disconnected")
        st.markdown(status_badge(status_text, is_connected), unsafe_allow_html=True)
    
    with col2:
        st.markdown(f"#### {i18n.t('tws_status')}")
        # Check actual TWS provider connection, not state file
        try:
            provider = get_tws_provider()
            tws_connected = provider.is_connected()
        except Exception:
            tws_connected = False
        tws_status = i18n.t("connected") if tws_connected else i18n.t("disconnected")
        st.markdown(status_badge(tws_status, tws_connected), unsafe_allow_html=True)
    
    with col3:
        st.markdown(f"#### {i18n.t('bot_status')}")
        bot_status = state.status
        is_running = bot_status == BotStatus.RUNNING.value
        is_error = bot_status == BotStatus.ERROR.value
        
        if is_error:
            st.markdown(
                f'<span style="background-color: #ef5350; color: white; padding: 4px 12px; '
                f'border-radius: 4px; font-size: 14px;">⚠️ {bot_status}</span>',
                unsafe_allow_html=True
            )
            if state.error_message:
                st.caption(f"Error: {state.error_message}")
        else:
            status_color = is_running or bot_status == BotStatus.STARTING.value
            st.markdown(status_badge(bot_status, status_color), unsafe_allow_html=True)
        
        # Show active strategy
        if state.active_strategy:
            st.caption(f"Strategy: {state.active_strategy}")


def _render_tws_account_snapshot() -> None:
    """Render a live snapshot of TWS account values."""
    st.markdown("#### TWS Account Snapshot")

    if "account_refresh_interval" not in st.session_state:
        st.session_state.account_refresh_interval = DEFAULT_ACCOUNT_REFRESH_INTERVAL
    if "account_last_refresh" not in st.session_state:
        st.session_state.account_last_refresh = 0.0

    col1, col2, col3 = st.columns([2, 2, 2])
    with col1:
        account_auto_refresh = st.toggle("Refresh account data", value=False, key="account_refresh_toggle")
    with col2:
        refresh_interval = st.selectbox(
            "Refresh interval",
            options=[5, 10, 15, 30, 60],
            index=2,
            key="account_refresh_interval_select",
        )
        st.session_state.account_refresh_interval = refresh_interval
    with col3:
        if st.button("🔄 Refresh now", key="account_refresh_now"):
            st.session_state.account_last_refresh = 0.0

    provider = None
    try:
        provider = get_tws_provider()
        tws_connected = provider.is_connected()
    except Exception:
        tws_connected = False

    if not tws_connected:
        st.info("TWS is not connected. Account values unavailable.")
        return

    now = time.time()
    should_refresh = account_auto_refresh and (
        now - st.session_state.account_last_refresh >= st.session_state.account_refresh_interval
    )

    if "account_snapshot" not in st.session_state or should_refresh:
        try:
            summary = provider.get_account_summary()
        except Exception:
            summary = {}
        st.session_state.account_snapshot = summary
        st.session_state.account_last_refresh = now

    snapshot = st.session_state.get("account_snapshot", {})

    def _get_value(tag: str) -> Optional[float]:
        # IB returns keys as "{account}_{tag}"
        for key, item in snapshot.items():
            if isinstance(item, dict) and item.get("tag") == tag:
                try:
                    return float(item.get("value"))
                except Exception:
                    return None
        return None

    net_liq = _get_value("NetLiquidation")
    cash = _get_value("TotalCashValue")
    gross = _get_value("GrossPositionValue")

    col_a, col_b, col_c = st.columns(3)
    with col_a:
        st.metric("Net Liquidation", format_currency(net_liq or 0.0))
    with col_b:
        st.metric("Cash", format_currency(cash or 0.0))
    with col_c:
        st.metric("Gross Position Value", format_currency(gross or 0.0))


def _render_metrics_row(state: BotState) -> None:
    """Render the key metrics row."""
    col1, col2, col3, col4 = st.columns(4)
    
    with col1:
        st.metric(
            label=i18n.t("account_equity"),
            value=format_currency(state.equity),
            delta=None
        )
    
    with col2:
        # Format daily PnL with color
        pnl_delta = f"{state.daily_pnl_percent:+.2f}%" if state.daily_pnl_percent != 0 else None
        st.metric(
            label=i18n.t("daily_pnl"),
            value=format_currency(state.daily_pnl),
            delta=pnl_delta,
            delta_color="normal" if state.daily_pnl >= 0 else "inverse"
        )
    
    with col3:
        st.metric(
            label=i18n.t("open_positions_count"),
            value=str(state.open_positions_count),
            delta=None
        )
    
    with col4:
        st.metric(
            label=i18n.t("pending_orders"),
            value=str(state.pending_orders_count),
            delta=None
        )
    
    # Second row of metrics
    col5, col6, col7, col8 = st.columns(4)
    
    with col5:
        st.metric(
            label="🔄 Trades Today",
            value=str(state.trades_today),
            delta=None
        )
    
    with col6:
        win_rate_str = f"{state.win_rate_today:.1f}%" if state.trades_today > 0 else "N/A"
        st.metric(
            label="🎯 Win Rate (Today)",
            value=win_rate_str,
            delta=None
        )
    
    with col7:
        st.metric(
            label="💵 Total P&L",
            value=format_currency(state.total_pnl),
            delta=None
        )
    
    with col8:
        # Calculate unrealized PnL from positions
        unrealized = sum(p.unrealized_pnl for p in state.positions) if state.positions else 0
        st.metric(
            label="📉 Unrealized P&L",
            value=format_currency(unrealized),
            delta=None
        )


def _render_positions_table(state: BotState) -> None:
    """Render the positions table."""
    st.markdown(f"#### {i18n.t('open_positions')}")
    
    if not state.positions:
        st.info(i18n.t("no_positions"), icon="📊")
        return
    
    # Build dataframe from positions
    positions_data = []
    for pos in state.positions:
        pnl_percent = ((pos.current_price - pos.entry_price) / pos.entry_price * 100) if pos.entry_price > 0 else 0
        positions_data.append({
            i18n.t("symbol"): pos.symbol,
            i18n.t("quantity"): f"{pos.quantity:,.0f}" if pos.quantity == int(pos.quantity) else f"{pos.quantity:,.2f}",
            i18n.t("entry"): f"${pos.entry_price:,.2f}",
            i18n.t("current"): f"${pos.current_price:,.2f}",
            i18n.t("pnl"): f"${pos.unrealized_pnl:+,.2f}",
            i18n.t("pnl_percent"): f"{pnl_percent:+.2f}%",
        })
    
    df = pd.DataFrame(positions_data)
    
    # Style the dataframe
    def color_pnl(val):
        if isinstance(val, str):
            if val.startswith('+') or (val.startswith('$') and not val.startswith('$-')):
                if '+' in val or (val.startswith('$') and '-' not in val and float(val.replace('$', '').replace(',', '').replace('%', '')) > 0):
                    return f'color: {COLORS["profit"]}'
            if '-' in val:
                return f'color: {COLORS["loss"]}'
        return ''
    
    styled_df = df.style.applymap(color_pnl, subset=['P&L', 'P&L %'])
    
    st.dataframe(
        styled_df,
        width="stretch",
        hide_index=True
    )


def _render_orders_table(state: BotState) -> None:
    """Render the orders table."""
    st.markdown(f"#### {i18n.t('active_orders')}")
    
    # Filter to active orders only
    active_orders = [o for o in state.orders if o.status in ("PENDING", "SUBMITTED", "PARTIALLY_FILLED")]
    
    if not active_orders:
        st.info(i18n.t("no_active_orders"), icon="📋")
        return
    
    # Build dataframe from orders
    orders_data = []
    for order in active_orders:
        price_str = f"${order.price:,.2f}" if order.price else "MARKET"
        orders_data.append({
            i18n.t("symbol"): order.symbol,
            i18n.t("side"): order.side,
            i18n.t("quantity"): f"{order.quantity:,.0f}",
            i18n.t("price"): price_str,
            i18n.t("status"): order.status,
            i18n.t("time"): order.submitted_time or "N/A",
        })
    
    df = pd.DataFrame(orders_data)
    
    # Style the dataframe
    def color_side(val):
        if val == "BUY":
            return f'color: {COLORS["profit"]}'
        elif val == "SELL":
            return f'color: {COLORS["loss"]}'
        return ''
    
    styled_df = df.style.applymap(color_side, subset=['Side'])
    
    st.dataframe(
        styled_df,
        width="stretch",
        hide_index=True
    )


def _render_bot_controls(state: BotState) -> None:
    """Render the bot control buttons."""
    st.markdown(f"#### {i18n.t('bot_controls')}")
    
    is_running = state.status in (BotStatus.RUNNING.value, BotStatus.STARTING.value)
    is_stopped = state.status in (BotStatus.STOPPED.value, BotStatus.ERROR.value)
    
    col1, col2, col3, col4 = st.columns([1, 1, 1, 2])
    
    with col1:
        # Start button
        start_disabled = is_running
        if st.button(
            i18n.t("start_bot"), 
            key="start_bot", 
            width="stretch",
            disabled=start_disabled,
            type="primary" if is_stopped else "secondary"
        ):
            if write_start_command():
                st.toast("Start command sent to bot", icon="🚀")
                st.rerun()
            else:
                st.error("Failed to send start command")
    
    with col2:
        # Stop button
        stop_disabled = not is_running
        if st.button(
            i18n.t("stop_bot"), 
            key="stop_bot", 
            width="stretch",
            disabled=stop_disabled
        ):
            if write_stop_signal():
                st.toast(i18n.t("stop_signal_sent"), icon="🛑")
                st.rerun()
            else:
                st.error("Failed to send stop signal")
    
    with col3:
        # Emergency stop button
        if st.button(
            i18n.t("emergency_stop"), 
            key="emergency_stop", 
            width="stretch",
            type="primary" if is_running else "secondary"
        ):
            # Show confirmation dialog
            _show_emergency_stop_dialog()
    
    with col4:
        # Status info
        if is_running:
            st.success("Bot is running", icon="✅")
        elif state.status == BotStatus.ERROR.value:
            st.error(f"Bot error: {state.error_message or 'Unknown error'}", icon="❌")
        else:
            st.info("Bot is stopped", icon="ℹ️")


@st.dialog("⚠️ Emergency Stop Confirmation")
def _show_emergency_stop_dialog():
    """Show emergency stop confirmation dialog."""
    st.warning(
        "This will:\n"
        "- Cancel ALL pending orders\n"
        "- Close ALL open positions at market\n"
        "- Stop the trading bot\n\n"
        "**This action cannot be undone!**"
    )
    
    col1, col2 = st.columns(2)
    with col1:
        if st.button("Cancel", width="stretch"):
            st.rerun()
    with col2:
        if st.button("🚨 CONFIRM EMERGENCY STOP", width="stretch", type="primary"):
            if write_emergency_stop():
                st.toast("Emergency stop triggered!", icon="🚨")
                time.sleep(0.5)
                st.rerun()
            else:
                st.error("Failed to trigger emergency stop")


def _render_logs_section(state: BotState) -> None:
    """Render the logs section."""
    st.markdown("#### 📜 Recent Activity Logs")
    
    # Get logs from state
    logs = state.recent_logs if state.recent_logs else []
    
    if not logs:
        logs = [
            f"[{datetime.now().strftime('%Y-%m-%d %H:%M:%S')}] [INFO] Monitoring UI initialized",
            f"[{datetime.now().strftime('%Y-%m-%d %H:%M:%S')}] [INFO] Waiting for bot state updates...",
        ]
    
    # Create a scrollable log area with proper styling
    log_container = st.container(height=250)
    with log_container:
        # Display logs in reverse order (newest first)
        for log in reversed(logs):
            # Color-code based on level
            if "[ERROR]" in log:
                st.markdown(f'<span style="color: {COLORS["loss"]}">{log}</span>', unsafe_allow_html=True)
            elif "[WARNING]" in log:
                st.markdown(f'<span style="color: #ffb74d">{log}</span>', unsafe_allow_html=True)
            elif "[DEBUG]" in log:
                st.markdown(f'<span style="color: {COLORS["text_secondary"]}">{log}</span>', unsafe_allow_html=True)
            else:
                st.text(log)
    
    # Log controls
    col1, col2, col3 = st.columns([1, 1, 4])
    with col1:
        if st.button("🔄 Refresh", key="refresh_logs"):
            st.rerun()
    with col2:
        if st.button("🗑️ Clear Signals", key="clear_signals"):
            clear_stop_signals()
            st.toast("Signal files cleared", icon="🗑️")

