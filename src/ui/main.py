"""
Main Streamlit application entry point.

This is the main UI for the Trading Bot system, providing:
- Live Trading monitoring tab
- Strategy Builder (RBTSUI) tab
"""

import streamlit as st
from pathlib import Path
import sys
import os
import hmac

# Add project root to path for imports
project_root = Path(__file__).parent.parent.parent
sys.path.insert(0, str(project_root))

# Page configuration - must be first Streamlit command
from src.ui.translations import translations

_env_lang = os.environ.get("LANG", "").lower()
_ui_lang = "fr" if _env_lang.startswith("fr") else "en"

st.set_page_config(
    page_title=translations[_ui_lang].get("page_title", "Trading Bot - Cobalt"),
    page_icon="📈",
    layout="wide",
    initial_sidebar_state="expanded",
    menu_items={
        "About": translations[_ui_lang].get(
            "about_text",
            "# Cobalt Trading Bot\nRule-based automated trading with Nautilus Trader."
        )
    }
)

from src.ui.styles import apply_theme, COLORS, status_badge, format_currency
from src.ui.tabs.monitoring import render_monitoring_tab, get_bot_state
from src.ui.tabs.strategy import render_strategy_tab
from src.ui.components.watchlist import render_watchlist_manager
from src.bot.tws_data_provider import get_tws_provider, reset_tws_provider
from src.ui.i18n import I18n
from src.config.settings import load_config


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

    settings = load_config(sync_to_db=False)

    if settings.auth.enabled and not st.session_state.get("authenticated", False):
        apply_theme()
        st.markdown(f"## {i18n.t('auth_login_title')}")
        username = st.text_input(i18n.t("auth_username"), key="auth_username")
        password = st.text_input(i18n.t("auth_password"), type="password", key="auth_password")
        if st.button(i18n.t("auth_login")):
            username_ok = hmac.compare_digest(username, settings.auth.username)
            password_ok = hmac.compare_digest(password, settings.auth.password)
            if username_ok and password_ok:
                st.session_state["authenticated"] = True
                st.rerun()
            else:
                st.error(i18n.t("auth_invalid"))
        st.stop()
    
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
        st.markdown(f"#### {i18n.t('active_strategy_header')}")
        strategy_name = st.session_state.get("active_strategy_name", i18n.t("none"))
        st.markdown(f"**{i18n.t('name')}:** {strategy_name}")
        
        rule_count = len(st.session_state.get("strategy_rules", []))
        st.markdown(f"**{i18n.t('rules')}:** {rule_count}")
        
        is_deployed = st.session_state.get("strategy_deployed", False)
        status_text = i18n.t("deployed") if is_deployed else i18n.t("draft")
        st.markdown(
            f"**Status:** {status_badge(status_text, is_deployed)}",
            unsafe_allow_html=True
        )
        
        st.divider()
        
        # Quick Stats - get from bot state
        st.markdown(f"#### {i18n.t('quick_stats')}")
        bot_state = get_bot_state()
        col1, col2 = st.columns(2)
        with col1:
            st.metric(i18n.t("equity"), format_currency(bot_state.equity))
        with col2:
            st.metric(i18n.t("daily_pnl"), format_currency(bot_state.daily_pnl))
        
        st.divider()
        
        # Settings link
        st.markdown(f"#### {i18n.t('settings')}")
        
        with st.expander(i18n.t("connection_settings"), expanded=not tws_connected):
            host = st.text_input(i18n.t("tws_host"), value="127.0.0.1", key="tws_host")
            port = st.number_input(i18n.t("tws_port"), value=7497, min_value=1, max_value=65535, key="tws_port")
            client_id = st.number_input(i18n.t("client_id"), value=10, min_value=1, max_value=999, key="tws_client_id")
            
            col1, col2 = st.columns(2)
            with col1:
                if st.button(i18n.t("connect"), key="connect_tws", use_container_width=True, disabled=tws_connected):
                    with st.spinner(i18n.t("connecting_tws")):
                        try:
                            # Reset provider to use new settings
                            reset_tws_provider()
                            provider = get_tws_provider()
                            provider.host = host
                            provider.port = int(port)
                            provider.client_id = int(client_id)
                            
                            if provider.connect(timeout=10.0):
                                st.session_state["tws_connected"] = True
                                st.success(i18n.t("connected_to_tws"))
                                st.rerun()
                            else:
                                st.error(i18n.t("failed_to_connect_tws"))
                        except Exception as e:
                            st.error(i18n.t("connection_error", error=str(e)))
            
            with col2:
                if st.button(i18n.t("disconnect"), key="disconnect_tws", use_container_width=True, disabled=not tws_connected):
                    try:
                        provider = get_tws_provider()
                        provider.disconnect()
                        reset_tws_provider()
                        st.session_state["tws_connected"] = False
                        st.success(i18n.t("disconnected_from_tws"))
                        st.rerun()
                    except Exception as e:
                        st.error(i18n.t("disconnect_error", error=str(e)))

        st.divider()

        # Language selector at bottom
        i18n.language_selector()
        
        # Footer
        st.markdown(
            f"""
            <div style="position: fixed; bottom: 1rem; color: {COLORS['text_secondary']}; font-size: 0.8em;">
                {i18n.t("footer_text")}
            </div>
            """,
            unsafe_allow_html=True
        )


if __name__ == "__main__":
    main()
