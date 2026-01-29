"""
Strategy Builder tab for the Streamlit UI (RBTSUI).

Allows users to create, edit, backtest, and deploy trading strategies
using proper Pydantic models and rule builder components.

Integrates with TWS (Trader Workstation) for real market data and
watchlist-based ticker selection.
"""

import streamlit as st
import json
from datetime import datetime, date, timedelta
from pathlib import Path
from typing import Optional, List, Tuple
import uuid

from src.bot.strategy.rules.models import Strategy, Rule, RuleScope, ActionType
from src.bot.strategy.validator import validate_strategy, is_valid
from src.bot.backtest_runner import BacktestEngine, BacktestResult, NAUTILUS_BACKTEST_AVAILABLE
from src.bot.tws_data_provider import get_tws_provider, TWSDataProvider
from src.ui.styles import COLORS
from src.ui.components import (
    render_rule_builder,
    render_rules_list,
    rule_to_human_readable,
)
from src.ui.components.charts import (
    render_equity_curve,
    render_metrics_cards,
    render_trade_table,
    render_trade_distribution,
    render_cumulative_pnl,
    render_quantstats_report,
)
from src.ui.i18n import I18n
from src.ui.translations import translations


# Project root (from src/ui/tabs -> src/ui -> src -> project root)
_PROJECT_ROOT = Path(__file__).parent.parent.parent.parent

# Watchlist file path for custom ticker lists
WATCHLIST_PATH = _PROJECT_ROOT / "config" / "watchlist.txt"

# Default strategy file path
ACTIVE_STRATEGY_PATH = _PROJECT_ROOT / "config" / "active_strategy.json"

# Reload signal file path (tells the bot to reload strategy)
RELOAD_SIGNAL_FILE = Path(__file__).parent.parent.parent / "config" / ".reload_signal"


def _get_i18n() -> I18n:
    if "i18n" not in st.session_state:
        st.session_state["i18n"] = I18n(translations)
    return st.session_state["i18n"]


def _load_strategy_from_session() -> Optional[Strategy]:
    """Load strategy from session state or return None."""
    if "current_strategy" in st.session_state:
        return st.session_state.current_strategy
    return None


def _save_strategy_to_session(strategy: Strategy) -> None:
    """Save strategy to session state."""
    st.session_state.current_strategy = strategy


def _load_strategy_from_file(filepath: Path) -> Optional[Strategy]:
    """Load strategy from JSON file."""
    try:
        if filepath.exists():
            with open(filepath, "r") as f:
                data = json.load(f)
            return Strategy.model_validate(data)
    except Exception as e:
        i18n = _get_i18n()
        st.error(i18n.t("error_loading_strategy", error=str(e)))
    return None


def _save_strategy_to_file(strategy: Strategy, filepath: Path) -> bool:
    """Save strategy to JSON file."""
    try:
        filepath.parent.mkdir(parents=True, exist_ok=True)
        with open(filepath, "w") as f:
            json.dump(strategy.model_dump(mode="json"), f, indent=2, default=str)
        return True
    except Exception as e:
        i18n = _get_i18n()
        st.error(i18n.t("error_saving_strategy", error=str(e)))
        return False


def _get_or_create_strategy() -> Strategy:
    """Get current strategy from session or create a new one."""
    # First check session state
    strategy = _load_strategy_from_session()
    if strategy:
        return strategy
    
    # Try to load from active_strategy.json
    strategy = _load_strategy_from_file(ACTIVE_STRATEGY_PATH)
    if strategy:
        _save_strategy_to_session(strategy)
        return strategy
    
    # Create a new empty strategy
    i18n = _get_i18n()
    strategy = Strategy(
        id=str(uuid.uuid4()),
        name=i18n.t("new_strategy"),
        version="1.0.0",
        description=i18n.t("created_with_strategy_builder"),
        rules=[],
    )
    _save_strategy_to_session(strategy)
    return strategy


def _load_watchlist_from_file() -> List[str]:
    """
    Load ticker symbols from the local watchlist file.
    
    Returns:
        List of ticker symbols from config/watchlist.txt
    """
    symbols = []
    try:
        if WATCHLIST_PATH.exists():
            with open(WATCHLIST_PATH, "r") as f:
                for line in f:
                    line = line.strip()
                    # Skip empty lines and comments
                    if line and not line.startswith("#"):
                        symbols.append(line.upper())
    except Exception as e:
        i18n = _get_i18n()
        st.warning(i18n.t("watchlist_load_failed", error=str(e)))
    return symbols


def _get_tws_connection_status() -> Tuple[bool, str]:
    """
    Check if TWS is connected and available.
    
    Returns:
        Tuple of (is_connected, status_message)
    """
    try:
        provider = get_tws_provider()
        if provider.is_connected():
            return True, _get_i18n().t("tws_connected_status")
        else:
            return False, _get_i18n().t("tws_not_connected_status")
    except Exception as e:
        return False, _get_i18n().t("tws_error_status", error=str(e)[:30])


def _get_available_tickers() -> Tuple[List[str], str]:
    """
    Get available ticker symbols for backtesting.
    
    First tries to fetch from TWS (positions, account holdings),
    then falls back to local watchlist file.
    
    Returns:
        Tuple of (list of ticker symbols, data source description)
    """
    # First, try to load from local watchlist file (always available)
    watchlist_symbols = _load_watchlist_from_file()
    
    # Try to get symbols from TWS
    try:
        provider = get_tws_provider()
        if provider.is_connected():
            # Get symbols from positions
            tws_symbols = provider.get_watchlist_symbols()
            if tws_symbols:
                # Merge TWS symbols with watchlist, TWS first
                combined = list(dict.fromkeys(tws_symbols + watchlist_symbols))
                return combined, _get_i18n().t("ticker_source_tws_watchlist")
    except Exception:
        pass
    
    # Return watchlist symbols if available
    if watchlist_symbols:
        return watchlist_symbols, _get_i18n().t("ticker_source_local_watchlist")
    
    # Default fallback symbols
    default_symbols = ["SPY", "QQQ", "AAPL", "MSFT", "GOOGL", "AMZN", "NVDA", "TSLA"]
    return default_symbols, _get_i18n().t("ticker_source_default")


def render_strategy_tab() -> None:
    """Render the Strategy Builder tab content."""
    i18n = _get_i18n()
    # Get or create strategy
    strategy = _get_or_create_strategy()
    
    # Strategy Header
    _render_strategy_header(strategy)
    
    st.divider()
    
    # Main content in two columns
    col1, col2 = st.columns([3, 2])
    
    with col1:
        # Rules Section
        _render_rules_section(strategy)
    
    with col2:
        # Backtest Section
        _render_backtest_section(strategy)
    
    st.divider()
    
    # Action Buttons
    _render_action_buttons(strategy)


def _render_strategy_header(strategy: Strategy) -> None:
    """Render the strategy header with name and description."""
    i18n = _get_i18n()
    col1, col2 = st.columns([3, 1])
    
    with col1:
        # Editable strategy name
        new_name = st.text_input(
            i18n.t("strategy_name"),
            value=strategy.name,
            key="strategy_name_input",
            label_visibility="collapsed",
            placeholder=i18n.t("strategy_name_placeholder"),
        )
        
        # Update if changed
        if new_name != strategy.name:
            strategy.name = new_name
            strategy.updated_at = datetime.now()
            _save_strategy_to_session(strategy)
        
        # Description
        st.markdown(
            f'<span style="color: {COLORS["text_secondary"]};">'
            f'{i18n.t("strategy_header_subtitle", count=len(strategy.rules))}'
            f'</span>',
            unsafe_allow_html=True
        )
    
    with col2:
        # Strategy status
        is_deployed = st.session_state.get("strategy_deployed", False)
        status_color = COLORS["accent_green"] if is_deployed else COLORS["accent_yellow"]
        status_text = i18n.t("strategy_status_deployed") if is_deployed else i18n.t("strategy_status_draft")
        st.markdown(
            f'<div style="text-align: right; color: {status_color};">'
            f'<strong>{status_text}</strong>'
            f'</div>',
            unsafe_allow_html=True
        )
        
        # Rule count summary
        global_rules = len([r for r in strategy.rules if r.scope == RuleScope.GLOBAL])
        ticker_rules = len([r for r in strategy.rules if r.scope == RuleScope.PER_TICKER])
        st.markdown(
            f'<div style="text-align: right; color: {COLORS["text_secondary"]}; font-size: 0.85em;">'
            f'{i18n.t("strategy_rule_counts", global_count=global_rules, ticker_count=ticker_rules)}'
            f'</div>',
            unsafe_allow_html=True
        )


def _render_rules_section(strategy: Strategy) -> None:
    """Render the rules list and rule builder."""
    i18n = _get_i18n()
    st.markdown(f"#### {i18n.t('trading_rules')}")
    
    # Callbacks for rule operations
    def on_delete_rule(rule_id: str):
        strategy.rules = [r for r in strategy.rules if r.id != rule_id]
        strategy.updated_at = datetime.now()
        _save_strategy_to_session(strategy)
        st.rerun()
    
    def on_toggle_rule(rule_id: str, enabled: bool):
        for rule in strategy.rules:
            if rule.id == rule_id:
                rule.enabled = enabled
                break
        strategy.updated_at = datetime.now()
        _save_strategy_to_session(strategy)
    
    # Display existing rules using the new component
    render_rules_list(
        rules=strategy.rules,
        on_delete=on_delete_rule,
        on_toggle=on_toggle_rule,
    )
    
    # Add Rule section
    st.markdown("---")
    with st.expander(i18n.t("add_new_rule"), expanded=False):
        # Use the new rule builder component
        new_rule = render_rule_builder(key_prefix="new_rule")
        
        if new_rule:
            # Add to strategy
            strategy.rules.append(new_rule)
            strategy.updated_at = datetime.now()
            _save_strategy_to_session(strategy)
            st.success(i18n.t("rule_added_success", name=new_rule.name))
            st.rerun()


def _render_backtest_section(strategy: Strategy) -> None:
    """Render the backtest configuration section with TWS integration."""
    i18n = _get_i18n()
    st.markdown(f"#### {i18n.t('backtest_configuration')}")
    
    # Show TWS connection status
    is_connected, status_msg = _get_tws_connection_status()
    status_color = COLORS["accent_green"] if is_connected else COLORS["accent_red"]
    st.markdown(
        f'<div style="color: {status_color}; font-size: 0.8em; margin-bottom: 8px;">'
        f'{status_msg}</div>',
        unsafe_allow_html=True
    )
    
    with st.container():
        # Get available tickers from TWS/watchlist
        available_tickers, ticker_source = _get_available_tickers()
        
        # Determine default selection
        default_selection = []
        if strategy.tickers:
            # Use strategy tickers if they exist in available list
            default_selection = [t for t in strategy.tickers if t in available_tickers]
        if not default_selection:
            # Default to first few symbols
            default_selection = available_tickers[:2] if len(available_tickers) >= 2 else available_tickers
        
        # Initialize session state if needed
        if "backtest_tickers" not in st.session_state:
            st.session_state.backtest_tickers = default_selection
            
        # Load from watchlist button
        col_lbl, col_btn = st.columns([3, 1])
        with col_lbl:
               st.markdown(i18n.t("select_tickers", source=ticker_source))
        with col_btn:
               if st.button(i18n.t("load_watchlist"), help=i18n.t("load_watchlist_help")):
                 watchlist_symbols = _load_watchlist_from_file()
                 st.session_state.backtest_tickers = watchlist_symbols
                 st.rerun()

        # Render unified symbol search component
        from src.ui.components.symbol_search import render_symbol_multiselect
        tickers = render_symbol_multiselect(
            session_key="backtest_tickers",
            url_key="bt_tickers",
            max_height=200
        )

        if tickers and tickers != strategy.tickers:
            strategy.tickers = tickers
            strategy.updated_at = datetime.now()
            _save_strategy_to_session(strategy)
        
        st.markdown("")  # Spacing
        
        # Date range
        col1, col2 = st.columns(2)
        with col1:
            start_date = st.date_input(
                i18n.t("start_date"),
                value=date.today() - timedelta(days=90),
                max_value=date.today() - timedelta(days=1),
                key="backtest_start_date"
            )
        with col2:
            end_date = st.date_input(
                i18n.t("end_date"),
                value=date.today(),
                max_value=date.today(),
                key="backtest_end_date"
            )
        
        # Validate date range
        if start_date >= end_date:
            st.warning(i18n.t("start_date_before_end_date"))
        
        # Initial capital and timeframe
        col1, col2 = st.columns(2)
        with col1:
            initial_capital = st.number_input(
                i18n.t("initial_capital"),
                min_value=1000,
                max_value=10000000,
                value=10000,
                step=1000,
                key="backtest_capital"
            )
        with col2:
            timeframe = st.selectbox(
                i18n.t("timeframe"),
                options=["5m", "15m", "30m", "1h", "1d"],
                index=0,
                key="backtest_timeframe"
            )
        
        # Data source toggle (TWS vs Sample)
        use_tws_data = st.checkbox(
            i18n.t("use_real_tws_data"),
            value=is_connected,
            disabled=not is_connected,
            help=i18n.t("use_real_tws_data_help"),
            key="use_tws_data"
        )

        use_nautilus = st.checkbox(
            i18n.t("use_nautilus_backtest"),
            value=False,
            help=i18n.t("use_nautilus_backtest_help"),
            key="use_nautilus_backtest",
            disabled=not NAUTILUS_BACKTEST_AVAILABLE,
        )

        if not NAUTILUS_BACKTEST_AVAILABLE:
            st.info(i18n.t("nautilus_backtest_unavailable"))
        
        if not is_connected and use_tws_data:
            st.warning(i18n.t("tws_not_connected_backtest"))
        
        # Run backtest button
        can_run = bool(tickers) and bool(strategy.rules) and start_date < end_date
        
        if not strategy.rules:
            st.info(i18n.t("add_rule_before_backtest"), icon="💡")
        
        if st.button(
            i18n.t("run_backtest"), 
            key="run_backtest", 
            width="stretch",
            disabled=not can_run
        ):
            if can_run:
                _run_backtest(
                    strategy,
                    tickers,
                    start_date,
                    end_date,
                    initial_capital,
                    timeframe,
                    use_tws_data=use_tws_data and is_connected,
                    use_nautilus=use_nautilus,
                )
        
        # Show results if available
        if "backtest_result" in st.session_state:
            _render_backtest_results(st.session_state.backtest_result)


def _run_backtest(
    strategy: Strategy,
    tickers: List[str],
    start_date: date,
    end_date: date,
    initial_capital: float,
    timeframe: str,
    use_tws_data: bool = True,
    use_nautilus: bool = False,
) -> None:
    """
    Execute the backtest and store results.
    
    Args:
        strategy: The strategy to backtest
        tickers: List of ticker symbols to test
        start_date: Start date for backtest period
        end_date: End date for backtest period
        initial_capital: Starting capital amount
        timeframe: Data timeframe (e.g., "5m", "1h", "1d")
        use_tws_data: Whether to use real TWS data (vs sample data)
    """
    i18n = _get_i18n()
    data_source_msg = i18n.t("data_source_real_tws") if use_tws_data else i18n.t("data_source_sample")
    with st.spinner(i18n.t("running_backtest", source=data_source_msg)):
        try:
            # Create backtest engine with TWS data flag
            engine = BacktestEngine(
                strategy=strategy,
                initial_capital=initial_capital,
                use_tws_data=use_tws_data,
                use_nautilus=use_nautilus,
            )
            
            # Run backtest
            result = engine.run(
                tickers=tickers,
                start_date=start_date,
                end_date=end_date,
                timeframe=timeframe,
            )
            
            # Store result in session state
            st.session_state.backtest_result = result
            
            # Show result summary with data source info
            data_source_info = f" {i18n.t('data_source_label', source=result.data_source)}" if hasattr(result, 'data_source') else ""
            st.success(
                i18n.t(
                    "backtest_complete",
                    data_source=data_source_info,
                    return_pct=f"{result.metrics.total_return_percent:+.2f}%",
                    trades=str(result.metrics.total_trades),
                )
            )
            
        except Exception as e:
            st.error(i18n.t("backtest_failed", error=str(e)))
            import traceback
            st.code(traceback.format_exc())


def _render_backtest_results(result: BacktestResult) -> None:
    """Render backtest results."""
    i18n = _get_i18n()
    st.markdown("---")
    st.markdown(f"##### {i18n.t('backtest_results')}")
    
    # Show data source badge
    if hasattr(result, 'data_source') and result.data_source:
        source_color = COLORS["accent_green"] if "TWS" in result.data_source else COLORS["accent_yellow"]
        st.markdown(
            f'<div style="display: inline-block; background-color: {source_color}20; '
            f'color: {source_color}; padding: 2px 8px; border-radius: 4px; '
            f'font-size: 0.75em; margin-bottom: 8px;">'
            f'{i18n.t("data_source_badge", source=result.data_source)}</div>',
            unsafe_allow_html=True
        )
    
    # Summary metrics in expandable section (collapsed by default for space)
    with st.expander(i18n.t("performance_metrics"), expanded=True):
        render_metrics_cards(result.metrics, result.initial_capital)
    
    # Equity curve
    with st.expander(i18n.t("equity_curve"), expanded=True):
        render_equity_curve(result)

    # QuantStats tear sheet
    with st.expander(i18n.t("quantstats_report"), expanded=False):
        render_quantstats_report(result)
    
    # Trade analysis
    with st.expander(i18n.t("trade_history", count=result.metrics.total_trades), expanded=False):
        # Trade table
        render_trade_table(result.trades)
        
        # Additional charts in columns
        if result.trades:
            col1, col2 = st.columns(2)
            with col1:
                render_trade_distribution(result.trades)
            with col2:
                render_cumulative_pnl(result.trades)
    
    # Clear results button
    if st.button(i18n.t("clear_results"), key="clear_backtest_results"):
        del st.session_state.backtest_result
        st.rerun()


def _render_action_buttons(strategy: Strategy) -> None:
    """Render the main action buttons."""
    i18n = _get_i18n()
    st.markdown(f"#### {i18n.t('actions')}")
    
    col1, col2, col3, col4, col5 = st.columns(5)
    
    with col1:
        # Import - file uploader
        uploaded_file = st.file_uploader(
            i18n.t("import_strategy"),
            type=["json"],
            key="import_strategy_file",
            label_visibility="collapsed",
        )
        if uploaded_file is not None:
            _import_strategy(uploaded_file)
    
    with col2:
        # Export as download
        if st.button(i18n.t("export_json"), key="export_strategy", width="stretch"):
            pass  # Download button rendered below
        
        # Prepare download
        json_str = json.dumps(
            strategy.model_dump(mode="json"),
            indent=2,
            default=str
        )
        st.download_button(
            label=i18n.t("download_strategy"),
            data=json_str,
            file_name=f"strategy_{strategy.id[:8]}_{datetime.now().strftime('%Y%m%d_%H%M%S')}.json",
            mime="application/json",
            key="download_strategy",
            width="stretch",
        )
    
    with col3:
        if st.button(i18n.t("save_draft"), key="save_draft", width="stretch"):
            strategy.updated_at = datetime.now()
            if _save_strategy_to_file(strategy, ACTIVE_STRATEGY_PATH):
                st.toast(i18n.t("strategy_saved_success"), icon="✅")
            else:
                st.toast(i18n.t("strategy_saved_failed"), icon="❌")
    
    with col4:
        if st.button(i18n.t("deploy_to_bot"), key="apply_to_bot", width="stretch"):
            _deploy_strategy(strategy)
    
    with col5:
        # Apply to Trading Bot (hot-reload)
        if st.button(i18n.t("apply_strategy"), key="apply_hot_reload", width="stretch"):
            _apply_strategy_hot_reload(strategy)


def _import_strategy(uploaded_file) -> None:
    """Import a strategy from uploaded JSON file."""
    try:
        data = json.load(uploaded_file)
        
        # Validate as Strategy model
        imported_strategy = Strategy.model_validate(data)
        
        # Generate new ID to avoid conflicts
        imported_strategy.id = str(uuid.uuid4())
        imported_strategy.updated_at = datetime.now()
        
        # Save to session
        _save_strategy_to_session(imported_strategy)
        
        i18n = _get_i18n()
        st.success(i18n.t("strategy_imported_success", name=imported_strategy.name))
        st.rerun()
        
    except json.JSONDecodeError:
        st.error(_get_i18n().t("invalid_json"))
    except Exception as e:
        st.error(_get_i18n().t("strategy_import_failed", error=str(e)))


def _deploy_strategy(strategy: Strategy) -> None:
    """Deploy the current strategy to the trading bot."""
    i18n = _get_i18n()
    if not strategy.rules:
        st.error(i18n.t("cannot_deploy_empty"))
        return
    
    # Validate strategy before deployment
    validation_errors = validate_strategy(strategy)
    if validation_errors:
        st.error(i18n.t("strategy_validation_failed"))
        for error in validation_errors:
            st.markdown(
                f'<div style="color: {COLORS["accent_red"]}; padding: 4px 8px; '
                f'background-color: rgba(255, 82, 82, 0.1); border-radius: 4px; '
                f'margin: 2px 0;">'
                f'• {error}'
                f'</div>',
                unsafe_allow_html=True
            )
        return
    
    # Show confirmation dialog
    st.warning(i18n.t("deploy_warning"))
    
    # Display rules summary
    st.markdown(i18n.t("rules_to_deploy"))
    for rule in strategy.rules:
        st.markdown(f"- {rule.name}: {rule_to_human_readable(rule)}")
    
    col1, col2 = st.columns(2)
    with col1:
        if st.button(i18n.t("confirm_deploy"), key="confirm_deploy", width="stretch"):
            # Save to active_strategy.json
            strategy.updated_at = datetime.now()
            if _save_strategy_to_file(strategy, ACTIVE_STRATEGY_PATH):
                st.session_state.strategy_deployed = True
                st.success(i18n.t("strategy_deployed_success"))
                st.toast(i18n.t("strategy_now_active"), icon="🚀")
                st.balloons()
            else:
                st.error(i18n.t("strategy_deploy_failed"))
    
    with col2:
        if st.button(i18n.t("cancel"), key="cancel_deploy", width="stretch"):
            st.rerun()


def _apply_strategy_hot_reload(strategy: Strategy) -> None:
    """
    Apply strategy to running trading bot with hot-reload.
    
    This validates the strategy, saves it to disk, and writes
    a reload signal file that the bot watches for to trigger
    a strategy reload without restart.
    """
    i18n = _get_i18n()
    if not strategy.rules:
        st.error(i18n.t("cannot_apply_empty"))
        return
    
    # Validate strategy before applying
    validation_errors = validate_strategy(strategy)
    if validation_errors:
        st.error(i18n.t("strategy_validation_failed_fix"))
        for error in validation_errors:
            st.markdown(
                f'<div style="color: {COLORS["accent_red"]}; padding: 4px 8px; '
                f'background-color: rgba(255, 82, 82, 0.1); border-radius: 4px; '
                f'margin: 2px 0; font-family: monospace;">'
                f'{error}'
                f'</div>',
                unsafe_allow_html=True
            )
        return
    
    # Strategy passed validation - show success with green checkmark
    st.success(i18n.t("strategy_validation_passed"))
    
    # Save strategy to active_strategy.json
    strategy.updated_at = datetime.now()
    if not _save_strategy_to_file(strategy, ACTIVE_STRATEGY_PATH):
        st.error(i18n.t("strategy_save_to_disk_failed"))
        return
    
    # Write reload signal file to trigger hot-reload
    try:
        RELOAD_SIGNAL_FILE.parent.mkdir(parents=True, exist_ok=True)
        with open(RELOAD_SIGNAL_FILE, "w") as f:
            f.write(datetime.now().isoformat())
        
        st.session_state.strategy_deployed = True
        st.success(i18n.t("strategy_applied_success"))
        
        # Check if bot is running
        from src.bot.state import BotState
        try:
            bot_state = BotState.load()
            if bot_state.status in ["running", "simulating"]:
                st.toast(
                    i18n.t("strategy_reload_by_bot"),
                    icon="♻️"
                )
                st.info(
                    i18n.t("bot_running_reload_notice")
                )
            else:
                st.warning(
                    i18n.t("bot_not_running_notice")
                )
        except Exception:
            # Bot state not available
            st.info(
                i18n.t("strategy_saved_reload_if_running")
            )
        
    except Exception as e:
        st.error(i18n.t("write_reload_failed", error=str(e)))
