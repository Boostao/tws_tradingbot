"""
Tokyo Night theme styles for the Streamlit UI.

Color Palette:
- Background Main: #1a1b26
- Background Cards: #16161e
- Text Primary: #c0caf5
- Text Secondary: #a9b1d6
- Accent Blue: #7aa2f7
- Accent Green (Profit): #9ece6a
- Accent Red (Loss): #f7768e
- Borders: #565f89
"""

import streamlit as st

# Color Constants
COLORS = {
    "bg_main": "#1a1b26",
    "bg_card": "#16161e",
    "bg_input": "#1f2335",
    "text_primary": "#c0caf5",
    "text_secondary": "#a9b1d6",
    "accent_blue": "#7aa2f7",
    "accent_green": "#9ece6a",
    "accent_red": "#f7768e",
    "accent_yellow": "#e0af68",
    "border": "#565f89",
    "border_light": "#3b4261",
}

DARK_THEME_CSS = """
<style>
    @font-face {
        font-family: 'IoskeleyMono';
        src: url('/static/fonts/IoskeleyMono-Regular.woff2') format('woff2');
    }
    
    /* Target specific text elements key components */
    h1, h2, h3, h4, h5, h6, p, label, .stMarkdown, .stTextInput input, .stTextArea textarea, [data-testid="stMetricValue"], [data-testid="stMetricLabel"] {
        font-family: 'IoskeleyMono', monospace !important;
    }
    
    /* Exclude specific Streamlit UI elements that use icons or system fonts */
    [data-testid="stIconMaterial"], .material-icons {
        font-family: 'Material Icons', 'Material Symbols Rounded' !important;
    }
    .stApp {
        background-color: #1a1b26;
    }
    
    /* Sidebar styling */
    [data-testid="stSidebar"] {
        background-color: #16161e;
        border-right: 1px solid #565f89;
    }
    
    [data-testid="stSidebar"] [data-testid="stMarkdownContainer"] p {
        color: #c0caf5;
    }
    
    /* Main content area */
    .main .block-container {
        padding-top: 2rem;
        padding-bottom: 2rem;
    }
    
    /* Headers */
    h1, h2, h3, h4, h5, h6 {
        color: #c0caf5 !important;
    }
    
    /* Regular text */
    p, span, label, .stMarkdown {
        color: #c0caf5;
    }
    
    /* Secondary text */
    .secondary-text {
        color: #a9b1d6;
    }
    
    /* Tab styling */
    .stTabs [data-baseweb="tab-list"] {
        background-color: #16161e;
        border-radius: 8px;
        padding: 4px;
        gap: 4px;
    }
    
    .stTabs [data-baseweb="tab"] {
        background-color: transparent;
        color: #a9b1d6;
        border-radius: 6px;
        padding: 8px 16px;
        font-weight: 500;
    }
    
    .stTabs [data-baseweb="tab"]:hover {
        background-color: #1f2335;
        color: #c0caf5;
    }
    
    .stTabs [aria-selected="true"] {
        background-color: #7aa2f7 !important;
        color: #ffffff !important;
    }
    
    /* Card-like containers */
    .card {
        background-color: #16161e;
        border: 1px solid #565f89;
        border-radius: 8px;
        padding: 1rem;
        margin-bottom: 1rem;
    }
    
    /* Metric cards */
    [data-testid="metric-container"] {
        background-color: #16161e;
        border: 1px solid #565f89;
        border-radius: 8px;
        padding: 1rem;
    }
    
    [data-testid="stMetricValue"] {
        color: #c0caf5;
        white-space: nowrap;
        overflow: visible;
        text-overflow: clip;
    }
    
    [data-testid="stMetricLabel"] {
        color: #a9b1d6;
        white-space: nowrap;
    }
    
    /* Positive/Negative delta colors */
    [data-testid="stMetricDelta"] svg {
        display: none;
    }
    
    /* Buttons */
    .stButton > button {
        background-color: #7aa2f7;
        color: white;
        border: none;
        border-radius: 6px;
        padding: 0.5rem 1rem;
        font-weight: 500;
        transition: background-color 0.2s;
    }
    
    .stButton > button:hover {
        background-color: #5a8fd7;
        border: none;
    }
    
    /* Secondary button style */
    .secondary-btn > button {
        background-color: #1f2335 !important;
        color: #c0caf5 !important;
        border: 1px solid #565f89 !important;
    }
    
    .secondary-btn > button:hover {
        background-color: #3b4261 !important;
    }
    
    /* Danger button style */
    .danger-btn > button {
        background-color: #f7768e !important;
    }
    
    .danger-btn > button:hover {
        background-color: #d6657a !important;
    }
    
    /* Success button style */
    .success-btn > button {
        background-color: #9ece6a !important;
    }
    
    .success-btn > button:hover {
        background-color: #7eb85a !important;
    }
    
    /* Input fields */
    .stTextInput > div > div > input,
    .stNumberInput > div > div > input,
    .stSelectbox > div > div > div {
        background-color: #1f2335;
        color: #c0caf5;
        border: 1px solid #565f89;
        border-radius: 6px;
    }
    
    .stTextInput > div > div > input:focus,
    .stNumberInput > div > div > input:focus {
        border-color: #7aa2f7;
        box-shadow: 0 0 0 1px #7aa2f7;
    }
    
    /* Selectbox */
    [data-baseweb="select"] {
        background-color: #1f2335;
    }
    
    [data-baseweb="select"] > div {
        background-color: #1f2335;
        border-color: #565f89;
    }
    
    /* Dropdown menu */
    [data-baseweb="popover"] {
        background-color: #16161e;
        border: 1px solid #565f89;
    }
    
    [data-baseweb="menu"] {
        background-color: #16161e;
    }
    
    [role="option"] {
        background-color: #16161e;
        color: #c0caf5;
    }
    
    [role="option"]:hover {
        background-color: #1f2335;
    }
    
    /* DataFrames / Tables */
    .stDataFrame {
        background-color: #16161e;
        border-radius: 8px;
    }
    
    [data-testid="stDataFrame"] > div {
        background-color: #16161e;
    }
    
    .stDataFrame thead tr th {
        background-color: #1f2335 !important;
        color: #a9b1d6 !important;
        border-bottom: 1px solid #565f89 !important;
    }
    
    .stDataFrame tbody tr td {
        background-color: #16161e !important;
        color: #c0caf5 !important;
        border-bottom: 1px solid #565f89 !important;
    }
    
    .stDataFrame tbody tr:hover td {
        background-color: #1f2335 !important;
    }
    
    /* Expander */
    .streamlit-expanderHeader {
        background-color: #16161e;
        border: 1px solid #565f89;
        border-radius: 6px;
        color: #c0caf5;
    }
    
    .streamlit-expanderContent {
        background-color: #16161e;
        border: 1px solid #565f89;
        border-top: none;
        border-radius: 0 0 6px 6px;
    }
    
    /* Divider */
    hr {
        border-color: #565f89;
    }
    
    /* Status indicators */
    .status-connected {
        color: #9ece6a;
    }
    
    .status-disconnected {
        color: #f7768e;
    }
    
    .status-pending {
        color: #e0af68;
    }
    
    /* Profit/Loss colors */
    .profit {
        color: #9ece6a !important;
    }
    
    .loss {
        color: #f7768e !important;
    }
    
    /* Scrollbar styling */
    ::-webkit-scrollbar {
        width: 8px;
        height: 8px;
    }
    
    ::-webkit-scrollbar-track {
        background: #16161e;
    }
    
    ::-webkit-scrollbar-thumb {
        background: #565f89;
        border-radius: 4px;
    }
    
    ::-webkit-scrollbar-thumb:hover {
        background: #3b4261;
    }
    
    /* Toast/Alert styling */
    .stAlert {
        background-color: #16161e;
        border: 1px solid #565f89;
        border-radius: 6px;
    }
    
    /* Code blocks */
    code {
        background-color: #1f2335;
        color: #c0caf5;
        padding: 2px 6px;
        border-radius: 4px;
    }
    
    /* File uploader */
    [data-testid="stFileUploader"] {
        background-color: #16161e;
        border: 1px dashed #565f89;
        border-radius: 8px;
    }
    
    /* Date input */
    .stDateInput > div > div > input {
        background-color: #1f2335;
        color: #c0caf5;
        border: 1px solid #565f89;
    }
    
    /* Checkbox */
    .stCheckbox label span {
        color: #c0caf5;
    }
    
    /* Radio buttons */
    .stRadio label span {
        color: #c0caf5;
    }
    
    /* Slider */
    .stSlider > div > div > div {
        background-color: #565f89;
    }
    
    .stSlider > div > div > div > div {
        background-color: #7aa2f7;
    }
    
    /* Progress bar */
    .stProgress > div > div > div {
        background-color: #7aa2f7;
    }
    
    /* Spinner */
    .stSpinner > div {
        border-color: #7aa2f7;
        border-right-color: transparent;
    }
    
    /* Hide Streamlit branding */
    #MainMenu {visibility: hidden;}
    footer {visibility: hidden;}
</style>
"""


def apply_theme() -> None:
    """Apply the Tokyo Night theme to the Streamlit app."""
    st.markdown(DARK_THEME_CSS, unsafe_allow_html=True)


def styled_metric_card(label: str, value: str, delta: str = None, delta_color: str = "normal") -> None:
    """
    Render a styled metric card.
    
    Args:
        label: The metric label
        value: The metric value
        delta: Optional delta/change value
        delta_color: 'normal', 'inverse', or 'off' for delta coloring
    """
    st.metric(label=label, value=value, delta=delta, delta_color=delta_color)


def status_badge(status: str, connected: bool = True) -> str:
    """
    Generate HTML for a status badge.
    
    Args:
        status: The status text to display
        connected: If True, shows green; if False, shows red
        
    Returns:
        HTML string for the badge
    """
    color = COLORS["accent_green"] if connected else COLORS["accent_red"]
    return f'<span style="color: {color}; font-weight: 500;">● {status}</span>'


def profit_loss_color(value: float) -> str:
    """Return the appropriate color for a profit/loss value."""
    if value > 0:
        return COLORS["accent_green"]
    elif value < 0:
        return COLORS["accent_red"]
    return COLORS["text_primary"]


def format_currency(value: float, show_sign: bool = False) -> str:
    """Format a value as currency (plain text, for st.metric)."""
    sign = "+" if value > 0 and show_sign else ""
    return f"{sign}${value:,.2f}"


def format_currency_html(value: float, show_sign: bool = False) -> str:
    """Format a value as currency with HTML coloring (for st.markdown)."""
    sign = "+" if value > 0 and show_sign else ""
    color = profit_loss_color(value)
    return f'<span style="color: {color};">{sign}${value:,.2f}</span>'
