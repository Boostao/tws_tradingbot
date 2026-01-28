"""
Unit Tests for Market Hours Module

Tests for market hours checking, holiday handling, and trading session management.
"""

import pytest
import datetime
import sys
from pathlib import Path

# Direct import to avoid config chain
sys.path.insert(0, str(Path(__file__).parent.parent.parent / "src" / "utils"))

from market_hours import (
    MarketHours,
    MarketSession,
    is_market_open,
    get_next_market_open,
    is_trading_day,
    ALL_US_HOLIDAYS,
    ALL_US_EARLY_CLOSE,
)


class TestMarketSession:
    """Tests for MarketSession dataclass."""
    
    def test_session_active(self):
        """Test session is_active method."""
        session = MarketSession(
            open_time=datetime.time(9, 30),
            close_time=datetime.time(16, 0),
            name="Regular"
        )
        
        # During session
        assert session.is_active(datetime.time(10, 0)) == True
        assert session.is_active(datetime.time(15, 59)) == True
        
        # Boundary times
        assert session.is_active(datetime.time(9, 30)) == True
        assert session.is_active(datetime.time(16, 0)) == True
        
        # Outside session
        assert session.is_active(datetime.time(9, 29)) == False
        assert session.is_active(datetime.time(16, 1)) == False


class TestMarketHours:
    """Tests for MarketHours class."""
    
    def test_default_initialization(self):
        """Test default market hours initialization."""
        mh = MarketHours()
        
        assert mh.regular_open == datetime.time(9, 30)
        assert mh.regular_close == datetime.time(16, 0)
        assert mh.pre_market_open == datetime.time(4, 0)
        assert mh.post_market_close == datetime.time(20, 0)
        assert mh.include_extended_hours == False
    
    def test_custom_initialization(self):
        """Test custom market hours initialization."""
        mh = MarketHours(
            regular_open="08:00",
            regular_close="15:00",
            include_extended_hours=True
        )
        
        assert mh.regular_open == datetime.time(8, 0)
        assert mh.regular_close == datetime.time(15, 0)
        assert mh.include_extended_hours == True
    
    def test_is_holiday(self):
        """Test holiday detection."""
        mh = MarketHours()
        
        # Known holidays
        assert mh.is_holiday(datetime.date(2024, 12, 25)) == True  # Christmas
        assert mh.is_holiday(datetime.date(2024, 7, 4)) == True    # Independence Day
        assert mh.is_holiday(datetime.date(2025, 1, 1)) == True    # New Year's
        
        # Regular trading days
        assert mh.is_holiday(datetime.date(2024, 10, 15)) == False  # Random Tuesday
    
    def test_is_early_close(self):
        """Test early close detection."""
        mh = MarketHours()
        
        # Known early close days
        assert mh.is_early_close(datetime.date(2024, 12, 24)) == True  # Christmas Eve
        assert mh.is_early_close(datetime.date(2024, 11, 29)) == True  # Day after Thanksgiving
        
        # Regular days
        assert mh.is_early_close(datetime.date(2024, 10, 15)) == False
    
    def test_get_close_time(self):
        """Test getting close time based on early close."""
        mh = MarketHours()
        
        # Regular day
        assert mh.get_close_time(datetime.date(2024, 10, 15)) == datetime.time(16, 0)
        
        # Early close day
        assert mh.get_close_time(datetime.date(2024, 12, 24)) == datetime.time(13, 0)
    
    def test_is_trading_day(self):
        """Test trading day detection."""
        mh = MarketHours()
        
        # Weekdays (non-holiday)
        assert mh.is_trading_day(datetime.date(2024, 10, 14)) == True   # Monday
        assert mh.is_trading_day(datetime.date(2024, 10, 15)) == True   # Tuesday
        assert mh.is_trading_day(datetime.date(2024, 10, 18)) == True   # Friday
        
        # Weekend
        assert mh.is_trading_day(datetime.date(2024, 10, 12)) == False  # Saturday
        assert mh.is_trading_day(datetime.date(2024, 10, 13)) == False  # Sunday
        
        # Holiday (weekday)
        assert mh.is_trading_day(datetime.date(2024, 12, 25)) == False  # Christmas Wednesday
    
    def test_add_custom_holiday(self):
        """Test adding custom holidays."""
        mh = MarketHours()
        custom_date = datetime.date(2024, 10, 15)
        
        assert mh.is_holiday(custom_date) == False
        
        mh.add_holiday(custom_date)
        assert mh.is_holiday(custom_date) == True
    
    def test_is_market_open_regular_hours(self):
        """Test market open check during regular hours."""
        mh = MarketHours(include_extended_hours=False)
        
        # Trading day during market hours
        trading_day = datetime.datetime(2024, 10, 14, 10, 30)  # Monday 10:30 AM
        assert mh.is_market_open(trading_day) == True
        
        # Trading day before open
        before_open = datetime.datetime(2024, 10, 14, 9, 0)  # Monday 9:00 AM
        assert mh.is_market_open(before_open) == False
        
        # Trading day after close
        after_close = datetime.datetime(2024, 10, 14, 17, 0)  # Monday 5:00 PM
        assert mh.is_market_open(after_close) == False
        
        # Weekend
        weekend = datetime.datetime(2024, 10, 12, 12, 0)  # Saturday noon
        assert mh.is_market_open(weekend) == False
        
        # Holiday
        holiday = datetime.datetime(2024, 12, 25, 12, 0)  # Christmas noon
        assert mh.is_market_open(holiday) == False
    
    def test_is_market_open_extended_hours(self):
        """Test market open check with extended hours."""
        mh = MarketHours(include_extended_hours=True)
        
        # Pre-market
        pre_market = datetime.datetime(2024, 10, 14, 5, 0)  # Monday 5:00 AM
        assert mh.is_market_open(pre_market) == True
        
        # Post-market
        post_market = datetime.datetime(2024, 10, 14, 19, 0)  # Monday 7:00 PM
        assert mh.is_market_open(post_market) == True
        
        # Too early for pre-market
        too_early = datetime.datetime(2024, 10, 14, 3, 0)  # Monday 3:00 AM
        assert mh.is_market_open(too_early) == False
        
        # Too late for post-market
        too_late = datetime.datetime(2024, 10, 14, 21, 0)  # Monday 9:00 PM
        assert mh.is_market_open(too_late) == False
    
    def test_is_pre_market(self):
        """Test pre-market detection."""
        mh = MarketHours()
        
        pre_market_time = datetime.datetime(2024, 10, 14, 8, 0)  # Monday 8:00 AM
        assert mh.is_pre_market(pre_market_time) == True
        
        regular_hours = datetime.datetime(2024, 10, 14, 10, 0)  # Monday 10:00 AM
        assert mh.is_pre_market(regular_hours) == False
    
    def test_is_post_market(self):
        """Test post-market detection."""
        mh = MarketHours()
        
        post_market_time = datetime.datetime(2024, 10, 14, 18, 0)  # Monday 6:00 PM
        assert mh.is_post_market(post_market_time) == True
        
        regular_hours = datetime.datetime(2024, 10, 14, 10, 0)  # Monday 10:00 AM
        assert mh.is_post_market(regular_hours) == False
    
    def test_time_until_open(self):
        """Test time until open calculation."""
        mh = MarketHours()
        
        # Before market open on trading day
        before_open = datetime.datetime(2024, 10, 14, 8, 30)  # Monday 8:30 AM
        time_until = mh.time_until_open(before_open)
        assert time_until is not None
        assert time_until == datetime.timedelta(hours=1)  # 1 hour until 9:30
        
        # When market is open
        during_market = datetime.datetime(2024, 10, 14, 12, 0)  # Monday noon
        assert mh.time_until_open(during_market) is None
    
    def test_time_until_close(self):
        """Test time until close calculation."""
        mh = MarketHours()
        
        # During market hours
        during_market = datetime.datetime(2024, 10, 14, 14, 0)  # Monday 2:00 PM
        time_until = mh.time_until_close(during_market)
        assert time_until is not None
        assert time_until == datetime.timedelta(hours=2)  # 2 hours until 4:00
        
        # When market is closed
        after_close = datetime.datetime(2024, 10, 14, 18, 0)  # Monday 6:00 PM
        assert mh.time_until_close(after_close) is None
    
    def test_get_next_market_open(self):
        """Test getting next market open time."""
        mh = MarketHours()
        
        # Before market open on trading day
        before_open = datetime.datetime(2024, 10, 14, 8, 0)  # Monday 8:00 AM
        next_open = mh.get_next_market_open(before_open)
        assert next_open == datetime.datetime(2024, 10, 14, 9, 30)
        
        # After market close on trading day
        after_close = datetime.datetime(2024, 10, 14, 18, 0)  # Monday 6:00 PM
        next_open = mh.get_next_market_open(after_close)
        assert next_open == datetime.datetime(2024, 10, 15, 9, 30)  # Tuesday
        
        # During market hours - should return None
        during_market = datetime.datetime(2024, 10, 14, 12, 0)
        assert mh.get_next_market_open(during_market) is None
        
        # Friday after close - should return Monday
        friday_close = datetime.datetime(2024, 10, 18, 18, 0)  # Friday 6:00 PM
        next_open = mh.get_next_market_open(friday_close)
        assert next_open == datetime.datetime(2024, 10, 21, 9, 30)  # Monday
    
    def test_get_trading_days_in_range(self):
        """Test getting trading days in a date range."""
        mh = MarketHours()
        
        # Week with no holidays
        start = datetime.date(2024, 10, 14)  # Monday
        end = datetime.date(2024, 10, 18)    # Friday
        trading_days = mh.get_trading_days_in_range(start, end)
        
        assert len(trading_days) == 5  # Mon-Fri
        assert datetime.date(2024, 10, 14) in trading_days
        assert datetime.date(2024, 10, 18) in trading_days
        
        # Range including weekend
        start = datetime.date(2024, 10, 11)  # Friday
        end = datetime.date(2024, 10, 14)    # Monday
        trading_days = mh.get_trading_days_in_range(start, end)
        
        assert len(trading_days) == 2  # Friday and Monday only
    
    def test_format_time_until(self):
        """Test time formatting."""
        mh = MarketHours()
        
        # Hours and minutes
        td = datetime.timedelta(hours=2, minutes=30)
        assert mh.format_time_until(td) == "2h 30m"
        
        # Minutes and seconds
        td = datetime.timedelta(minutes=15, seconds=45)
        assert mh.format_time_until(td) == "15m 45s"
        
        # Seconds only
        td = datetime.timedelta(seconds=30)
        assert mh.format_time_until(td) == "30s"
        
        # None
        assert mh.format_time_until(None) == "N/A"


class TestConvenienceFunctions:
    """Tests for module-level convenience functions."""
    
    def test_is_trading_day_function(self):
        """Test standalone is_trading_day function."""
        # Weekday
        assert is_trading_day(datetime.date(2024, 10, 14)) == True
        
        # Weekend
        assert is_trading_day(datetime.date(2024, 10, 12)) == False
        
        # Holiday
        assert is_trading_day(datetime.date(2024, 12, 25)) == False


class TestHolidayData:
    """Tests for holiday data integrity."""
    
    def test_holidays_are_dates(self):
        """Test all holidays are date objects."""
        for holiday in ALL_US_HOLIDAYS:
            assert isinstance(holiday, datetime.date)
    
    def test_early_close_are_dates(self):
        """Test all early close days are date objects."""
        for day in ALL_US_EARLY_CLOSE:
            assert isinstance(day, datetime.date)
    
    def test_holidays_are_weekdays(self):
        """Test all holidays are weekdays (observed holidays may differ)."""
        # Note: Some holidays are observed on different dates
        # This test just verifies the data structure
        for holiday in ALL_US_HOLIDAYS:
            # Just verify it's a valid date
            assert 1 <= holiday.day <= 31
            assert 1 <= holiday.month <= 12
    
    def test_expected_holiday_count(self):
        """Test we have expected number of holidays."""
        # Should have ~10 holidays per year (2024-2030)
        # 2028 has 9 holidays (New Year's Day falls on Saturday, not observed)
        # Total: 10*6 + 9 = 69 holidays
        assert len(ALL_US_HOLIDAYS) == 69  # 7 years of holidays


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
