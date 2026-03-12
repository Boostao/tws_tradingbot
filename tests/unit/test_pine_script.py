from src.bot.strategy.pine_script import strategy_to_pine_script
from src.bot.strategy.rules.models import (
    ActionType,
    Condition,
    ConditionType,
    Indicator,
    IndicatorType,
    PriceSource,
    Rule,
    RuleScope,
    Strategy,
    TimeframeUnit,
)


def test_generates_pine_script_from_enabled_rules() -> None:
    buy_rule = Rule(
        name="EMA Buy",
        scope=RuleScope.PER_TICKER,
        action=ActionType.BUY,
        enabled=True,
        condition=Condition(
            type=ConditionType.CROSSES_ABOVE,
            indicator_a=Indicator(
                type=IndicatorType.EMA,
                length=9,
                timeframe=TimeframeUnit.M5,
                source=PriceSource.CLOSE,
            ),
            indicator_b=Indicator(
                type=IndicatorType.EMA,
                length=21,
                timeframe=TimeframeUnit.M5,
                source=PriceSource.CLOSE,
            ),
        ),
    )

    disabled_sell_rule = Rule(
        name="Disabled Sell",
        scope=RuleScope.PER_TICKER,
        action=ActionType.SELL,
        enabled=False,
        condition=Condition(
            type=ConditionType.LESS_THAN,
            indicator_a=Indicator(type=IndicatorType.PRICE, source=PriceSource.CLOSE),
            threshold=100.0,
        ),
    )

    strategy = Strategy(name="Pine Export", rules=[buy_rule, disabled_sell_rule])

    result = strategy_to_pine_script(strategy)

    assert "//@version=6" in result.script
    assert 'strategy("Pine Export"' in result.script
    assert "ta.crossover" in result.script
    assert "sellCondition = false" in result.script
    assert "No enabled rules found" not in "\n".join(result.warnings)


def test_returns_warning_when_no_enabled_rules() -> None:
    strategy = Strategy(name="Empty", rules=[])

    result = strategy_to_pine_script(strategy)

    assert "buyCondition = false" in result.script
    assert any("No enabled rules found" in warning for warning in result.warnings)


def test_uses_tuple_style_for_macd_and_bollinger_and_formula_for_slope() -> None:
    macd_rule = Rule(
        name="MACD Signal Buy",
        scope=RuleScope.PER_TICKER,
        action=ActionType.BUY,
        enabled=True,
        condition=Condition(
            type=ConditionType.GREATER_THAN,
            indicator_a=Indicator(
                type=IndicatorType.MACD,
                source=PriceSource.CLOSE,
                component="signal",
                params={"fast_period": 12, "slow_period": 26, "signal_period": 9},
            ),
            threshold=0.0,
        ),
    )

    bollinger_rule = Rule(
        name="Price Below Lower Band",
        scope=RuleScope.PER_TICKER,
        action=ActionType.SELL,
        enabled=True,
        condition=Condition(
            type=ConditionType.LESS_THAN,
            indicator_a=Indicator(type=IndicatorType.PRICE, source=PriceSource.CLOSE),
            indicator_b=Indicator(
                type=IndicatorType.BOLLINGER,
                source=PriceSource.CLOSE,
                component="lower",
                params={"period": 20, "std_dev": 2.0},
            ),
        ),
    )

    slope_rule = Rule(
        name="EMA Slope Filter",
        scope=RuleScope.PER_TICKER,
        action=ActionType.FILTER,
        enabled=True,
        condition=Condition(
            type=ConditionType.SLOPE_ABOVE,
            indicator_a=Indicator(type=IndicatorType.EMA, length=9, source=PriceSource.CLOSE),
            threshold=0.1,
            lookback_periods=3,
        ),
    )

    strategy = Strategy(name="Tuple + Slope", rules=[macd_rule, bollinger_rule, slope_rule])

    result = strategy_to_pine_script(strategy)

    assert "ta.slope(" not in result.script
    assert "nz((ta.ema(close, 9) - ta.ema(close, 9)[3]) / 3, 0)" in result.script

    assert "ta.macd(close, 12, 26, 9)" in result.script
    assert "[macdLine_" in result.script
    assert "macdSignal_" in result.script
    assert "(macdSignal_" in result.script
    assert "> 0.0" in result.script

    assert "ta.bb(close, 20, 2.0)" in result.script
    assert "[bbBasis_" in result.script
    assert "bbLower_" in result.script
    assert "< bbLower_" in result.script
