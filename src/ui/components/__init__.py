"""
UI Components Package

Reusable Streamlit components for the trading bot UI.
"""

from src.ui.components.rule_builder import (
    render_rule_builder,
    render_indicator_inputs,
    rule_to_human_readable,
)
from src.ui.components.rule_display import (
    render_rule_card,
    render_rules_list,
    render_compact_rule_summary,
)
from src.ui.components.charts import (
    render_equity_curve,
    render_metrics_cards,
    render_trade_table,
    render_trade_distribution,
    render_cumulative_pnl,
    render_quantstats_report,
)
from src.ui.components.rule_chart import (
    render_rule_mini_chart,
    load_sample_data,
)
from src.ui.components.watchlist import (
    render_watchlist_manager,
    render_watchlist_compact,
)

__all__ = [
    # Rule builder
    "render_rule_builder",
    "render_indicator_inputs",
    "rule_to_human_readable",
    # Rule display
    "render_rule_card",
    "render_rules_list",
    "render_compact_rule_summary",
    # Charts
    "render_equity_curve",
    "render_metrics_cards",
    "render_trade_table",
    "render_trade_distribution",
    "render_quantstats_report",
    "render_cumulative_pnl",
    # Rule mini-charts
    "render_rule_mini_chart",
    "load_sample_data",
    # Watchlist
    "render_watchlist_manager",
    "render_watchlist_compact",
]
