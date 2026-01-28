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
from src.bot.backtest_runner import BacktestEngine, BacktestResult
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
)


# Project root (from src/ui/tabs -> src/ui -> src -> project root)
_PROJECT_ROOT = Path(__file__).parent.parent.parent.parent

# Watchlist file path for custom ticker lists
WATCHLIST_PATH = _PROJECT_ROOT / "config" / "watchlist.txt"

# Default strategy file path
ACTIVE_STRATEGY_PATH = _PROJECT_ROOT / "config" / "active_strategy.json"

# Reload signal file path (tells the bot to reload strategy)
RELOAD_SIGNAL_FILE = Path(__file__).parent.parent.parent / "config" / ".reload_signal"


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
        st.error(f"Error loading strategy: {e}")
    return None


def _save_strategy_to_file(strategy: Strategy, filepath: Path) -> bool:
    """Save strategy to JSON file."""
    try:
        filepath.parent.mkdir(parents=True, exist_ok=True)
        with open(filepath, "w") as f:
            json.dump(strategy.model_dump(mode="json"), f, indent=2, default=str)
        return True
    except Exception as e:
        st.error(f"Error saving strategy: {e}")
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
    strategy = Strategy(
        id=str(uuid.uuid4()),
        name="New Strategy",
        version="1.0.0",
        description="Created with Strategy Builder",
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
        st.warning(f"Could not load watchlist file: {e}")
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
            return True, "🟢 TWS Connected"
        else:
            return False, "🔴 TWS Not Connected"
    except Exception as e:
        return False, f"🔴 TWS Error: {str(e)[:30]}"


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
                return combined, "TWS + Watchlist"
    except Exception:
        pass
    
    # Return watchlist symbols if available
    if watchlist_symbols:
        return watchlist_symbols, "Local Watchlist"
    
    # Default fallback symbols
    default_symbols = ["SPY", "QQQ", "AAPL", "MSFT", "GOOGL", "AMZN", "NVDA", "TSLA"]
    return default_symbols, "Default"


def render_strategy_tab() -> None:
    """Render the Strategy Builder tab content."""
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
    col1, col2 = st.columns([3, 1])
    
    with col1:
        # Editable strategy name
        new_name = st.text_input(
            "Strategy Name",
            value=strategy.name,
            key="strategy_name_input",
            label_visibility="collapsed",
            placeholder="Enter strategy name...",
        )
        
        # Update if changed
        if new_name != strategy.name:
            strategy.name = new_name
            strategy.updated_at = datetime.now()
            _save_strategy_to_session(strategy)
        
        # Description
        st.markdown(
            f'<span style="color: {COLORS["text_secondary"]};">'
            f'Configure your trading rules and backtest before deploying • '
            f'{len(strategy.rules)} rule(s) defined'
            f'</span>',
            unsafe_allow_html=True
        )
    
    with col2:
        # Strategy status
        is_deployed = st.session_state.get("strategy_deployed", False)
        status_color = COLORS["accent_green"] if is_deployed else COLORS["accent_yellow"]
        status_text = "🟢 Deployed" if is_deployed else "🟡 Draft"
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
            f'🌍 {global_rules} global • 🎯 {ticker_rules} per-ticker'
            f'</div>',
            unsafe_allow_html=True
        )


def _render_rules_section(strategy: Strategy) -> None:
    """Render the rules list and rule builder."""
    st.markdown("#### Trading Rules")
    
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
    with st.expander("➕ Add New Rule", expanded=False):
        # Use the new rule builder component
        new_rule = render_rule_builder(key_prefix="new_rule")
        
        if new_rule:
            # Add to strategy
            strategy.rules.append(new_rule)
            strategy.updated_at = datetime.now()
            _save_strategy_to_session(strategy)
            st.success(f"Rule '{new_rule.name}' added successfully!")
            st.rerun()


def _render_backtest_section(strategy: Strategy) -> None:
    """Render the backtest configuration section with TWS integration."""
    st.markdown("#### Backtest Configuration")
    
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
             st.markdown(f"**Select Tickers** ({ticker_source})")
        with col_btn:
             if st.button("📥 Load Watchlist", help="Replace current selection with watchlist"):
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
                "Start Date",
                value=date.today() - timedelta(days=90),
                max_value=date.today() - timedelta(days=1),
                key="backtest_start_date"
            )
        with col2:
            end_date = st.date_input(
                "End Date",
                value=date.today(),
                max_value=date.today(),
                key="backtest_end_date"
            )
        
        # Validate date range
        if start_date >= end_date:
            st.warning("Start date must be before end date")
        
        # Initial capital and timeframe
        col1, col2 = st.columns(2)
        with col1:
            initial_capital = st.number_input(
                "Initial Capital ($)",
                min_value=1000,
                max_value=10000000,
                value=10000,
                step=1000,
                key="backtest_capital"
            )
        with col2:
            timeframe = st.selectbox(
                "Timeframe",
                options=["5m", "15m", "30m", "1h", "1d"],
                index=0,
                key="backtest_timeframe"
            )
        
        # Data source toggle (TWS vs Sample)
        use_tws_data = st.checkbox(
            "Use Real TWS Data",
            value=is_connected,
            disabled=not is_connected,
            help="When enabled and TWS is connected, uses real historical market data. Otherwise uses sample/synthetic data.",
            key="use_tws_data"
        )
        
        if not is_connected and use_tws_data:
            st.warning("TWS is not connected. Will use sample data for backtesting.")
        
        # Run backtest button
        can_run = bool(tickers) and bool(strategy.rules) and start_date < end_date
        
        if not strategy.rules:
            st.info("Add at least one trading rule before running backtest.", icon="💡")
        
        if st.button(
            "🚀 Run Backtest", 
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
                    use_tws_data=use_tws_data and is_connected
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
    use_tws_data: bool = True
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
    data_source_msg = "real TWS data" if use_tws_data else "sample data"
    with st.spinner(f"Running backtest with {data_source_msg}... This may take a moment."):
        try:
            # Create backtest engine with TWS data flag
            engine = BacktestEngine(
                strategy=strategy,
                initial_capital=initial_capital,
                use_tws_data=use_tws_data,
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
            data_source_info = f" (Data: {result.data_source})" if hasattr(result, 'data_source') else ""
            st.success(
                f"Backtest complete!{data_source_info} "
                f"Return: {result.metrics.total_return_percent:+.2f}% | "
                f"Trades: {result.metrics.total_trades}"
            )
            
        except Exception as e:
            st.error(f"Backtest failed: {e}")
            import traceback
            st.code(traceback.format_exc())


def _render_backtest_results(result: BacktestResult) -> None:
    """Render backtest results."""
    st.markdown("---")
    st.markdown("##### 📊 Backtest Results")
    
    # Show data source badge
    if hasattr(result, 'data_source') and result.data_source:
        source_color = COLORS["accent_green"] if "TWS" in result.data_source else COLORS["accent_yellow"]
        st.markdown(
            f'<div style="display: inline-block; background-color: {source_color}20; '
            f'color: {source_color}; padding: 2px 8px; border-radius: 4px; '
            f'font-size: 0.75em; margin-bottom: 8px;">'
            f'📊 Data: {result.data_source}</div>',
            unsafe_allow_html=True
        )
    
    # Summary metrics in expandable section (collapsed by default for space)
    with st.expander("📈 Performance Metrics", expanded=True):
        render_metrics_cards(result.metrics, result.initial_capital)
    
    # Equity curve
    with st.expander("📉 Equity Curve", expanded=True):
        render_equity_curve(result)
    
    # Trade analysis
    with st.expander(f"📋 Trade History ({result.metrics.total_trades} trades)", expanded=False):
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
    if st.button("🗑️ Clear Results", key="clear_backtest_results"):
        del st.session_state.backtest_result
        st.rerun()


def _render_action_buttons(strategy: Strategy) -> None:
    """Render the main action buttons."""
    st.markdown("#### Actions")
    
    col1, col2, col3, col4, col5 = st.columns(5)
    
    with col1:
        # Import - file uploader
        uploaded_file = st.file_uploader(
            "Import Strategy",
            type=["json"],
            key="import_strategy_file",
            label_visibility="collapsed",
        )
        if uploaded_file is not None:
            _import_strategy(uploaded_file)
    
    with col2:
        # Export as download
        if st.button("📤 Export JSON", key="export_strategy", width="stretch"):
            pass  # Download button rendered below
        
        # Prepare download
        json_str = json.dumps(
            strategy.model_dump(mode="json"),
            indent=2,
            default=str
        )
        st.download_button(
            label="📥 Download Strategy",
            data=json_str,
            file_name=f"strategy_{strategy.id[:8]}_{datetime.now().strftime('%Y%m%d_%H%M%S')}.json",
            mime="application/json",
            key="download_strategy",
            width="stretch",
        )
    
    with col3:
        if st.button("💾 Save Draft", key="save_draft", width="stretch"):
            strategy.updated_at = datetime.now()
            if _save_strategy_to_file(strategy, ACTIVE_STRATEGY_PATH):
                st.toast("Strategy saved successfully!", icon="✅")
            else:
                st.toast("Failed to save strategy", icon="❌")
    
    with col4:
        if st.button("🚀 Deploy to Bot", key="apply_to_bot", width="stretch"):
            _deploy_strategy(strategy)
    
    with col5:
        # Apply to Trading Bot (hot-reload)
        if st.button("⚡ Apply Strategy", key="apply_hot_reload", width="stretch"):
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
        
        st.success(f"Strategy '{imported_strategy.name}' imported successfully!")
        st.rerun()
        
    except json.JSONDecodeError:
        st.error("Invalid JSON file. Please upload a valid strategy JSON.")
    except Exception as e:
        st.error(f"Error importing strategy: {e}")


def _deploy_strategy(strategy: Strategy) -> None:
    """Deploy the current strategy to the trading bot."""
    if not strategy.rules:
        st.error("Cannot deploy an empty strategy. Please add at least one rule.")
        return
    
    # Validate strategy before deployment
    validation_errors = validate_strategy(strategy)
    if validation_errors:
        st.error("❌ Strategy validation failed:")
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
    st.warning(
        "⚠️ This will deploy the strategy to the LIVE trading bot. "
        "Make sure you have tested it thoroughly!"
    )
    
    # Display rules summary
    st.markdown("**Rules to deploy:**")
    for rule in strategy.rules:
        st.markdown(f"- {rule.name}: {rule_to_human_readable(rule)}")
    
    col1, col2 = st.columns(2)
    with col1:
        if st.button("✅ Confirm Deployment", key="confirm_deploy", width="stretch"):
            # Save to active_strategy.json
            strategy.updated_at = datetime.now()
            if _save_strategy_to_file(strategy, ACTIVE_STRATEGY_PATH):
                st.session_state.strategy_deployed = True
                st.success("Strategy deployed successfully!")
                st.toast("Strategy is now active on the trading bot", icon="🚀")
                st.balloons()
            else:
                st.error("Failed to deploy strategy")
    
    with col2:
        if st.button("❌ Cancel", key="cancel_deploy", width="stretch"):
            st.rerun()


def _apply_strategy_hot_reload(strategy: Strategy) -> None:
    """
    Apply strategy to running trading bot with hot-reload.
    
    This validates the strategy, saves it to disk, and writes
    a reload signal file that the bot watches for to trigger
    a strategy reload without restart.
    """
    if not strategy.rules:
        st.error("Cannot apply an empty strategy. Please add at least one rule.")
        return
    
    # Validate strategy before applying
    validation_errors = validate_strategy(strategy)
    if validation_errors:
        st.error("❌ Strategy validation failed. Please fix the following errors:")
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
    st.success("✅ Strategy validation passed")
    
    # Save strategy to active_strategy.json
    strategy.updated_at = datetime.now()
    if not _save_strategy_to_file(strategy, ACTIVE_STRATEGY_PATH):
        st.error("Failed to save strategy to disk")
        return
    
    # Write reload signal file to trigger hot-reload
    try:
        RELOAD_SIGNAL_FILE.parent.mkdir(parents=True, exist_ok=True)
        with open(RELOAD_SIGNAL_FILE, "w") as f:
            f.write(datetime.now().isoformat())
        
        st.session_state.strategy_deployed = True
        st.success("⚡ Strategy applied successfully!")
        
        # Check if bot is running
        from src.bot.state import BotState
        try:
            bot_state = BotState.load()
            if bot_state.status in ["running", "simulating"]:
                st.toast(
                    "Strategy will be reloaded by the running bot",
                    icon="♻️"
                )
                st.info(
                    "ℹ️ The trading bot is running and will automatically "
                    "reload the strategy within a few seconds."
                )
            else:
                st.warning(
                    "⚠️ The trading bot is not currently running. "
                    "Start the bot to begin trading with this strategy."
                )
        except Exception:
            # Bot state not available
            st.info(
                "ℹ️ Strategy saved. If the bot is running, it will "
                "automatically reload the strategy."
            )
        
    except Exception as e:
        st.error(f"Failed to write reload signal: {e}")
