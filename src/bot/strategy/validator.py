"""
Strategy Validation Module

Validates trading strategies to ensure they are well-formed and can be
safely deployed to the trading bot.

Validation checks:
- At least one rule exists
- All rules have valid indicator configurations
- No duplicate rule IDs
- Required fields are present
- Timeframes are consistent where needed
"""

import logging
from typing import List, Set, Tuple, Optional
from collections import Counter

from src.bot.strategy.rules.models import (
    Strategy,
    Rule,
    Condition,
    Indicator,
    RuleScope,
    IndicatorType,
    ConditionType,
    ActionType,
    TimeframeUnit,
)


logger = logging.getLogger(__name__)


class ValidationError:
    """Represents a validation error with severity and details."""
    
    def __init__(
        self,
        message: str,
        severity: str = "error",
        rule_id: Optional[str] = None,
        rule_name: Optional[str] = None,
    ):
        self.message = message
        self.severity = severity  # "error", "warning", "info"
        self.rule_id = rule_id
        self.rule_name = rule_name
    
    def __str__(self) -> str:
        prefix = ""
        if self.rule_name:
            prefix = f"[Rule: {self.rule_name}] "
        return f"{prefix}{self.message}"
    
    def __repr__(self) -> str:
        return f"ValidationError({self.severity}: {self.message})"


def validate_strategy(strategy: Strategy) -> List[str]:
    """
    Validate a trading strategy for deployment.
    
    Returns a list of error messages. Empty list means strategy is valid.
    
    Args:
        strategy: The Strategy object to validate
        
    Returns:
        List of error message strings (empty if valid)
    """
    errors: List[str] = []
    
    # 1. Check strategy has required basic fields
    if not strategy.name or not strategy.name.strip():
        errors.append("Strategy name is required")
    
    if not strategy.id:
        errors.append("Strategy ID is required")
    
    # 2. Check at least one rule exists
    if not strategy.rules:
        errors.append("Strategy must have at least one rule")
        return errors  # Can't continue without rules
    
    # 3. Check for duplicate rule IDs
    rule_ids = [r.id for r in strategy.rules]
    duplicates = [id for id, count in Counter(rule_ids).items() if count > 1]
    if duplicates:
        errors.append(f"Duplicate rule IDs found: {', '.join(duplicates)}")
    
    # 4. Validate each rule
    for rule in strategy.rules:
        rule_errors = validate_rule(rule)
        errors.extend(rule_errors)
    
    # 5. Check strategy-level consistency
    consistency_errors = check_strategy_consistency(strategy)
    errors.extend(consistency_errors)
    
    # 6. Validate tickers
    if strategy.tickers:
        ticker_errors = validate_tickers(strategy.tickers)
        errors.extend(ticker_errors)
    else:
        # Warning: no tickers defined
        pass  # This is okay - can be set later
    
    # 7. Validate position settings
    if strategy.initial_capital <= 0:
        errors.append("Initial capital must be positive")
    
    if strategy.max_positions < 1:
        errors.append("Max positions must be at least 1")
    
    if strategy.position_size_mode not in ("equal", "fixed", "percent"):
        errors.append(f"Invalid position size mode: {strategy.position_size_mode}")
    
    return errors


def validate_rule(rule: Rule) -> List[str]:
    """
    Validate a single trading rule.
    
    Args:
        rule: The Rule object to validate
        
    Returns:
        List of error messages for this rule
    """
    errors: List[str] = []
    prefix = f"[Rule: {rule.name}] "
    
    # 1. Check required fields
    if not rule.name or not rule.name.strip():
        errors.append(f"{prefix}Rule name is required")
    
    if not rule.id:
        errors.append(f"{prefix}Rule ID is required")
    
    # 2. Validate scope
    try:
        if isinstance(rule.scope, str):
            RuleScope(rule.scope)
        elif not isinstance(rule.scope, RuleScope):
            errors.append(f"{prefix}Invalid scope: {rule.scope}")
    except ValueError:
        errors.append(f"{prefix}Invalid scope value: {rule.scope}")
    
    # 3. Validate action
    try:
        if isinstance(rule.action, str):
            ActionType(rule.action)
        elif not isinstance(rule.action, ActionType):
            errors.append(f"{prefix}Invalid action: {rule.action}")
    except ValueError:
        errors.append(f"{prefix}Invalid action value: {rule.action}")
    
    # 4. Validate condition
    condition_errors = validate_condition(rule.condition, prefix)
    errors.extend(condition_errors)
    
    # 5. Check scope-action consistency
    scope_val = rule.scope.value if hasattr(rule.scope, 'value') else rule.scope
    action_val = rule.action.value if hasattr(rule.action, 'value') else rule.action
    
    if scope_val == "global" and action_val in ("buy", "sell"):
        errors.append(f"{prefix}Global rules should use FILTER action, not BUY/SELL")
    
    return errors


def validate_condition(condition: Condition, prefix: str = "") -> List[str]:
    """
    Validate a condition.
    
    Args:
        condition: The Condition object to validate
        prefix: Prefix for error messages
        
    Returns:
        List of error messages
    """
    errors: List[str] = []
    
    # 1. Validate condition type
    try:
        cond_type = condition.type.value if hasattr(condition.type, 'value') else condition.type
        ConditionType(cond_type)
    except ValueError:
        errors.append(f"{prefix}Invalid condition type: {condition.type}")
        return errors  # Can't continue with invalid type
    
    cond_type = condition.type.value if hasattr(condition.type, 'value') else condition.type
    
    # 2. Validate indicator_a (always required)
    if not condition.indicator_a:
        errors.append(f"{prefix}Primary indicator (indicator_a) is required")
    else:
        ind_errors = validate_indicator(condition.indicator_a, prefix + "Indicator A: ")
        errors.extend(ind_errors)
    
    # 3. Check indicator_b for crossover conditions
    if cond_type in ("crosses_above", "crosses_below"):
        if not condition.indicator_b:
            errors.append(f"{prefix}Crossover conditions require indicator_b")
        else:
            ind_errors = validate_indicator(condition.indicator_b, prefix + "Indicator B: ")
            errors.extend(ind_errors)
    
    # 4. Check threshold for comparison conditions
    if cond_type in ("greater_than", "less_than", "slope_above", "slope_below"):
        if condition.indicator_b is None and condition.threshold is None:
            errors.append(f"{prefix}Comparison conditions require either indicator_b or threshold")
    
    # 5. Check range conditions
    if cond_type == "within_range":
        if not condition.range_start or not condition.range_end:
            errors.append(f"{prefix}Range conditions require range_start and range_end")
        else:
            # Validate time format
            import re
            time_pattern = r"^\d{1,2}:\d{2}$"
            if not re.match(time_pattern, condition.range_start):
                errors.append(f"{prefix}Invalid range_start format: {condition.range_start} (expected HH:MM)")
            if not re.match(time_pattern, condition.range_end):
                errors.append(f"{prefix}Invalid range_end format: {condition.range_end} (expected HH:MM)")
    
    # 6. Validate lookback periods
    if condition.lookback_periods < 1:
        errors.append(f"{prefix}Lookback periods must be at least 1")
    
    return errors


def validate_indicator(indicator: Indicator, prefix: str = "") -> List[str]:
    """
    Validate an indicator configuration.
    
    Args:
        indicator: The Indicator object to validate
        prefix: Prefix for error messages
        
    Returns:
        List of error messages
    """
    errors: List[str] = []
    
    # 1. Validate indicator type
    try:
        ind_type = indicator.type.value if hasattr(indicator.type, 'value') else indicator.type
        IndicatorType(ind_type)
    except ValueError:
        errors.append(f"{prefix}Invalid indicator type: {indicator.type}")
        return errors
    
    ind_type = indicator.type.value if hasattr(indicator.type, 'value') else indicator.type
    
    # 2. Check length requirement for certain indicators
    # Note: MACD and Bollinger now use params primarily, but simple ones still need length
    indicators_requiring_length = ["ema", "sma", "rsi"]
    if ind_type in indicators_requiring_length:
        if indicator.length is None or indicator.length < 1:
            errors.append(f"{prefix}{ind_type.upper()} requires a length >= 1")

    # 3. Check specific requirements for complex indicators
    if ind_type == "macd":
        # MACD needs either length (legacy) or params with periods
        has_params = indicator.params and "fast_period" in indicator.params and "slow_period" in indicator.params
        if not (indicator.length or has_params):
             errors.append(f"{prefix}MACD requires either 'length' or valid parameters (fast_period, slow_period)")
             
    if ind_type == "bollinger":
        # Bollinger needs length or period in params
        has_period = indicator.length or (indicator.params and "period" in indicator.params)
        if not has_period:
            errors.append(f"{prefix}Bollinger Bands requires 'length' or 'period' in params")

    # 4. Validate timeframe
    try:
        tf = indicator.timeframe.value if hasattr(indicator.timeframe, 'value') else indicator.timeframe
        TimeframeUnit(tf)
    except ValueError:
        errors.append(f"{prefix}Invalid timeframe: {indicator.timeframe}")
    
    # 4. Validate length bounds
    if indicator.length is not None:
        if indicator.length < 1:
            errors.append(f"{prefix}Length must be >= 1")
        if indicator.length > 500:
            errors.append(f"{prefix}Length must be <= 500")
    
    return errors


def validate_tickers(tickers: List[str]) -> List[str]:
    """
    Validate a list of ticker symbols.
    
    Args:
        tickers: List of ticker symbols
        
    Returns:
        List of error messages
    """
    errors: List[str] = []
    
    import re
    # Basic ticker pattern (alphanumeric, 1-10 chars, may have exchange suffix)
    ticker_pattern = r"^[A-Za-z0-9\.\-]{1,20}$"
    
    for ticker in tickers:
        if not ticker or not ticker.strip():
            errors.append("Empty ticker symbol found")
        elif not re.match(ticker_pattern, ticker):
            errors.append(f"Invalid ticker format: {ticker}")
    
    # Check for duplicates
    seen = set()
    for ticker in tickers:
        if ticker.upper() in seen:
            errors.append(f"Duplicate ticker: {ticker}")
        seen.add(ticker.upper())
    
    return errors


def check_strategy_consistency(strategy: Strategy) -> List[str]:
    """
    Check for consistency issues across the strategy.
    
    Args:
        strategy: The Strategy to check
        
    Returns:
        List of consistency error messages
    """
    errors: List[str] = []
    
    # 1. Check for at least one non-filter rule if there are filter rules
    filter_rules = [r for r in strategy.rules if _get_action(r) == "filter"]
    signal_rules = [r for r in strategy.rules if _get_action(r) in ("buy", "sell")]
    
    if filter_rules and not signal_rules:
        errors.append("Strategy has filter rules but no BUY/SELL rules - no trades will be generated")
    
    # 2. Check for at least one buy rule
    buy_rules = [r for r in strategy.rules if _get_action(r) == "buy"]
    sell_rules = [r for r in strategy.rules if _get_action(r) == "sell"]
    
    if not buy_rules:
        errors.append("Strategy has no BUY rules - consider adding entry signals")
    
    # 3. Warn if no sell rules (might be intentional for buy-and-hold)
    # This is just a warning, not an error
    
    # 4. Check timeframe consistency within crossover rules
    for rule in strategy.rules:
        cond = rule.condition
        cond_type = cond.type.value if hasattr(cond.type, 'value') else cond.type
        
        if cond_type in ("crosses_above", "crosses_below"):
            if cond.indicator_a and cond.indicator_b:
                tf_a = cond.indicator_a.timeframe
                tf_b = cond.indicator_b.timeframe
                tf_a_val = tf_a.value if hasattr(tf_a, 'value') else tf_a
                tf_b_val = tf_b.value if hasattr(tf_b, 'value') else tf_b
                
                if tf_a_val != tf_b_val:
                    errors.append(
                        f"[Rule: {rule.name}] Crossover indicators have different timeframes "
                        f"({tf_a_val} vs {tf_b_val}) - this may cause unexpected behavior"
                    )
    
    return errors


def _get_action(rule: Rule) -> str:
    """Helper to get action value as string."""
    return rule.action.value if hasattr(rule.action, 'value') else rule.action


def get_validation_summary(errors: List[str]) -> Tuple[int, int, int]:
    """
    Get a summary count of errors, warnings, and info messages.
    
    Args:
        errors: List of error messages
        
    Returns:
        Tuple of (error_count, warning_count, info_count)
    """
    error_count = len([e for e in errors if "warning" not in e.lower()])
    warning_count = len([e for e in errors if "warning" in e.lower()])
    info_count = len([e for e in errors if "info" in e.lower()])
    
    return error_count, warning_count, info_count


def is_valid(strategy: Strategy) -> bool:
    """
    Quick check if a strategy is valid.
    
    Args:
        strategy: The Strategy to validate
        
    Returns:
        True if valid, False otherwise
    """
    errors = validate_strategy(strategy)
    # Filter out warnings for validity check
    critical_errors = [e for e in errors if "warning" not in e.lower() and "consider" not in e.lower()]
    return len(critical_errors) == 0
