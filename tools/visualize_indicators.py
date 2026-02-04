#!/usr/bin/env python3
"""
Indicator Visualization Tool

Fetches historical data from TWS (or fallback) and plots all available
rule engine indicators for visual validation.

Usage:
    python tools/visualize_indicators.py --ticker TSLA --period 5m --duration "2 D"
"""

import os
import sys
import argparse
import logging
from datetime import datetime
import pandas as pd
import numpy as np
import plotly.graph_objects as go
from plotly.subplots import make_subplots

# Add src to path
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from src.bot.tws_data_provider import TWSDataProvider
from src.bot.strategy.rules.indicators import IndicatorFactory
from src.bot.strategy.rules.models import Indicator, IndicatorType, TimeframeUnit, PriceSource
from src.config.settings import get_settings

# Configure logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger("viz")

def parse_args():
    parser = argparse.ArgumentParser(description="Visualize Indicators")
    parser.add_argument("--ticker", type=str, default="SPY", help="Ticker symbol")
    parser.add_argument("--period", type=str, default="5m", help="Bar period/timeframe (1m, 5m, 1h, 1d)")
    parser.add_argument("--duration", type=str, default="5 D", help="Data duration (e.g., '2 D', '1 W')")
    parser.add_argument("--no-show", action="store_true", help="Do not open browser")
    return parser.parse_args()

def map_period_to_tws(period):
    mapping = {
        "1m": "1 min",
        "5m": "5 mins",
        "15m": "15 mins",
        "30m": "30 mins",
        "1h": "1 hour",
        "4h": "4 hours",
        "1d": "1 day",
    }
    return mapping.get(period, "5 mins")

def fetch_data(ticker, period, duration):
    """Fetch data from TWS."""
    logger.info(f"Connecting to TWS to fetch {ticker}...")
    try:
        provider = TWSDataProvider()
        connected = provider.connect()
        if not connected:
            logger.error("Could not connect to TWS.")
            return None
        
        bar_size = map_period_to_tws(period)
        logger.info(f"Requesting {duration} of {bar_size} bars...")
        
        bars = provider.get_historical_data(
            symbol=ticker,
            duration=duration,
            bar_size=bar_size,
            what_to_show="TRADES"
        )
        
        provider.disconnect()
        
        if not bars.empty and 'timestamp' in bars.columns:
            # Clean up timestamp if needed (IB sometimes returns formatted string)
            bars['timestamp'] = pd.to_datetime(bars['timestamp'])
            
        return bars
        
    except Exception as e:
        logger.exception(f"Error fetching data: {e}")
        return None

def calculate_indicator(bars, indicator_type):
    """Calculate indicator series."""
    results = {}
    
    try:
        if indicator_type == IndicatorType.EMA:
            # EMA 20
            ind = Indicator(type=IndicatorType.EMA, length=20, source=PriceSource.CLOSE)
            results["EMA (20)"] = IndicatorFactory.create_indicator_series(ind, bars)
            
        elif indicator_type == IndicatorType.SMA:
            # SMA 50
            ind = Indicator(type=IndicatorType.SMA, length=50, source=PriceSource.CLOSE)
            results["SMA (50)"] = IndicatorFactory.create_indicator_series(ind, bars)
            
        elif indicator_type == IndicatorType.RSI:
            # RSI 14
            ind = Indicator(type=IndicatorType.RSI, length=14)
            results["RSI (14)"] = IndicatorFactory.create_indicator_series(ind, bars)
            
        elif indicator_type == IndicatorType.MACD:
            # MACD 12, 26, 9
            ind_macd = Indicator(type=IndicatorType.MACD, component="macd")
            ind_signal = Indicator(type=IndicatorType.MACD, component="signal")
            ind_hist = Indicator(type=IndicatorType.MACD, component="histogram")
            
            results["MACD Line"] = IndicatorFactory.create_indicator_series(ind_macd, bars)
            results["Signal Line"] = IndicatorFactory.create_indicator_series(ind_signal, bars)
            results["Histogram"] = IndicatorFactory.create_indicator_series(ind_hist, bars)
            
        elif indicator_type == IndicatorType.BOLLINGER:
            # BB 20, 2.0
            ind_u = Indicator(type=IndicatorType.BOLLINGER, length=20, params={"std_dev": 2.0}, component="upper")
            ind_m = Indicator(type=IndicatorType.BOLLINGER, length=20, params={"std_dev": 2.0}, component="middle")
            ind_l = Indicator(type=IndicatorType.BOLLINGER, length=20, params={"std_dev": 2.0}, component="lower")
            
            results["BB Upper"] = IndicatorFactory.create_indicator_series(ind_u, bars)
            results["BB Middle"] = IndicatorFactory.create_indicator_series(ind_m, bars)
            results["BB Lower"] = IndicatorFactory.create_indicator_series(ind_l, bars)
        
        elif indicator_type == IndicatorType.STOCHASTIC:
            # Stoch 14, 3, 3
            ind_k = Indicator(type=IndicatorType.STOCHASTIC, params={"k_period": 14, "d_period": 3}, component="k")
            ind_d = Indicator(type=IndicatorType.STOCHASTIC, params={"k_period": 14, "d_period": 3}, component="d")
            
            results["Stoch %K"] = IndicatorFactory.create_indicator_series(ind_k, bars)
            results["Stoch %D"] = IndicatorFactory.create_indicator_series(ind_d, bars)
            
        elif indicator_type == IndicatorType.OBV:
            ind = Indicator(type=IndicatorType.OBV)
            results["OBV"] = IndicatorFactory.create_indicator_series(ind, bars)
            
        elif indicator_type == IndicatorType.ALLIGATOR:
            ind_j = Indicator(type=IndicatorType.ALLIGATOR, component="jaw")
            ind_t = Indicator(type=IndicatorType.ALLIGATOR, component="teeth")
            ind_l = Indicator(type=IndicatorType.ALLIGATOR, component="lips")
            
            results["Alligator Jaw"] = IndicatorFactory.create_indicator_series(ind_j, bars)
            results["Alligator Teeth"] = IndicatorFactory.create_indicator_series(ind_t, bars)
            results["Alligator Lips"] = IndicatorFactory.create_indicator_series(ind_l, bars)
            
    except Exception as e:
        logger.warning(f"Failed to calc {indicator_type.value}: {e}")
        
    return results

def main():
    args = parse_args()
    
    # Fetch Data
    bars = fetch_data(args.ticker, args.period, args.duration)
    
    if bars is None or bars.empty:
        logger.warning("No data returned. Using random sample data for demonstration logic if available or exiting.")
        # Fallback to sample data for testing without TWS
        try:
             # Just use random data if connection failed to show the plotting works?
             # Or try to load from sample/ folder
             sample_file = f"data/sample/{args.ticker}_{args.period}.csv"
             if os.path.exists(sample_file):
                 logger.info(f"Loading sample file: {sample_file}")
                 bars = pd.read_csv(sample_file, parse_dates=["timestamp"])
             else:
                 # Generate dummy data
                 logger.info("Generating dummy data")
                 dates = pd.date_range(end=datetime.now(), periods=200, freq=args.period)
                 bars = pd.DataFrame({
                     "timestamp": dates,
                     "open": np.random.randn(200).cumsum() + 100,
                     "close": np.random.randn(200).cumsum() + 100,
                     "high": np.random.randn(200).cumsum() + 105,
                     "low": np.random.randn(200).cumsum() + 95,
                     "volume": np.random.randint(100, 1000, 200)
                 })
                 # Fix HL
                 bars["high"] = bars[["open", "close"]].max(axis=1) + 1
                 bars["low"] = bars[["open", "close"]].min(axis=1) - 1
        except Exception:
            logger.error("Failed to generate fallback data.")
            return

    # Create Plot
    fig = make_subplots(
        rows=5, cols=1,
        shared_xaxes=True,
        vertical_spacing=0.02,
        row_heights=[0.4, 0.15, 0.15, 0.15, 0.15],
        subplot_titles=(f"{args.ticker} Price", "RSI", "MACD", "Stochastic", "OBV")
    )

    # 1. Price Candle
    fig.add_trace(go.Candlestick(
        x=bars['timestamp'],
        open=bars['open'], high=bars['high'],
        low=bars['low'], close=bars['close'],
        name='Price'
    ), row=1, col=1)

    # Calculate and Add Indicators
    
    # Overlays
    overlays = [IndicatorType.EMA, IndicatorType.SMA, IndicatorType.BOLLINGER, IndicatorType.ALLIGATOR]
    colors = {"EMA (20)": "orange", "SMA (50)": "blue", "BB Upper": "gray", "BB Lower": "gray", "BB Middle": "cyan"}
    
    for ind_type in overlays:
        results = calculate_indicator(bars, ind_type)
        for name, series in results.items():
            if np.all(np.isnan(series)): continue
            
            line_props = dict(width=1)
            if "Upper" in name or "Lower" in name:
                line_props['dash'] = 'dash'
            
            color = colors.get(name, None)
            fig.add_trace(go.Scatter(
                x=bars['timestamp'], y=series,
                name=name, line=dict(color=color, **line_props)
            ), row=1, col=1)

    # RSI
    rsi_res = calculate_indicator(bars, IndicatorType.RSI)
    if "RSI (14)" in rsi_res:
        fig.add_trace(go.Scatter(x=bars['timestamp'], y=rsi_res["RSI (14)"], name="RSI", line=dict(color="purple")), row=2, col=1)
        fig.add_hline(y=70, line_dash="dash", line_color="red", row=2, col=1)
        fig.add_hline(y=30, line_dash="dash", line_color="green", row=2, col=1)

    # MACD
    macd_res = calculate_indicator(bars, IndicatorType.MACD)
    if macd_res:
        fig.add_trace(go.Scatter(x=bars['timestamp'], y=macd_res["MACD Line"], name="MACD", line=dict(color="blue")), row=3, col=1)
        fig.add_trace(go.Scatter(x=bars['timestamp'], y=macd_res["Signal Line"], name="Signal", line=dict(color="orange")), row=3, col=1)
        fig.add_trace(go.Bar(x=bars['timestamp'], y=macd_res["Histogram"], name="Hist"), row=3, col=1)

    # Stochastic
    stoch_res = calculate_indicator(bars, IndicatorType.STOCHASTIC)
    if stoch_res:
        fig.add_trace(go.Scatter(x=bars['timestamp'], y=stoch_res["Stoch %K"], name="Stoch %K", line=dict(color="blue")), row=4, col=1)
        fig.add_trace(go.Scatter(x=bars['timestamp'], y=stoch_res["Stoch %D"], name="Stoch %D", line=dict(color="orange")), row=4, col=1)
        fig.add_hline(y=80, line_dash="dash", line_color="red", row=4, col=1)
        fig.add_hline(y=20, line_dash="dash", line_color="green", row=4, col=1)

    # OBV
    obv_res = calculate_indicator(bars, IndicatorType.OBV)
    if "OBV" in obv_res:
        fig.add_trace(go.Scatter(x=bars['timestamp'], y=obv_res["OBV"], name="OBV", line=dict(color="black")), row=5, col=1)

    fig.update_layout(height=1200, title_text=f"Indicator Analysis: {args.ticker}", xaxis_rangeslider_visible=False)
    
    if not args.no_show:
        fig.show()

if __name__ == "__main__":
    main()
