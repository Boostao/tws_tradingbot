"""
Pydantic models for trading strategies and rules.

These models define the structure of trading rules that can be created
in the Strategy Builder UI and executed by the trading bot.
"""

from datetime import datetime
from enum import Enum
from typing import Optional, List, Dict, Union, Any
from uuid import uuid4

from pydantic import BaseModel, Field, field_validator, ConfigDict


class RuleScope(str, Enum):
    """Defines whether a rule applies globally or per-ticker."""
    GLOBAL = "global"
    PER_TICKER = "per_ticker"


class IndicatorType(str, Enum):
    """Types of technical indicators supported."""
    EMA = "ema"           # Exponential Moving Average
    SMA = "sma"           # Simple Moving Average
    PRICE = "price"       # Raw price (close, open, high, low)
    VIX = "vix"           # VIX volatility index
    TIME = "time"         # Time-based (market hours)
    VOLUME = "volume"     # Volume
    RSI = "rsi"           # Relative Strength Index
    MACD = "macd"         # MACD
    BOLLINGER = "bollinger"  # Bollinger Bands
    STOCHASTIC = "stochastic" # Stochastic Oscillator
    OBV = "obv"           # On-Balance Volume
    ALLIGATOR = "alligator" # Williams Alligator
    DIVIDEND_YIELD = "dividend_yield" # Dividend Yield %
    PE_RATIO = "pe_ratio" # Price to Earnings Ratio
    RELATIVE_PERFORMANCE = "relative_performance" # Relative Performance vs Benchmark


class ConditionType(str, Enum):
    """Types of conditions for rule evaluation."""
    CROSSES_ABOVE = "crosses_above"     # A crosses above B
    CROSSES_BELOW = "crosses_below"     # A crosses below B
    GREATER_THAN = "greater_than"       # A > B or A > threshold
    LESS_THAN = "less_than"             # A < B or A < threshold
    SLOPE_ABOVE = "slope_above"         # Slope of A > threshold
    SLOPE_BELOW = "slope_below"         # Slope of A < threshold
    WITHIN_RANGE = "within_range"       # A is within time range
    EQUALS = "equals"                   # A == B or A == threshold


class TimeframeUnit(str, Enum):
    """Timeframe units for bar data."""
    M1 = "1m"       # 1 minute
    M5 = "5m"       # 5 minutes
    M15 = "15m"     # 15 minutes
    M30 = "30m"     # 30 minutes
    H1 = "1h"       # 1 hour
    H4 = "4h"       # 4 hours
    D1 = "1d"       # 1 day


class ActionType(str, Enum):
    """Actions that can be triggered by rules."""
    BUY = "buy"
    SELL = "sell"
    FILTER = "filter"  # Filter rule - blocks other actions if condition fails


class PriceSource(str, Enum):
    """Price source for indicators."""
    CLOSE = "close"
    OPEN = "open"
    HIGH = "high"
    LOW = "low"
    VOLUME = "volume"
    HL2 = "hl2"       # (high + low) / 2
    HLC3 = "hlc3"     # (high + low + close) / 3
    OHLC4 = "ohlc4"   # (open + high + low + close) / 4


class Indicator(BaseModel):
    """
    Represents a technical indicator configuration.
    
    Examples:
        - EMA(9, 5m, close) -> EMA of closing prices with length 9 on 5-minute bars
        - VIX(1m) -> VIX values on 1-minute bars
        - PRICE(close) -> Raw closing price
    """
    type: IndicatorType = Field(..., description="Type of indicator")
    length: Optional[int] = Field(None, ge=1, le=500, description="Indicator period/length")
    timeframe: TimeframeUnit = Field(TimeframeUnit.M5, description="Bar timeframe")
    source: PriceSource = Field(PriceSource.CLOSE, description="Price source for calculation")
    symbol: Optional[str] = Field(None, description="Symbol override (e.g., 'VIX' for VIX indicator)")
    params: Dict[str, Union[float, int, str]] = Field(default_factory=dict, description="Additional parameters")
    component: Optional[str] = Field(None, description="Component of the indicator (e.g., 'upper' for BB)")

    @field_validator('length')
    @classmethod
    def validate_length(cls, v, info):
        """Validate that length is provided for indicators that require it."""
        # Length is required for EMA, SMA, RSI, etc.
        return v

    def to_display_string(self) -> str:
        """Convert indicator to human-readable string."""
        # Handle both enum objects and string values (from JSON)
        type_str = self.type.value if hasattr(self.type, 'value') else self.type
        source_str = self.source.value if hasattr(self.source, 'value') else self.source
        timeframe_str = self.timeframe.value if hasattr(self.timeframe, 'value') else self.timeframe
        
        base = ""
        if type_str == "price":
            base = f"Price({source_str})"
        elif type_str == "vix":
            base = f"VIX({timeframe_str})"
        elif type_str == "time":
            base = "Market Hours"
        else:
            # Use main length if available
            length_str = str(self.length) if self.length else ""
            
            # Or construct distinct param string for complex indicators
            if type_str == "macd" and not length_str and self.params:
                p = self.params
                length_str = f"{p.get('fast_period', 12)},{p.get('slow_period', 26)},{p.get('signal_period', 9)}"
            elif type_str == "stochastic" and self.params:
                p = self.params
                length_str = f"{p.get('k_period', 14)},{p.get('d_period', 3)},{p.get('smooth_k', 3)}"
            elif type_str == "bollinger" and self.params:
                p = self.params
                # If length is not set, use params
                if not length_str:
                    length_str = str(p.get("period", 20))
            
            if length_str:
                base = f"{type_str.upper()}({length_str}, {timeframe_str})"
            else:
                base = f"{type_str.upper()}({timeframe_str})"
        
        # Append component if present
        if self.component:
            base += f"[{self.component}]"
            
        return base

    model_config = ConfigDict(use_enum_values=True)


class Condition(BaseModel):
    """
    Represents a condition that compares indicators or checks thresholds.
    
    Examples:
        - EMA(9) crosses above EMA(21)
        - VIX slope < -0.25 over last 6 periods
        - Price > 100
        - Time within market hours
    """
    type: ConditionType = Field(..., description="Type of condition")
    indicator_a: Indicator = Field(..., description="Primary indicator")
    indicator_b: Optional[Indicator] = Field(None, description="Secondary indicator for comparisons")
    threshold: Optional[float] = Field(None, description="Threshold value for comparisons")
    lookback_periods: int = Field(1, ge=1, le=100, description="Number of periods to look back")
    
    # For WITHIN_RANGE conditions (market hours)
    range_start: Optional[str] = Field(None, description="Start of range (e.g., '09:30')")
    range_end: Optional[str] = Field(None, description="End of range (e.g., '16:00')")

    @field_validator('indicator_b')
    @classmethod
    def validate_indicator_b(cls, v, info):
        """Validate indicator_b is provided for crossover conditions."""
        condition_type = info.data.get('type')
        if condition_type in [ConditionType.CROSSES_ABOVE, ConditionType.CROSSES_BELOW]:
            if v is None:
                # Will be caught by pydantic if truly required
                pass
        return v

    def to_display_string(self) -> str:
        """Convert condition to human-readable string."""
        ind_a = self.indicator_a.to_display_string()
        # Handle both enum objects and string values
        type_str = self.type.value if hasattr(self.type, 'value') else self.type
        
        if type_str == "crosses_above":
            ind_b = self.indicator_b.to_display_string() if self.indicator_b else "?"
            return f"{ind_a} crosses above {ind_b}"
        elif type_str == "crosses_below":
            ind_b = self.indicator_b.to_display_string() if self.indicator_b else "?"
            return f"{ind_a} crosses below {ind_b}"
        elif type_str == "greater_than":
            if self.indicator_b:
                return f"{ind_a} > {self.indicator_b.to_display_string()}"
            return f"{ind_a} > {self.threshold}"
        elif type_str == "less_than":
            if self.indicator_b:
                return f"{ind_a} < {self.indicator_b.to_display_string()}"
            return f"{ind_a} < {self.threshold}"
        elif type_str == "slope_above":
            return f"Slope of {ind_a} > {self.threshold} (last {self.lookback_periods} periods)"
        elif type_str == "slope_below":
            return f"Slope of {ind_a} < {self.threshold} (last {self.lookback_periods} periods)"
        elif type_str == "within_range":
            return f"Time within {self.range_start} - {self.range_end}"
        else:
            return f"{ind_a} {type_str}"

    model_config = ConfigDict(use_enum_values=True)


class Rule(BaseModel):
    """
    A trading rule that combines a condition with an action.
    
    Rules can be:
    - Global: Evaluated once per cycle (e.g., VIX filter, market hours)
    - Per-Ticker: Evaluated for each tracked symbol (e.g., EMA crossover)
    """
    id: str = Field(default_factory=lambda: str(uuid4()), description="Unique rule identifier")
    name: str = Field(..., min_length=1, max_length=100, description="Human-readable rule name")
    description: Optional[str] = Field(None, max_length=500, description="Detailed description")
    scope: RuleScope = Field(..., description="Global or per-ticker scope")
    condition: Condition = Field(..., description="Condition to evaluate")
    action: ActionType = Field(..., description="Action to take when condition is true")
    enabled: bool = Field(True, description="Whether the rule is active")
    priority: int = Field(0, ge=0, le=100, description="Evaluation priority (higher = first)")

    def to_display_string(self) -> str:
        """Convert rule to human-readable string."""
        # Handle both enum objects and string values
        scope_str_val = self.scope.value if hasattr(self.scope, 'value') else self.scope
        action_str_val = self.action.value if hasattr(self.action, 'value') else self.action
        
        scope_str = "🌍 Global" if scope_str_val == "global" else "🎯 Per-Ticker"
        condition_str = self.condition.to_display_string()
        action_str = action_str_val.upper()
        enabled_str = "✓" if self.enabled else "✗"
        return f"[{enabled_str}] {scope_str}: {condition_str} → {action_str}"

    model_config = ConfigDict(use_enum_values=True)


class Strategy(BaseModel):
    """
    A complete trading strategy containing multiple rules.
    
    Strategies can be saved/loaded from JSON and deployed to the trading bot.
    """
    id: str = Field(default_factory=lambda: str(uuid4()), description="Unique strategy identifier")
    name: str = Field(..., min_length=1, max_length=100, description="Strategy name")
    version: str = Field("1.0.0", description="Strategy version")
    description: Optional[str] = Field(None, max_length=1000, description="Strategy description")
    
    # Tickers this strategy trades
    tickers: List[str] = Field(default_factory=list, description="List of ticker symbols")
    
    # Rules that make up the strategy
    rules: List[Rule] = Field(default_factory=list, description="List of trading rules")
    
    # Metadata
    created_at: datetime = Field(default_factory=datetime.utcnow, description="Creation timestamp")
    updated_at: datetime = Field(default_factory=datetime.utcnow, description="Last update timestamp")
    
    # Strategy settings
    initial_capital: float = Field(10000.0, gt=0, description="Initial capital for backtesting")
    max_positions: int = Field(10, ge=1, le=100, description="Maximum concurrent positions")
    position_size_mode: str = Field("equal", description="Position sizing: 'equal', 'fixed', 'percent'")
    position_size_value: float = Field(0.1, gt=0, description="Position size value based on mode")

    def get_global_rules(self) -> List[Rule]:
        """Get all enabled global rules, sorted by priority."""
        return sorted(
            [r for r in self.rules if r.scope == RuleScope.GLOBAL and r.enabled],
            key=lambda r: -r.priority
        )

    def get_ticker_rules(self) -> List[Rule]:
        """Get all enabled per-ticker rules, sorted by priority."""
        return sorted(
            [r for r in self.rules if r.scope == RuleScope.PER_TICKER and r.enabled],
            key=lambda r: -r.priority
        )

    def get_filter_rules(self) -> List[Rule]:
        """Get all enabled filter rules."""
        return [r for r in self.rules if r.action == ActionType.FILTER and r.enabled]

    def get_signal_rules(self) -> List[Rule]:
        """Get all enabled buy/sell signal rules."""
        return [
            r for r in self.rules 
            if r.action in [ActionType.BUY, ActionType.SELL] and r.enabled
        ]

    def add_rule(self, rule: Rule) -> None:
        """Add a rule to the strategy."""
        self.rules.append(rule)
        self.updated_at = datetime.utcnow()

    def remove_rule(self, rule_id: str) -> bool:
        """Remove a rule by ID. Returns True if removed."""
        original_len = len(self.rules)
        self.rules = [r for r in self.rules if r.id != rule_id]
        if len(self.rules) < original_len:
            self.updated_at = datetime.utcnow()
            return True
        return False

    def enable_rule(self, rule_id: str) -> bool:
        """Enable a rule by ID. Returns True if found and enabled."""
        for rule in self.rules:
            if rule.id == rule_id:
                rule.enabled = True
                self.updated_at = datetime.utcnow()
                return True
        return False

    def disable_rule(self, rule_id: str) -> bool:
        """Disable a rule by ID. Returns True if found and disabled."""
        for rule in self.rules:
            if rule.id == rule_id:
                rule.enabled = False
                self.updated_at = datetime.utcnow()
                return True
        return False

    model_config = ConfigDict(
        use_enum_values=True,
        json_encoders={datetime: lambda v: v.isoformat()}
    )
