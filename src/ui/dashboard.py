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

logger = get_logger(__name__)

# Global state
bot_running = False
bot_thread = None
status_messages: List[str] = []
positions_df = pd.DataFrame()
market_data = {}

def display_logs():
    st.header("📋 Monitoring & Logging")
    if status_messages:
        st.text_area("Recent Messages", "\n".join(status_messages[-20:]), height=200)
    else:
        st.info("No messages yet")

def display_api_calls():
    st.header("🔗 API Calls & Connections")
    connected = data_provider.is_connected()
    status = "🟢 Connected" if connected else "🔴 Disconnected"
    st.metric("TWS Connection Status", status)

    if st.button("Test Connection"):
        if data_provider.connect():
            add_status_message("TWS connection test successful")
            st.success("Connected to TWS")
        else:
            add_status_message("TWS connection test failed")
            st.error("Failed to connect to TWS")

def bot_control():
    global bot_running, bot_thread

    st.header("🎮 Bot Control")

    col1, col2 = st.columns(2)

    with col1:
        if st.button("▶️ Start Bot", disabled=engine.is_running()):
            start_bot()

    with col2:
        if st.button("⏹️ Stop Bot", disabled=not engine.is_running()):
            stop_bot()

    st.subheader("Configuration")
    starting_capital = st.number_input(
        "Starting Capital ($)",
        min_value=1000,
        max_value=10000000,
        value=config.get('bot.starting_capital', 100000),
        step=1000
    )

    if starting_capital != config.get('bot.starting_capital', 100000):
        config.set('bot.starting_capital', starting_capital)
        st.success("Starting capital updated")

def display_stock_status():
    st.header("📊 Stock Status")

    # Positions
    st.subheader("Current Positions")
    positions = order_manager.get_positions()
    if positions:
        pos_data = []
        for symbol, qty in positions.items():
            pos_data.append({'Symbol': symbol, 'Quantity': qty})
        df = pd.DataFrame(pos_data)
        st.dataframe(df)
    else:
        st.info("No positions currently held")

    # Risk Manager Positions
    st.subheader("Risk Manager Positions")
    risk_positions = risk_manager.get_positions()
    if risk_positions:
        risk_data = []
        for symbol, data in risk_positions.items():
            risk_data.append({
                'Symbol': symbol,
                'Quantity': data['quantity'],
                'Avg Price': f"${data['avg_price']:.2f}",
                'Value': f"${data['value']:.2f}"
            })
        df = pd.DataFrame(risk_data)
        st.dataframe(df)

    # Account Summary
    st.subheader("Account Summary")
    account_info = data_provider.get_account_summary()
    if account_info:
        for key, value in account_info.items():
            if key in ['TotalCashValue', 'NetLiquidation', 'BuyingPower']:
                st.metric(key, f"${float(value):,.2f}")
    else:
        st.info("Account summary not available")

    # Daily P&L
    daily_pnl = risk_manager.get_daily_pnl()
    st.metric("Daily P&L", f"${daily_pnl:.2f}", delta=f"{daily_pnl:.2f}")

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
    if not engine.is_running():
        engine.start()
        add_status_message("Bot started")
        logger.info("Trading bot started")

def stop_bot():
    if engine.is_running():
        engine.stop()
        add_status_message("Bot stopped")
        logger.info("Trading bot stopped")



def add_status_message(message: str):
    """Add a status message."""
    timestamp = time.strftime("%H:%M:%S")
    status_messages.append(f"[{timestamp}] {message}")
    # Keep only last 100 messages
    if len(status_messages) > 100:
        status_messages = status_messages[-100:]