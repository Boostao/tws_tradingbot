"""
Integration Tests: Strategy Workflow

Tests the complete workflow of creating, saving, loading, validating,
and backtesting trading strategies.
"""

import pytest
import json
import tempfile
from pathlib import Path
from datetime import datetime, date, timedelta
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
from src.bot.strategy.rules.serialization import save_strategy, load_strategy
from src.bot.strategy.validator import validate_strategy, is_valid
from src.bot.backtest_runner import BacktestEngine


class TestStrategyCreationWorkflow:
    """Test creating strategies programmatically (simulating UI creation)."""
    
    def test_create_simple_strategy(self):
        """Test creating a simple EMA crossover strategy."""
        # Create indicators
        ema_fast = Indicator(
            type=IndicatorType.EMA,
            length=9,
            timeframe=TimeframeUnit.M5,
        )
        ema_slow = Indicator(
            type=IndicatorType.EMA,
            length=21,
            timeframe=TimeframeUnit.M5,
        )
        
        # Create condition
        condition = Condition(
            type=ConditionType.CROSSES_ABOVE,
            indicator_a=ema_fast,
            indicator_b=ema_slow,
        )
        
        # Create rule
        rule = Rule(
            id=str(uuid4()),
            name="EMA Crossover Buy",
            scope=RuleScope.PER_TICKER,
            action=ActionType.BUY,
            condition=condition,
            enabled=True,
        )
        
        # Create strategy
        strategy = Strategy(
            id=str(uuid4()),
            name="Simple EMA Strategy",
            version="1.0.0",
            description="Buy when EMA(9) crosses above EMA(21)",
            tickers=["SPY", "QQQ"],
            rules=[rule],
        )
        
        # Verify strategy is valid
        assert strategy.name == "Simple EMA Strategy"
        assert len(strategy.rules) == 1
        assert strategy.rules[0].action == ActionType.BUY
    
    def test_create_strategy_with_filter(self):
        """Test creating a strategy with global filter rules."""
        # Create VIX slope filter
        vix_indicator = Indicator(
            type=IndicatorType.VIX,
            timeframe=TimeframeUnit.M5,
        )
        
        vix_condition = Condition(
            type=ConditionType.SLOPE_BELOW,
            indicator_a=vix_indicator,
            threshold=0.0,
            lookback_periods=6,
        )
        
        vix_filter = Rule(
            id=str(uuid4()),
            name="VIX Slope Filter",
            scope=RuleScope.GLOBAL,
            action=ActionType.FILTER,
            condition=vix_condition,
        )
        
        # Create time filter
        time_indicator = Indicator(
            type=IndicatorType.TIME,
            timeframe=TimeframeUnit.M5,
        )
        
        time_condition = Condition(
            type=ConditionType.WITHIN_RANGE,
            indicator_a=time_indicator,
            range_start="09:30",
            range_end="15:30",
        )
        
        time_filter = Rule(
            id=str(uuid4()),
            name="Market Hours Filter",
            scope=RuleScope.GLOBAL,
            action=ActionType.FILTER,
            condition=time_condition,
        )
        
        # Create buy rule
        ema_fast = Indicator(type=IndicatorType.EMA, length=9, timeframe=TimeframeUnit.M5)
        ema_slow = Indicator(type=IndicatorType.EMA, length=21, timeframe=TimeframeUnit.M5)
        
        buy_rule = Rule(
            id=str(uuid4()),
            name="EMA Crossover",
            scope=RuleScope.PER_TICKER,
            action=ActionType.BUY,
            condition=Condition(
                type=ConditionType.CROSSES_ABOVE,
                indicator_a=ema_fast,
                indicator_b=ema_slow,
            ),
        )
        
        # Create strategy
        strategy = Strategy(
            id=str(uuid4()),
            name="Filtered EMA Strategy",
            version="1.0.0",
            description="EMA crossover with VIX and time filters",
            tickers=["SPY"],
            rules=[vix_filter, time_filter, buy_rule],
        )
        
        # Verify
        assert len(strategy.rules) == 3
        global_rules = strategy.get_global_rules()
        ticker_rules = strategy.get_ticker_rules()
        assert len(global_rules) == 2
        assert len(ticker_rules) == 1


class TestStrategySaveLoadWorkflow:
    """Test saving and loading strategies."""
    
    def test_save_and_load_strategy(self):
        """Test saving a strategy to JSON and loading it back."""
        # Create a strategy
        strategy = Strategy(
            id="test-strategy-001",
            name="Test Strategy",
            version="1.0.0",
            description="A test strategy for integration testing",
            tickers=["SPY", "QQQ", "IWM"],
            rules=[
                Rule(
                    id="rule-001",
                    name="Test Rule",
                    scope=RuleScope.PER_TICKER,
                    action=ActionType.BUY,
                    condition=Condition(
                        type=ConditionType.CROSSES_ABOVE,
                        indicator_a=Indicator(type=IndicatorType.EMA, length=9),
                        indicator_b=Indicator(type=IndicatorType.EMA, length=21),
                    ),
                )
            ],
        )
        
        # Save to temp file
        with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
            temp_path = Path(f.name)
        
        try:
            save_strategy(strategy, temp_path)
            
            # Verify file exists
            assert temp_path.exists()
            
            # Load it back
            loaded = load_strategy(temp_path)
            
            # Verify contents match
            assert loaded.id == strategy.id
            assert loaded.name == strategy.name
            assert loaded.version == strategy.version
            assert len(loaded.rules) == len(strategy.rules)
            assert loaded.tickers == strategy.tickers
            
        finally:
            # Cleanup
            temp_path.unlink(missing_ok=True)
    
    def test_load_example_strategy(self):
        """Test loading the example strategy from the strategies folder."""
        example_path = Path(__file__).parent.parent.parent / "strategies" / "example_strategy.json"
        
        if example_path.exists():
            strategy = load_strategy(example_path)
            assert strategy is not None
            assert len(strategy.rules) > 0
            assert strategy.name


class TestStrategyValidationWorkflow:
    """Test strategy validation."""
    
    def test_valid_strategy_passes(self):
        """Test that a valid strategy passes validation."""
        strategy = Strategy(
            id="valid-strategy",
            name="Valid Strategy",
            version="1.0.0",
            rules=[
                Rule(
                    id="rule-1",
                    name="Buy Rule",
                    scope=RuleScope.PER_TICKER,
                    action=ActionType.BUY,
                    condition=Condition(
                        type=ConditionType.CROSSES_ABOVE,
                        indicator_a=Indicator(type=IndicatorType.EMA, length=9),
                        indicator_b=Indicator(type=IndicatorType.EMA, length=21),
                    ),
                )
            ],
        )
        
        errors = validate_strategy(strategy)
        # Filter out non-critical warnings
        critical_errors = [e for e in errors if "warning" not in e.lower() and "consider" not in e.lower()]
        assert len(critical_errors) == 0, f"Unexpected errors: {critical_errors}"
    
    def test_empty_strategy_fails(self):
        """Test that an empty strategy fails validation."""
        strategy = Strategy(
            id="empty-strategy",
            name="Empty Strategy",
            version="1.0.0",
            rules=[],
        )
        
        errors = validate_strategy(strategy)
        assert len(errors) > 0
        assert any("at least one rule" in e.lower() for e in errors)
    
    def test_is_valid_helper(self):
        """Test the is_valid helper function."""
        valid_strategy = Strategy(
            id="test",
            name="Test",
            version="1.0",
            rules=[
                Rule(
                    id="r1",
                    name="Test Rule",
                    scope=RuleScope.PER_TICKER,
                    action=ActionType.BUY,
                    condition=Condition(
                        type=ConditionType.GREATER_THAN,
                        indicator_a=Indicator(type=IndicatorType.PRICE),
                        threshold=100.0,
                    ),
                )
            ],
        )
        
        empty_strategy = Strategy(id="empty", name="Empty", version="1.0", rules=[])
        
        assert is_valid(valid_strategy) == True
        assert is_valid(empty_strategy) == False


class TestBacktestWorkflow:
    """Test the backtest workflow."""
    
    @pytest.fixture
    def sample_strategy(self):
        """Create a sample strategy for backtesting."""
        return Strategy(
            id="backtest-strategy",
            name="Backtest Test Strategy",
            version="1.0.0",
            tickers=["SPY"],
            rules=[
                Rule(
                    id="buy-rule",
                    name="EMA Crossover Buy",
                    scope=RuleScope.PER_TICKER,
                    action=ActionType.BUY,
                    condition=Condition(
                        type=ConditionType.CROSSES_ABOVE,
                        indicator_a=Indicator(type=IndicatorType.EMA, length=9),
                        indicator_b=Indicator(type=IndicatorType.EMA, length=21),
                    ),
                ),
                Rule(
                    id="sell-rule",
                    name="EMA Crossover Sell",
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
    
    def test_backtest_engine_initialization(self, sample_strategy):
        """Test that the backtest engine initializes correctly."""
        engine = BacktestEngine(strategy=sample_strategy, initial_capital=10000)
        
        assert engine.strategy == sample_strategy
        assert engine.initial_capital == 10000
    
    def test_backtest_with_sample_data(self, sample_strategy):
        """Test running a backtest with sample data."""
        # Check if sample data exists
        sample_data_path = Path(__file__).parent.parent.parent / "data" / "sample" / "SPY_5min.csv"
        
        if not sample_data_path.exists():
            pytest.skip("Sample data not available")
        
        engine = BacktestEngine(strategy=sample_strategy, initial_capital=10000)
        
        # Run backtest
        result = engine.run(
            tickers=["SPY"],
            start_date=date(2024, 1, 2),
            end_date=date(2024, 1, 10),
            timeframe="5m",
        )
        
        # Verify result structure
        assert result is not None
        assert result.metrics is not None
        assert result.equity_curve is not None
        assert hasattr(result.metrics, 'total_return_percent')
        assert hasattr(result.metrics, 'total_trades')
    
    def test_backtest_result_metrics(self, sample_strategy):
        """Test that backtest results contain expected metrics."""
        sample_data_path = Path(__file__).parent.parent.parent / "data" / "sample" / "SPY_5min.csv"
        
        if not sample_data_path.exists():
            pytest.skip("Sample data not available")
        
        engine = BacktestEngine(strategy=sample_strategy, initial_capital=10000)
        result = engine.run(
            tickers=["SPY"],
            start_date=date(2024, 1, 2),
            end_date=date(2024, 1, 10),
            timeframe="5m",
        )
        
        # Check metrics exist and are reasonable
        assert result.metrics.total_trades >= 0
        assert -100 <= result.metrics.total_return_percent <= 1000  # Reasonable range
        assert 0 <= result.metrics.win_rate <= 100
        assert result.metrics.max_drawdown_percent >= 0


class TestCompleteWorkflow:
    """Test the complete strategy creation to backtest workflow."""
    
    def test_end_to_end_workflow(self):
        """
        Test complete workflow:
        1. Create strategy
        2. Save to file
        3. Load from file
        4. Validate
        5. Run backtest (if data available)
        """
        # 1. Create strategy
        strategy = Strategy(
            id=str(uuid4()),
            name="E2E Test Strategy",
            version="1.0.0",
            description="End-to-end test strategy",
            tickers=["SPY"],
            rules=[
                Rule(
                    id=str(uuid4()),
                    name="Test Buy",
                    scope=RuleScope.PER_TICKER,
                    action=ActionType.BUY,
                    condition=Condition(
                        type=ConditionType.CROSSES_ABOVE,
                        indicator_a=Indicator(type=IndicatorType.EMA, length=9),
                        indicator_b=Indicator(type=IndicatorType.EMA, length=21),
                    ),
                ),
            ],
        )
        
        # 2. Save to file
        with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
            temp_path = Path(f.name)
        
        try:
            save_strategy(strategy, temp_path)
            assert temp_path.exists()
            
            # 3. Load from file
            loaded_strategy = load_strategy(temp_path)
            assert loaded_strategy.name == strategy.name
            
            # 4. Validate
            errors = validate_strategy(loaded_strategy)
            critical_errors = [e for e in errors if "warning" not in e.lower() and "consider" not in e.lower()]
            assert len(critical_errors) == 0, f"Validation errors: {critical_errors}"
            
            # 5. Run backtest (if data available)
            sample_data_path = Path(__file__).parent.parent.parent / "data" / "sample" / "SPY_5min.csv"
            
            if sample_data_path.exists():
                engine = BacktestEngine(strategy=loaded_strategy, initial_capital=10000)
                result = engine.run(
                    tickers=["SPY"],
                    start_date=date(2024, 1, 2),
                    end_date=date(2024, 1, 10),
                    timeframe="5m",
                )
                assert result is not None
            
        finally:
            temp_path.unlink(missing_ok=True)
