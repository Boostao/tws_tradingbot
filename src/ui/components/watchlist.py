"""
Watchlist Management Component - Fast JS-based implementation.

All add/remove operations happen instantly via DOM manipulation.
Changes are persisted to file in the background.
"""

import streamlit as st
import streamlit.components.v1 as components
import json
from pathlib import Path
from typing import List

from src.bot.tws_data_provider import get_tws_provider
from src.ui.i18n import I18n
from src.ui.translations import translations


# Project root for file paths
_PROJECT_ROOT = Path(__file__).parent.parent.parent.parent
WATCHLIST_PATH = _PROJECT_ROOT / "config" / "watchlist.txt"


def _load_watchlist() -> List[str]:
    """Load watchlist symbols from file."""
    symbols = []
    if WATCHLIST_PATH.exists():
        try:
            with open(WATCHLIST_PATH, "r") as f:
                for line in f:
                    line = line.strip()
                    if line and not line.startswith("#"):
                        symbols.append(line.upper())
        except Exception:
            pass
    return symbols


def _save_watchlist(symbols: List[str]) -> bool:
    """Save watchlist symbols to file."""
    try:
        WATCHLIST_PATH.parent.mkdir(parents=True, exist_ok=True)
        with open(WATCHLIST_PATH, "w") as f:
            f.write("# TWS Traderbot Watchlist\n")
            for symbol in symbols:
                if symbol.strip():
                    f.write(f"{symbol.upper()}\n")
        return True
    except Exception:
        return False


def _get_i18n() -> I18n:
    if "i18n" not in st.session_state:
        st.session_state["i18n"] = I18n(translations)
    return st.session_state["i18n"]


def render_watchlist_manager() -> None:
    """Render fast JS-based watchlist manager."""
    i18n = _get_i18n()
    
    st.markdown(f"### {i18n.t('watchlist')}")
    
    # Load watchlist from file (source of truth)
    watchlist = _load_watchlist()
    
    # Check for sync request from JS
    params = st.query_params
    if "wl" in params:
        try:
            import urllib.parse
            new_list = json.loads(urllib.parse.unquote(params["wl"]))
            if isinstance(new_list, list):
                _save_watchlist(new_list)
                watchlist = new_list
        except:
            pass
        st.query_params.clear()
        st.rerun()
    
    # TWS status
    try:
        provider = get_tws_provider()
        is_connected = provider.is_connected()
    except:
        is_connected = False
    
    # Load symbol database
    from src.ui.components.symbol_search import load_symbol_database
    symbols_db = load_symbol_database()
    
    # Render unified JS component
    _render_js_component(symbols_db, watchlist, is_connected, i18n)
    
    # Minimal Streamlit fallback controls
    with st.expander(i18n.t("manual_controls")):
        col1, col2 = st.columns(2)
        with col1:
            if st.button(i18n.t("reload_from_file")):
                st.rerun()
        with col2:
            if st.button(i18n.t("clear_all")):
                _save_watchlist([])
                st.rerun()
        
        # Manual add
        manual = st.text_input(i18n.t("add_symbol_manually"), placeholder=i18n.t("example_symbol"))
        if manual:
            manual = manual.upper().strip()
            if manual and manual not in watchlist:
                watchlist.append(manual)
                _save_watchlist(watchlist)
                st.rerun()


def _render_js_component(symbols_db: List[dict], watchlist: List[str], tws_connected: bool, i18n: I18n) -> None:
    """Render the all-in-one JS component."""
    
    symbols_json = json.dumps(symbols_db)
    watchlist_json = json.dumps(watchlist)
    watchlist_path = str(WATCHLIST_PATH)
    
    html = f"""
<!DOCTYPE html>
<html>
<head>
<style>
* {{ box-sizing: border-box; margin: 0; padding: 0; }}
body {{ font-family: system-ui, -apple-system, sans-serif; background: transparent; color: #e0e0e0; }}

.container {{ padding: 4px 0; }}

/* Search */
.search-box {{ position: relative; margin-bottom: 12px; }}
.search-input {{
    width: 100%; padding: 8px 12px 8px 32px; font-size: 13px;
    border: 1px solid #404040; border-radius: 6px;
    background: #1a1a1a; color: #e0e0e0; outline: none;
}}
.search-input:focus {{ border-color: #4a9eff; }}
.search-input::placeholder {{ color: #666; }}
.search-icon {{ position: absolute; left: 10px; top: 50%; transform: translateY(-50%); color: #666; font-size: 12px; }}

.dropdown {{
    position: absolute; top: 100%; left: 0; right: 0;
    max-height: 200px; overflow-y: auto;
    background: #1a1a1a; border: 1px solid #404040; border-radius: 6px;
    box-shadow: 0 4px 12px rgba(0,0,0,0.5); z-index: 100; display: none;
}}
.dropdown.show {{ display: block; }}
.dd-item {{
    padding: 6px 10px; cursor: pointer; display: flex; align-items: center;
    border-bottom: 1px solid #2a2a2a; font-size: 12px;
}}
.dd-item:hover, .dd-item.sel {{ background: #2a2a2a; }}
.dd-sym {{ font-weight: 600; min-width: 50px; color: #e0e0e0; }}
.dd-name {{ color: #888; margin-left: 8px; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }}
.dd-exch {{ color: #666; font-size: 10px; background: #2a2a2a; padding: 1px 4px; border-radius: 3px; }}
.hl {{ color: #4a9eff; }}
.no-res {{ padding: 10px; color: #666; text-align: center; font-size: 12px; }}

/* Chips */
.chips {{ display: flex; flex-wrap: wrap; gap: 6px; min-height: 28px; }}
.chip {{
    display: inline-flex; align-items: center; gap: 3px;
    background: #2a2a2a; padding: 3px 4px 3px 8px; border-radius: 4px;
    font-size: 12px; font-weight: 500;
}}
.chip-x {{
    width: 16px; height: 16px; display: flex; align-items: center; justify-content: center;
    background: transparent; border: none; color: #666; cursor: pointer;
    border-radius: 50%; font-size: 14px; line-height: 1;
}}
.chip-x:hover {{ background: #ff4444; color: white; }}
.empty {{ color: #666; font-size: 12px; font-style: italic; }}

/* Header */
.header {{ display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; font-size: 12px; color: #888; }}

/* Toast */
.toast {{
    position: fixed; bottom: 10px; left: 50%; transform: translateX(-50%);
    background: #4caf50; color: white; padding: 6px 12px; border-radius: 4px;
    font-size: 12px; opacity: 0; transition: opacity 0.2s; z-index: 1000;
}}
.toast.show {{ opacity: 1; }}
.toast.warn {{ background: #ff9800; }}
</style>
</head>
<body>
<div class="container">
    <div class="search-box">
        <span class="search-icon">🔍</span>
        <input type="text" class="search-input" id="inp" placeholder="{i18n.t('search_to_add')}" autocomplete="off"/>
        <div class="dropdown" id="dd"><div id="res"></div></div>
    </div>
    <div class="header">
        <span id="cnt">0 {i18n.t('symbols')}</span>
        <span>{i18n.t('tws_short') if tws_connected else i18n.t('local_short')}</span>
    </div>
    <div class="chips" id="chips"></div>
</div>
<div class="toast" id="toast"></div>

<script>
const DB = {symbols_json};
let WL = {watchlist_json};
let sel = -1, filt = [], timer = null;

const inp = document.getElementById('inp');
const dd = document.getElementById('dd');
const res = document.getElementById('res');
const chips = document.getElementById('chips');
const cnt = document.getElementById('cnt');
const toast = document.getElementById('toast');

function msg(t, warn) {{
    toast.textContent = t;
    toast.className = 'toast show' + (warn ? ' warn' : '');
    setTimeout(() => toast.classList.remove('show'), 1500);
}}

function render() {{
    cnt.textContent = WL.length + ' {i18n.t('symbol')}' + (WL.length !== 1 ? 's' : '');
    if (!WL.length) {{ chips.innerHTML = '<span class="empty">{i18n.t('empty')}</span>'; return; }}
    chips.innerHTML = WL.map(s => 
        '<div class="chip"><span>' + s + '</span><button class="chip-x" onclick="rm(\\''+s+'\\')">×</button></div>'
    ).join('');
}}

function add(s) {{
    s = s.toUpperCase().trim();
    if (!s) return;
    if (WL.includes(s)) {{ msg('{i18n.t('symbol_exists')} ' + s, true); return; }}
    WL.push(s);
    render();
    msg('{i18n.t('added')} ' + s);
    save();
    inp.value = '';
    dd.classList.remove('show');
    filt = [];
}}

function rm(s) {{
    const i = WL.indexOf(s);
    if (i === -1) return;
    WL.splice(i, 1);
    render();
    msg('{i18n.t('removed')} ' + s);
    save();
}}

function save() {{
    // Save via URL param (triggers Streamlit sync)
    const url = new URL(window.parent.location.href);
    url.searchParams.set('wl', encodeURIComponent(JSON.stringify(WL)));
    // Use history.replaceState to avoid full reload, then trigger minimal reload
    window.parent.history.replaceState(null, '', url.toString());
    // Store backup
    try {{ localStorage.setItem('wl_backup', JSON.stringify(WL)); }} catch(e) {{}}
}}

function fuzzy(t, q) {{
    if (!t || !q) return null;
    t = t.toLowerCase(); q = q.toLowerCase();
    if (t === q) return 1000;
    if (t.startsWith(q)) return 900 - t.length;
    const i = t.indexOf(q);
    if (i !== -1) return 500 - i;
    return null;
}}

function search(q) {{
    if (!q) return [];
    q = q.toUpperCase();
    const scored = [];
    for (const it of DB) {{
        if (WL.includes(it.symbol)) continue;
        const ss = fuzzy(it.symbol, q), ns = fuzzy(it.name, q);
        let sc = 0;
        if (ss !== null) sc = ss + 500;
        if (ns !== null && ns + 200 > sc) sc = ns + 200;
        if (sc > 0) scored.push({{ ...it, sc }});
    }}
    scored.sort((a, b) => b.sc - a.sc);
    return scored.slice(0, 10);
}}

function esc(s) {{ return s ? s.replace(/</g, '&lt;').replace(/>/g, '&gt;') : ''; }}

function renderDD() {{
    if (!filt.length) {{
        res.innerHTML = inp.value ? '<div class="no-res">No results</div>' : '';
        return;
    }}
    res.innerHTML = filt.map((it, i) => 
        '<div class="dd-item' + (i === sel ? ' sel' : '') + '" onclick="pick(' + i + ')">' +
        '<span class="dd-sym">' + esc(it.symbol) + '</span>' +
        '<span class="dd-name">' + esc(it.name || '') + '</span>' +
        '<span class="dd-exch">' + esc(it.exchange || 'US') + '</span></div>'
    ).join('');
}}

function pick(i) {{
    if (i >= 0 && i < filt.length) add(filt[i].symbol);
}}

inp.addEventListener('input', () => {{
    clearTimeout(timer);
    timer = setTimeout(() => {{
        sel = -1;
        const q = inp.value.trim();
        filt = q ? search(q) : [];
        dd.classList.toggle('show', filt.length > 0 || q.length > 0);
        renderDD();
    }}, 100);
}});

inp.addEventListener('keydown', e => {{
    if (!dd.classList.contains('show')) {{
        if (e.key === 'Enter' && inp.value.trim()) {{
            // Direct add if no dropdown
            add(inp.value.trim());
        }}
        return;
    }}
    if (e.key === 'ArrowDown') {{ e.preventDefault(); sel = Math.min(sel + 1, filt.length - 1); renderDD(); }}
    else if (e.key === 'ArrowUp') {{ e.preventDefault(); sel = Math.max(sel - 1, 0); renderDD(); }}
    else if (e.key === 'Enter') {{ e.preventDefault(); pick(sel >= 0 ? sel : 0); }}
    else if (e.key === 'Escape') {{ dd.classList.remove('show'); sel = -1; }}
}});

document.addEventListener('click', e => {{
    if (!e.target.closest('.search-box')) dd.classList.remove('show');
}});

render();
</script>
</body>
</html>
"""
    
    components.html(html, height=220, scrolling=False)


def render_watchlist_compact() -> List[str]:
    """Return current watchlist for use in other components."""
    return _load_watchlist()
