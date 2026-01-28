"""
Rule Builder Component

Provides a form-based interface for creating trading rules.
Integrates with the strategy rule models for type-safe rule creation.
"""

import streamlit as st
from typing import Optional, Tuple
from uuid import uuid4

from src.bot.strategy.rules.models import (
    Rule,
    RuleScope,
    Condition,
    ConditionType,
    Indicator,
    IndicatorType,
    TimeframeUnit,
    ActionType,
    PriceSource,
)
from src.ui.styles import COLORS


# Mapping for UI display labels to enum values
INDICATOR_TYPE_OPTIONS = {
    "EMA": IndicatorType.EMA,
    "SMA": IndicatorType.SMA,
    "Price": IndicatorType.PRICE,
    "VIX": IndicatorType.VIX,
    "RSI": IndicatorType.RSI,
    "Volume": IndicatorType.VOLUME,
    "Time": IndicatorType.TIME,
    "MACD": IndicatorType.MACD,
    "Bollinger Bands": IndicatorType.BOLLINGER,
    "Stochastic": IndicatorType.STOCHASTIC,
    "On-Balance Volume": IndicatorType.OBV,
    "Williams Alligator": IndicatorType.ALLIGATOR,
    "Dividend Yield": IndicatorType.DIVIDEND_YIELD,
    "P/E Ratio": IndicatorType.PE_RATIO,
    "Rel. Performance": IndicatorType.RELATIVE_PERFORMANCE,
}

TIMEFRAME_OPTIONS = {
    "1m": TimeframeUnit.M1,
    "5m": TimeframeUnit.M5,
    "15m": TimeframeUnit.M15,
    "30m": TimeframeUnit.M30,
    "1h": TimeframeUnit.H1,
    "4h": TimeframeUnit.H4,
    "1d": TimeframeUnit.D1,
}

SOURCE_OPTIONS = {
    "Close": PriceSource.CLOSE,
    "Open": PriceSource.OPEN,
    "High": PriceSource.HIGH,
    "Low": PriceSource.LOW,
    "HL2": PriceSource.HL2,
    "HLC3": PriceSource.HLC3,
}

CONDITION_TYPE_OPTIONS = {
    "crosses above": ConditionType.CROSSES_ABOVE,
    "crosses below": ConditionType.CROSSES_BELOW,
    "greater than (>)": ConditionType.GREATER_THAN,
    "less than (<)": ConditionType.LESS_THAN,
    "slope above (>)": ConditionType.SLOPE_ABOVE,
    "slope below (<)": ConditionType.SLOPE_BELOW,
    "within time range": ConditionType.WITHIN_RANGE,
}

ACTION_OPTIONS = {
    "BUY": ActionType.BUY,
    "SELL": ActionType.SELL,
    "FILTER": ActionType.FILTER,
}


def rule_to_human_readable(rule: Rule) -> str:
    """
    Convert a Rule to a human-readable description string.
    
    Args:
        rule: The Rule object to describe
        
    Returns:
        Human-readable string like "EMA(9, 5m) crosses above EMA(21, 5m)"
    """
    condition = rule.condition
    
    def format_ind(ind):
        itype = ind.type.value if hasattr(ind.type, 'value') else ind.type
        itf = ind.timeframe.value if hasattr(ind.timeframe, 'value') else ind.timeframe
        
        if itype in ['ema', 'sma', 'rsi']:
            base = f"{itype.upper()}({ind.length}, {itf})"
        elif itype == 'price':
            src = ind.source.value if hasattr(ind.source, 'value') else ind.source
            base = f"Price({src})"
        elif itype == 'vix':
            base = f"VIX({itf})"
        elif itype == 'time':
            base = "Time"
        elif itype == 'volume':
            base = f"Volume({itf})"
        else:
            base = f"{itype.upper()}({itf})"
            
        if ind.component:
            base += f"[{ind.component}]"
        return base

    ind_a_str = format_ind(condition.indicator_a)
    
    # Get condition type
    cond_type = condition.type.value if hasattr(condition.type, 'value') else condition.type
    
    # Build condition string based on type
    if cond_type in ['crosses_above', 'crosses_below']:
        # Need indicator B
        ind_b = condition.indicator_b
        if ind_b:
            ind_b_str = format_ind(ind_b)
        else:
            ind_b_str = "?"
        
        operator = "crosses above" if cond_type == 'crosses_above' else "crosses below"
        return f"{ind_a_str} {operator} {ind_b_str}"
    
    elif cond_type in ['greater_than', 'less_than', 'equals']:
        operator_map = {
            'greater_than': '>',
            'less_than': '<',
            'equals': '=='
        }
        operator = operator_map.get(cond_type, cond_type)
        
        if condition.indicator_b:
            ind_b_str = format_ind(condition.indicator_b)
            return f"{ind_a_str} {operator} {ind_b_str}"
        elif condition.threshold is not None:
            return f"{ind_a_str} {operator} {condition.threshold}"
        else:
            return f"{ind_a_str} {operator} ?"
    
    elif cond_type in ['slope_above', 'slope_below']:
        operator = "slope >" if cond_type == 'slope_above' else "slope <"
        threshold = condition.threshold if condition.threshold is not None else 0
        return f"{ind_a_str} {operator} {threshold} (over {condition.lookback_periods} periods)"
    
    elif cond_type == 'within_range':
        start = condition.range_start or "?"
        end = condition.range_end or "?"
        return f"Time within {start} - {end}"
    
    else:
        return f"{ind_a_str} {cond_type}"


def render_indicator_inputs(
    prefix: str,
    default_type: str = "EMA",
    default_length: int = 9,
    default_timeframe: str = "5m",
    default_source: str = "Close"
) -> Tuple[Indicator, bool]:
    """
    Render indicator configuration inputs.
    
    Args:
        prefix: Unique prefix for widget keys
        default_type: Default indicator type
        default_length: Default indicator length
        default_timeframe: Default timeframe
        default_source: Default price source
        
    Returns:
        Tuple of (Indicator object, is_valid)
    """
    col1, col2, col3 = st.columns([2, 1, 1])
    
    with col1:
        # Sort keys but keep EMA first for convenience if desired, or just sorted
        # Current INDICATOR_TYPE_OPTIONS is ordered by insertion in Python 3.7+
        keys = list(INDICATOR_TYPE_OPTIONS.keys())
        ind_type_label = st.selectbox(
            "Type",
            options=keys,
            index=keys.index(default_type) if default_type in keys else 0,
            key=f"{prefix}_type"
        )
        ind_type = INDICATOR_TYPE_OPTIONS[ind_type_label]
    
    with col2:
        tf_keys = list(TIMEFRAME_OPTIONS.keys())
        tf_label = st.selectbox(
            "Timeframe",
            options=tf_keys,
            index=tf_keys.index(default_timeframe) if default_timeframe in tf_keys else 1,
            key=f"{prefix}_timeframe"
        )
        timeframe = TIMEFRAME_OPTIONS[tf_label]
    
    with col3:
        src_keys = list(SOURCE_OPTIONS.keys())
        source_label = st.selectbox(
            "Source",
            options=src_keys,
            index=src_keys.index(default_source) if default_source in src_keys else 0,
            key=f"{prefix}_source"
        )
        source = SOURCE_OPTIONS[source_label]
    
    # Dynamic parameters based on type
    length = None
    params = {}
    component = None
    
    if ind_type_label in ["EMA", "SMA", "RSI"]:
        length = st.number_input(
            "Length",
            min_value=1,
            max_value=500,
            value=default_length,
            key=f"{prefix}_length"
        )
        
    elif ind_type_label == "MACD":
        c1, c2, c3, c4 = st.columns(4)
        with c1:
            params["fast_period"] = st.number_input("Fast Length", value=12, key=f"{prefix}_fast")
        with c2:
            params["slow_period"] = st.number_input("Slow Length", value=26, key=f"{prefix}_slow")
        with c3:
            params["signal_period"] = st.number_input("Signal Length", value=9, key=f"{prefix}_signal")
        with c4:
            component = st.selectbox("Output", ["macd", "signal", "histogram"], key=f"{prefix}_comp")
            
    elif ind_type_label == "Bollinger Bands":
        c1, c2, c3, c4 = st.columns(4)
        with c1:
            length = st.number_input("Length", value=20, key=f"{prefix}_len")
        with c2:
            params["std_dev"] = st.number_input("StdDev", value=2.0, key=f"{prefix}_std")
        with c3:
            params["offset"] = st.number_input("Offset", value=0, key=f"{prefix}_offset")
        with c4:
            component = st.selectbox("Band", ["upper", "middle", "lower"], key=f"{prefix}_comp")
            
    elif ind_type_label == "Stochastic":
        c1, c2, c3, c4 = st.columns(4)
        with c1:
            params["k_period"] = st.number_input("%K Length", value=14, key=f"{prefix}_k")
        with c2:
            params["d_period"] = st.number_input("%D Smoothing", value=3, key=f"{prefix}_d")
        with c3:
            params["smooth_k"] = st.number_input("%K Smoothing", value=3, key=f"{prefix}_smooth")
        with c4:
            component = st.selectbox("Line", ["k", "d"], key=f"{prefix}_comp")
            
    elif ind_type_label == "Williams Alligator":
        with st.expander("Alligator Settings"):
            c1, c2, c3 = st.columns(3)
            with c1:
                params["jaw_period"] = st.number_input("Jaw Length", value=13, key=f"{prefix}_jaw_p")
                params["jaw_shift"] = st.number_input("Jaw Offset", value=8, key=f"{prefix}_jaw_s")
            with c2:
                params["teeth_period"] = st.number_input("Teeth Length", value=8, key=f"{prefix}_teeth_p")
                params["teeth_shift"] = st.number_input("Teeth Offset", value=5, key=f"{prefix}_teeth_s")
            with c3:
                params["lips_period"] = st.number_input("Lips Length", value=5, key=f"{prefix}_lips_p")
                params["lips_shift"] = st.number_input("Lips Offset", value=3, key=f"{prefix}_lips_s")
        component = st.selectbox("Line", ["jaw", "teeth", "lips"], key=f"{prefix}_comp")

    # Build indicator
    indicator = Indicator(
        type=ind_type,
        length=length,
        timeframe=timeframe,
        source=source,
        params=params,
        component=component
    )
    
    # Valid if we have a length for types that need it
    is_valid = True
    if ind_type_label in ["EMA", "SMA", "RSI"] and length is None:
        is_valid = False
    
    return indicator, is_valid


def render_rule_builder(key_prefix: str = "rule_builder") -> Optional[Rule]:
    """
    Render the complete rule builder form.
    
    Args:
        key_prefix: Prefix for all widget keys to ensure uniqueness
        
    Returns:
        Rule object if form is submitted successfully, None otherwise
    """
    st.markdown("##### Create New Rule")
    
    # Rule name and scope row
    col1, col2 = st.columns([2, 1])
    
    with col1:
        rule_name = st.text_input(
            "Rule Name",
            placeholder="e.g., EMA Crossover Buy Signal",
            key=f"{key_prefix}_name"
        )
    
    with col2:
        scope_label = st.radio(
            "Scope",
            options=["Per-Ticker", "Global"],
            horizontal=True,
            key=f"{key_prefix}_scope"
        )
        rule_scope = RuleScope.GLOBAL if scope_label == "Global" else RuleScope.PER_TICKER
    
    st.markdown("---")
    
    # Indicator A section
    st.markdown(f'<span style="color: {COLORS["accent_blue"]}; font-weight: bold;">📊 Indicator A</span>', unsafe_allow_html=True)
    indicator_a, ind_a_valid = render_indicator_inputs(
        prefix=f"{key_prefix}_ind_a",
        default_type="EMA",
        default_length=9,
        default_timeframe="5m"
    )
    
    st.markdown("---")
    
    # Condition type
    col1, col2 = st.columns([2, 1])
    
    with col1:
        condition_label = st.selectbox(
            "Condition Type",
            options=list(CONDITION_TYPE_OPTIONS.keys()),
            key=f"{key_prefix}_condition"
        )
        condition_type = CONDITION_TYPE_OPTIONS[condition_label]
    
    with col2:
        lookback = st.number_input(
            "Lookback Periods",
            min_value=1,
            max_value=100,
            value=1,
            key=f"{key_prefix}_lookback",
            help="Number of periods for slope calculation"
        )
    
    # Determine what additional inputs to show
    needs_indicator_b = condition_type in [
        ConditionType.CROSSES_ABOVE,
        ConditionType.CROSSES_BELOW,
        ConditionType.GREATER_THAN,
        ConditionType.LESS_THAN,
    ]
    needs_threshold = condition_type in [
        ConditionType.SLOPE_ABOVE,
        ConditionType.SLOPE_BELOW,
        ConditionType.GREATER_THAN,
        ConditionType.LESS_THAN,
    ]
    needs_time_range = condition_type == ConditionType.WITHIN_RANGE
    
    indicator_b = None
    threshold = None
    range_start = None
    range_end = None
    
    if needs_indicator_b:
        st.markdown("---")
        st.markdown(f'<span style="color: {COLORS["accent_green"]}; font-weight: bold;">📊 Indicator B</span>', unsafe_allow_html=True)
        
        # Option to compare to indicator or threshold
        if needs_threshold:
            compare_to = st.radio(
                "Compare to",
                options=["Another Indicator", "Threshold Value"],
                horizontal=True,
                key=f"{key_prefix}_compare_to"
            )
            
            if compare_to == "Another Indicator":
                indicator_b, ind_b_valid = render_indicator_inputs(
                    prefix=f"{key_prefix}_ind_b",
                    default_type="EMA",
                    default_length=21,
                    default_timeframe="5m"
                )
            else:
                threshold = st.number_input(
                    "Threshold Value",
                    value=0.0,
                    step=0.01,
                    format="%.4f",
                    key=f"{key_prefix}_threshold"
                )
        else:
            indicator_b, ind_b_valid = render_indicator_inputs(
                prefix=f"{key_prefix}_ind_b",
                default_type="EMA",
                default_length=21,
                default_timeframe="5m"
            )
    
    elif needs_threshold:
        st.markdown("---")
        threshold = st.number_input(
            "Threshold Value",
            value=0.0,
            step=0.01,
            format="%.4f",
            key=f"{key_prefix}_threshold",
            help="The value to compare the slope against"
        )
    
    elif needs_time_range:
        st.markdown("---")
        st.markdown(f'<span style="color: {COLORS["accent_blue"]}; font-weight: bold;">⏰ Time Range</span>', unsafe_allow_html=True)
        col1, col2 = st.columns(2)
        with col1:
            range_start = st.text_input(
                "Start Time (HH:MM)",
                value="09:30",
                key=f"{key_prefix}_range_start",
                help="Market open time in 24-hour format"
            )
        with col2:
            range_end = st.text_input(
                "End Time (HH:MM)",
                value="16:00",
                key=f"{key_prefix}_range_end",
                help="Market close time in 24-hour format"
            )
    
    st.markdown("---")
    
    # Action selection
    col1, col2 = st.columns(2)
    
    with col1:
        action_label = st.selectbox(
            "Action",
            options=list(ACTION_OPTIONS.keys()),
            key=f"{key_prefix}_action",
            help="BUY/SELL for signals, FILTER for conditions that must pass"
        )
        action = ACTION_OPTIONS[action_label]
    
    with col2:
        priority = st.slider(
            "Priority",
            min_value=0,
            max_value=100,
            value=0,
            key=f"{key_prefix}_priority",
            help="Higher priority rules are evaluated first"
        )
    
    # Preview
    st.markdown("---")
    st.markdown("**Preview:**")
    
    # Build preview condition
    preview_condition = Condition(
        type=condition_type,
        indicator_a=indicator_a,
        indicator_b=indicator_b,
        threshold=threshold,
        lookback_periods=lookback,
        range_start=range_start,
        range_end=range_end
    )
    preview_rule = Rule(
        id=str(uuid4()),
        name=rule_name or "Unnamed Rule",
        scope=rule_scope,
        condition=preview_condition,
        action=action,
        priority=priority
    )
    
    # Show preview
    preview_text = rule_to_human_readable(preview_rule)
    scope_badge = "🌍 Global" if rule_scope == RuleScope.GLOBAL else "🎯 Per-Ticker"
    
    st.markdown(
        f'<div style="background-color: {COLORS["bg_card"]}; '
        f'border: 1px solid {COLORS["border"]}; border-radius: 8px; '
        f'padding: 1rem; margin: 0.5rem 0;">'
        f'<span style="color: {COLORS["accent_blue"]};">{scope_badge}</span> '
        f'<span style="color: {COLORS["text_primary"]};">{preview_text}</span> '
        f'→ <span style="color: {COLORS["accent_green"] if action == ActionType.BUY else COLORS["accent_red"] if action == ActionType.SELL else COLORS["accent_yellow"]};">'
        f'{action_label}</span>'
        f'</div>',
        unsafe_allow_html=True
    )
    
    # Add button
    if st.button("➕ Add Rule", key=f"{key_prefix}_add_btn", width="stretch", type="primary"):
        if not rule_name:
            st.error("Please enter a rule name")
            return None
        
        # Build the final rule
        condition = Condition(
            type=condition_type,
            indicator_a=indicator_a,
            indicator_b=indicator_b,
            threshold=threshold,
            lookback_periods=lookback,
            range_start=range_start,
            range_end=range_end
        )
        
        rule = Rule(
            id=str(uuid4()),
            name=rule_name,
            scope=rule_scope,
            condition=condition,
            action=action,
            enabled=True,
            priority=priority
        )
        
        return rule
    
    return None
