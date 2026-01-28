"""
Unit tests for UI style helpers.
"""

import sys
from pathlib import Path
from types import SimpleNamespace

# Provide a minimal streamlit stub for import-time usage
sys.modules.setdefault("streamlit", SimpleNamespace(markdown=lambda *a, **k: None, metric=lambda *a, **k: None))

# Add src to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent.parent / "src" / "ui"))

from styles import COLORS, status_badge, profit_loss_color, format_currency, format_currency_html


def test_profit_loss_color_positive_negative_zero():
    assert profit_loss_color(10.0) == COLORS["accent_green"]
    assert profit_loss_color(-1.0) == COLORS["accent_red"]
    assert profit_loss_color(0.0) == COLORS["text_primary"]


def test_format_currency_sign_and_precision():
    assert format_currency(1234.567) == "$1,234.57"
    assert format_currency(1234.0, show_sign=True) == "+$1,234.00"
    assert format_currency(-12.3, show_sign=True) == "$-12.30"


def test_format_currency_html_colors():
    positive = format_currency_html(10.0, show_sign=True)
    negative = format_currency_html(-10.0, show_sign=True)
    zero = format_currency_html(0.0)

    assert COLORS["accent_green"] in positive
    assert "+$10.00" in positive
    assert COLORS["accent_red"] in negative
    assert "$-10.00" in negative
    assert COLORS["text_primary"] in zero


def test_status_badge_color_based_on_connection():
    connected = status_badge("Connected", connected=True)
    disconnected = status_badge("Disconnected", connected=False)

    assert COLORS["accent_green"] in connected
    assert "Connected" in connected
    assert COLORS["accent_red"] in disconnected
    assert "Disconnected" in disconnected
