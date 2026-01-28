"""
Main Streamlit application entry point.

This is the main UI for the Trading Bot system, providing:
- Live Trading monitoring tab
- Strategy Builder (RBTSUI) tab
"""

import streamlit as st
from pathlib import Path
import sys

# Page configuration - must be first Streamlit command
st.set_page_config(
    page_title="Trading Bot - Cobalt",
    page_icon="📈",
    layout="wide",
    initial_sidebar_state="expanded",
    menu_items={
        'About': "# Cobalt Trading Bot\nRule-based automated trading with Nautilus Trader."
    }
)

# Add project root to path for imports
project_root = Path(__file__).parent.parent.parent
sys.path.insert(0, str(project_root))

from src.ui.styles import apply_theme, COLORS, status_badge, format_currency
from src.ui.tabs.monitoring import render_monitoring_tab, get_bot_state
from src.ui.tabs.strategy import render_strategy_tab
from src.ui.components.watchlist import render_watchlist_manager
from src.bot.tws_data_provider import get_tws_provider, reset_tws_provider
from src.ui.translations import translations
from src.ui.i18n import I18n


def main():
    """Main application entry point."""
    # Browser language detection
    st.components.v1.html("""
    <script>
    if (!window.location.search.includes('lang=')) {
        const lang = navigator.language || navigator.userLanguage;
        const lang_code = lang.split('-')[0];
        if (['en', 'fr'].includes(lang_code)) {
            window.location.search = '?lang=' + lang_code;
        }
    }
    </script>
    """, height=0)
    
    if 'lang' in st.query_params:
        detected = st.query_params['lang']
        if detected in ['en', 'fr']:
            st.session_state['lang'] = detected
        st.query_params.clear()
        st.rerun()
    
    i18n = I18n(translations)
    st.session_state['i18n'] = i18n
    
    # Apply dark theme
    apply_theme()
    
    # Render sidebar
    _render_sidebar(i18n)
    
    # Main content area with tabs
    tab_live, tab_watchlist, tab_strategy = st.tabs([
        i18n.t("live_trading"),
        i18n.t("watchlist"),
        i18n.t("strategy_builder")
    ])
    
    with tab_live:
        render_monitoring_tab()
    
    with tab_watchlist:
        render_watchlist_manager()
    
    with tab_strategy:
        render_strategy_tab()
    
def _render_sidebar(i18n):
    """Render the sidebar content."""
    with st.sidebar:
        # Logo/Title area
        st.markdown(
            f"""
            <div style="text-align: center; padding: 1rem 0;">
                <h1 style="color: {COLORS['accent_blue']}; margin: 0;">{i18n.t("cobalt_title")}</h1>
                <p style="color: {COLORS['text_secondary']}; margin: 0.5rem 0 0 0; font-size: 0.9em;">
                    {i18n.t("rule_based_trading_bot")}
                </p>
            </div>
            """,
            unsafe_allow_html=True
        )
        
        st.divider()
        
        # Connection Status - check actual TWS connection
        st.markdown(f"#### {i18n.t('connection')}")
        
        # Check actual TWS connection status
        try:
            provider = get_tws_provider()
            tws_connected = provider.is_connected()
        except Exception:
            tws_connected = False
        
        # Update session state
        st.session_state["tws_connected"] = tws_connected
        
        st.markdown(
            f"**{i18n.t('tws')}:** {status_badge(i18n.t('connected') if tws_connected else i18n.t('disconnected'), tws_connected)}",
            unsafe_allow_html=True
        )
        
        # Bot status
        bot_running = st.session_state.get("bot_running", False)
        st.markdown(
            f"**{i18n.t('bot')}:** {status_badge(i18n.t('running') if bot_running else i18n.t('stopped'), bot_running)}",
            unsafe_allow_html=True
        )
        
        st.divider()
        
        # Active Strategy
        st.markdown("#### 📋 Active Strategy")
        strategy_name = st.session_state.get("active_strategy_name", "None")
        st.markdown(f"**Name:** {strategy_name}")
        
        rule_count = len(st.session_state.get("strategy_rules", []))
        st.markdown(f"**Rules:** {rule_count}")
        
        is_deployed = st.session_state.get("strategy_deployed", False)
        status_text = "Deployed" if is_deployed else "Draft"
        st.markdown(
            f"**Status:** {status_badge(status_text, is_deployed)}",
            unsafe_allow_html=True
        )
        
        st.divider()
        
        # Quick Stats - get from bot state
        st.markdown("#### 📊 Quick Stats")
        bot_state = get_bot_state()
        col1, col2 = st.columns(2)
        with col1:
            st.metric("Equity", format_currency(bot_state.equity))
        with col2:
            st.metric("Daily P&L", format_currency(bot_state.daily_pnl))
        
        st.divider()
        
        # Settings link
        st.markdown("#### ⚙️ Settings")
        
        with st.expander("Connection Settings", expanded=not tws_connected):
            host = st.text_input("TWS Host", value="127.0.0.1", key="tws_host")
            port = st.number_input("TWS Port", value=7497, min_value=1, max_value=65535, key="tws_port")
            client_id = st.number_input("Client ID", value=10, min_value=1, max_value=999, key="tws_client_id")
            
            col1, col2 = st.columns(2)
            with col1:
                if st.button("🔌 Connect", key="connect_tws", use_container_width=True, disabled=tws_connected):
                    with st.spinner("Connecting to TWS..."):
                        try:
                            # Reset provider to use new settings
                            reset_tws_provider()
                            provider = get_tws_provider()
                            provider.host = host
                            provider.port = int(port)
                            provider.client_id = int(client_id)
                            
                            if provider.connect(timeout=10.0):
                                st.session_state["tws_connected"] = True
                                st.success("✅ Connected to TWS!")
                                st.rerun()
                            else:
                                st.error("❌ Failed to connect. Check TWS is running and API is enabled.")
                        except Exception as e:
                            st.error(f"❌ Connection error: {str(e)}")
            
            with col2:
                if st.button("🔌 Disconnect", key="disconnect_tws", use_container_width=True, disabled=not tws_connected):
                    try:
                        provider = get_tws_provider()
                        provider.disconnect()
                        reset_tws_provider()
                        st.session_state["tws_connected"] = False
                        st.success("Disconnected from TWS")
                        st.rerun()
                    except Exception as e:
                        st.error(f"Error disconnecting: {e}")

        st.divider()

        # Language selector at bottom
        i18n.language_selector()
        
        # Footer
        st.markdown(
            f"""
            <div style="position: fixed; bottom: 1rem; color: {COLORS['text_secondary']}; font-size: 0.8em;">
                Cobalt v0.1.0 • Nautilus Trader
            </div>
            """,
            unsafe_allow_html=True
        )


if __name__ == "__main__":
    main()
