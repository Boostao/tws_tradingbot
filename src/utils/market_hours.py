"""
Market Hours Module

Handles market hours checking, holidays, and trading session management.
Supports US equity market hours with configurable pre/post market.
"""

import datetime
from typing import Optional, List, Set
from dataclasses import dataclass


# US Market Holidays for 2024-2030
# These are NYSE/NASDAQ observed holidays
US_MARKET_HOLIDAYS_2024 = {
    datetime.date(2024, 1, 1),    # New Year's Day
    datetime.date(2024, 1, 15),   # Martin Luther King Jr. Day
    datetime.date(2024, 2, 19),   # Presidents' Day
    datetime.date(2024, 3, 29),   # Good Friday
    datetime.date(2024, 5, 27),   # Memorial Day
    datetime.date(2024, 6, 19),   # Juneteenth
    datetime.date(2024, 7, 4),    # Independence Day
    datetime.date(2024, 9, 2),    # Labor Day
    datetime.date(2024, 11, 28),  # Thanksgiving Day
    datetime.date(2024, 12, 25),  # Christmas Day
}

US_MARKET_HOLIDAYS_2025 = {
    datetime.date(2025, 1, 1),    # New Year's Day
    datetime.date(2025, 1, 20),   # Martin Luther King Jr. Day
    datetime.date(2025, 2, 17),   # Presidents' Day
    datetime.date(2025, 4, 18),   # Good Friday
    datetime.date(2025, 5, 26),   # Memorial Day
    datetime.date(2025, 6, 19),   # Juneteenth
    datetime.date(2025, 7, 4),    # Independence Day
    datetime.date(2025, 9, 1),    # Labor Day
    datetime.date(2025, 11, 27),  # Thanksgiving Day
    datetime.date(2025, 12, 25),  # Christmas Day
}

US_MARKET_HOLIDAYS_2026 = {
    datetime.date(2026, 1, 1),    # New Year's Day
    datetime.date(2026, 1, 19),   # Martin Luther King Jr. Day
    datetime.date(2026, 2, 16),   # Presidents' Day
    datetime.date(2026, 4, 3),    # Good Friday
    datetime.date(2026, 5, 25),   # Memorial Day
    datetime.date(2026, 6, 19),   # Juneteenth
    datetime.date(2026, 7, 3),    # Independence Day (observed, July 4 is Saturday)
    datetime.date(2026, 9, 7),    # Labor Day
    datetime.date(2026, 11, 26),  # Thanksgiving Day
    datetime.date(2026, 12, 25),  # Christmas Day
}

US_MARKET_HOLIDAYS_2027 = {
    datetime.date(2027, 1, 1),    # New Year's Day
    datetime.date(2027, 1, 18),   # Martin Luther King Jr. Day
    datetime.date(2027, 2, 15),   # Presidents' Day
    datetime.date(2027, 3, 26),   # Good Friday
    datetime.date(2027, 5, 31),   # Memorial Day
    datetime.date(2027, 6, 18),   # Juneteenth (observed, June 19 is Saturday)
    datetime.date(2027, 7, 5),    # Independence Day (observed, July 4 is Sunday)
    datetime.date(2027, 9, 6),    # Labor Day
    datetime.date(2027, 11, 25),  # Thanksgiving Day
    datetime.date(2027, 12, 24),  # Christmas Day (observed, Dec 25 is Saturday)
}

US_MARKET_HOLIDAYS_2028 = {
    datetime.date(2028, 1, 17),   # Martin Luther King Jr. Day
    datetime.date(2028, 2, 21),   # Presidents' Day
    datetime.date(2028, 4, 14),   # Good Friday
    datetime.date(2028, 5, 29),   # Memorial Day
    datetime.date(2028, 6, 19),   # Juneteenth
    datetime.date(2028, 7, 4),    # Independence Day
    datetime.date(2028, 9, 4),    # Labor Day
    datetime.date(2028, 11, 23),  # Thanksgiving Day
    datetime.date(2028, 12, 25),  # Christmas Day
}

US_MARKET_HOLIDAYS_2029 = {
    datetime.date(2029, 1, 1),    # New Year's Day
    datetime.date(2029, 1, 15),   # Martin Luther King Jr. Day
    datetime.date(2029, 2, 19),   # Presidents' Day
    datetime.date(2029, 3, 30),   # Good Friday
    datetime.date(2029, 5, 28),   # Memorial Day
    datetime.date(2029, 6, 19),   # Juneteenth
    datetime.date(2029, 7, 4),    # Independence Day
    datetime.date(2029, 9, 3),    # Labor Day
    datetime.date(2029, 11, 22),  # Thanksgiving Day
    datetime.date(2029, 12, 25),  # Christmas Day
}

US_MARKET_HOLIDAYS_2030 = {
    datetime.date(2030, 1, 1),    # New Year's Day
    datetime.date(2030, 1, 21),   # Martin Luther King Jr. Day
    datetime.date(2030, 2, 18),   # Presidents' Day
    datetime.date(2030, 4, 19),   # Good Friday
    datetime.date(2030, 5, 27),   # Memorial Day
    datetime.date(2030, 6, 19),   # Juneteenth
    datetime.date(2030, 7, 4),    # Independence Day
    datetime.date(2030, 9, 2),    # Labor Day
    datetime.date(2030, 11, 28),  # Thanksgiving Day
    datetime.date(2030, 12, 25),  # Christmas Day
}

# Early close days (1:00 PM ET)
US_EARLY_CLOSE_2024 = {
    datetime.date(2024, 7, 3),     # Day before Independence Day
    datetime.date(2024, 11, 29),   # Day after Thanksgiving
    datetime.date(2024, 12, 24),   # Christmas Eve
}

US_EARLY_CLOSE_2025 = {
    datetime.date(2025, 7, 3),     # Day before Independence Day
    datetime.date(2025, 11, 28),   # Day after Thanksgiving
    datetime.date(2025, 12, 24),   # Christmas Eve
}

US_EARLY_CLOSE_2026 = {
    # July 3 is a holiday (Independence Day observed), no early close before
    datetime.date(2026, 11, 27),   # Day after Thanksgiving
    datetime.date(2026, 12, 24),   # Christmas Eve
}

US_EARLY_CLOSE_2027 = {
    datetime.date(2027, 7, 2),     # Day before Independence Day (observed July 5)
    datetime.date(2027, 11, 26),   # Day after Thanksgiving
    # Dec 24 is a holiday (Christmas observed), no early close
}

US_EARLY_CLOSE_2028 = {
    datetime.date(2028, 7, 3),     # Day before Independence Day
    datetime.date(2028, 11, 24),   # Day after Thanksgiving
    # Dec 24 is Sunday, no early close
}

US_EARLY_CLOSE_2029 = {
    datetime.date(2029, 7, 3),     # Day before Independence Day
    datetime.date(2029, 11, 23),   # Day after Thanksgiving
    datetime.date(2029, 12, 24),   # Christmas Eve
}

US_EARLY_CLOSE_2030 = {
    datetime.date(2030, 7, 3),     # Day before Independence Day
    datetime.date(2030, 11, 29),   # Day after Thanksgiving
    datetime.date(2030, 12, 24),   # Christmas Eve
}

# Combine all holidays
ALL_US_HOLIDAYS: Set[datetime.date] = (
    US_MARKET_HOLIDAYS_2024 | US_MARKET_HOLIDAYS_2025 | US_MARKET_HOLIDAYS_2026 |
    US_MARKET_HOLIDAYS_2027 | US_MARKET_HOLIDAYS_2028 | US_MARKET_HOLIDAYS_2029 |
    US_MARKET_HOLIDAYS_2030
)
ALL_US_EARLY_CLOSE: Set[datetime.date] = (
    US_EARLY_CLOSE_2024 | US_EARLY_CLOSE_2025 | US_EARLY_CLOSE_2026 |
    US_EARLY_CLOSE_2027 | US_EARLY_CLOSE_2028 | US_EARLY_CLOSE_2029 |
    US_EARLY_CLOSE_2030
)


@dataclass
class MarketSession:
    """Represents a market trading session."""
    open_time: datetime.time
    close_time: datetime.time
    name: str = "Regular"
    
    def is_active(self, current_time: datetime.time) -> bool:
        """Check if the session is currently active."""
        return self.open_time <= current_time <= self.close_time


class MarketHours:
    """
    Market hours management for US equity markets.
    
    Default hours (Eastern Time):
        - Pre-market: 4:00 AM - 9:30 AM
        - Regular: 9:30 AM - 4:00 PM
        - Post-market: 4:00 PM - 8:00 PM
    """
    
    def __init__(
        self,
        regular_open: str = "09:30",
        regular_close: str = "16:00",
        pre_market_open: str = "04:00",
        post_market_close: str = "20:00",
        timezone: str = "US/Eastern",
        include_extended_hours: bool = False
    ):
        """
        Initialize market hours.
        
        Args:
            regular_open: Regular session open time (HH:MM)
            regular_close: Regular session close time (HH:MM)
            pre_market_open: Pre-market open time (HH:MM)
            post_market_close: Post-market close time (HH:MM)
            timezone: Market timezone (default US/Eastern)
            include_extended_hours: Whether to include pre/post market
        """
        self.regular_open = datetime.datetime.strptime(regular_open, "%H:%M").time()
        self.regular_close = datetime.datetime.strptime(regular_close, "%H:%M").time()
        self.pre_market_open = datetime.datetime.strptime(pre_market_open, "%H:%M").time()
        self.post_market_close = datetime.datetime.strptime(post_market_close, "%H:%M").time()
        self.timezone = timezone
        self.include_extended_hours = include_extended_hours
        
        # Early close time (1:00 PM ET)
        self.early_close = datetime.time(13, 0)
        
        # Custom holidays (can be extended)
        self.custom_holidays: Set[datetime.date] = set()
    
    def add_holiday(self, date: datetime.date) -> None:
        """Add a custom holiday."""
        self.custom_holidays.add(date)
    
    def is_holiday(self, date: Optional[datetime.date] = None) -> bool:
        """Check if the given date is a market holiday."""
        if date is None:
            date = datetime.date.today()
        return date in ALL_US_HOLIDAYS or date in self.custom_holidays
    
    def is_early_close(self, date: Optional[datetime.date] = None) -> bool:
        """Check if the given date is an early close day."""
        if date is None:
            date = datetime.date.today()
        return date in ALL_US_EARLY_CLOSE
    
    def get_close_time(self, date: Optional[datetime.date] = None) -> datetime.time:
        """Get the market close time for a given date (accounts for early close)."""
        if date is None:
            date = datetime.date.today()
        
        if self.is_early_close(date):
            return self.early_close
        return self.regular_close
    
    def is_trading_day(self, date: Optional[datetime.date] = None) -> bool:
        """Check if the given date is a trading day (weekday and not holiday)."""
        if date is None:
            date = datetime.date.today()
        
        # Must be weekday (Monday=0 through Friday=4)
        if date.weekday() >= 5:
            return False
        
        # Must not be a holiday
        if self.is_holiday(date):
            return False
        
        return True
    
    def is_market_open(self, dt: Optional[datetime.datetime] = None) -> bool:
        """
        Check if the market is currently open.
        
        Args:
            dt: Datetime to check (defaults to now)
            
        Returns:
            True if market is open (regular hours, or extended if enabled)
        """
        if dt is None:
            dt = datetime.datetime.now()
        
        # Check if it's a trading day
        if not self.is_trading_day(dt.date()):
            return False
        
        current_time = dt.time()
        close_time = self.get_close_time(dt.date())
        
        if self.include_extended_hours:
            # Extended hours: pre-market open to post-market close
            return self.pre_market_open <= current_time <= self.post_market_close
        else:
            # Regular hours only
            return self.regular_open <= current_time <= close_time
    
    def is_regular_hours(self, dt: Optional[datetime.datetime] = None) -> bool:
        """Check if we're in regular trading hours (9:30 AM - 4:00 PM ET)."""
        if dt is None:
            dt = datetime.datetime.now()
        
        if not self.is_trading_day(dt.date()):
            return False
        
        current_time = dt.time()
        close_time = self.get_close_time(dt.date())
        
        return self.regular_open <= current_time <= close_time
    
    def is_pre_market(self, dt: Optional[datetime.datetime] = None) -> bool:
        """Check if we're in pre-market hours (4:00 AM - 9:30 AM ET)."""
        if dt is None:
            dt = datetime.datetime.now()
        
        if not self.is_trading_day(dt.date()):
            return False
        
        current_time = dt.time()
        return self.pre_market_open <= current_time < self.regular_open
    
    def is_post_market(self, dt: Optional[datetime.datetime] = None) -> bool:
        """Check if we're in post-market hours (4:00 PM - 8:00 PM ET)."""
        if dt is None:
            dt = datetime.datetime.now()
        
        if not self.is_trading_day(dt.date()):
            return False
        
        current_time = dt.time()
        close_time = self.get_close_time(dt.date())
        
        return close_time < current_time <= self.post_market_close
    
    def time_until_open(self, dt: Optional[datetime.datetime] = None) -> Optional[datetime.timedelta]:
        """
        Calculate time remaining until market opens.
        
        Returns:
            Timedelta until open, or None if market is already open
        """
        if dt is None:
            dt = datetime.datetime.now()
        
        if self.is_market_open(dt):
            return None
        
        next_open = self.get_next_market_open(dt)
        if next_open is None:
            return None
        
        return next_open - dt
    
    def time_until_close(self, dt: Optional[datetime.datetime] = None) -> Optional[datetime.timedelta]:
        """
        Calculate time remaining until market closes.
        
        Returns:
            Timedelta until close, or None if market is closed
        """
        if dt is None:
            dt = datetime.datetime.now()
        
        if not self.is_market_open(dt):
            return None
        
        close_time = self.get_close_time(dt.date())
        if self.include_extended_hours:
            close_time = self.post_market_close
        
        close_dt = datetime.datetime.combine(dt.date(), close_time)
        return close_dt - dt
    
    def get_next_market_open(self, dt: Optional[datetime.datetime] = None) -> Optional[datetime.datetime]:
        """
        Get the next market open datetime.
        
        Args:
            dt: Starting datetime (defaults to now)
            
        Returns:
            Datetime of next market open, or None if already open
        """
        if dt is None:
            dt = datetime.datetime.now()
        
        # If market is currently open, return None
        if self.is_market_open(dt):
            return None
        
        current_date = dt.date()
        current_time = dt.time()
        
        open_time = self.pre_market_open if self.include_extended_hours else self.regular_open
        
        # If today is a trading day and before market open
        if self.is_trading_day(current_date) and current_time < open_time:
            return datetime.datetime.combine(current_date, open_time)
        
        # Find the next trading day
        next_date = current_date + datetime.timedelta(days=1)
        for _ in range(10):  # Look up to 10 days ahead
            if self.is_trading_day(next_date):
                return datetime.datetime.combine(next_date, open_time)
            next_date += datetime.timedelta(days=1)
        
        return None
    
    def get_next_market_close(self, dt: Optional[datetime.datetime] = None) -> Optional[datetime.datetime]:
        """
        Get the next market close datetime.
        
        Args:
            dt: Starting datetime (defaults to now)
            
        Returns:
            Datetime of next market close
        """
        if dt is None:
            dt = datetime.datetime.now()
        
        current_date = dt.date()
        
        # If market is currently open
        if self.is_market_open(dt):
            close_time = self.get_close_time(current_date)
            if self.include_extended_hours:
                close_time = self.post_market_close
            return datetime.datetime.combine(current_date, close_time)
        
        # Find next trading day
        next_open = self.get_next_market_open(dt)
        if next_open:
            close_time = self.get_close_time(next_open.date())
            if self.include_extended_hours:
                close_time = self.post_market_close
            return datetime.datetime.combine(next_open.date(), close_time)
        
        return None
    
    def get_trading_days_in_range(
        self, 
        start_date: datetime.date, 
        end_date: datetime.date
    ) -> List[datetime.date]:
        """
        Get all trading days within a date range.
        
        Args:
            start_date: Start of range (inclusive)
            end_date: End of range (inclusive)
            
        Returns:
            List of trading days
        """
        trading_days = []
        current = start_date
        
        while current <= end_date:
            if self.is_trading_day(current):
                trading_days.append(current)
            current += datetime.timedelta(days=1)
        
        return trading_days
    
    def format_time_until(self, td: Optional[datetime.timedelta]) -> str:
        """Format a timedelta as a human-readable string."""
        if td is None:
            return "N/A"
        
        total_seconds = int(td.total_seconds())
        hours, remainder = divmod(total_seconds, 3600)
        minutes, seconds = divmod(remainder, 60)
        
        if hours > 0:
            return f"{hours}h {minutes}m"
        elif minutes > 0:
            return f"{minutes}m {seconds}s"
        else:
            return f"{seconds}s"


# Convenience functions
def is_market_open(include_extended: bool = False) -> bool:
    """Check if US equity market is currently open."""
    mh = MarketHours(include_extended_hours=include_extended)
    return mh.is_market_open()


def get_next_market_open() -> Optional[datetime.datetime]:
    """Get the next US equity market open time."""
    mh = MarketHours()
    return mh.get_next_market_open()


def is_trading_day(date: Optional[datetime.date] = None) -> bool:
    """Check if a date is a US equity trading day."""
    mh = MarketHours()
    return mh.is_trading_day(date)
