import streamlit as st
import time
import pandas as pd
from typing import List
import threading

from src.config import config
from src.utils import get_logger
from src.bot.data_provider import data_provider
from src.bot.engine import engine
from src.bot.order_manager import order_manager
from src.bot.risk_manager import risk_manager
from src.ui.i18n import I18n
from src.ui.translations import translations

logger = get_logger(__name__)

# Global state
bot_running = False
bot_thread = None
status_messages: List[str] = []
positions_df = pd.DataFrame()
market_data = {}


def _get_i18n() -> I18n:
    if "i18n" not in st.session_state:
        st.session_state["i18n"] = I18n(translations)
    return st.session_state["i18n"]

def display_logs():
    i18n = _get_i18n()
    st.header(i18n.t("monitoring_logging"))
    if status_messages:
        st.text_area(i18n.t("recent_messages"), "\n".join(status_messages[-20:]), height=200)
    else:
        st.info(i18n.t("no_messages_yet"))

def display_api_calls():
    i18n = _get_i18n()
    st.header(i18n.t("api_calls_connections"))
    connected = data_provider.is_connected()
    status = i18n.t("connected") if connected else i18n.t("disconnected")
    st.metric(i18n.t("tws_connection_status"), status)

    if st.button(i18n.t("test_connection")):
        if data_provider.connect():
            add_status_message(i18n.t("tws_connection_success"))
            st.success(i18n.t("connected_to_tws"))
        else:
            add_status_message(i18n.t("tws_connection_failed"))
            st.error(i18n.t("failed_to_connect_tws"))

def bot_control():
    global bot_running, bot_thread

    i18n = _get_i18n()
    st.header(i18n.t("bot_control"))

    col1, col2 = st.columns(2)

    with col1:
        if st.button(i18n.t("start_bot"), disabled=engine.is_running()):
            start_bot()

    with col2:
        if st.button(i18n.t("stop_bot"), disabled=not engine.is_running()):
            stop_bot()

    st.subheader(i18n.t("configuration"))
    starting_capital = st.number_input(
        i18n.t("starting_capital"),
        min_value=1000,
        max_value=10000000,
        value=config.get('bot.starting_capital', 100000),
        step=1000
    )

    if starting_capital != config.get('bot.starting_capital', 100000):
        config.set('bot.starting_capital', starting_capital)
        st.success(i18n.t("starting_capital_updated"))

def display_stock_status():
    i18n = _get_i18n()
    st.header(i18n.t("stock_status"))

    # Positions
    st.subheader(i18n.t("current_positions"))
    positions = order_manager.get_positions()
    if positions:
        pos_data = []
        for symbol, qty in positions.items():
            pos_data.append({i18n.t('symbol'): symbol, i18n.t('quantity'): qty})
        df = pd.DataFrame(pos_data)
        st.dataframe(df)
    else:
        st.info(i18n.t("no_positions_currently_held"))

    # Risk Manager Positions
    st.subheader(i18n.t("risk_manager_positions"))
    risk_positions = risk_manager.get_positions()
    if risk_positions:
        risk_data = []
        for symbol, data in risk_positions.items():
            risk_data.append({
                i18n.t('symbol'): symbol,
                i18n.t('quantity'): data['quantity'],
                i18n.t('avg_price'): f"${data['avg_price']:.2f}",
                i18n.t('value'): f"${data['value']:.2f}"
            })
        df = pd.DataFrame(risk_data)
        st.dataframe(df)

    # Account Summary
    st.subheader(i18n.t("account_summary"))
    account_info = data_provider.get_account_summary()
    if account_info:
        for key, value in account_info.items():
            if key in ['TotalCashValue', 'NetLiquidation', 'BuyingPower']:
                st.metric(key, f"${float(value):,.2f}")
    else:
        st.info(i18n.t("account_summary_not_available"))

    # Daily P&L
    daily_pnl = risk_manager.get_daily_pnl()
    st.metric(i18n.t("daily_pnl"), f"${daily_pnl:.2f}", delta=f"{daily_pnl:.2f}")

def display_dashboard():
    # Update data
    update_data()

    # Layout
    col1, col2 = st.columns([2, 1])

    with col1:
        display_stock_status()

    with col2:
        display_api_calls()
        bot_control()

    display_logs()

def update_data():
    """Update positions and market data."""
    global positions_df, market_data

    # Placeholder for positions update
    # In real implementation, get from data_provider
    pass

def start_bot():
    i18n = _get_i18n()
    if not engine.is_running():
        engine.start()
        add_status_message(i18n.t("bot_started"))
        logger.info("Trading bot started")

def stop_bot():
    i18n = _get_i18n()
    if engine.is_running():
        engine.stop()
        add_status_message(i18n.t("bot_stopped"))
        logger.info("Trading bot stopped")



def add_status_message(message: str):
    """Add a status message."""
    timestamp = time.strftime("%H:%M:%S")
    status_messages.append(f"[{timestamp}] {message}")
    # Keep only last 100 messages
    if len(status_messages) > 100:
        status_messages = status_messages[-100:]