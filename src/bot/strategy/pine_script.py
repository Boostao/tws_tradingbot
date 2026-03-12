from __future__ import annotations

from dataclasses import dataclass
from typing import List

from src.bot.strategy.rules.models import (
    Condition,
    Indicator,
    Strategy,
)


@dataclass
class PineScriptResult:
    script: str
    warnings: List[str]


def _session_expr(start: str | None, end: str | None) -> str:
    if not start or not end:
        return "false"
    start_fmt = start.replace(":", "")
    end_fmt = end.replace(":", "")
    return f'not na(time(timeframe.period, "{start_fmt}-{end_fmt}"))'


def _source_expr(source: str) -> str:
    normalized = (source or "close").lower()
    if normalized in {"close", "open", "high", "low", "volume", "hl2", "hlc3", "ohlc4"}:
        return normalized
    return "close"


def _next_pine_var(prefix: str, state: dict[str, int]) -> str:
    state["counter"] += 1
    return f"{prefix}_{state['counter']}"


def _indicator_expr(
    indicator: Indicator,
    warnings: List[str],
    rule_name: str,
    setup_lines: List[str],
    state: dict[str, int],
) -> str:
    ind_type = indicator.type.value if hasattr(indicator.type, "value") else str(indicator.type)
    source = _source_expr(indicator.source.value if hasattr(indicator.source, "value") else str(indicator.source))
    length = indicator.length or 14
    params = indicator.params or {}

    if indicator.symbol:
        warnings.append(
            f"Rule '{rule_name}': cross-symbol indicator '{indicator.symbol}' is not directly supported; using chart symbol instead."
        )

    if ind_type == "price":
        return source
    if ind_type == "ema":
        return f"ta.ema({source}, {length})"
    if ind_type == "sma":
        return f"ta.sma({source}, {length})"
    if ind_type == "rsi":
        return f"ta.rsi({source}, {length})"
    if ind_type == "volume":
        return "volume"
    if ind_type == "obv":
        return "ta.obv(close, volume)"
    if ind_type == "macd":
        fast_period = int(params.get("fast_period", 12))
        slow_period = int(params.get("slow_period", 26))
        signal_period = int(params.get("signal_period", 9))
        component = (indicator.component or "macd").lower()
        macd_line = _next_pine_var("macdLine", state)
        macd_signal = _next_pine_var("macdSignal", state)
        macd_hist = _next_pine_var("macdHist", state)
        setup_lines.append(
            f"[{macd_line}, {macd_signal}, {macd_hist}] = ta.macd({source}, {fast_period}, {slow_period}, {signal_period})"
        )
        if component == "signal":
            return macd_signal
        if component == "histogram":
            return macd_hist
        return macd_line
    if ind_type == "bollinger":
        period = int(params.get("period", indicator.length or 20))
        std_dev = float(params.get("std_dev", 2))
        component = (indicator.component or "middle").lower()
        bb_basis = _next_pine_var("bbBasis", state)
        bb_upper = _next_pine_var("bbUpper", state)
        bb_lower = _next_pine_var("bbLower", state)
        setup_lines.append(f"[{bb_basis}, {bb_upper}, {bb_lower}] = ta.bb({source}, {period}, {std_dev})")
        if component == "upper":
            return bb_upper
        if component == "lower":
            return bb_lower
        return bb_basis
    if ind_type == "stochastic":
        k_period = int(params.get("k_period", 14))
        d_period = int(params.get("d_period", 3))
        smooth_k = int(params.get("smooth_k", 3))
        component = (indicator.component or "k").lower()
        k_expr = f"ta.sma(ta.stoch(close, high, low, {k_period}), {smooth_k})"
        if component == "d":
            return f"ta.sma({k_expr}, {d_period})"
        return k_expr
    if ind_type == "time":
        return "time"

    warnings.append(f"Rule '{rule_name}': indicator type '{ind_type}' is unsupported in Pine output.")
    return "na"


def _condition_expr(
    condition: Condition,
    warnings: List[str],
    rule_name: str,
    setup_lines: List[str],
    state: dict[str, int],
) -> str:
    cond_type = condition.type.value if hasattr(condition.type, "value") else str(condition.type)
    indicator_a = _indicator_expr(condition.indicator_a, warnings, rule_name, setup_lines, state)

    if cond_type == "within_range":
        return _session_expr(condition.range_start, condition.range_end)

    indicator_b = (
        _indicator_expr(condition.indicator_b, warnings, rule_name, setup_lines, state)
        if condition.indicator_b
        else None
    )
    threshold = condition.threshold
    lookback = max(1, int(condition.lookback_periods or 1))

    if cond_type == "crosses_above":
        if not indicator_b:
            warnings.append(f"Rule '{rule_name}': crosses_above requires indicator_b.")
            return "false"
        return f"ta.crossover({indicator_a}, {indicator_b})"
    if cond_type == "crosses_below":
        if not indicator_b:
            warnings.append(f"Rule '{rule_name}': crosses_below requires indicator_b.")
            return "false"
        return f"ta.crossunder({indicator_a}, {indicator_b})"
    if cond_type == "greater_than":
        if indicator_b:
            return f"({indicator_a} > {indicator_b})"
        if threshold is not None:
            return f"({indicator_a} > {threshold})"
        return "false"
    if cond_type == "less_than":
        if indicator_b:
            return f"({indicator_a} < {indicator_b})"
        if threshold is not None:
            return f"({indicator_a} < {threshold})"
        return "false"
    if cond_type == "equals":
        if indicator_b:
            return f"({indicator_a} == {indicator_b})"
        if threshold is not None:
            return f"({indicator_a} == {threshold})"
        return "false"
    if cond_type == "slope_above":
        if threshold is None:
            return "false"
        slope_expr = f"nz(({indicator_a} - {indicator_a}[{lookback}]) / {lookback}, 0)"
        return f"({slope_expr} > {threshold})"
    if cond_type == "slope_below":
        if threshold is None:
            return "false"
        slope_expr = f"nz(({indicator_a} - {indicator_a}[{lookback}]) / {lookback}, 0)"
        return f"({slope_expr} < {threshold})"

    warnings.append(f"Rule '{rule_name}': condition type '{cond_type}' is unsupported in Pine output.")
    return "false"


def _join_and(expressions: List[str]) -> str:
    if not expressions:
        return "true"
    return " and ".join([f"({expr})" for expr in expressions])


def _join_or(expressions: List[str]) -> str:
    if not expressions:
        return "false"
    return " or ".join([f"({expr})" for expr in expressions])


def strategy_to_pine_script(strategy: Strategy) -> PineScriptResult:
    warnings: List[str] = []
    enabled_rules = [rule for rule in strategy.rules if rule.enabled]
    setup_lines: List[str] = []
    state = {"counter": 0}

    filter_conditions: List[str] = []
    buy_conditions: List[str] = []
    sell_conditions: List[str] = []

    for rule in enabled_rules:
        cond_expr = _condition_expr(rule.condition, warnings, rule.name, setup_lines, state)
        action = rule.action.value if hasattr(rule.action, "value") else str(rule.action)
        if action == "filter":
            filter_conditions.append(cond_expr)
        elif action == "buy":
            buy_conditions.append(cond_expr)
        elif action == "sell":
            sell_conditions.append(cond_expr)
        else:
            warnings.append(f"Rule '{rule.name}': action '{action}' is unsupported in Pine output.")

    filter_expr = _join_and(filter_conditions)
    buy_expr = _join_or(buy_conditions)
    sell_expr = _join_or(sell_conditions)

    strategy_name = (strategy.name or "Generated Strategy").replace('"', "'")

    script_lines = [
        "//@version=6",
        f'strategy("{strategy_name}", overlay=true, initial_capital={float(strategy.initial_capital)})',
        "",
        "// Generated from enabled rules",
    ]

    if setup_lines:
        script_lines.extend(setup_lines)
        script_lines.append("")

    script_lines.extend([
        f"filterCondition = {filter_expr}",
        f"buyCondition = {buy_expr}",
        f"sellCondition = {sell_expr}",
        "",
        "longEntry = filterCondition and buyCondition",
        "shortEntry = filterCondition and sellCondition",
        "",
        "if (longEntry)",
        '    strategy.entry("Long", strategy.long)',
        "",
        "if (shortEntry)",
        '    strategy.entry("Short", strategy.short)',
    ])

    if not enabled_rules:
        warnings.append("No enabled rules found; output script will not open trades.")

    return PineScriptResult(script="\n".join(script_lines) + "\n", warnings=warnings)
