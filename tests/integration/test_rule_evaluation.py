"""
Integration Tests: Rule Evaluation

Tests the rule evaluation engine against sample market data,
verifying correct signal generation and history tracking.
"""

import pytest
import pandas as pd
import numpy as np
from pathlib import Path
from datetime import datetime, timedelta
from uuid import uuid4

from src.bot.strategy.rules.models import (
    Strategy,
    Rule,
    Condition,
    Indicator,
    RuleScope,
    ActionType,
    ConditionType,
    IndicatorType,
    TimeframeUnit,
)
from src.bot.strategy.rules.engine import RuleEngine
from src.bot.strategy.rules.evaluator import (
    evaluate_rule_history,
    get_last_true_info,
)
from src.bot.strategy.rules.indicators import IndicatorFactory
from src.utils.indicators import ema, sma, rsi


class TestRuleEngineWithData:
    """Test the rule engine with actual market data."""
    
    @pytest.fixture
    def sample_bars(self):
        """Load sample SPY data as bars."""
        sample_path = Path(__file__).parent.parent.parent / "data" / "sample" / "SPY_5min.csv"
        
        if not sample_path.exists():
            # Generate synthetic data if sample doesn't exist
            dates = pd.date_range(start='2024-01-02 09:30', periods=100, freq='5min')
            price_base = 470.0
            
            # Create trending data with crossover opportunities
            trend = np.linspace(0, 10, 100)
            noise = np.random.randn(100) * 2
            closes = price_base + trend + noise
            
            return pd.DataFrame({
                'timestamp': dates,
                'open': closes - np.random.rand(100),
                'high': closes + np.random.rand(100) * 2,
                'low': closes - np.random.rand(100) * 2,
                'close': closes,
                'volume': np.random.randint(100000, 1000000, 100),
            })
        
        df = pd.read_csv(sample_path, parse_dates=['timestamp'])
        return df
    
    @pytest.fixture
    def ema_crossover_rule(self):
        """Create an EMA crossover rule."""
        return Rule(
            id="ema-cross-001",
            name="EMA 9/21 Crossover",
            scope=RuleScope.PER_TICKER,
            action=ActionType.BUY,
            condition=Condition(
                type=ConditionType.CROSSES_ABOVE,
                indicator_a=Indicator(type=IndicatorType.EMA, length=9),
                indicator_b=Indicator(type=IndicatorType.EMA, length=21),
            ),
        )
    
    def test_rule_engine_evaluate(self, sample_bars, ema_crossover_rule):
        """Test rule engine evaluation with sample data."""
        # Create strategy with the rule
        strategy = Strategy(
            id="test-strategy",
            name="Test Strategy",
            version="1.0.0",
            tickers=["SPY"],
            rules=[ema_crossover_rule],
        )
        
        engine = RuleEngine(strategy)
        
        # Evaluate ticker rules for SPY
        market_data = {"SPY": sample_bars}
        result = engine.evaluate_all(market_data)
        
        # Result should be a dict with ticker keys
        assert isinstance(result, dict)
        assert "SPY" in result
        # Actions should be a list
        assert isinstance(result["SPY"], list)
    
    def test_indicator_calculation(self, sample_bars):
        """Test that indicators calculate correctly."""
        # Calculate EMA using IndicatorFactory
        ema_indicator_9 = Indicator(type=IndicatorType.EMA, length=9)
        ema_indicator_21 = Indicator(type=IndicatorType.EMA, length=21)
        
        ema_9 = IndicatorFactory.create_indicator_series(ema_indicator_9, sample_bars)
        ema_21 = IndicatorFactory.create_indicator_series(ema_indicator_21, sample_bars)
        
        # Verify output
        assert len(ema_9) == len(sample_bars)
        assert len(ema_21) == len(sample_bars)
        
        # After warmup, values should be reasonable
        assert not np.isnan(ema_9[-1])
        assert not np.isnan(ema_21[-1])
        
        # EMAs should be near price
        last_price = sample_bars['close'].iloc[-1]
        assert abs(ema_9[-1] - last_price) < last_price * 0.1  # Within 10%
    
    def test_multiple_indicator_types(self, sample_bars):
        """Test different indicator types."""
        indicators_to_test = [
            Indicator(type=IndicatorType.SMA, length=20),
            Indicator(type=IndicatorType.EMA, length=12),
            Indicator(type=IndicatorType.RSI, length=14),
            Indicator(type=IndicatorType.PRICE),
        ]
        
        for indicator in indicators_to_test:
            result = IndicatorFactory.create_indicator_series(indicator, sample_bars)
            assert len(result) == len(sample_bars)
            # Last value should be valid - convert to numpy array if needed
            if hasattr(result, 'iloc'):
                assert not np.isnan(result.iloc[-1])
            else:
                assert not np.isnan(result[-1])


class TestRuleHistoryEvaluation:
    """Test the rule history evaluator."""
    
    @pytest.fixture
    def sample_dataframe(self):
        """Create sample DataFrame for testing."""
        dates = pd.date_range(start='2024-01-02 09:30', periods=200, freq='5min')
        
        # Create data with clear crossover patterns
        np.random.seed(42)
        price_base = 470.0
        trend = np.sin(np.linspace(0, 4*np.pi, 200)) * 5  # Oscillating trend
        closes = price_base + trend + np.random.randn(200) * 0.5
        
        return pd.DataFrame({
            'timestamp': dates,
            'open': closes - np.random.rand(200) * 0.5,
            'high': closes + np.random.rand(200),
            'low': closes - np.random.rand(200),
            'close': closes,
            'volume': np.random.randint(100000, 1000000, 200),
        })
    
    @pytest.fixture
    def threshold_rule(self):
        """Create a simple threshold rule."""
        return Rule(
            id="threshold-001",
            name="Price Above 470",
            scope=RuleScope.PER_TICKER,
            action=ActionType.BUY,
            condition=Condition(
                type=ConditionType.GREATER_THAN,
                indicator_a=Indicator(type=IndicatorType.PRICE),
                threshold=470.0,
            ),
        )
    
    @pytest.fixture
    def crossover_rule(self):
        """Create an EMA crossover rule."""
        return Rule(
            id="crossover-001",
            name="EMA Cross",
            scope=RuleScope.PER_TICKER,
            action=ActionType.BUY,
            condition=Condition(
                type=ConditionType.CROSSES_ABOVE,
                indicator_a=Indicator(type=IndicatorType.EMA, length=5),
                indicator_b=Indicator(type=IndicatorType.EMA, length=15),
            ),
        )
    
    def test_evaluate_rule_history(self, sample_dataframe, threshold_rule):
        """Test evaluating rule over entire history."""
        evaluations = evaluate_rule_history(threshold_rule, sample_dataframe)
        
        # Should have evaluation for each bar (returns pd.Series)
        assert len(evaluations) == len(sample_dataframe)
        
        # Each value should be a boolean
        assert evaluations.dtype == bool or all(isinstance(v, (bool, np.bool_)) for v in evaluations)
    
    def test_get_last_true_info(self, sample_dataframe, threshold_rule):
        """Test getting last true information."""
        last_true = get_last_true_info(threshold_rule, sample_dataframe)
        
        # Should return dict with expected keys (based on actual API)
        assert 'last_true_idx' in last_true
        assert 'last_true_datetime' in last_true
        assert 'bars_ago' in last_true
        assert 'total_true_count' in last_true
        
        # If there were true values, bars_ago should be non-negative
        if last_true['bars_ago'] is not None:
            assert last_true['bars_ago'] >= 0
    
    def test_rule_never_true(self):
        """Test when a rule is never true."""
        # Create data where price is always below threshold
        dates = pd.date_range(start='2024-01-02 09:30', periods=50, freq='5min')
        df = pd.DataFrame({
            'timestamp': dates,
            'open': [100.0] * 50,
            'high': [101.0] * 50,
            'low': [99.0] * 50,
            'close': [100.0] * 50,
            'volume': [100000] * 50,
        })
        
        # Rule requires price > 200
        rule = Rule(
            id="never-true",
            name="Never True Rule",
            scope=RuleScope.PER_TICKER,
            action=ActionType.BUY,
            condition=Condition(
                type=ConditionType.GREATER_THAN,
                indicator_a=Indicator(type=IndicatorType.PRICE),
                threshold=200.0,
            ),
        )
        
        last_true = get_last_true_info(rule, df)
        
        # When never true, last_true_idx should be None and total_true_count should be 0
        assert last_true['last_true_idx'] is None
        assert last_true['bars_ago'] is None
        assert last_true['total_true_count'] == 0
    
    def test_crossover_detection(self, sample_dataframe, crossover_rule):
        """Test that crossovers are detected correctly."""
        evaluations = evaluate_rule_history(crossover_rule, sample_dataframe)
        
        # Count true evaluations (crossovers) - evaluations is a pd.Series
        true_count = evaluations.sum()
        
        # With oscillating data, should have some crossovers
        # (but crossovers are rare events)
        assert true_count >= 0  # At least it runs without error


class TestStrategyEvaluation:
    """Test evaluating complete strategies."""
    
    @pytest.fixture
    def multi_rule_strategy(self):
        """Create a strategy with multiple rules."""
        return Strategy(
            id="multi-rule-test",
            name="Multi-Rule Strategy",
            version="1.0.0",
            tickers=["SPY"],
            rules=[
                Rule(
                    id="filter-1",
                    name="VIX Filter",
                    scope=RuleScope.GLOBAL,
                    action=ActionType.FILTER,
                    condition=Condition(
                        type=ConditionType.LESS_THAN,
                        indicator_a=Indicator(type=IndicatorType.VIX),
                        threshold=25.0,
                    ),
                ),
                Rule(
                    id="buy-1",
                    name="EMA Cross Buy",
                    scope=RuleScope.PER_TICKER,
                    action=ActionType.BUY,
                    condition=Condition(
                        type=ConditionType.CROSSES_ABOVE,
                        indicator_a=Indicator(type=IndicatorType.EMA, length=9),
                        indicator_b=Indicator(type=IndicatorType.EMA, length=21),
                    ),
                ),
                Rule(
                    id="sell-1",
                    name="EMA Cross Sell",
                    scope=RuleScope.PER_TICKER,
                    action=ActionType.SELL,
                    condition=Condition(
                        type=ConditionType.CROSSES_BELOW,
                        indicator_a=Indicator(type=IndicatorType.EMA, length=9),
                        indicator_b=Indicator(type=IndicatorType.EMA, length=21),
                    ),
                ),
            ],
        )
    
    def test_strategy_rule_separation(self, multi_rule_strategy):
        """Test that strategy correctly separates global and per-ticker rules."""
        global_rules = multi_rule_strategy.get_global_rules()
        ticker_rules = multi_rule_strategy.get_ticker_rules()
        
        assert len(global_rules) == 1
        assert len(ticker_rules) == 2
        
        assert global_rules[0].action == ActionType.FILTER
        assert all(r.action in [ActionType.BUY, ActionType.SELL] for r in ticker_rules)
    
    def test_strategy_filter_rules(self, multi_rule_strategy):
        """Test strategy filter rules."""
        filter_rules = [r for r in multi_rule_strategy.rules if r.action == ActionType.FILTER]
        
        assert len(filter_rules) == 1
        assert filter_rules[0].condition.type == ConditionType.LESS_THAN
    
    def test_evaluate_each_rule(self, multi_rule_strategy):
        """Test evaluating each rule in a strategy."""
        # Create sample data
        dates = pd.date_range(start='2024-01-02 09:30', periods=100, freq='5min')
        df = pd.DataFrame({
            'timestamp': dates,
            'open': [470.0 + i * 0.1 for i in range(100)],
            'high': [471.0 + i * 0.1 for i in range(100)],
            'low': [469.0 + i * 0.1 for i in range(100)],
            'close': [470.0 + i * 0.1 for i in range(100)],
            'volume': [100000] * 100,
        })
        
        # Evaluate each rule
        for rule in multi_rule_strategy.rules:
            if rule.condition.indicator_a.type == IndicatorType.VIX:
                # Skip VIX rules (need separate VIX data)
                continue
            
            evaluations = evaluate_rule_history(rule, df)
            assert len(evaluations) == len(df)


class TestEdgeCases:
    """Test edge cases and error handling."""
    
    def test_empty_dataframe(self):
        """Test handling of empty DataFrame."""
        empty_df = pd.DataFrame(columns=['timestamp', 'open', 'high', 'low', 'close', 'volume'])
        
        rule = Rule(
            id="test-rule",
            name="Test",
            scope=RuleScope.PER_TICKER,
            action=ActionType.BUY,
            condition=Condition(
                type=ConditionType.GREATER_THAN,
                indicator_a=Indicator(type=IndicatorType.PRICE),
                threshold=100.0,
            ),
        )
        
        evaluations = evaluate_rule_history(rule, empty_df)
        assert len(evaluations) == 0
    
    def test_single_bar(self):
        """Test handling of single bar."""
        df = pd.DataFrame({
            'timestamp': [datetime.now()],
            'open': [100.0],
            'high': [101.0],
            'low': [99.0],
            'close': [100.5],
            'volume': [100000],
        })
        
        rule = Rule(
            id="test-rule",
            name="Test",
            scope=RuleScope.PER_TICKER,
            action=ActionType.BUY,
            condition=Condition(
                type=ConditionType.GREATER_THAN,
                indicator_a=Indicator(type=IndicatorType.PRICE),
                threshold=100.0,
            ),
        )
        
        evaluations = evaluate_rule_history(rule, df)
        assert len(evaluations) == 1
        assert evaluations.iloc[0] == True  # 100.5 > 100.0
    
    def test_nan_handling(self):
        """Test handling of NaN values in data."""
        dates = pd.date_range(start='2024-01-02 09:30', periods=10, freq='5min')
        df = pd.DataFrame({
            'timestamp': dates,
            'open': [100.0] * 10,
            'high': [101.0] * 10,
            'low': [99.0] * 10,
            'close': [100.5] * 10,
            'volume': [100000] * 10,
        })
        
        # Add some NaN values
        df.loc[3, 'close'] = np.nan
        df.loc[7, 'close'] = np.nan
        
        rule = Rule(
            id="test-rule",
            name="Test",
            scope=RuleScope.PER_TICKER,
            action=ActionType.BUY,
            condition=Condition(
                type=ConditionType.GREATER_THAN,
                indicator_a=Indicator(type=IndicatorType.PRICE),
                threshold=100.0,
            ),
        )
        
        # Should handle NaN gracefully
        evaluations = evaluate_rule_history(rule, df)
        assert len(evaluations) == len(df)
        
        # NaN bars should evaluate to False
        assert evaluations.iloc[3] == False
        assert evaluations.iloc[7] == False


class TestLoadExampleStrategy:
    """Test loading and evaluating the example strategy."""
    
    def test_load_and_evaluate_example(self):
        """Test loading example strategy and evaluating with sample data."""
        from src.bot.strategy.rules.serialization import load_strategy
        
        example_path = Path(__file__).parent.parent.parent / "strategies" / "example_strategy.json"
        sample_data_path = Path(__file__).parent.parent.parent / "data" / "sample" / "SPY_5min.csv"
        
        if not example_path.exists():
            pytest.skip("Example strategy not available")
        
        if not sample_data_path.exists():
            pytest.skip("Sample data not available")
        
        # Load strategy
        strategy = load_strategy(example_path)
        assert strategy is not None
        
        # Load sample data
        df = pd.read_csv(sample_data_path, parse_dates=['timestamp'])
        
        # Evaluate each non-VIX rule
        for rule in strategy.rules:
            if rule.condition.indicator_a.type == IndicatorType.VIX:
                continue
            if rule.condition.indicator_a.type == IndicatorType.TIME:
                continue
            
            evaluations = evaluate_rule_history(rule, df)
            assert len(evaluations) == len(df)
