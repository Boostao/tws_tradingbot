"""
Charts Component Module

Provides Plotly chart visualizations for backtest results.
Uses TradingView-style dark theme colors.
"""

import streamlit as st
import plotly.graph_objects as go
import plotly.express as px
from plotly.subplots import make_subplots
import pandas as pd
from typing import List, Optional

from src.bot.backtest_runner import BacktestResult, BacktestMetrics, Trade
from src.ui.styles import COLORS


def render_equity_curve(result: BacktestResult) -> None:
    """
    Render an equity curve chart from backtest results.
    
    Shows:
    - Equity line over time
    - Drawdown periods shaded in red
    - Initial capital reference line
    
    Args:
        result: BacktestResult containing equity_curve DataFrame
    """
    equity_df = result.equity_curve
    
    if equity_df.empty:
        st.info("No equity data available to display.")
        return
    
    # Create figure with secondary y-axis for drawdown
    fig = make_subplots(
        rows=2, cols=1,
        row_heights=[0.75, 0.25],
        shared_xaxes=True,
        vertical_spacing=0.05,
        subplot_titles=("Equity Curve", "Drawdown")
    )
    
    # Main equity line
    fig.add_trace(
        go.Scatter(
            x=equity_df["timestamp"],
            y=equity_df["equity"],
            mode="lines",
            name="Equity",
            line=dict(color=COLORS["accent_blue"], width=2),
            hovertemplate="<b>%{x}</b><br>Equity: $%{y:,.2f}<extra></extra>"
        ),
        row=1, col=1
    )
    
    # Initial capital reference line
    fig.add_hline(
        y=result.initial_capital,
        line_dash="dash",
        line_color=COLORS["text_secondary"],
        annotation_text=f"Initial: ${result.initial_capital:,.0f}",
        annotation_position="right",
        row=1, col=1
    )
    
    # Fill area under equity curve
    fig.add_trace(
        go.Scatter(
            x=equity_df["timestamp"],
            y=equity_df["equity"],
            fill="tozeroy",
            fillcolor=f"rgba(41, 98, 255, 0.1)",
            line=dict(color="rgba(0,0,0,0)"),
            showlegend=False,
            hoverinfo="skip"
        ),
        row=1, col=1
    )
    
    # Drawdown chart
    if "drawdown_pct" in equity_df.columns:
        fig.add_trace(
            go.Scatter(
                x=equity_df["timestamp"],
                y=-equity_df["drawdown_pct"],
                mode="lines",
                name="Drawdown %",
                fill="tozeroy",
                fillcolor=f"rgba(239, 83, 80, 0.3)",
                line=dict(color=COLORS["accent_red"], width=1),
                hovertemplate="<b>%{x}</b><br>Drawdown: %{y:.2f}%<extra></extra>"
            ),
            row=2, col=1
        )
    
    # Mark trade entry/exit points on equity curve
    trades_df = result.get_trades_df()
    if not trades_df.empty:
        # Entry points
        buy_trades = trades_df[trades_df["side"] == "BUY"]
        if not buy_trades.empty:
            # Match entry times to equity
            entry_equities = []
            for _, trade in buy_trades.iterrows():
                eq_row = equity_df[equity_df["timestamp"] >= trade["entry_time"]]
                if not eq_row.empty:
                    entry_equities.append(eq_row.iloc[0]["equity"])
                else:
                    entry_equities.append(None)
            
            valid_entries = [
                (t, e) for t, e in zip(buy_trades["entry_time"], entry_equities) 
                if e is not None
            ]
            
            if valid_entries:
                times, equities = zip(*valid_entries)
                fig.add_trace(
                    go.Scatter(
                        x=list(times),
                        y=list(equities),
                        mode="markers",
                        name="Entry",
                        marker=dict(
                            color=COLORS["accent_green"],
                            size=8,
                            symbol="triangle-up"
                        ),
                        hovertemplate="<b>Entry</b><br>%{x}<extra></extra>"
                    ),
                    row=1, col=1
                )
    
    # Update layout with dark theme
    fig.update_layout(
        height=500,
        showlegend=True,
        legend=dict(
            orientation="h",
            yanchor="bottom",
            y=1.02,
            xanchor="right",
            x=1,
            font=dict(color=COLORS["text_primary"])
        ),
        paper_bgcolor=COLORS["bg_main"],
        plot_bgcolor=COLORS["bg_card"],
        font=dict(color=COLORS["text_primary"]),
        margin=dict(l=50, r=20, t=60, b=20),
        hovermode="x unified",
    )
    
    # Update axes
    fig.update_xaxes(
        showgrid=True,
        gridcolor=COLORS["border"],
        zeroline=False,
        showline=True,
        linecolor=COLORS["border"],
    )
    
    fig.update_yaxes(
        showgrid=True,
        gridcolor=COLORS["border"],
        zeroline=False,
        showline=True,
        linecolor=COLORS["border"],
        tickformat="$,.0f",
        row=1, col=1
    )
    
    fig.update_yaxes(
        showgrid=True,
        gridcolor=COLORS["border"],
        zeroline=True,
        zerolinecolor=COLORS["border"],
        showline=True,
        linecolor=COLORS["border"],
        tickformat=".1f",
        ticksuffix="%",
        row=2, col=1
    )
    
    # Update subplot titles color
    for annotation in fig.layout.annotations:
        annotation.font.color = COLORS["text_primary"]
    
    st.plotly_chart(fig, width="stretch")


def render_metrics_cards(metrics: BacktestMetrics, initial_capital: float = 10000) -> None:
    """
    Render backtest metrics as styled cards.
    
    Args:
        metrics: BacktestMetrics object with performance data
        initial_capital: Initial capital for context
    """
    # First row - main metrics
    col1, col2, col3, col4 = st.columns(4)
    
    with col1:
        _render_metric_card(
            "Total Return",
            f"${metrics.total_return:,.2f}",
            f"{metrics.total_return_percent:+.2f}%",
            is_positive=metrics.total_return >= 0
        )
    
    with col2:
        _render_metric_card(
            "Sharpe Ratio",
            f"{metrics.sharpe_ratio:.2f}",
            "Risk-adjusted return",
            is_positive=metrics.sharpe_ratio > 0
        )
    
    with col3:
        _render_metric_card(
            "Max Drawdown",
            f"-{metrics.max_drawdown_percent:.2f}%",
            f"${metrics.max_drawdown:,.2f}",
            is_positive=False,
            neutral=metrics.max_drawdown_percent < 10
        )
    
    with col4:
        _render_metric_card(
            "Win Rate",
            f"{metrics.win_rate:.1f}%",
            f"{metrics.winning_trades}/{metrics.total_trades} trades",
            is_positive=metrics.win_rate >= 50
        )
    
    # Second row - trade stats
    st.markdown("---")
    col1, col2, col3, col4 = st.columns(4)
    
    with col1:
        _render_metric_card(
            "Total Trades",
            str(metrics.total_trades),
            f"W: {metrics.winning_trades} / L: {metrics.losing_trades}",
            neutral=True
        )
    
    with col2:
        _render_metric_card(
            "Avg Win",
            f"${metrics.avg_win:,.2f}" if metrics.avg_win > 0 else "$0.00",
            "Per winning trade",
            is_positive=True
        )
    
    with col3:
        _render_metric_card(
            "Avg Loss",
            f"${metrics.avg_loss:,.2f}" if metrics.avg_loss > 0 else "$0.00",
            "Per losing trade",
            is_positive=False
        )
    
    with col4:
        pf_display = f"{metrics.profit_factor:.2f}" if metrics.profit_factor < float('inf') else "∞"
        _render_metric_card(
            "Profit Factor",
            pf_display,
            "Gross profit / loss",
            is_positive=metrics.profit_factor > 1
        )


def _render_metric_card(
    title: str,
    value: str,
    subtitle: str,
    is_positive: bool = True,
    neutral: bool = False
) -> None:
    """
    Render a single metric card.
    
    Args:
        title: Card title
        value: Main value to display
        subtitle: Subtitle/description
        is_positive: Whether the value represents positive performance
        neutral: If True, use neutral styling regardless of is_positive
    """
    if neutral:
        value_color = COLORS["text_primary"]
    else:
        value_color = COLORS["accent_green"] if is_positive else COLORS["accent_red"]
    
    st.markdown(
        f'''
        <div style="
            background-color: {COLORS["bg_card"]};
            border: 1px solid {COLORS["border"]};
            border-radius: 8px;
            padding: 1rem;
            text-align: center;
        ">
            <div style="color: {COLORS["text_secondary"]}; font-size: 0.85em; margin-bottom: 4px;">
                {title}
            </div>
            <div style="color: {value_color}; font-size: 1.5em; font-weight: bold;">
                {value}
            </div>
            <div style="color: {COLORS["text_secondary"]}; font-size: 0.75em; margin-top: 4px;">
                {subtitle}
            </div>
        </div>
        ''',
        unsafe_allow_html=True
    )


def render_trade_table(trades: List[Trade]) -> None:
    """
    Render a styled table of all trades.
    
    Args:
        trades: List of Trade objects
    """
    if not trades:
        st.info("No trades executed during the backtest.")
        return
    
    # Convert to DataFrame
    df = pd.DataFrame([t.to_dict() for t in trades])
    
    # Format columns
    df["entry_time"] = pd.to_datetime(df["entry_time"]).dt.strftime("%Y-%m-%d %H:%M")
    df["exit_time"] = pd.to_datetime(df["exit_time"]).dt.strftime("%Y-%m-%d %H:%M")
    df["entry_price"] = df["entry_price"].apply(lambda x: f"${x:.2f}")
    df["exit_price"] = df["exit_price"].apply(lambda x: f"${x:.2f}" if pd.notna(x) else "-")
    df["quantity"] = df["quantity"].apply(lambda x: f"{x:.2f}")
    df["pnl"] = df["pnl"].apply(lambda x: f"${x:+,.2f}")
    df["pnl_percent"] = df["pnl_percent"].apply(lambda x: f"{x:+.2f}%")
    
    # Rename columns for display
    df = df.rename(columns={
        "entry_time": "Entry Time",
        "exit_time": "Exit Time",
        "symbol": "Symbol",
        "side": "Side",
        "quantity": "Qty",
        "entry_price": "Entry $",
        "exit_price": "Exit $",
        "pnl": "P&L",
        "pnl_percent": "P&L %"
    })
    
    # Reorder columns
    df = df[["Entry Time", "Exit Time", "Symbol", "Side", "Qty", "Entry $", "Exit $", "P&L", "P&L %"]]
    
    # Display with styling
    st.markdown(
        f'<div style="color: {COLORS["text_secondary"]}; margin-bottom: 0.5rem;">'
        f'Showing {len(df)} trades'
        f'</div>',
        unsafe_allow_html=True
    )
    
    # Use st.dataframe with column configuration
    st.dataframe(
        df,
        width="stretch",
        hide_index=True,
        height=min(400, len(df) * 35 + 40),
    )


def render_trade_distribution(trades: List[Trade]) -> None:
    """
    Render a histogram of trade P&L distribution.
    
    Args:
        trades: List of Trade objects
    """
    if not trades:
        return
    
    pnls = [t.pnl for t in trades]
    
    fig = go.Figure()
    
    # Separate winning and losing trades
    wins = [p for p in pnls if p > 0]
    losses = [p for p in pnls if p <= 0]
    
    if wins:
        fig.add_trace(
            go.Histogram(
                x=wins,
                name="Wins",
                marker_color=COLORS["accent_green"],
                opacity=0.7,
            )
        )
    
    if losses:
        fig.add_trace(
            go.Histogram(
                x=losses,
                name="Losses",
                marker_color=COLORS["accent_red"],
                opacity=0.7,
            )
        )
    
    fig.update_layout(
        title="Trade P&L Distribution",
        xaxis_title="P&L ($)",
        yaxis_title="Count",
        barmode="overlay",
        height=300,
        paper_bgcolor=COLORS["bg_main"],
        plot_bgcolor=COLORS["bg_card"],
        font=dict(color=COLORS["text_primary"]),
        showlegend=True,
        legend=dict(
            orientation="h",
            yanchor="bottom",
            y=1.02,
            xanchor="right",
            x=1
        ),
    )
    
    fig.update_xaxes(
        showgrid=True,
        gridcolor=COLORS["border"],
        zeroline=True,
        zerolinecolor=COLORS["text_secondary"],
    )
    
    fig.update_yaxes(
        showgrid=True,
        gridcolor=COLORS["border"],
    )
    
    st.plotly_chart(fig, width="stretch")


def render_cumulative_pnl(trades: List[Trade]) -> None:
    """
    Render cumulative P&L chart from trades.
    
    Args:
        trades: List of Trade objects
    """
    if not trades:
        return
    
    # Sort trades by exit time
    sorted_trades = sorted(trades, key=lambda t: t.exit_time or t.entry_time)
    
    cumulative = []
    running_pnl = 0
    
    for trade in sorted_trades:
        running_pnl += trade.pnl
        cumulative.append({
            "time": trade.exit_time or trade.entry_time,
            "cumulative_pnl": running_pnl,
            "trade_pnl": trade.pnl,
            "symbol": trade.symbol
        })
    
    df = pd.DataFrame(cumulative)
    
    fig = go.Figure()
    
    # Cumulative line
    fig.add_trace(
        go.Scatter(
            x=df["time"],
            y=df["cumulative_pnl"],
            mode="lines+markers",
            name="Cumulative P&L",
            line=dict(color=COLORS["accent_blue"], width=2),
            marker=dict(
                size=8,
                color=[COLORS["accent_green"] if p > 0 else COLORS["accent_red"] for p in df["trade_pnl"]]
            ),
            hovertemplate=(
                "<b>%{x}</b><br>"
                "Cumulative: $%{y:,.2f}<br>"
                "<extra></extra>"
            )
        )
    )
    
    # Zero line
    fig.add_hline(
        y=0,
        line_dash="dash",
        line_color=COLORS["text_secondary"],
    )
    
    fig.update_layout(
        title="Cumulative P&L by Trade",
        xaxis_title="Time",
        yaxis_title="Cumulative P&L ($)",
        height=300,
        paper_bgcolor=COLORS["bg_main"],
        plot_bgcolor=COLORS["bg_card"],
        font=dict(color=COLORS["text_primary"]),
        showlegend=False,
    )
    
    fig.update_xaxes(
        showgrid=True,
        gridcolor=COLORS["border"],
    )
    
    fig.update_yaxes(
        showgrid=True,
        gridcolor=COLORS["border"],
        tickformat="$,.0f",
    )
    
    st.plotly_chart(fig, width="stretch")
