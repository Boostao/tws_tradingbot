"""
Rule Engine Module

The core evaluation engine that processes trading rules against market data.
Handles both global (filter) rules and per-ticker signal rules.
"""

import logging
from datetime import datetime
from typing import Dict, List, Tuple, Optional, Set
import pandas as pd

from src.bot.strategy.rules.models import (
    Strategy,
    Rule,
    RuleScope,
    ActionType,
    TimeframeUnit,
    IndicatorType,
)
from src.bot.strategy.rules.conditions import ConditionEvaluator


logger = logging.getLogger(__name__)


class RuleEngine:
    """
    Main rule evaluation engine for the trading bot.
    
    Processes a Strategy's rules against market data to generate trading signals.
    
    Evaluation flow:
    1. Global rules are evaluated first (filters)
    2. If any global filter fails, no signals are generated
    3. Per-ticker rules are evaluated for each symbol
    4. Actions (BUY/SELL) are collected and returned
    """
    
    def __init__(self, strategy: Strategy):
        """
        Initialize the rule engine with a strategy.
        
        Args:
            strategy: The Strategy containing rules to evaluate
        """
        self.strategy = strategy
        self._global_rules = strategy.get_global_rules()
        self._ticker_rules = strategy.get_ticker_rules()
        self._last_evaluation_time: Optional[datetime] = None
        self._rule_results: Dict[str, bool] = {}  # Cache of last rule evaluation results
    
    def reload_strategy(self, strategy: Strategy) -> None:
        """
        Reload the engine with a new strategy.
        
        Args:
            strategy: New strategy to use
        """
        self.strategy = strategy
        self._global_rules = strategy.get_global_rules()
        self._ticker_rules = strategy.get_ticker_rules()
        self._rule_results.clear()
        logger.info(f"Strategy reloaded: {strategy.name} with {len(strategy.rules)} rules")
    
    def evaluate_global_rules(
        self,
        market_data: Dict[str, pd.DataFrame],
        vix_bars: Optional[pd.DataFrame] = None,
        current_time: Optional[datetime] = None
    ) -> bool:
        """
        Evaluate all global rules (filters).
        
        Global rules act as gates - ALL must pass for trading to proceed.
        
        Args:
            market_data: Dict mapping symbols to their bar DataFrames
            vix_bars: Optional VIX bar data for VIX-based rules
            current_time: Current time for time-based rules
            
        Returns:
            True if ALL global rules pass, False if any fail
        """
        if current_time is None:
            current_time = datetime.now()
        
        self._last_evaluation_time = current_time
        
        for rule in self._global_rules:
            if not rule.enabled:
                continue
            
            try:
                # Use first available bar data for global rules
                # Global rules typically use VIX or time-based conditions
                bars = self._get_bars_for_rule(rule, market_data)
                
                evaluator = ConditionEvaluator(rule.condition)
                result = evaluator.evaluate(bars, vix_bars, current_time, market_data)
                
                self._rule_results[rule.id] = result
                
                # For filter rules, condition must be True to pass
                # For action rules at global scope, we just track the result
                action_type = rule.action.value if hasattr(rule.action, 'value') else rule.action
                
                if action_type == "filter" and not result:
                    logger.debug(f"Global rule '{rule.name}' failed (filter)")
                    return False
                
                logger.debug(f"Global rule '{rule.name}' evaluated: {result}")
                
            except Exception as e:
                logger.error(f"Error evaluating global rule '{rule.name}': {e}")
                self._rule_results[rule.id] = False
                # On error, fail safe (treat as filter failure)
                return False
        
        return True
    
    def evaluate_ticker_rules(
        self,
        ticker: str,
        bars: pd.DataFrame,
        vix_bars: Optional[pd.DataFrame] = None,
        current_time: Optional[datetime] = None,
        market_data: Optional[Dict[str, pd.DataFrame]] = None,
    ) -> List[str]:
        """
        Evaluate per-ticker rules for a specific symbol.
        
        Args:
            ticker: The ticker symbol being evaluated
            bars: Bar data for this ticker
            vix_bars: Optional VIX bar data
            current_time: Current time for time-based rules
            
        Returns:
            List of action strings triggered ("BUY", "SELL")
        """
        if current_time is None:
            current_time = datetime.now()
        
        actions: List[str] = []
        
        for rule in self._ticker_rules:
            if not rule.enabled:
                continue
            
            try:
                evaluator = ConditionEvaluator(rule.condition)
                result = evaluator.evaluate(bars, vix_bars, current_time, market_data or {ticker: bars})
                
                self._rule_results[rule.id] = result
                
                if result:
                    action_type = rule.action.value if hasattr(rule.action, 'value') else rule.action
                    
                    if action_type in ["buy", "sell"]:
                        actions.append(action_type.upper())
                        logger.info(f"Rule '{rule.name}' triggered {action_type.upper()} for {ticker}")
                    elif action_type == "filter":
                        # Per-ticker filter - if it fails, skip remaining rules for this ticker
                        logger.debug(f"Per-ticker filter '{rule.name}' passed for {ticker}")
                else:
                    action_type = rule.action.value if hasattr(rule.action, 'value') else rule.action
                    if action_type == "filter":
                        logger.debug(f"Per-ticker filter '{rule.name}' failed for {ticker}")
                        return []
                
                logger.debug(f"Ticker rule '{rule.name}' for {ticker}: {result}")
                
            except Exception as e:
                logger.error(f"Error evaluating rule '{rule.name}' for {ticker}: {e}")
                self._rule_results[rule.id] = False
        
        return actions
    
    def evaluate_all(
        self,
        market_data: Dict[str, pd.DataFrame],
        vix_bars: Optional[pd.DataFrame] = None,
        current_time: Optional[datetime] = None
    ) -> Dict[str, List[str]]:
        """
        Evaluate all rules and return signals per ticker.
        
        This is the main entry point for a complete evaluation cycle.
        
        Args:
            market_data: Dict mapping symbols to their bar DataFrames
            vix_bars: Optional VIX bar data
            current_time: Current time
            
        Returns:
            Dict mapping ticker symbols to list of actions (e.g., {"AAPL": ["BUY"]})
        """
        if current_time is None:
            current_time = datetime.now()
        
        # First evaluate global rules
        if not self.evaluate_global_rules(market_data, vix_bars, current_time):
            logger.debug("Global rules failed, no signals generated")
            return {}
        
        # Then evaluate per-ticker rules for each ticker in the strategy
        signals: Dict[str, List[str]] = {}
        
        for ticker in self.strategy.tickers:
            if ticker not in market_data:
                logger.warning(f"No data available for ticker {ticker}")
                continue
            
            bars = market_data[ticker]
            actions = self.evaluate_ticker_rules(ticker, bars, vix_bars, current_time, market_data)
            
            if actions:
                signals[ticker] = actions
        
        return signals
    
    def get_required_data_subscriptions(self) -> List[Tuple[str, TimeframeUnit]]:
        """
        Get list of data subscriptions required for this strategy.
        
        Returns:
            List of (symbol, timeframe) tuples needed for rule evaluation
        """
        subscriptions: Set[Tuple[str, TimeframeUnit]] = set()
        
        # Add subscriptions for strategy tickers
        for ticker in self.strategy.tickers:
            # Find the timeframes used in ticker rules
            for rule in self._ticker_rules:
                timeframe = self._get_rule_timeframe(rule)
                subscriptions.add((ticker, timeframe))
        
        # Add VIX subscription if any rule uses VIX
        for rule in self.strategy.rules:
            if self._rule_uses_vix(rule):
                timeframe = self._get_rule_timeframe(rule)
                subscriptions.add(("VIX", timeframe))
        
        # Add any explicit symbols from indicators
        for rule in self.strategy.rules:
            indicator_a = rule.condition.indicator_a
            if indicator_a.symbol:
                timeframe = indicator_a.timeframe
                subscriptions.add((indicator_a.symbol, timeframe))
            
            indicator_b = rule.condition.indicator_b
            if indicator_b and indicator_b.symbol:
                timeframe = indicator_b.timeframe
                subscriptions.add((indicator_b.symbol, timeframe))
        
        return list(subscriptions)
    
    def get_rule_result(self, rule_id: str) -> Optional[bool]:
        """
        Get the last evaluation result for a specific rule.
        
        Args:
            rule_id: The rule's unique ID
            
        Returns:
            Last evaluation result, or None if not yet evaluated
        """
        return self._rule_results.get(rule_id)
    
    def get_all_rule_results(self) -> Dict[str, bool]:
        """Get all rule evaluation results from the last cycle."""
        return self._rule_results.copy()
    
    def _get_bars_for_rule(
        self,
        rule: Rule,
        market_data: Dict[str, pd.DataFrame]
    ) -> pd.DataFrame:
        """
        Get the appropriate bar data for evaluating a rule.
        
        Args:
            rule: The rule being evaluated
            market_data: Available market data
            
        Returns:
            DataFrame with bar data to use
        """
        # Check if indicator has explicit symbol
        indicator = rule.condition.indicator_a
        indicator_type = indicator.type.value if hasattr(indicator.type, 'value') else indicator.type
        
        if indicator.symbol and indicator.symbol in market_data:
            return market_data[indicator.symbol]
        
        # For VIX type, try to get VIX data
        if indicator_type == "vix" and "VIX" in market_data:
            return market_data["VIX"]
        
        # For time-based rules, any data will work (we just need the structure)
        if indicator_type == "time":
            # Return first available data
            for data in market_data.values():
                return data
        
        # Default: return first available data
        for data in market_data.values():
            return data
        
        # If no data available, return empty DataFrame
        return pd.DataFrame()
    
    def _get_rule_timeframe(self, rule: Rule) -> TimeframeUnit:
        """Get the primary timeframe for a rule."""
        return rule.condition.indicator_a.timeframe
    
    def _rule_uses_vix(self, rule: Rule) -> bool:
        """Check if a rule uses VIX data."""
        indicator_type_a = rule.condition.indicator_a.type
        type_a_str = indicator_type_a.value if hasattr(indicator_type_a, 'value') else indicator_type_a
        
        if type_a_str == "vix":
            return True
        
        if rule.condition.indicator_b:
            indicator_type_b = rule.condition.indicator_b.type
            type_b_str = indicator_type_b.value if hasattr(indicator_type_b, 'value') else indicator_type_b
            if type_b_str == "vix":
                return True
        
        return False


def create_rule_engine(strategy: Strategy) -> RuleEngine:
    """
    Convenience function to create a rule engine.
    
    Args:
        strategy: Strategy to evaluate
        
    Returns:
        Configured RuleEngine instance
    """
    return RuleEngine(strategy)
