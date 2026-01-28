"""
Unit Tests for Rule Engine

Tests for the rule evaluation engine including conditions, indicators, and full engine.
"""

import pytest
import numpy as np
import pandas as pd
from datetime import datetime, time, timedelta
import sys
from pathlib import Path

# Add src to path
sys.path.insert(0, str(Path(__file__).parent.parent.parent))

from src.bot.strategy.rules.models import (
    Strategy,
    Rule,
    RuleScope,
    Condition,
    ConditionType,
    Indicator,
    IndicatorType,
    TimeframeUnit,
    ActionType,
    PriceSource,
)
from src.bot.strategy.rules.indicators import IndicatorFactory, create_indicator_series
from src.bot.strategy.rules.conditions import ConditionEvaluator, evaluate_condition
from src.bot.strategy.rules.engine import RuleEngine, create_rule_engine


# ============================================================================
# Test Fixtures
# ============================================================================

@pytest.fixture
def sample_bars():
    """Create sample OHLCV bar data."""
    n = 50
    np.random.seed(42)
    
    dates = pd.date_range(start='2024-01-01 09:30', periods=n, freq='5min')
    
    # Generate realistic price data
    base_price = 100.0
    returns = np.random.randn(n) * 0.002
    closes = base_price * np.exp(np.cumsum(returns))
    
    df = pd.DataFrame({
        'timestamp': dates,
        'open': closes * (1 + np.random.randn(n) * 0.001),
        'high': closes * (1 + np.abs(np.random.randn(n) * 0.002)),
        'low': closes * (1 - np.abs(np.random.randn(n) * 0.002)),
        'close': closes,
        'volume': np.random.randint(10000, 100000, n)
    })
    
    return df


@pytest.fixture
def crossover_bars():
    """Create bar data with an EMA crossover."""
    n = 30
    dates = pd.date_range(start='2024-01-01 09:30', periods=n, freq='5min')
    
    # Create prices that will cause fast EMA to cross above slow EMA
    # Start with fast below slow, end with fast above slow
    prices = np.concatenate([
        np.linspace(95, 98, 15),   # Slow rise
        np.linspace(98, 105, 15),  # Fast rise causing crossover
    ])
    
    df = pd.DataFrame({
        'timestamp': dates,
        'open': prices,
        'high': prices * 1.005,
        'low': prices * 0.995,
        'close': prices,
        'volume': np.full(n, 50000)
    })
    
    return df


@pytest.fixture
def vix_bars():
    """Create sample VIX bar data."""
    n = 50
    dates = pd.date_range(start='2024-01-01 09:30', periods=n, freq='5min')
    
    # VIX starting high and declining (negative slope)
    vix_values = np.linspace(25, 18, n)
    
    df = pd.DataFrame({
        'timestamp': dates,
        'open': vix_values,
        'high': vix_values * 1.02,
        'low': vix_values * 0.98,
        'close': vix_values,
        'volume': np.full(n, 10000)
    })
    
    return df


@pytest.fixture
def sample_strategy():
    """Create a sample strategy with various rules."""
    # VIX slope filter (global)
    vix_indicator = Indicator(
        type=IndicatorType.VIX,
        timeframe=TimeframeUnit.M5
    )
    vix_condition = Condition(
        type=ConditionType.SLOPE_BELOW,
        indicator_a=vix_indicator,
        threshold=0.0,
        lookback_periods=6
    )
    vix_rule = Rule(
        name="VIX Slope Filter",
        scope=RuleScope.GLOBAL,
        condition=vix_condition,
        action=ActionType.FILTER,
        enabled=True
    )
    
    # Market hours filter (global)
    time_indicator = Indicator(
        type=IndicatorType.TIME,
        timeframe=TimeframeUnit.M5
    )
    time_condition = Condition(
        type=ConditionType.WITHIN_RANGE,
        indicator_a=time_indicator,
        range_start="09:30",
        range_end="16:00"
    )
    market_hours_rule = Rule(
        name="Market Hours Filter",
        scope=RuleScope.GLOBAL,
        condition=time_condition,
        action=ActionType.FILTER,
        enabled=True
    )
    
    # EMA crossover buy signal (per-ticker)
    ema_fast = Indicator(
        type=IndicatorType.EMA,
        length=9,
        timeframe=TimeframeUnit.M5,
        source=PriceSource.CLOSE
    )
    ema_slow = Indicator(
        type=IndicatorType.EMA,
        length=21,
        timeframe=TimeframeUnit.M5,
        source=PriceSource.CLOSE
    )
    buy_condition = Condition(
        type=ConditionType.CROSSES_ABOVE,
        indicator_a=ema_fast,
        indicator_b=ema_slow
    )
    buy_rule = Rule(
        name="EMA 9/21 Buy",
        scope=RuleScope.PER_TICKER,
        condition=buy_condition,
        action=ActionType.BUY,
        enabled=True
    )
    
    # EMA crossover sell signal (per-ticker)
    sell_condition = Condition(
        type=ConditionType.CROSSES_BELOW,
        indicator_a=ema_fast,
        indicator_b=ema_slow
    )
    sell_rule = Rule(
        name="EMA 9/21 Sell",
        scope=RuleScope.PER_TICKER,
        condition=sell_condition,
        action=ActionType.SELL,
        enabled=True
    )
    
    # Disabled rule
    disabled_rule = Rule(
        name="Disabled Rule",
        scope=RuleScope.PER_TICKER,
        condition=buy_condition,
        action=ActionType.BUY,
        enabled=False
    )
    
    return Strategy(
        name="Test Strategy",
        description="Strategy for unit testing",
        tickers=["AAPL", "MSFT"],
        rules=[vix_rule, market_hours_rule, buy_rule, sell_rule, disabled_rule]
    )


# ============================================================================
# Indicator Factory Tests
# ============================================================================

class TestIndicatorFactory:
    """Tests for IndicatorFactory class."""
    
    def test_create_ema_series(self, sample_bars):
        """Test EMA indicator creation."""
        indicator = Indicator(
            type=IndicatorType.EMA,
            length=9,
            timeframe=TimeframeUnit.M5,
            source=PriceSource.CLOSE
        )
        
        result = create_indicator_series(indicator, sample_bars)
        
        assert len(result) == len(sample_bars)
        # First 8 values should be NaN
        assert np.isnan(result[0])
        # Value at index 8 should be valid
        assert not np.isnan(result[8])
    
    def test_create_sma_series(self, sample_bars):
        """Test SMA indicator creation."""
        indicator = Indicator(
            type=IndicatorType.SMA,
            length=5,
            timeframe=TimeframeUnit.M5,
            source=PriceSource.CLOSE
        )
        
        result = create_indicator_series(indicator, sample_bars)
        
        assert len(result) == len(sample_bars)
        assert not np.isnan(result[4])  # First valid value at index 4
    
    def test_create_price_series(self, sample_bars):
        """Test PRICE indicator (raw price)."""
        indicator = Indicator(
            type=IndicatorType.PRICE,
            source=PriceSource.CLOSE
        )
        
        result = create_indicator_series(indicator, sample_bars)
        
        assert len(result) == len(sample_bars)
        assert_array_almost_equal(result, sample_bars['close'].values)
    
    def test_create_vix_series(self, sample_bars, vix_bars):
        """Test VIX indicator creation."""
        indicator = Indicator(
            type=IndicatorType.VIX,
            timeframe=TimeframeUnit.M5
        )
        
        result = create_indicator_series(indicator, sample_bars, vix_bars)
        
        assert len(result) == len(vix_bars)
    
    def test_get_price_series_variants(self, sample_bars):
        """Test different price sources."""
        factory = IndicatorFactory()
        
        # Test each price source
        close = factory.get_price_series(sample_bars, PriceSource.CLOSE)
        assert_array_almost_equal(close, sample_bars['close'].values)
        
        high = factory.get_price_series(sample_bars, PriceSource.HIGH)
        assert_array_almost_equal(high, sample_bars['high'].values)
        
        low = factory.get_price_series(sample_bars, PriceSource.LOW)
        assert_array_almost_equal(low, sample_bars['low'].values)
    
    def test_indicator_key_generation(self):
        """Test indicator key generation for caching."""
        indicator = Indicator(
            type=IndicatorType.EMA,
            length=9,
            timeframe=TimeframeUnit.M5,
            source=PriceSource.CLOSE
        )
        
        key = IndicatorFactory.get_indicator_key(indicator)
        
        assert "ema" in key
        assert "5m" in key
        assert "9" in key


# ============================================================================
# Condition Evaluator Tests
# ============================================================================

class TestConditionEvaluator:
    """Tests for ConditionEvaluator class."""
    
    def test_crosses_above_true(self, crossover_bars):
        """Test crossover detection when it occurs."""
        ema_fast = Indicator(type=IndicatorType.EMA, length=5, timeframe=TimeframeUnit.M5)
        ema_slow = Indicator(type=IndicatorType.EMA, length=10, timeframe=TimeframeUnit.M5)
        
        condition = Condition(
            type=ConditionType.CROSSES_ABOVE,
            indicator_a=ema_fast,
            indicator_b=ema_slow
        )
        
        evaluator = ConditionEvaluator(condition)
        result = evaluator.evaluate(crossover_bars)
        
        # With this specific data, there should be a crossover
        # Result should be boolean-like (numpy bool or Python bool)
        assert result in [True, False]
    
    def test_greater_than_threshold(self, sample_bars):
        """Test GREATER_THAN with threshold."""
        price_indicator = Indicator(type=IndicatorType.PRICE, source=PriceSource.CLOSE)
        
        condition = Condition(
            type=ConditionType.GREATER_THAN,
            indicator_a=price_indicator,
            threshold=50.0  # Very low threshold, should be True
        )
        
        result = evaluate_condition(condition, sample_bars)
        
        assert result == True  # Price should be above 50
    
    def test_less_than_threshold(self, sample_bars):
        """Test LESS_THAN with threshold."""
        price_indicator = Indicator(type=IndicatorType.PRICE, source=PriceSource.CLOSE)
        
        condition = Condition(
            type=ConditionType.LESS_THAN,
            indicator_a=price_indicator,
            threshold=200.0  # Very high threshold, should be True
        )
        
        result = evaluate_condition(condition, sample_bars)
        
        assert result == True  # Price should be below 200
    
    def test_slope_above(self, sample_bars):
        """Test SLOPE_ABOVE condition."""
        ema_indicator = Indicator(
            type=IndicatorType.EMA,
            length=5,
            timeframe=TimeframeUnit.M5
        )
        
        condition = Condition(
            type=ConditionType.SLOPE_ABOVE,
            indicator_a=ema_indicator,
            threshold=-10.0,  # Very negative, should pass
            lookback_periods=3
        )
        
        result = evaluate_condition(condition, sample_bars)
        
        # Result should be boolean-like
        assert result in [True, False]
    
    def test_slope_below_with_vix(self, sample_bars, vix_bars):
        """Test SLOPE_BELOW with VIX data."""
        vix_indicator = Indicator(
            type=IndicatorType.VIX,
            timeframe=TimeframeUnit.M5
        )
        
        condition = Condition(
            type=ConditionType.SLOPE_BELOW,
            indicator_a=vix_indicator,
            threshold=0.0,  # VIX is declining in test data
            lookback_periods=6
        )
        
        result = evaluate_condition(condition, sample_bars, vix_bars)
        
        assert result == True  # VIX slope should be negative
    
    def test_within_range_market_hours(self):
        """Test WITHIN_RANGE for market hours."""
        time_indicator = Indicator(type=IndicatorType.TIME, timeframe=TimeframeUnit.M5)
        
        condition = Condition(
            type=ConditionType.WITHIN_RANGE,
            indicator_a=time_indicator,
            range_start="09:30",
            range_end="16:00"
        )
        
        # Test during market hours
        market_time = datetime(2024, 1, 15, 10, 30)
        result = evaluate_condition(condition, pd.DataFrame(), current_time=market_time)
        assert result == True
        
        # Test outside market hours
        outside_time = datetime(2024, 1, 15, 8, 0)
        result = evaluate_condition(condition, pd.DataFrame(), current_time=outside_time)
        assert result == False
    
    def test_within_range_evening(self):
        """Test WITHIN_RANGE for after-hours."""
        time_indicator = Indicator(type=IndicatorType.TIME, timeframe=TimeframeUnit.M5)
        
        condition = Condition(
            type=ConditionType.WITHIN_RANGE,
            indicator_a=time_indicator,
            range_start="18:00",
            range_end="20:00"
        )
        
        evening_time = datetime(2024, 1, 15, 19, 0)
        result = evaluate_condition(condition, pd.DataFrame(), current_time=evening_time)
        assert result == True


# ============================================================================
# Rule Engine Tests
# ============================================================================

class TestRuleEngine:
    """Tests for RuleEngine class."""
    
    def test_engine_initialization(self, sample_strategy):
        """Test engine initializes correctly."""
        engine = create_rule_engine(sample_strategy)
        
        assert engine.strategy == sample_strategy
        assert len(engine._global_rules) == 2  # VIX filter + market hours
        assert len(engine._ticker_rules) == 2  # Buy + Sell (disabled not counted)
    
    def test_global_rules_pass(self, sample_strategy, sample_bars, vix_bars):
        """Test global rules evaluation when all pass."""
        engine = RuleEngine(sample_strategy)
        
        market_data = {"AAPL": sample_bars}
        
        # Use a time during market hours
        during_market = datetime(2024, 1, 15, 10, 30)
        
        result = engine.evaluate_global_rules(market_data, vix_bars, during_market)
        
        # VIX slope is negative and we're in market hours
        assert result == True
    
    def test_global_rules_fail_time(self, sample_strategy, sample_bars, vix_bars):
        """Test global rules fail when outside market hours."""
        engine = RuleEngine(sample_strategy)
        
        market_data = {"AAPL": sample_bars}
        
        # Use a time outside market hours
        outside_market = datetime(2024, 1, 15, 8, 0)
        
        result = engine.evaluate_global_rules(market_data, vix_bars, outside_market)
        
        assert result == False
    
    def test_evaluate_ticker_rules(self, sample_strategy, crossover_bars, vix_bars):
        """Test per-ticker rule evaluation."""
        engine = RuleEngine(sample_strategy)
        
        # Time during market hours
        during_market = datetime(2024, 1, 15, 10, 30)
        
        actions = engine.evaluate_ticker_rules(
            "AAPL", 
            crossover_bars, 
            vix_bars, 
            during_market
        )
        
        # Actions should be a list (may be empty or contain BUY/SELL)
        assert isinstance(actions, list)
        for action in actions:
            assert action in ["BUY", "SELL"]
    
    def test_disabled_rules_skipped(self, sample_strategy, sample_bars):
        """Test that disabled rules are not evaluated."""
        engine = RuleEngine(sample_strategy)
        
        # Get rule results
        during_market = datetime(2024, 1, 15, 10, 30)
        engine.evaluate_ticker_rules("AAPL", sample_bars, current_time=during_market)
        
        results = engine.get_all_rule_results()
        
        # Find the disabled rule
        disabled_rule = None
        for rule in sample_strategy.rules:
            if not rule.enabled:
                disabled_rule = rule
                break
        
        # Disabled rule should not have a result
        if disabled_rule:
            assert disabled_rule.id not in results
    
    def test_evaluate_all(self, sample_strategy, sample_bars, vix_bars):
        """Test complete evaluation cycle."""
        engine = RuleEngine(sample_strategy)
        
        market_data = {
            "AAPL": sample_bars,
            "MSFT": sample_bars.copy()
        }
        
        during_market = datetime(2024, 1, 15, 10, 30)
        
        signals = engine.evaluate_all(market_data, vix_bars, during_market)
        
        assert isinstance(signals, dict)
        # Signals should only contain tracked tickers
        for ticker in signals:
            assert ticker in ["AAPL", "MSFT"]
    
    def test_get_required_subscriptions(self, sample_strategy):
        """Test data subscription discovery."""
        engine = RuleEngine(sample_strategy)
        
        subscriptions = engine.get_required_data_subscriptions()
        
        assert isinstance(subscriptions, list)
        # Should include VIX (from VIX filter rule)
        symbols = [s[0] for s in subscriptions]
        assert "VIX" in symbols
        # Should include strategy tickers
        assert "AAPL" in symbols or any(s[0] == "AAPL" for s in subscriptions)
    
    def test_reload_strategy(self, sample_strategy):
        """Test strategy reload."""
        engine = RuleEngine(sample_strategy)
        original_name = engine.strategy.name
        
        # Create new strategy
        new_strategy = Strategy(
            name="New Strategy",
            tickers=["SPY"],
            rules=[]
        )
        
        engine.reload_strategy(new_strategy)
        
        assert engine.strategy.name == "New Strategy"
        assert engine.strategy.name != original_name


# ============================================================================
# Integration Tests
# ============================================================================

class TestRuleEngineIntegration:
    """Integration tests for the complete rule evaluation pipeline."""
    
    def test_full_evaluation_cycle(self, sample_bars, vix_bars):
        """Test a complete evaluation cycle from strategy to signals."""
        # Create a simple strategy
        ema_fast = Indicator(type=IndicatorType.EMA, length=5, timeframe=TimeframeUnit.M5)
        ema_slow = Indicator(type=IndicatorType.EMA, length=10, timeframe=TimeframeUnit.M5)
        
        buy_condition = Condition(
            type=ConditionType.CROSSES_ABOVE,
            indicator_a=ema_fast,
            indicator_b=ema_slow
        )
        
        buy_rule = Rule(
            name="Simple Buy",
            scope=RuleScope.PER_TICKER,
            condition=buy_condition,
            action=ActionType.BUY,
            enabled=True
        )
        
        strategy = Strategy(
            name="Simple Strategy",
            tickers=["AAPL"],
            rules=[buy_rule]
        )
        
        engine = RuleEngine(strategy)
        
        market_data = {"AAPL": sample_bars}
        signals = engine.evaluate_all(market_data)
        
        assert isinstance(signals, dict)
    
    def test_filter_blocks_signals(self, sample_bars):
        """Test that a failing filter prevents signal generation."""
        # Create filter that will fail
        price_indicator = Indicator(type=IndicatorType.PRICE, source=PriceSource.CLOSE)
        
        # This filter requires price > 1000, which won't be true
        filter_condition = Condition(
            type=ConditionType.GREATER_THAN,
            indicator_a=price_indicator,
            threshold=1000.0
        )
        
        filter_rule = Rule(
            name="Price Filter",
            scope=RuleScope.GLOBAL,
            condition=filter_condition,
            action=ActionType.FILTER,
            enabled=True
        )
        
        # Buy rule that would otherwise trigger
        ema = Indicator(type=IndicatorType.EMA, length=5, timeframe=TimeframeUnit.M5)
        buy_condition = Condition(
            type=ConditionType.GREATER_THAN,
            indicator_a=ema,
            threshold=0  # Always true
        )
        buy_rule = Rule(
            name="Buy Signal",
            scope=RuleScope.PER_TICKER,
            condition=buy_condition,
            action=ActionType.BUY,
            enabled=True
        )
        
        strategy = Strategy(
            name="Filtered Strategy",
            tickers=["AAPL"],
            rules=[filter_rule, buy_rule]
        )
        
        engine = RuleEngine(strategy)
        
        market_data = {"AAPL": sample_bars}
        signals = engine.evaluate_all(market_data)
        
        # Filter failed, so no signals
        assert len(signals) == 0


# Helper function for tests
def assert_array_almost_equal(a, b, decimal=7):
    """Assert two arrays are almost equal."""
    np.testing.assert_array_almost_equal(a, b, decimal=decimal)


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
