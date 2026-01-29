"""
Strategy optimizer using Optuna.

Runs multiple backtests with different parameter values to find an
optimized strategy configuration.
"""

from __future__ import annotations

import argparse
import json
from dataclasses import dataclass
from datetime import date
from pathlib import Path
from typing import Callable, List, Optional, Tuple

import optuna

from src.bot.backtest_runner import BacktestEngine
from src.bot.strategy.rules.models import (
    Condition,
    ConditionType,
    Indicator,
    IndicatorType,
    Strategy,
)


@dataclass(frozen=True)
class ParamSpec:
    name: str
    suggest: Callable[[optuna.trial.Trial], float | int]
    apply_value: Callable[[Strategy, float | int], None]


def _threshold_range(value: float) -> Tuple[float, float]:
    if value == 0:
        return (-1.0, 1.0)
    low = value * 0.5
    high = value * 1.5
    return (min(low, high), max(low, high))


def _add_length_param(
    specs: List[ParamSpec],
    rule_index: int,
    indicator_label: str,
    indicator: Indicator,
    min_len: int,
    max_len: int,
) -> None:
    if indicator.length is None:
        return

    param_name = f"rule_{rule_index}.{indicator_label}.length"

    def _apply(strategy: Strategy, value: int) -> None:
        strategy.rules[rule_index].condition
        target = _get_indicator(strategy, rule_index, indicator_label)
        target.length = int(value)

    specs.append(
        ParamSpec(
            name=param_name,
            suggest=lambda trial: trial.suggest_int(param_name, min_len, max_len),
            apply_value=_apply,
        )
    )


def _get_indicator(strategy: Strategy, rule_index: int, label: str) -> Indicator:
    condition = strategy.rules[rule_index].condition
    if label == "a":
        return condition.indicator_a
    if label == "b":
        if condition.indicator_b is None:
            raise ValueError("Indicator B is missing")
        return condition.indicator_b
    raise ValueError(f"Unknown indicator label: {label}")


def _add_param_int(
    specs: List[ParamSpec],
    rule_index: int,
    indicator_label: str,
    param_key: str,
    min_val: int,
    max_val: int,
) -> None:
    param_name = f"rule_{rule_index}.{indicator_label}.params.{param_key}"

    def _apply(strategy: Strategy, value: int) -> None:
        target = _get_indicator(strategy, rule_index, indicator_label)
        target.params[param_key] = int(value)

    specs.append(
        ParamSpec(
            name=param_name,
            suggest=lambda trial: trial.suggest_int(param_name, min_val, max_val),
            apply_value=_apply,
        )
    )


def _add_param_float(
    specs: List[ParamSpec],
    rule_index: int,
    indicator_label: str,
    param_key: str,
    min_val: float,
    max_val: float,
    step: Optional[float] = None,
) -> None:
    param_name = f"rule_{rule_index}.{indicator_label}.params.{param_key}"

    def _apply(strategy: Strategy, value: float) -> None:
        target = _get_indicator(strategy, rule_index, indicator_label)
        target.params[param_key] = float(value)

    specs.append(
        ParamSpec(
            name=param_name,
            suggest=lambda trial: trial.suggest_float(
                param_name, min_val, max_val, step=step
            ),
            apply_value=_apply,
        )
    )


def _add_threshold_param(
    specs: List[ParamSpec],
    rule_index: int,
    condition: Condition,
) -> None:
    if condition.threshold is None:
        return

    low, high = _threshold_range(float(condition.threshold))
    param_name = f"rule_{rule_index}.condition.threshold"

    def _apply(strategy: Strategy, value: float) -> None:
        strategy.rules[rule_index].condition.threshold = float(value)

    specs.append(
        ParamSpec(
            name=param_name,
            suggest=lambda trial: trial.suggest_float(param_name, low, high),
            apply_value=_apply,
        )
    )


def _add_lookback_param(specs: List[ParamSpec], rule_index: int, condition: Condition) -> None:
    if condition.lookback_periods is None:
        return

    param_name = f"rule_{rule_index}.condition.lookback_periods"

    def _apply(strategy: Strategy, value: int) -> None:
        strategy.rules[rule_index].condition.lookback_periods = int(value)

    specs.append(
        ParamSpec(
            name=param_name,
            suggest=lambda trial: trial.suggest_int(param_name, 1, 50),
            apply_value=_apply,
        )
    )


def build_default_search_space(strategy: Strategy) -> List[ParamSpec]:
    specs: List[ParamSpec] = []

    for rule_index, rule in enumerate(strategy.rules):
        condition = rule.condition

        # Threshold and lookback
        if condition.type in {
            ConditionType.GREATER_THAN,
            ConditionType.LESS_THAN,
            ConditionType.SLOPE_ABOVE,
            ConditionType.SLOPE_BELOW,
            ConditionType.EQUALS,
        }:
            _add_threshold_param(specs, rule_index, condition)

        if condition.type in {ConditionType.SLOPE_ABOVE, ConditionType.SLOPE_BELOW}:
            _add_lookback_param(specs, rule_index, condition)

        for label, indicator in (("a", condition.indicator_a), ("b", condition.indicator_b)):
            if indicator is None:
                continue

            indicator_type = indicator.type

            if indicator_type in {IndicatorType.EMA, IndicatorType.SMA}:
                _add_length_param(specs, rule_index, label, indicator, 5, 200)
            elif indicator_type == IndicatorType.RSI:
                _add_length_param(specs, rule_index, label, indicator, 5, 50)
            elif indicator_type == IndicatorType.BOLLINGER:
                _add_length_param(specs, rule_index, label, indicator, 10, 50)
                _add_param_float(specs, rule_index, label, "std_dev", 1.0, 3.5, step=0.1)
            elif indicator_type == IndicatorType.MACD:
                _add_param_int(specs, rule_index, label, "fast_period", 5, 20)
                _add_param_int(specs, rule_index, label, "slow_period", 20, 50)
                _add_param_int(specs, rule_index, label, "signal_period", 5, 20)
            elif indicator_type == IndicatorType.STOCHASTIC:
                _add_param_int(specs, rule_index, label, "k_period", 5, 20)
                _add_param_int(specs, rule_index, label, "d_period", 3, 10)
                _add_param_int(specs, rule_index, label, "smooth_k", 1, 5)
            elif indicator_type == IndicatorType.ALLIGATOR:
                _add_param_int(specs, rule_index, label, "jaw_period", 5, 20)
                _add_param_int(specs, rule_index, label, "teeth_period", 5, 20)
                _add_param_int(specs, rule_index, label, "lips_period", 3, 15)

    return specs


def _apply_params(strategy: Strategy, specs: List[ParamSpec], trial: optuna.trial.Trial) -> None:
    for spec in specs:
        value = spec.suggest(trial)
        spec.apply_value(strategy, value)


def _apply_param_values(strategy: Strategy, specs: List[ParamSpec], values: dict) -> None:
    for spec in specs:
        if spec.name in values:
            spec.apply_value(strategy, values[spec.name])


def _parse_date(value: str) -> date:
    return date.fromisoformat(value)


def optimize_strategy(
    strategy_path: Path,
    tickers: List[str],
    start_date: date,
    end_date: date,
    timeframe: str,
    trials: int,
    use_tws_data: bool,
    use_nautilus: bool,
    output_path: Path,
) -> None:
    strategy = Strategy.model_validate_json(strategy_path.read_text())

    specs = build_default_search_space(strategy)
    if not specs:
        raise ValueError("No tunable parameters found in the strategy.")

    def objective(trial: optuna.trial.Trial) -> float:
        candidate = strategy.model_copy(deep=True)
        _apply_params(candidate, specs, trial)

        engine = BacktestEngine(
            strategy=candidate,
            initial_capital=candidate.initial_capital,
            use_tws_data=use_tws_data,
            use_nautilus=use_nautilus,
        )
        result = engine.run(
            tickers=tickers,
            start_date=start_date,
            end_date=end_date,
            timeframe=timeframe,
        )

        score = float(result.metrics.total_return_percent)
        if result.metrics.total_trades == 0:
            score -= 1000.0
        trial.set_user_attr("trades", result.metrics.total_trades)
        trial.set_user_attr("max_drawdown", result.metrics.max_drawdown_percent)
        return score

    study = optuna.create_study(direction="maximize")
    study.optimize(objective, n_trials=trials)

    best_params = study.best_trial.params
    optimized = strategy.model_copy(deep=True)
    _apply_param_values(optimized, specs, best_params)
    output_path.write_text(optimized.model_dump_json(indent=2))

    summary = {
        "best_score": study.best_value,
        "best_params": best_params,
        "output": str(output_path),
    }
    print(json.dumps(summary, indent=2))


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Optimize a strategy using Optuna")
    parser.add_argument("--strategy", required=True, type=Path, help="Path to strategy JSON")
    parser.add_argument("--tickers", required=True, nargs="+", help="Tickers to backtest")
    parser.add_argument("--start-date", required=True, type=_parse_date, help="YYYY-MM-DD")
    parser.add_argument("--end-date", required=True, type=_parse_date, help="YYYY-MM-DD")
    parser.add_argument("--timeframe", default="5m", help="Bar timeframe")
    parser.add_argument("--trials", type=int, default=50, help="Optuna trials")
    parser.add_argument("--use-tws-data", action="store_true", help="Use real TWS data")
    parser.add_argument("--use-nautilus", action="store_true", help="Use Nautilus backtest")
    parser.add_argument("--output", required=True, type=Path, help="Output optimized strategy JSON")
    return parser


def main() -> None:
    parser = build_parser()
    args = parser.parse_args()

    optimize_strategy(
        strategy_path=args.strategy,
        tickers=args.tickers,
        start_date=args.start_date,
        end_date=args.end_date,
        timeframe=args.timeframe,
        trials=args.trials,
        use_tws_data=args.use_tws_data,
        use_nautilus=args.use_nautilus,
        output_path=args.output,
    )


if __name__ == "__main__":
    main()