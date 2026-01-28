"""
Rule Display Component

Provides styled card display for trading rules with enable/disable and delete functionality.
"""

import streamlit as st
from typing import Callable, Optional
import pandas as pd

from src.bot.strategy.rules.models import Rule, RuleScope, ActionType
from src.ui.styles import COLORS
from src.ui.components.rule_builder import rule_to_human_readable
from src.ui.i18n import I18n
from src.ui.translations import translations


def render_rule_card(
    rule: Rule,
    index: int,
    on_delete: Optional[Callable[[str], None]] = None,
    on_toggle: Optional[Callable[[str, bool], None]] = None,
    show_chart_placeholder: bool = True
) -> bool:
    """
    Render a rule as a styled card.
    
    Args:
        rule: The Rule object to display
        index: Index for unique key generation
        on_delete: Callback when delete is clicked, receives rule_id
        on_toggle: Callback when toggle is clicked, receives (rule_id, new_state)
        show_chart_placeholder: Whether to show the chart placeholder area
        
    Returns:
        Current enabled state of the rule
    """
    i18n = _get_i18n()
    # Get scope info
    scope_val = rule.scope.value if hasattr(rule.scope, 'value') else rule.scope
    is_global = scope_val == "global"
    scope_emoji = "🌍" if is_global else "🎯"
    scope_label = i18n.t("scope_global") if is_global else i18n.t("scope_per_ticker")
    
    # Get action info
    action_val = rule.action.value if hasattr(rule.action, 'value') else rule.action
    if action_val == "buy":
        action_color = COLORS["accent_green"]
        action_label = i18n.t("action_buy")
    elif action_val == "sell":
        action_color = COLORS["accent_red"]
        action_label = i18n.t("action_sell")
    else:
        action_color = COLORS["accent_yellow"]
        action_label = i18n.t("action_filter")
    
    # Build condition description
    condition_text = rule_to_human_readable(rule)
    
    # Card container
    with st.container():
        # Apply card styling
        st.markdown(
            f'<div style="background-color: {COLORS["bg_card"]}; '
            f'border: 1px solid {COLORS["border"]}; border-radius: 8px; '
            f'padding: 1rem; margin-bottom: 0.5rem;"'
            f'class="rule-card">',
            unsafe_allow_html=True
        )
        
        # Header row with name, scope badge, and controls
        col1, col2, col3 = st.columns([4, 1, 1])
        
        with col1:
            # Rule name with enabled/disabled styling
            name_style = f"color: {COLORS['text_primary']}" if rule.enabled else f"color: {COLORS['text_secondary']}; text-decoration: line-through"
            st.markdown(
                f'<div style="{name_style}; font-weight: bold; font-size: 1.1em;">'
                f'{scope_emoji} {rule.name}'
                f'</div>',
                unsafe_allow_html=True
            )
            
            # Scope and action badges
            st.markdown(
                f'<div style="margin-top: 4px;">'
                f'<span style="background-color: {COLORS["bg_input"]}; color: {COLORS["text_secondary"]}; '
                f'padding: 2px 8px; border-radius: 4px; font-size: 0.8em; margin-right: 8px;">{scope_label}</span>'
                f'<span style="background-color: {action_color}22; color: {action_color}; '
                f'padding: 2px 8px; border-radius: 4px; font-size: 0.8em;">{action_label}</span>'
                f'</div>',
                unsafe_allow_html=True
            )
        
        with col2:
            # Enable/disable toggle
            enabled = st.toggle(
                i18n.t("enabled"),
                value=rule.enabled,
                key=f"rule_toggle_{rule.id}_{index}",
                label_visibility="collapsed"
            )
            if enabled != rule.enabled and on_toggle:
                on_toggle(rule.id, enabled)
        
        with col3:
            # Delete button
            if st.button("🗑️", key=f"delete_rule_{rule.id}_{index}", help=i18n.t("delete_rule")):
                if on_delete:
                    on_delete(rule.id)
        
        # Condition description
        st.markdown(
            f'<div style="color: {COLORS["text_secondary"]}; font-size: 0.95em; '
            f'margin: 0.75rem 0; padding: 0.5rem; background-color: {COLORS["bg_main"]}; '
            f'border-radius: 4px; font-family: monospace;">'
            f'📐 {condition_text}'
            f'</div>',
            unsafe_allow_html=True
        )
        
        # Priority indicator
        if rule.priority > 0:
            st.markdown(
                f'<div style="color: {COLORS["text_secondary"]}; font-size: 0.8em;">'
                f'{i18n.t("priority_label", priority=rule.priority)}'
                f'</div>',
                unsafe_allow_html=True
            )
        
        # Chart for "Last True" visualization
        if show_chart_placeholder:
            with st.expander(i18n.t("view_rule_performance"), expanded=False):
                _render_rule_performance_chart(rule, index)
        
        # Close the card div
        st.markdown('</div>', unsafe_allow_html=True)
    
    return enabled


def _render_rule_performance_chart(rule: Rule, index: int) -> None:
    """
    Render the rule performance mini-chart.
    
    Loads sample data and displays the indicator values with TRUE markers.
    Uses caching to avoid reloading data on every rerun.
    """
    # Get cache key for this rule's data
    cache_key = f"rule_chart_data_{rule.id}"
    
    # Try to load from cache
    if cache_key not in st.session_state:
        with st.spinner(_get_i18n().t("loading_chart_data")):
            # Load sample data
            from src.ui.components.rule_chart import load_sample_data
            
            # Determine which symbol to load based on indicator
            indicator = rule.condition.indicator_a
            ind_type = indicator.type.value if hasattr(indicator.type, 'value') else indicator.type
            
            if ind_type == "vix":
                bars = load_sample_data("VIX", "5m")
                vix_bars = bars  # VIX data is the bars
            else:
                bars = load_sample_data("SPY", "5m")
                vix_bars = load_sample_data("VIX", "5m")
            
            # Store in session state cache
            st.session_state[cache_key] = {
                'bars': bars,
                'vix_bars': vix_bars
            }
    
    # Get cached data
    cached = st.session_state.get(cache_key, {})
    bars = cached.get('bars')
    vix_bars = cached.get('vix_bars')
    
    if bars is None or len(bars) == 0:
        i18n = _get_i18n()
        st.markdown(
            f'<div style="background-color: {COLORS["bg_main"]}; '
            f'border: 1px dashed {COLORS["border"]}; border-radius: 4px; '
            f'padding: 1.5rem; text-align: center; color: {COLORS["text_secondary"]};">'
            f'{i18n.t("no_sample_data")}'
            f'</div>',
            unsafe_allow_html=True
        )
    else:
        # Render the mini-chart
        from src.ui.components.rule_chart import render_rule_mini_chart
        render_rule_mini_chart(rule, bars, vix_bars, num_bars=100, height=180)


def render_rules_list(
    rules: list,
    on_delete: Optional[Callable[[str], None]] = None,
    on_toggle: Optional[Callable[[str, bool], None]] = None
) -> None:
    """
    Render a list of rules as cards.
    
    Args:
        rules: List of Rule objects
        on_delete: Callback for delete actions
        on_toggle: Callback for toggle actions
    """
    i18n = _get_i18n()
    if not rules:
        st.info(
            i18n.t("no_rules_defined"),
            icon="📝"
        )
        return
    
    # Separate global and per-ticker rules
    global_rules = [r for r in rules if (r.scope.value if hasattr(r.scope, 'value') else r.scope) == "global"]
    ticker_rules = [r for r in rules if (r.scope.value if hasattr(r.scope, 'value') else r.scope) == "per_ticker"]
    
    # Display global rules first (filters)
    if global_rules:
        st.markdown(
            f'<div style="color: {COLORS["text_secondary"]}; font-size: 0.9em; margin-bottom: 0.5rem;">'
            f'{i18n.t("global_filters", count=len(global_rules))}'
            f'</div>',
            unsafe_allow_html=True
        )
        for i, rule in enumerate(global_rules):
            render_rule_card(rule, i, on_delete, on_toggle)
    
    # Display per-ticker rules
    if ticker_rules:
        st.markdown(
            f'<div style="color: {COLORS["text_secondary"]}; font-size: 0.9em; margin-bottom: 0.5rem; margin-top: 1rem;">'
            f'{i18n.t("per_ticker_signals", count=len(ticker_rules))}'
            f'</div>',
            unsafe_allow_html=True
        )
        for i, rule in enumerate(ticker_rules):
            render_rule_card(rule, i + len(global_rules), on_delete, on_toggle)


def render_compact_rule_summary(rule: Rule) -> None:
    """
    Render a compact one-line summary of a rule.
    
    Args:
        rule: The Rule to summarize
    """
    i18n = _get_i18n()
    scope_val = rule.scope.value if hasattr(rule.scope, 'value') else rule.scope
    scope_emoji = "🌍" if scope_val == "global" else "🎯"
    
    action_val = rule.action.value if hasattr(rule.action, 'value') else rule.action
    action_color = (
        COLORS["accent_green"] if action_val == "buy" 
        else COLORS["accent_red"] if action_val == "sell" 
        else COLORS["accent_yellow"]
    )
    
    enabled_icon = "✓" if rule.enabled else "✗"
    enabled_color = COLORS["accent_green"] if rule.enabled else COLORS["accent_red"]
    
    condition_text = rule_to_human_readable(rule)
    
    st.markdown(
        f'<div style="font-size: 0.9em; padding: 4px 0;">'
        f'<span style="color: {enabled_color};">[{enabled_icon}]</span> '
        f'{scope_emoji} '
        f'<span style="color: {COLORS["text_primary"]};">{rule.name}</span>: '
        f'<span style="color: {COLORS["text_secondary"]};">{condition_text}</span> '
        f'→ <span style="color: {action_color};">{action_val.upper()}</span>'
        f'</div>',
        unsafe_allow_html=True
    )


def _get_i18n() -> I18n:
    if "i18n" not in st.session_state:
        st.session_state["i18n"] = I18n(translations)
    return st.session_state["i18n"]
