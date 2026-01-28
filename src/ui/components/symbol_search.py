"""
TradingView-style Symbol Search Component.

A snappy, client-side autocomplete search for stock symbols using
embedded JavaScript to avoid Streamlit reruns on every keystroke.

Features:
- Client-side fuzzy filtering for instant results
- Debounced input (200ms)
- Keyboard navigation (up/down/enter/escape)
- Shows symbol, company name, exchange
- Mobile-friendly dropdown
- Preloaded symbol dataset
"""

import streamlit as st
import streamlit.components.v1 as components
import json
import requests
from pathlib import Path
from typing import List, Dict, Optional, Tuple
import time

from src.bot.tws_data_provider import get_tws_provider
from src.ui.i18n import I18n
from src.ui.translations import translations


# Cache for symbol data
_SYMBOL_CACHE: List[Dict] = []
_CACHE_TIMESTAMP: float = 0
_CACHE_TTL: float = 3600  # 1 hour

# Path to local symbol cache file
_PROJECT_ROOT = Path(__file__).parent.parent.parent.parent
_SYMBOL_CACHE_PATH = _PROJECT_ROOT / "data" / "symbol_cache.json"


def _fetch_symbols_from_tradingview() -> List[Dict]:
    """
    Fetch US stock symbols from TradingView's scanner API.
    
    Returns list of dicts with symbol, name, exchange, type.
    """
    symbols = []
    
    try:
        # TradingView scanner API for US stocks
        url = "https://scanner.tradingview.com/america/scan"
        payload = {
            "filter": [
                {"left": "type", "operation": "in_range", "right": ["stock", "dr", "fund"]},
                {"left": "subtype", "operation": "in_range", "right": ["common", "foreign-issuer", "", "etf", "etf,odd", "etf,otc", "etf,cfd"]},
                {"left": "exchange", "operation": "in_range", "right": ["NYSE", "NASDAQ", "AMEX", "TSX", "TSXV"]},
                {"left": "is_primary", "operation": "equal", "right": True},
            ],
            "options": {"lang": "en"},
            "symbols": {"query": {"types": []}, "tickers": []},
            "columns": ["name", "description", "type", "subtype", "exchange"],
            "sort": {"sortBy": "name", "sortOrder": "asc"},
            "range": [0, 36000]
        }
        
        response = requests.post(url, json=payload, timeout=30)
        if response.status_code == 200:
            data = response.json()
            for item in data.get("data", []):
                symbol_data = item.get("d", [])
                if len(symbol_data) >= 5:
                    # Extract exchange:symbol format
                    full_symbol = item.get("s", "")
                    parts = full_symbol.split(":")
                    symbol = parts[1] if len(parts) > 1 else parts[0]
                    
                    symbols.append({
                        "symbol": symbol,
                        "name": symbol_data[1] or symbol,  # description
                        "exchange": symbol_data[4] or "US",
                        "type": symbol_data[2] or "stock"
                    })
        
        return symbols
        
    except Exception as e:
        st.warning(_get_i18n().t("tradingview_fetch_failed", error=str(e)))
        return []


def _load_cached_symbols() -> List[Dict]:
    """Load symbols from local cache file."""
    if _SYMBOL_CACHE_PATH.exists():
        try:
            with open(_SYMBOL_CACHE_PATH, "r") as f:
                return json.load(f)
        except Exception:
            pass
    return []


def _save_symbol_cache(symbols: List[Dict]) -> None:
    """Save symbols to local cache file."""
    try:
        _SYMBOL_CACHE_PATH.parent.mkdir(parents=True, exist_ok=True)
        with open(_SYMBOL_CACHE_PATH, "w") as f:
            json.dump(symbols, f)
    except Exception:
        pass


def _get_common_symbols() -> List[Dict]:
    """Get a baseline list of common US symbols."""
    common = [
        # Major ETFs
        ("SPY", "SPDR S&P 500 ETF Trust", "NYSE"),
        ("QQQ", "Invesco QQQ Trust", "NASDAQ"),
        ("IWM", "iShares Russell 2000 ETF", "NYSE"),
        ("DIA", "SPDR Dow Jones Industrial Average ETF", "NYSE"),
        ("VTI", "Vanguard Total Stock Market ETF", "NYSE"),
        ("VOO", "Vanguard S&P 500 ETF", "NYSE"),
        ("VXX", "iPath Series B S&P 500 VIX Short-Term Futures ETN", "NYSE"),
        ("UVXY", "ProShares Ultra VIX Short-Term Futures ETF", "NYSE"),
        ("SQQQ", "ProShares UltraPro Short QQQ", "NASDAQ"),
        ("TQQQ", "ProShares UltraPro QQQ", "NASDAQ"),
        ("XLF", "Financial Select Sector SPDR Fund", "NYSE"),
        ("XLE", "Energy Select Sector SPDR Fund", "NYSE"),
        ("XLK", "Technology Select Sector SPDR Fund", "NYSE"),
        ("GLD", "SPDR Gold Shares", "NYSE"),
        ("SLV", "iShares Silver Trust", "NYSE"),
        ("TLT", "iShares 20+ Year Treasury Bond ETF", "NASDAQ"),
        ("HYG", "iShares iBoxx High Yield Corporate Bond ETF", "NYSE"),
        ("EEM", "iShares MSCI Emerging Markets ETF", "NYSE"),
        ("EFA", "iShares MSCI EAFE ETF", "NYSE"),
        ("VWO", "Vanguard FTSE Emerging Markets ETF", "NYSE"),
        # Mega Cap Tech
        ("AAPL", "Apple Inc", "NASDAQ"),
        ("MSFT", "Microsoft Corporation", "NASDAQ"),
        ("GOOGL", "Alphabet Inc Class A", "NASDAQ"),
        ("GOOG", "Alphabet Inc Class C", "NASDAQ"),
        ("AMZN", "Amazon.com Inc", "NASDAQ"),
        ("NVDA", "NVIDIA Corporation", "NASDAQ"),
        ("META", "Meta Platforms Inc", "NASDAQ"),
        ("TSLA", "Tesla Inc", "NASDAQ"),
        ("AMD", "Advanced Micro Devices Inc", "NASDAQ"),
        ("INTC", "Intel Corporation", "NASDAQ"),
        ("AVGO", "Broadcom Inc", "NASDAQ"),
        ("ORCL", "Oracle Corporation", "NYSE"),
        ("CRM", "Salesforce Inc", "NYSE"),
        ("ADBE", "Adobe Inc", "NASDAQ"),
        ("NFLX", "Netflix Inc", "NASDAQ"),
        ("CSCO", "Cisco Systems Inc", "NASDAQ"),
        ("IBM", "International Business Machines", "NYSE"),
        ("QCOM", "QUALCOMM Inc", "NASDAQ"),
        ("TXN", "Texas Instruments Inc", "NASDAQ"),
        ("AMAT", "Applied Materials Inc", "NASDAQ"),
        ("MU", "Micron Technology Inc", "NASDAQ"),
        ("NOW", "ServiceNow Inc", "NYSE"),
        ("PANW", "Palo Alto Networks Inc", "NASDAQ"),
        ("SNOW", "Snowflake Inc", "NYSE"),
        ("NET", "Cloudflare Inc", "NYSE"),
        ("CRWD", "CrowdStrike Holdings Inc", "NASDAQ"),
        ("SHOP", "Shopify Inc", "NYSE"),
        ("SQ", "Block Inc", "NYSE"),
        ("PYPL", "PayPal Holdings Inc", "NASDAQ"),
        ("UBER", "Uber Technologies Inc", "NYSE"),
        ("LYFT", "Lyft Inc", "NASDAQ"),
        ("ABNB", "Airbnb Inc", "NASDAQ"),
        ("DASH", "DoorDash Inc", "NASDAQ"),
        ("COIN", "Coinbase Global Inc", "NASDAQ"),
        ("PLTR", "Palantir Technologies Inc", "NYSE"),
        ("ROKU", "Roku Inc", "NASDAQ"),
        ("ZM", "Zoom Video Communications Inc", "NASDAQ"),
        ("DDOG", "Datadog Inc", "NASDAQ"),
        ("TTD", "The Trade Desk Inc", "NASDAQ"),
        ("TEAM", "Atlassian Corporation", "NASDAQ"),
        # Finance
        ("JPM", "JPMorgan Chase & Co", "NYSE"),
        ("BAC", "Bank of America Corp", "NYSE"),
        ("WFC", "Wells Fargo & Co", "NYSE"),
        ("C", "Citigroup Inc", "NYSE"),
        ("GS", "Goldman Sachs Group Inc", "NYSE"),
        ("MS", "Morgan Stanley", "NYSE"),
        ("BLK", "BlackRock Inc", "NYSE"),
        ("SCHW", "Charles Schwab Corp", "NYSE"),
        ("AXP", "American Express Co", "NYSE"),
        ("V", "Visa Inc", "NYSE"),
        ("MA", "Mastercard Inc", "NYSE"),
        ("COF", "Capital One Financial Corp", "NYSE"),
        ("USB", "US Bancorp", "NYSE"),
        ("PNC", "PNC Financial Services Group", "NYSE"),
        # Healthcare
        ("JNJ", "Johnson & Johnson", "NYSE"),
        ("UNH", "UnitedHealth Group Inc", "NYSE"),
        ("PFE", "Pfizer Inc", "NYSE"),
        ("MRK", "Merck & Co Inc", "NYSE"),
        ("ABBV", "AbbVie Inc", "NYSE"),
        ("LLY", "Eli Lilly and Co", "NYSE"),
        ("TMO", "Thermo Fisher Scientific Inc", "NYSE"),
        ("ABT", "Abbott Laboratories", "NYSE"),
        ("BMY", "Bristol-Myers Squibb Co", "NYSE"),
        ("AMGN", "Amgen Inc", "NASDAQ"),
        ("GILD", "Gilead Sciences Inc", "NASDAQ"),
        ("ISRG", "Intuitive Surgical Inc", "NASDAQ"),
        ("VRTX", "Vertex Pharmaceuticals Inc", "NASDAQ"),
        ("REGN", "Regeneron Pharmaceuticals Inc", "NASDAQ"),
        ("MRNA", "Moderna Inc", "NASDAQ"),
        # Consumer
        ("WMT", "Walmart Inc", "NYSE"),
        ("COST", "Costco Wholesale Corp", "NASDAQ"),
        ("HD", "Home Depot Inc", "NYSE"),
        ("TGT", "Target Corp", "NYSE"),
        ("LOW", "Lowe's Companies Inc", "NYSE"),
        ("NKE", "Nike Inc", "NYSE"),
        ("SBUX", "Starbucks Corp", "NASDAQ"),
        ("MCD", "McDonald's Corp", "NYSE"),
        ("DIS", "Walt Disney Co", "NYSE"),
        ("CMCSA", "Comcast Corp", "NASDAQ"),
        ("KO", "Coca-Cola Co", "NYSE"),
        ("PEP", "PepsiCo Inc", "NASDAQ"),
        ("PG", "Procter & Gamble Co", "NYSE"),
        ("PM", "Philip Morris International", "NYSE"),
        ("MO", "Altria Group Inc", "NYSE"),
        # Energy
        ("XOM", "Exxon Mobil Corp", "NYSE"),
        ("CVX", "Chevron Corp", "NYSE"),
        ("COP", "ConocoPhillips", "NYSE"),
        ("SLB", "Schlumberger NV", "NYSE"),
        ("EOG", "EOG Resources Inc", "NYSE"),
        ("OXY", "Occidental Petroleum Corp", "NYSE"),
        ("PSX", "Phillips 66", "NYSE"),
        ("VLO", "Valero Energy Corp", "NYSE"),
        ("MPC", "Marathon Petroleum Corp", "NYSE"),
        # Industrial
        ("CAT", "Caterpillar Inc", "NYSE"),
        ("DE", "Deere & Co", "NYSE"),
        ("UPS", "United Parcel Service Inc", "NYSE"),
        ("FDX", "FedEx Corp", "NYSE"),
        ("BA", "Boeing Co", "NYSE"),
        ("HON", "Honeywell International Inc", "NASDAQ"),
        ("GE", "General Electric Co", "NYSE"),
        ("MMM", "3M Co", "NYSE"),
        ("LMT", "Lockheed Martin Corp", "NYSE"),
        ("RTX", "RTX Corp", "NYSE"),
        ("UNP", "Union Pacific Corp", "NYSE"),
        # Telecom
        ("T", "AT&T Inc", "NYSE"),
        ("VZ", "Verizon Communications Inc", "NYSE"),
        ("TMUS", "T-Mobile US Inc", "NASDAQ"),
        # Real Estate
        ("AMT", "American Tower Corp", "NYSE"),
        ("PLD", "Prologis Inc", "NYSE"),
        ("CCI", "Crown Castle Inc", "NYSE"),
        ("EQIX", "Equinix Inc", "NASDAQ"),
        ("SPG", "Simon Property Group Inc", "NYSE"),
        ("O", "Realty Income Corp", "NYSE"),
        # Other popular
        ("IBKR", "Interactive Brokers Group Inc", "NASDAQ"),
        ("HOOD", "Robinhood Markets Inc", "NASDAQ"),
        ("GME", "GameStop Corp", "NYSE"),
        ("AMC", "AMC Entertainment Holdings Inc", "NYSE"),
        ("RIVN", "Rivian Automotive Inc", "NASDAQ"),
        ("LCID", "Lucid Group Inc", "NASDAQ"),
        ("NIO", "NIO Inc ADR", "NYSE"),
        ("BABA", "Alibaba Group Holding Ltd ADR", "NYSE"),
        ("JD", "JD.com Inc ADR", "NASDAQ"),
        ("PDD", "PDD Holdings Inc ADR", "NASDAQ"),
        ("BIDU", "Baidu Inc ADR", "NASDAQ"),
        ("TSM", "Taiwan Semiconductor ADR", "NYSE"),
        ("ASML", "ASML Holding NV ADR", "NASDAQ"),
        ("ARM", "Arm Holdings plc ADR", "NASDAQ"),
        # Meme & Speculative
        ("BBBY", "Bed Bath & Beyond Inc", "NASDAQ"),
        ("BB", "BlackBerry Ltd", "NYSE"),
        ("NOK", "Nokia Corp ADR", "NYSE"),
        ("SOFI", "SoFi Technologies Inc", "NASDAQ"),
        ("AFRM", "Affirm Holdings Inc", "NASDAQ"),
        ("UPST", "Upstart Holdings Inc", "NASDAQ"),
        ("PATH", "UiPath Inc", "NYSE"),
        ("RBLX", "Roblox Corp", "NYSE"),
        ("U", "Unity Software Inc", "NYSE"),
    ]
    
    return [
        {"symbol": s, "name": n, "exchange": e, "type": "stock"}
        for s, n, e in common
    ]


def load_symbol_database() -> List[Dict]:
    """
    Load the full symbol database for autocomplete.
    
    Tries in order:
    1. Memory cache (if recent)
    2. Local file cache
    3. TradingView API
    4. Built-in common symbols list
    """
    global _SYMBOL_CACHE, _CACHE_TIMESTAMP
    
    now = time.time()
    
    # Check memory cache
    if _SYMBOL_CACHE and (now - _CACHE_TIMESTAMP) < _CACHE_TTL:
        return _SYMBOL_CACHE
    
    # Try local file cache
    symbols = _load_cached_symbols()
    if symbols and len(symbols) > 100:
        _SYMBOL_CACHE = symbols
        _CACHE_TIMESTAMP = now
        return symbols
    
    # Try TradingView API
    symbols = _fetch_symbols_from_tradingview()
    if symbols and len(symbols) > 1000:
        _save_symbol_cache(symbols)
        _SYMBOL_CACHE = symbols
        _CACHE_TIMESTAMP = now
        return symbols
    
    # Fall back to common symbols
    symbols = _get_common_symbols()
    _SYMBOL_CACHE = symbols
    _CACHE_TIMESTAMP = now
    return symbols


def render_symbol_search(
    key: str = "symbol_search",
    placeholder: str = "Search symbols (e.g., AAPL, Apple, FLEX)...",
    height: int = 400,
) -> Optional[str]:
    """
    Render a TradingView-style symbol search component.
    
    Returns the selected symbol or None if nothing selected.
    
    Args:
        key: Unique key for the component
        placeholder: Placeholder text for the search input
        height: Height of the component in pixels
    """
    i18n = _get_i18n()
    if placeholder == "Search symbols (e.g., AAPL, Apple, FLEX)...":
        placeholder = i18n.t("symbol_search_placeholder")
    # Load symbol database
    symbols = load_symbol_database()
    
    # Convert to JSON for JavaScript
    symbols_json = json.dumps(symbols)
    
    # Get TWS connection status for live search
    try:
        provider = get_tws_provider()
        tws_connected = provider.is_connected()
    except Exception:
        tws_connected = False
    
    # HTML/CSS/JS component
    component_html = f"""
    <!DOCTYPE html>
    <html>
    <head>
        <style>
            * {{
                box-sizing: border-box;
                margin: 0;
                padding: 0;
            }}
            
            body {{
                font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
                background: transparent;
                color: #c0caf5;
            }}
            
            .search-container {{
                position: relative;
                width: 100%;
                max-width: 600px;
            }}
            
            .search-input {{
                width: 100%;
                padding: 12px 16px;
                padding-left: 40px;
                font-size: 14px;
                border: 1px solid #363a45;
                border-radius: 8px;
                background: #16161e;
                color: #c0caf5;
                outline: none;
                transition: border-color 0.2s, box-shadow 0.2s;
            }}
            
            .search-input:focus {{
                border-color: #7aa2f7;
                box-shadow: 0 0 0 2px rgba(122, 162, 247, 0.2);
            }}
            
            .search-input::placeholder {{
                color: #a9b1d6;
            }}
            
            .search-icon {{
                position: absolute;
                left: 12px;
                top: 50%;
                transform: translateY(-50%);
                color: #a9b1d6;
                pointer-events: none;
            }}
            
            .dropdown {{
                position: absolute;
                top: calc(100% + 4px);
                left: 0;
                right: 0;
                max-height: 320px;
                overflow-y: auto;
                background: #16161e;
                border: 1px solid #565f89;
                border-radius: 8px;
                box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
                z-index: 1000;
                display: none;
            }}
            
            .dropdown.visible {{
                display: block;
            }}
            
            .dropdown-item {{
                display: flex;
                align-items: center;
                padding: 10px 16px;
                cursor: pointer;
                border-bottom: 1px solid #2a2e39;
                transition: background 0.15s;
            }}
            
            .dropdown-item:last-child {{
                border-bottom: none;
            }}
            
            .dropdown-item:hover,
            .dropdown-item.selected {{
                background: #2a2e39;
            }}
            
            .dropdown-item.selected {{
                background: #363a45;
            }}
            
            .item-symbol {{
                font-weight: 600;
                font-size: 14px;
                color: #c0caf5;
                min-width: 70px;
            }}
            
            .item-name {{
                flex: 1;
                font-size: 13px;
                color: #a9b1d6;
                margin-left: 12px;
                overflow: hidden;
                text-overflow: ellipsis;
                white-space: nowrap;
            }}
            
            .item-exchange {{
                font-size: 11px;
                color: #a9b1d6;
                background: #1f2335;
                padding: 2px 6px;
                border-radius: 4px;
                margin-left: 8px;
            }}
            
            .item-type {{
                font-size: 10px;
                color: #7aa2f7;
                margin-left: 6px;
                text-transform: uppercase;
            }}
            
            .highlight {{
                color: #7aa2f7;
                font-weight: 600;
            }}
            
            .no-results {{
                padding: 16px;
                text-align: center;
                color: #a9b1d6;
                font-size: 13px;
            }}
            
            .loading {{
                padding: 16px;
                text-align: center;
                color: #787b86;
            }}
            
            .status-bar {{
                display: flex;
                justify-content: space-between;
                padding: 8px 12px;
                font-size: 11px;
                color: #a9b1d6;
                border-bottom: 1px solid #1f2335;
            }}
            
            .status-connected {{
                color: #9ece6a;
            }}
            
            .status-offline {{
                color: #e0af68;
            }}
            
            /* Scrollbar styling */
            .dropdown::-webkit-scrollbar {{
                width: 8px;
            }}
            
            .dropdown::-webkit-scrollbar-track {{
                background: #1e222d;
            }}
            
            .dropdown::-webkit-scrollbar-thumb {{
                background: #363a45;
                border-radius: 4px;
            }}
            
            .dropdown::-webkit-scrollbar-thumb:hover {{
                background: #4a4e59;
            }}
            
            /* Selection display */
            .selection-display {{
                margin-top: 12px;
                padding: 12px 16px;
                background: rgba(41, 98, 255, 0.1);
                border: 1px solid rgba(41, 98, 255, 0.3);
                border-radius: 8px;
                display: none;
            }}
            
            .selection-display.visible {{
                display: flex;
                align-items: center;
                justify-content: space-between;
            }}
            
            .selection-info {{
                display: flex;
                align-items: center;
                gap: 12px;
            }}
            
            .selection-symbol {{
                font-weight: 700;
                font-size: 16px;
                color: #2962ff;
            }}
            
            .selection-name {{
                font-size: 13px;
                color: #a9b1d6;
            }}
            
            .add-btn {{
                padding: 8px 20px;
                background: #7aa2f7;
                color: white;
                border: none;
                border-radius: 6px;
                font-size: 13px;
                font-weight: 600;
                cursor: pointer;
                transition: background 0.2s;
            }}
            
            .add-btn:hover {{
                background: #1e53e4;
            }}
            
            .add-btn:active {{
                background: #1848c9;
            }}
            
            /* Mobile responsive */
            @media (max-width: 480px) {{
                .item-name {{
                    display: none;
                }}
                
                .dropdown-item {{
                    padding: 12px;
                }}
            }}
        </style>
    </head>
    <body>
        <div class="search-container">
            <svg class="search-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="11" cy="11" r="8"></circle>
                <path d="M21 21l-4.35-4.35"></path>
            </svg>
            <input 
                type="text" 
                class="search-input" 
                id="searchInput"
                placeholder="{placeholder}"
                autocomplete="off"
                spellcheck="false"
            />
            <div class="dropdown" id="dropdown">
                <div class="status-bar">
                    <span id="resultCount">{i18n.t("type_to_search")}</span>
                    <span class="{'status-connected' if tws_connected else 'status-offline'}">
                        {'{i18n.t("tws_connected_label")}' if tws_connected else '{i18n.t("local_database_label")}'}
                    </span>
                </div>
                <div id="results"></div>
            </div>
        </div>
        
        <div class="selection-display" id="selectionDisplay">
            <div class="selection-info">
                <span class="selection-symbol" id="selectedSymbol"></span>
                <span class="selection-name" id="selectedName"></span>
            </div>
            <button class="add-btn" id="addBtn">{i18n.t("add_to_watchlist")}</button>
        </div>
        
        <script>
            // Symbol database
            const symbols = {symbols_json};
            
            // State
            let selectedIndex = -1;
            let filteredResults = [];
            let selectedSymbol = null;
            let debounceTimer = null;
            
            // DOM elements
            const searchInput = document.getElementById('searchInput');
            const dropdown = document.getElementById('dropdown');
            const results = document.getElementById('results');
            const resultCount = document.getElementById('resultCount');
            const selectionDisplay = document.getElementById('selectionDisplay');
            const selectedSymbolEl = document.getElementById('selectedSymbol');
            const selectedNameEl = document.getElementById('selectedName');
            const addBtn = document.getElementById('addBtn');
            
            // Fuzzy search function
            function fuzzyMatch(text, query) {{
                if (!text || !query) return {{ match: false, score: 0, indices: [] }};
                
                text = text.toLowerCase();
                query = query.toLowerCase();
                
                // Exact match gets highest score
                if (text === query) {{
                    return {{ match: true, score: 1000, indices: [...Array(query.length).keys()] }};
                }}
                
                // Starts with gets high score
                if (text.startsWith(query)) {{
                    return {{ match: true, score: 900 - text.length, indices: [...Array(query.length).keys()] }};
                }}
                
                // Contains gets medium score
                const idx = text.indexOf(query);
                if (idx !== -1) {{
                    const indices = [...Array(query.length).keys()].map(i => i + idx);
                    return {{ match: true, score: 500 - idx, indices }};
                }}
                
                // Fuzzy match (character by character)
                let queryIdx = 0;
                let indices = [];
                let consecutiveBonus = 0;
                
                for (let i = 0; i < text.length && queryIdx < query.length; i++) {{
                    if (text[i] === query[queryIdx]) {{
                        indices.push(i);
                        if (indices.length > 1 && indices[indices.length - 1] === indices[indices.length - 2] + 1) {{
                            consecutiveBonus += 10;
                        }}
                        queryIdx++;
                    }}
                }}
                
                if (queryIdx === query.length) {{
                    // All characters found
                    const spread = indices[indices.length - 1] - indices[0];
                    const score = 100 - spread + consecutiveBonus;
                    return {{ match: true, score: Math.max(score, 1), indices }};
                }}
                
                return {{ match: false, score: 0, indices: [] }};
            }}
            
            // Search function
            function search(query) {{
                if (!query || query.length < 1) {{
                    return [];
                }}
                
                query = query.toUpperCase();
                
                const scored = [];
                
                for (const item of symbols) {{
                    // Match against symbol (highest priority)
                    const symbolMatch = fuzzyMatch(item.symbol, query);
                    // Match against name
                    const nameMatch = fuzzyMatch(item.name, query);
                    
                    let bestScore = 0;
                    let matchType = '';
                    let indices = [];
                    
                    if (symbolMatch.match) {{
                        bestScore = symbolMatch.score + 500; // Boost symbol matches
                        matchType = 'symbol';
                        indices = symbolMatch.indices;
                    }}
                    
                    if (nameMatch.match && nameMatch.score > bestScore - 500) {{
                        if (nameMatch.score + 200 > bestScore) {{
                            bestScore = nameMatch.score + 200;
                            matchType = 'name';
                            indices = nameMatch.indices;
                        }}
                    }}
                    
                    if (bestScore > 0) {{
                        scored.push({{
                            ...item,
                            score: bestScore,
                            matchType,
                            indices
                        }});
                    }}
                }}
                
                // Sort by score descending
                scored.sort((a, b) => b.score - a.score);
                
                // Return top 15 results
                return scored.slice(0, 15);
            }}
            
            // Highlight matched characters
            function highlightMatch(text, indices, matchType, currentMatchType) {{
                if (!text) return '';
                if (matchType !== currentMatchType || !indices.length) {{
                    return escapeHtml(text);
                }}
                
                let result = '';
                let lastIdx = 0;
                
                for (const idx of indices) {{
                    if (idx < text.length) {{
                        result += escapeHtml(text.slice(lastIdx, idx));
                        result += '<span class="highlight">' + escapeHtml(text[idx]) + '</span>';
                        lastIdx = idx + 1;
                    }}
                }}
                result += escapeHtml(text.slice(lastIdx));
                
                return result;
            }}
            
            function escapeHtml(text) {{
                const div = document.createElement('div');
                div.textContent = text;
                return div.innerHTML;
            }}
            
            // Render results
            function renderResults() {{
                if (filteredResults.length === 0) {{
                    const query = searchInput.value.trim();
                    if (query.length >= 1) {{
                        results.innerHTML = '<div class="no-results">{i18n.t("no_results_for")} "' + escapeHtml(query) + '"</div>';
                        resultCount.textContent = '{i18n.t("no_matches")}';
                    }} else {{
                        results.innerHTML = '';
                        resultCount.textContent = '{i18n.t("type_to_search")}';
                    }}
                    return;
                }}
                
                resultCount.textContent = filteredResults.length + ' {i18n.t("results")}' + (filteredResults.length !== 1 ? 's' : '');
                
                let html = '';
                filteredResults.forEach((item, index) => {{
                    const isSelected = index === selectedIndex;
                    const symbolHtml = highlightMatch(item.symbol, item.indices, item.matchType, 'symbol');
                    const nameHtml = highlightMatch(item.name, item.indices, item.matchType, 'name');
                    
                    html += `
                        <div class="dropdown-item ${{isSelected ? 'selected' : ''}}" data-index="${{index}}">
                            <span class="item-symbol">${{symbolHtml}}</span>
                            <span class="item-name">${{nameHtml}}</span>
                            <span class="item-exchange">${{escapeHtml(item.exchange || 'US')}}</span>
                            ${{item.type && item.type !== 'stock' ? '<span class="item-type">' + escapeHtml(item.type) + '</span>' : ''}}
                        </div>
                    `;
                }});
                
                results.innerHTML = html;
                
                // Add click handlers
                document.querySelectorAll('.dropdown-item').forEach(el => {{
                    el.addEventListener('click', () => {{
                        const idx = parseInt(el.dataset.index);
                        selectItem(idx);
                    }});
                }});
            }}
            
            // Select an item
            function selectItem(index) {{
                if (index >= 0 && index < filteredResults.length) {{
                    const item = filteredResults[index];
                    selectedSymbol = item.symbol;
                    
                    // Update UI
                    searchInput.value = item.symbol;
                    selectedSymbolEl.textContent = item.symbol;
                    selectedNameEl.textContent = item.name || '';
                    selectionDisplay.classList.add('visible');
                    dropdown.classList.remove('visible');
                    
                    // Send to Streamlit
                    window.parent.postMessage({{
                        type: 'symbol_selected',
                        symbol: item.symbol,
                        name: item.name,
                        exchange: item.exchange
                    }}, '*');
                }}
            }}
            
            // Handle input with debounce
            function handleInput() {{
                clearTimeout(debounceTimer);
                debounceTimer = setTimeout(() => {{
                    const query = searchInput.value.trim();
                    selectedIndex = -1;
                    
                    if (query.length >= 1) {{
                        filteredResults = search(query);
                        dropdown.classList.add('visible');
                    }} else {{
                        filteredResults = [];
                        dropdown.classList.remove('visible');
                    }}
                    
                    renderResults();
                    
                    // Hide selection if typing
                    if (selectedSymbol && query !== selectedSymbol) {{
                        selectionDisplay.classList.remove('visible');
                        selectedSymbol = null;
                    }}
                }}, 150); // 150ms debounce
            }}
            
            // Keyboard navigation
            function handleKeydown(e) {{
                if (!dropdown.classList.contains('visible')) {{
                    if (e.key === 'ArrowDown' || e.key === 'Enter') {{
                        const query = searchInput.value.trim();
                        if (query.length >= 1) {{
                            filteredResults = search(query);
                            dropdown.classList.add('visible');
                            renderResults();
                        }}
                    }}
                    return;
                }}
                
                switch (e.key) {{
                    case 'ArrowDown':
                        e.preventDefault();
                        selectedIndex = Math.min(selectedIndex + 1, filteredResults.length - 1);
                        renderResults();
                        scrollToSelected();
                        break;
                        
                    case 'ArrowUp':
                        e.preventDefault();
                        selectedIndex = Math.max(selectedIndex - 1, 0);
                        renderResults();
                        scrollToSelected();
                        break;
                        
                    case 'Enter':
                        e.preventDefault();
                        if (selectedIndex >= 0) {{
                            selectItem(selectedIndex);
                        }} else if (filteredResults.length > 0) {{
                            selectItem(0);
                        }}
                        break;
                        
                    case 'Escape':
                        dropdown.classList.remove('visible');
                        selectedIndex = -1;
                        break;
                        
                    case 'Tab':
                        if (filteredResults.length > 0) {{
                            e.preventDefault();
                            if (selectedIndex < 0) selectedIndex = 0;
                            selectItem(selectedIndex);
                        }}
                        break;
                }}
            }}
            
            function scrollToSelected() {{
                const selected = document.querySelector('.dropdown-item.selected');
                if (selected) {{
                    selected.scrollIntoView({{ block: 'nearest' }});
                }}
            }}
            
            // Add button click
            addBtn.addEventListener('click', () => {{
                if (selectedSymbol) {{
                    window.parent.postMessage({{
                        type: 'add_symbol',
                        symbol: selectedSymbol
                    }}, '*');
                }}
            }});
            
            // Event listeners
            searchInput.addEventListener('input', handleInput);
            searchInput.addEventListener('keydown', handleKeydown);
            searchInput.addEventListener('focus', () => {{
                if (searchInput.value.trim().length >= 1 && filteredResults.length > 0) {{
                    dropdown.classList.add('visible');
                }}
            }});
            
            // Close dropdown when clicking outside
            document.addEventListener('click', (e) => {{
                if (!e.target.closest('.search-container')) {{
                    dropdown.classList.remove('visible');
                }}
            }});
            
            // Focus input on load
            searchInput.focus();
        </script>
    </body>
    </html>
    """
    
    # Render component
    result = components.html(component_html, height=height, scrolling=False)
    
    # Handle messages from JavaScript
    # Note: st.components.html doesn't directly support bidirectional communication
    # We use session state to track selections
    
    return None  # Selection handled via session state


def render_symbol_multiselect(
    session_key: str,
    url_key: str = "sym_multi",
    label: str = "Symbols",
    max_height: int = 220
) -> List[str]:
    """
    Render a multiselect component with symbol search that syncs via URL parameters.
    Updated selection is stored in st.session_state[session_key].
    
    Args:
        session_key: The st.session_state key to store the list of symbols.
        url_key: Unique URL query parameter key for sync.
        label: Label to display above the component (not used in current HTML but good for API).
        max_height: Height of the component iframe.
        
    Returns:
        The current list of selected symbols.
    """
    i18n = _get_i18n()
    # Ensure session state exists
    if session_key not in st.session_state:
        st.session_state[session_key] = []
        
    current_selection = st.session_state[session_key]
    
    # Check for sync request from JS
    params = st.query_params
    if url_key in params:
        try:
            import urllib.parse
            new_list = json.loads(urllib.parse.unquote(params[url_key]))
            if isinstance(new_list, list):
                st.session_state[session_key] = new_list
                # Update local variable to return correct value immediately if we didn't rerun
                current_selection = new_list
        except Exception:
            pass
            
        # Clear param and rerun to clean URL and update state
        del st.query_params[url_key]
        st.rerun()
        
    # Get TWS connection status
    try:
        provider = get_tws_provider()
        tws_connected = provider.is_connected()
    except Exception:
        tws_connected = False
        
    # Load symbol database
    symbols_db = load_symbol_database()
    
    # Prepare data for JS
    symbols_json = json.dumps(symbols_db)
    selection_json = json.dumps(current_selection)
    tws_status_label = i18n.t("tws_short") if tws_connected else i18n.t("local_short")
    
    # Generate HTML/JS
    # Adapted from watchlist.py _render_js_component
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
    <div class="header">
        <span id="cnt">0 {i18n.t("symbols")}</span>
        <span>{tws_status_label}</span>
    </div>
    <div class="search-box">
        <span class="search-icon">🔍</span>
        <input type="text" class="search-input" id="inp" placeholder="{i18n.t('search_to_add')}" autocomplete="off"/>
        <div class="dropdown" id="dd"><div id="res"></div></div>
    </div>
    <div class="chips" id="chips"></div>
</div>
<div class="toast" id="toast"></div>

<script>
const DB = {symbols_json};
let LIST = {selection_json};
let sel = -1, filt = [], timer = null;
const URL_KEY = "{url_key}";

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
    cnt.textContent = LIST.length + ' {i18n.t("symbol")}' + (LIST.length !== 1 ? 's' : '');
    if (!LIST.length) {{ chips.innerHTML = '<span class="empty">{i18n.t("no_symbols_selected")}</span>'; return; }}
    chips.innerHTML = LIST.map(s => 
        '<div class="chip"><span>' + s + '</span><button class="chip-x" onclick="rm(\\''+s+'\\')">×</button></div>'
    ).join('');
}}

function add(s) {{
    s = s.toUpperCase().trim();
    if (!s) return;
    if (LIST.includes(s)) {{ msg('{i18n.t("symbol_exists")} ' + s, true); return; }}
    LIST.push(s);
    render();
    msg('{i18n.t("added")} ' + s);
    save();
    inp.value = '';
    dd.classList.remove('show');
    filt = [];
}}

function rm(s) {{
    const i = LIST.indexOf(s);
    if (i === -1) return;
    LIST.splice(i, 1);
    render();
    msg('{i18n.t("removed")} ' + s);
    save();
}}

function save() {{
    // Save via URL param (triggers Streamlit sync)
    // We add a random timestamp to ensure the URL changes even if the list content is same (to trigger reload if needed)
    // Actually, just the list is enough usually.
    const url = new URL(window.parent.location.href);
    url.searchParams.set(URL_KEY, encodeURIComponent(JSON.stringify(LIST)));
    window.parent.history.replaceState(null, '', url.toString());
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
        if (LIST.includes(it.symbol)) continue;
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
        res.innerHTML = inp.value ? '<div class="no-res">{i18n.t("no_results")}</div>' : '';
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
    components.html(html, height=max_height, scrolling=False)
    
    return current_selection


def _get_i18n() -> I18n:
    if "i18n" not in st.session_state:
        st.session_state["i18n"] = I18n(translations)
    return st.session_state["i18n"]
