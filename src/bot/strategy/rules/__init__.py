"""
Rule data models and related classes.

This module provides the core data structures for defining trading rules and strategies.
"""

from .models import (
    RuleScope,
    IndicatorType,
    ConditionType,
    TimeframeUnit,
    ActionType,
    Indicator,
    Condition,
    Rule,
    Strategy,
)
from .serialization import (
    save_strategy,
    load_strategy,
    validate_strategy_json,
)
from .indicators import (
    IndicatorFactory,
    create_indicator_series,
)
from .conditions import (
    ConditionEvaluator,
    evaluate_condition,
)
from .engine import (
    RuleEngine,
    create_rule_engine,
)
from .evaluator import (
    evaluate_rule_history,
    evaluate_condition_history,
    get_last_true_info,
)

__all__ = [
    # Enums
    'RuleScope',
    'IndicatorType',
    'ConditionType',
    'TimeframeUnit',
    'ActionType',
    # Models
    'Indicator',
    'Condition',
    'Rule',
    'Strategy',
    # Serialization
    'save_strategy',
    'load_strategy',
    'validate_strategy_json',
    # Indicators
    'IndicatorFactory',
    'create_indicator_series',
    # Conditions
    'ConditionEvaluator',
    'evaluate_condition',
    # Engine
    'RuleEngine',
    'create_rule_engine',
    # Evaluator (history)
    'evaluate_rule_history',
    'evaluate_condition_history',
    'get_last_true_info',
]
