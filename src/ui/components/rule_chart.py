"""
Rule Mini-Chart Component

Renders small Plotly charts showing rule indicator values and when
conditions evaluated to TRUE. Used in the Strategy Builder to visualize
rule performance.
"""

import streamlit as st
import plotly.graph_objects as go
from plotly.subplots import make_subplots
import pandas as pd
import numpy as np
from typing import Optional, List, Tuple
from datetime import datetime

from src.bot.strategy.rules.models import (
    Rule,
    Condition,
    ConditionType,
    Indicator,
    IndicatorType,
)
from src.bot.strategy.rules.indicators import IndicatorFactory
from src.bot.strategy.rules.evaluator import evaluate_rule_history, get_last_true_info
from src.ui.styles import COLORS
from src.ui.i18n import I18n
from src.ui.translations import translations


# Chart styling constants matching Tokyo Night theme
CHART_BG_COLOR = "#1a1b26"
CHART_PAPER_COLOR = "#1a1b26"
CHART_GRID_COLOR = "#1f2335"
CHART_TEXT_COLOR = "#a9b1d6"
INDICATOR_A_COLOR = "#7aa2f7"  # Blue
INDICATOR_B_COLOR = "#e0af68"  # Yellow
TRUE_MARKER_COLOR = "#9ece6a"  # Green
THRESHOLD_LINE_COLOR = "#f7768e"  # Red


def render_rule_mini_chart(
    rule: Rule,
    bars: pd.DataFrame,
    vix_bars: Optional[pd.DataFrame] = None,
    num_bars: int = 100,
    height: int = 180
) -> None:
    """
    Render a mini-chart showing rule indicators and TRUE points.
    
    Args:
        rule: The Rule to visualize
        bars: DataFrame with OHLCV bar data
        vix_bars: Optional VIX data for VIX-based rules
        num_bars: Number of bars to display (default 100)
        height: Chart height in pixels
    """
    if bars is None or len(bars) == 0:
        st.warning(_get_i18n().t("no_bar_data_visualization"))
        return
    
    # Limit to last N bars
    display_bars = bars.tail(num_bars).copy()
    
    if vix_bars is not None:
        display_vix = vix_bars.tail(num_bars).copy()
    else:
        display_vix = None
    
    # Determine chart type based on condition
    condition = rule.condition
    condition_type = condition.type.value if hasattr(condition.type, 'value') else condition.type
    
    # Create the appropriate chart
    if condition_type in ("crosses_above", "crosses_below"):
        fig = _create_crossover_chart(rule, display_bars, display_vix, height)
    elif condition_type in ("slope_above", "slope_below"):
        fig = _create_slope_chart(rule, display_bars, display_vix, height)
    elif condition_type in ("greater_than", "less_than"):
        fig = _create_threshold_chart(rule, display_bars, display_vix, height)
    elif condition_type == "within_range":
        fig = _create_time_range_chart(rule, display_bars, height)
    else:
        fig = _create_generic_chart(rule, display_bars, display_vix, height)
    
    # Display the chart
    st.plotly_chart(fig, width="stretch", config={'displayModeBar': False})
    
    # Show "Last True" info
    _render_last_true_info(rule, bars, vix_bars)


def _get_x_axis(bars: pd.DataFrame) -> List:
    """Get appropriate x-axis values from bar data."""
    if 'timestamp' in bars.columns:
        return pd.to_datetime(bars['timestamp']).tolist()
    elif isinstance(bars.index, pd.DatetimeIndex):
        return bars.index.tolist()
    else:
        return list(range(len(bars)))


def _apply_dark_theme(fig: go.Figure, height: int) -> go.Figure:
    """Apply dark theme styling to a Plotly figure."""
    fig.update_layout(
        height=height,
        margin=dict(l=0, r=0, t=20, b=20),
        paper_bgcolor=CHART_PAPER_COLOR,
        plot_bgcolor=CHART_BG_COLOR,
        font=dict(color=CHART_TEXT_COLOR, size=10),
        showlegend=True,
        legend=dict(
            yanchor="top",
            y=0.99,
            xanchor="left",
            x=0.01,
            bgcolor="rgba(30, 34, 45, 0.8)",
            bordercolor=CHART_GRID_COLOR,
            borderwidth=1,
            font=dict(size=9)
        ),
        xaxis=dict(
            showgrid=True,
            gridcolor=CHART_GRID_COLOR,
            zeroline=False,
            showticklabels=True,
        ),
        yaxis=dict(
            showgrid=True,
            gridcolor=CHART_GRID_COLOR,
            zeroline=False,
            side='right',
        ),
    )
    return fig


def _add_true_markers(
    fig: go.Figure,
    rule: Rule,
    bars: pd.DataFrame,
    vix_bars: Optional[pd.DataFrame],
    x_values: List,
    indicator_series: np.ndarray
) -> go.Figure:
    """Add markers where the rule condition was TRUE."""
    # Evaluate rule history
    history = evaluate_rule_history(rule, bars, vix_bars)
    
    # Get indices where TRUE
    true_mask = history.values
    
    if np.any(true_mask):
        true_x = [x_values[i] for i in range(len(true_mask)) if true_mask[i]]
        true_y = [indicator_series[i] for i in range(len(true_mask)) if true_mask[i]]
        
        # Filter out NaN values
        valid_points = [(x, y) for x, y in zip(true_x, true_y) if not np.isnan(y)]
        if valid_points:
            true_x, true_y = zip(*valid_points)
            
            fig.add_trace(go.Scatter(
                x=list(true_x),
                y=list(true_y),
                mode='markers',
                name=_get_i18n().t("true_label"),
                marker=dict(
                    symbol='triangle-up',
                    size=10,
                    color=TRUE_MARKER_COLOR,
                    line=dict(color='white', width=1)
                ),
                hovertemplate=f"{_get_i18n().t('true_label')}<br>%{{x}}<extra></extra>"
            ))
    
    return fig


def _create_crossover_chart(
    rule: Rule,
    bars: pd.DataFrame,
    vix_bars: Optional[pd.DataFrame],
    height: int
) -> go.Figure:
    """Create chart for crossover conditions (crosses above/below)."""
    condition = rule.condition
    x_values = _get_x_axis(bars)
    
    fig = go.Figure()
    
    # Get indicator A series
    series_a = IndicatorFactory.create_indicator_series(
        condition.indicator_a, bars, vix_bars
    )
    
    # Plot indicator A
    ind_a_name = condition.indicator_a.to_display_string()
    fig.add_trace(go.Scatter(
        x=x_values,
        y=series_a,
        mode='lines',
        name=ind_a_name,
        line=dict(color=INDICATOR_A_COLOR, width=2),
        hovertemplate=f'{ind_a_name}: %{{y:.2f}}<extra></extra>'
    ))
    
    # Get indicator B series if available
    if condition.indicator_b:
        series_b = IndicatorFactory.create_indicator_series(
            condition.indicator_b, bars, vix_bars
        )
        
        ind_b_name = condition.indicator_b.to_display_string()
        fig.add_trace(go.Scatter(
            x=x_values,
            y=series_b,
            mode='lines',
            name=ind_b_name,
            line=dict(color=INDICATOR_B_COLOR, width=2),
            hovertemplate=f'{ind_b_name}: %{{y:.2f}}<extra></extra>'
        ))
    
    # Add TRUE markers
    fig = _add_true_markers(fig, rule, bars, vix_bars, x_values, series_a)
    
    # Apply dark theme
    fig = _apply_dark_theme(fig, height)
    
    return fig


def _create_slope_chart(
    rule: Rule,
    bars: pd.DataFrame,
    vix_bars: Optional[pd.DataFrame],
    height: int
) -> go.Figure:
    """Create chart for slope conditions."""
    from src.utils.indicators import slope_series
    
    condition = rule.condition
    x_values = _get_x_axis(bars)
    
    # Create subplot with two y-axes
    fig = make_subplots(rows=2, cols=1, row_heights=[0.6, 0.4], shared_xaxes=True,
                        vertical_spacing=0.05)
    
    # Get indicator series
    series_a = IndicatorFactory.create_indicator_series(
        condition.indicator_a, bars, vix_bars
    )
    
    # Plot indicator value
    ind_a_name = condition.indicator_a.to_display_string()
    fig.add_trace(go.Scatter(
        x=x_values,
        y=series_a,
        mode='lines',
        name=ind_a_name,
        line=dict(color=INDICATOR_A_COLOR, width=2),
    ), row=1, col=1)
    
    # Calculate and plot slope
    slopes = slope_series(series_a, condition.lookback_periods)
    
    fig.add_trace(go.Scatter(
        x=x_values,
        y=slopes,
        mode='lines',
        name=f'Slope ({condition.lookback_periods} bars)',
        line=dict(color=INDICATOR_B_COLOR, width=1.5),
    ), row=2, col=1)
    
    # Add threshold line
    if condition.threshold is not None:
        fig.add_hline(
            y=condition.threshold,
            line=dict(color=THRESHOLD_LINE_COLOR, width=1, dash='dash'),
            annotation_text=_get_i18n().t("threshold_label", value=str(condition.threshold)),
            annotation_position="bottom right",
            row=2, col=1
        )
    
    # Add TRUE markers on slope chart
    history = evaluate_rule_history(rule, bars, vix_bars)
    true_mask = history.values
    
    if np.any(true_mask):
        true_x = [x_values[i] for i in range(len(true_mask)) if true_mask[i]]
        true_y = [slopes[i] for i in range(len(true_mask)) if true_mask[i]]
        
        valid_points = [(x, y) for x, y in zip(true_x, true_y) if not np.isnan(y)]
        if valid_points:
            true_x, true_y = zip(*valid_points)
            fig.add_trace(go.Scatter(
                x=list(true_x),
                y=list(true_y),
                mode='markers',
                name=_get_i18n().t("true_label"),
                marker=dict(symbol='triangle-up', size=8, color=TRUE_MARKER_COLOR),
            ), row=2, col=1)
    
    # Apply dark theme
    fig = _apply_dark_theme(fig, height + 40)  # Extra height for two rows
    fig.update_yaxes(title_text=ind_a_name, row=1, col=1)
    fig.update_yaxes(title_text="Slope", row=2, col=1)
    
    return fig


def _create_threshold_chart(
    rule: Rule,
    bars: pd.DataFrame,
    vix_bars: Optional[pd.DataFrame],
    height: int
) -> go.Figure:
    """Create chart for threshold comparison conditions."""
    condition = rule.condition
    x_values = _get_x_axis(bars)
    
    fig = go.Figure()
    
    # Get indicator A series
    series_a = IndicatorFactory.create_indicator_series(
        condition.indicator_a, bars, vix_bars
    )
    
    # Plot indicator A
    ind_a_name = condition.indicator_a.to_display_string()
    fig.add_trace(go.Scatter(
        x=x_values,
        y=series_a,
        mode='lines',
        name=ind_a_name,
        line=dict(color=INDICATOR_A_COLOR, width=2),
    ))
    
    # Plot indicator B or threshold line
    if condition.indicator_b:
        series_b = IndicatorFactory.create_indicator_series(
            condition.indicator_b, bars, vix_bars
        )
        ind_b_name = condition.indicator_b.to_display_string()
        fig.add_trace(go.Scatter(
            x=x_values,
            y=series_b,
            mode='lines',
            name=ind_b_name,
            line=dict(color=INDICATOR_B_COLOR, width=2),
        ))
    elif condition.threshold is not None:
        # Add horizontal threshold line
        fig.add_hline(
            y=condition.threshold,
            line=dict(color=THRESHOLD_LINE_COLOR, width=2, dash='dash'),
            annotation_text=_get_i18n().t("threshold_label", value=str(condition.threshold)),
            annotation_position="bottom right",
        )
    
    # Add TRUE markers
    fig = _add_true_markers(fig, rule, bars, vix_bars, x_values, series_a)
    
    # Apply dark theme
    fig = _apply_dark_theme(fig, height)
    
    return fig


def _create_time_range_chart(
    rule: Rule,
    bars: pd.DataFrame,
    height: int
) -> go.Figure:
    """Create chart for time range conditions."""
    condition = rule.condition
    x_values = _get_x_axis(bars)
    
    fig = go.Figure()
    
    # Create a bar showing in/out of range
    history = evaluate_rule_history(rule, bars, None)
    
    # Color bars based on TRUE/FALSE
    colors = [TRUE_MARKER_COLOR if v else CHART_GRID_COLOR for v in history.values]
    
    fig.add_trace(go.Bar(
        x=x_values,
        y=[1] * len(bars),
        marker_color=colors,
        name=_get_i18n().t("in_range"),
        hovertemplate='%{x}<br>In Range: %{customdata}<extra></extra>',
        customdata=['Yes' if v else 'No' for v in history.values]
    ))
    
    # Apply dark theme
    fig = _apply_dark_theme(fig, height)
    fig.update_yaxes(visible=False)
    fig.update_layout(
        title=dict(
            text=_get_i18n().t("time_range_label", start=condition.range_start, end=condition.range_end),
            font=dict(size=11, color=CHART_TEXT_COLOR)
        )
    )
    
    return fig


def _create_generic_chart(
    rule: Rule,
    bars: pd.DataFrame,
    vix_bars: Optional[pd.DataFrame],
    height: int
) -> go.Figure:
    """Create a generic chart for any condition type."""
    condition = rule.condition
    x_values = _get_x_axis(bars)
    
    fig = go.Figure()
    
    # Get indicator A series
    series_a = IndicatorFactory.create_indicator_series(
        condition.indicator_a, bars, vix_bars
    )
    
    # Plot indicator A
    ind_a_name = condition.indicator_a.to_display_string()
    fig.add_trace(go.Scatter(
        x=x_values,
        y=series_a,
        mode='lines',
        name=ind_a_name,
        line=dict(color=INDICATOR_A_COLOR, width=2),
    ))
    
    # Add TRUE markers
    fig = _add_true_markers(fig, rule, bars, vix_bars, x_values, series_a)
    
    # Apply dark theme
    fig = _apply_dark_theme(fig, height)
    
    return fig


def _render_last_true_info(
    rule: Rule,
    bars: pd.DataFrame,
    vix_bars: Optional[pd.DataFrame]
) -> None:
    """Render the 'Last True' information below the chart."""
    info = get_last_true_info(rule, bars, vix_bars)
    
    if info['last_true_idx'] is None:
        st.markdown(
            f'<div style="text-align: center; color: {COLORS["text_secondary"]}; '
            f'font-size: 0.9em; margin-top: -10px;">'
            f'{_get_i18n().t("last_true_never")}'
            f'</div>',
            unsafe_allow_html=True
        )
    else:
        bars_ago = info['bars_ago']
        total_true = info['total_true_count']
        
        # Format datetime if available
        dt_str = ""
        if info['last_true_datetime'] is not None:
            try:
                if isinstance(info['last_true_datetime'], datetime):
                    dt_str = info['last_true_datetime'].strftime('%Y-%m-%d %H:%M')
                else:
                    dt_str = str(info['last_true_datetime'])
                dt_str = f" ({dt_str})"
            except:
                pass
        
        # Color based on recency
        if bars_ago == 0:
            color = COLORS["accent_green"]
            ago_text = _get_i18n().t("current_bar")
        elif bars_ago <= 5:
            color = COLORS["accent_green"]
            ago_text = _get_i18n().t("bars_ago", count=bars_ago)
        elif bars_ago <= 20:
            color = COLORS["accent_yellow"]
            ago_text = _get_i18n().t("bars_ago", count=bars_ago)
        else:
            color = COLORS["text_secondary"]
            ago_text = _get_i18n().t("bars_ago", count=bars_ago)
        
        st.markdown(
            f'<div style="text-align: center; font-size: 0.9em; margin-top: -10px;">'
            f'<span style="color: {color};">{_get_i18n().t("last_true", ago=ago_text, dt=dt_str)}</span>'
            f' &nbsp;|&nbsp; '
            f'<span style="color: {COLORS["text_secondary"]};">{_get_i18n().t("total_occurrences", count=total_true)}</span>'
            f'</div>',
            unsafe_allow_html=True
        )


def _get_i18n() -> I18n:
    if "i18n" not in st.session_state:
        st.session_state["i18n"] = I18n(translations)
    return st.session_state["i18n"]


def load_sample_data(symbol: str = "SPY", timeframe: str = "5m") -> Optional[pd.DataFrame]:
    """
    Load sample data for chart visualization.
    
    Looks for CSV files in data/sample/ directory.
    
    Args:
        symbol: Ticker symbol (e.g., 'SPY', 'VIX')
        timeframe: Timeframe string (e.g., '5m', '1h')
        
    Returns:
        DataFrame with OHLCV data or None if not found
    """
    from pathlib import Path
    import os
    
    # Find the project root (contains src/ directory)
    # Try multiple approaches for robustness
    base_path = None
    
    # Approach 1: Use __file__ if available and valid
    try:
        current_file = Path(__file__).resolve()
        # Go up from src/ui/components/rule_chart.py to project root
        test_path = current_file.parent.parent.parent.parent / "data" / "sample"
        if test_path.exists():
            base_path = test_path
    except (NameError, TypeError):
        pass
    
    # Approach 2: Use current working directory
    if base_path is None:
        test_path = Path.cwd() / "data" / "sample"
        if test_path.exists():
            base_path = test_path
    
    # Approach 3: Try known project location
    if base_path is None:
        test_path = Path("/home/bruno/Work/Active project/cobalt/tws_traderbot/data/sample")
        if test_path.exists():
            base_path = test_path
    
    if base_path is None or not base_path.exists():
        return None
    
    # Convert timeframe variations (5m -> 5min, etc.)
    tf_variations = [timeframe]
    if timeframe.endswith("m"):
        tf_variations.append(timeframe[:-1] + "min")  # 5m -> 5min
    elif timeframe.endswith("min"):
        tf_variations.append(timeframe[:-3] + "m")    # 5min -> 5m
    if timeframe.endswith("h"):
        tf_variations.append(timeframe[:-1] + "hr")   # 1h -> 1hr
        tf_variations.append(timeframe[:-1] + "hour") # 1h -> 1hour
    
    possible_files = []
    for tf in tf_variations:
        possible_files.extend([
            base_path / f"{symbol}_{tf}.csv",
            base_path / f"{symbol.lower()}_{tf}.csv",
            base_path / f"{symbol.upper()}_{tf}.csv",
        ])
    possible_files.append(base_path / f"{symbol}.csv")
    
    for file_path in possible_files:
        if file_path.exists():
            try:
                df = pd.read_csv(file_path, parse_dates=['timestamp'])
                return df
            except Exception:
                pass  # Try next file
    
    return None
