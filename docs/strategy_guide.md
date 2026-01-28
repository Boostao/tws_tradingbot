# Strategy Guide

A comprehensive guide to creating effective trading strategies with TWS Trader Bot.

## Table of Contents

1. [Understanding the Rule System](#understanding-the-rule-system)
2. [Indicator Types](#indicator-types)
3. [Condition Types](#condition-types)
4. [Rule Scopes and Actions](#rule-scopes-and-actions)
5. [Example Strategies](#example-strategies)
6. [Best Practices](#best-practices)
7. [Advanced Techniques](#advanced-techniques)

---

## Understanding the Rule System

TWS Trader Bot uses a **rule-based strategy system** where each strategy consists of:

- **Rules**: Individual conditions that trigger actions
- **Conditions**: Logic that compares indicators to thresholds or other indicators
- **Indicators**: Technical analysis values calculated from market data

### Strategy Flow

```
Market Data → Indicators → Conditions → Rules → Actions
```

1. **Market data** (price, volume) flows into the system
2. **Indicators** are calculated (EMA, RSI, etc.)
3. **Conditions** are evaluated (Is EMA(9) > EMA(21)?)
4. **Rules** determine actions based on conditions
5. **Actions** are executed (BUY, SELL, or FILTER)

---

## Indicator Types

### Price-Based Indicators

#### PRICE
The current closing price of the instrument.
- **Parameters**: None
- **Use Case**: Simple price threshold strategies
- **Example**: "Buy when PRICE > 100"

#### SMA (Simple Moving Average)
The arithmetic mean of prices over a specified period.
- **Parameters**: `length` (default: 20)
- **Use Case**: Trend identification, support/resistance
- **Example**: "Buy when PRICE crosses above SMA(50)"
- **Note**: Slower to react than EMA, but less prone to false signals

#### EMA (Exponential Moving Average)
A weighted moving average that gives more importance to recent prices.
- **Parameters**: `length` (default: 20)
- **Use Case**: Trend following, crossover strategies
- **Example**: "Buy when EMA(9) crosses above EMA(21)"
- **Note**: More responsive than SMA to recent price changes

### Momentum Indicators

#### RSI (Relative Strength Index)
Measures the speed and magnitude of price movements (0-100 scale).
- **Parameters**: `length` (default: 14)
- **Use Case**: Overbought/oversold conditions
- **Values**:
  - Above 70: Overbought (potential sell)
  - Below 30: Oversold (potential buy)
- **Example**: "Buy when RSI(14) < 30"

#### MACD (Moving Average Convergence Divergence)
Shows the relationship between two EMAs.
- **Parameters**: 
  - `fast_length` (default: 12)
  - `slow_length` (default: 26)
  - `signal_length` (default: 9)
- **Use Case**: Trend direction and momentum
- **Example**: "Buy when MACD crosses above MACD Signal"

#### STOCHASTIC
Compares closing price to price range over a period.
- **Parameters**:
  - `k_period` (default: 14)
  - `d_period` (default: 3)
- **Use Case**: Overbought/oversold in ranging markets
- **Values**: 0-100 scale (similar interpretation to RSI)

### Volatility Indicators

#### ATR (Average True Range)
Measures market volatility.
- **Parameters**: `length` (default: 14)
- **Use Case**: Position sizing, stop-loss placement
- **Example**: "Set stop-loss at 2x ATR below entry"

#### BOLLINGER (Bollinger Bands)
Price channels based on standard deviation.
- **Parameters**:
  - `length` (default: 20)
  - `std_dev` (default: 2.0)
- **Returns**: Upper band, middle band (SMA), lower band
- **Use Case**: Volatility breakouts, mean reversion
- **Example**: "Buy when PRICE < BOLLINGER_LOWER"

### Volume Indicators

#### VOLUME
Trading volume for the current bar.
- **Parameters**: None
- **Use Case**: Confirmation of price moves
- **Example**: "Buy only when VOLUME > SMA(VOLUME, 20)"

#### VWAP (Volume Weighted Average Price)
Average price weighted by volume.
- **Parameters**: None (resets daily)
- **Use Case**: Institutional trading reference
- **Example**: "Buy when PRICE > VWAP"

### Market Indicators

#### VIX
The CBOE Volatility Index (fear gauge).
- **Parameters**: None
- **Use Case**: Market sentiment filter
- **Values**:
  - Below 15: Low volatility (complacent)
  - 15-25: Normal
  - Above 25: High volatility (fear)
  - Above 35: Extreme fear
- **Example**: "Only trade when VIX < 25"

#### TIME
Current time of day.
- **Parameters**: None
- **Use Case**: Session filtering
- **Example**: "Only trade between 09:30 and 15:30"

---

## Condition Types

### Comparison Conditions

#### GREATER_THAN
True when indicator_a > threshold or indicator_b.
```
indicator_a > threshold
indicator_a > indicator_b
```

#### LESS_THAN
True when indicator_a < threshold or indicator_b.
```
indicator_a < threshold
indicator_a < indicator_b
```

#### EQUALS
True when indicator_a equals threshold (with tolerance).
```
indicator_a == threshold (±tolerance)
```

### Crossover Conditions

#### CROSSES_ABOVE
True when indicator_a crosses above indicator_b or threshold.
- **Previous bar**: indicator_a <= indicator_b
- **Current bar**: indicator_a > indicator_b
- **Use Case**: Trend change signals

#### CROSSES_BELOW
True when indicator_a crosses below indicator_b or threshold.
- **Previous bar**: indicator_a >= indicator_b
- **Current bar**: indicator_a < indicator_b
- **Use Case**: Trend reversal signals

### Range Conditions

#### WITHIN_RANGE
True when indicator value is within specified range.
```
range_start <= indicator_a <= range_end
```
- **Use Case**: Time filters, price zones

#### OUTSIDE_RANGE
True when indicator value is outside specified range.
```
indicator_a < range_start OR indicator_a > range_end
```
- **Use Case**: Breakout detection

### Trend Conditions

#### SLOPE_ABOVE
True when the slope of indicator_a over lookback periods is above threshold.
```
(indicator_a[0] - indicator_a[-lookback]) / lookback > threshold
```
- **Parameters**: `lookback_periods`, `threshold`
- **Use Case**: Trend strength confirmation

#### SLOPE_BELOW
True when the slope is below threshold.
- **Use Case**: Identifying weakening trends or downtrends

#### INCREASING
True when indicator has been increasing over lookback periods.
- **All values**: indicator[i] > indicator[i-1]
- **Use Case**: Momentum confirmation

#### DECREASING
True when indicator has been decreasing over lookback periods.
- **Use Case**: Identifying declining momentum

---

## Rule Scopes and Actions

### Rule Scopes

#### GLOBAL
Rules that apply across all tickers and must ALL be true before any trading occurs.
- **Use Case**: Market regime filters
- **Examples**:
  - VIX filter
  - Time-of-day filter
  - Market direction filter

#### PER_TICKER
Rules evaluated independently for each ticker in the strategy.
- **Use Case**: Entry and exit signals
- **Examples**:
  - EMA crossover for SPY
  - RSI oversold for QQQ

### Rule Actions

#### FILTER
A condition that must be true for other rules to execute.
- **Behavior**: If ANY filter rule is false, no trades occur
- **Typically used with**: GLOBAL scope
- **Example**: VIX must be below 25 for any trades

#### BUY
Generates a buy signal when condition is true.
- **Behavior**: Opens a long position or adds to existing
- **Typically used with**: PER_TICKER scope

#### SELL
Generates a sell signal when condition is true.
- **Behavior**: Closes long position or opens short
- **Typically used with**: PER_TICKER scope

---

## Example Strategies

### Strategy 1: Simple EMA Crossover

A basic trend-following strategy using moving average crossovers.

```json
{
  "name": "Simple EMA Crossover",
  "version": "1.0.0",
  "description": "Buy when EMA(9) crosses above EMA(21), sell when it crosses below",
  "tickers": ["SPY", "QQQ"],
  "rules": [
    {
      "name": "EMA Cross Buy",
      "scope": "per_ticker",
      "action": "buy",
      "condition": {
        "type": "crosses_above",
        "indicator_a": {"type": "ema", "length": 9},
        "indicator_b": {"type": "ema", "length": 21}
      }
    },
    {
      "name": "EMA Cross Sell",
      "scope": "per_ticker",
      "action": "sell",
      "condition": {
        "type": "crosses_below",
        "indicator_a": {"type": "ema", "length": 9},
        "indicator_b": {"type": "ema", "length": 21}
      }
    }
  ]
}
```

**Pros**: Simple, works in trending markets
**Cons**: Whipsaws in ranging markets

---

### Strategy 2: Filtered Mean Reversion

RSI-based mean reversion with VIX and time filters.

```json
{
  "name": "Filtered Mean Reversion",
  "version": "1.0.0",
  "description": "Buy oversold RSI conditions when VIX is calm",
  "tickers": ["SPY"],
  "rules": [
    {
      "name": "Low VIX Filter",
      "scope": "global",
      "action": "filter",
      "condition": {
        "type": "less_than",
        "indicator_a": {"type": "vix"},
        "threshold": 22.0
      }
    },
    {
      "name": "Market Hours Filter",
      "scope": "global",
      "action": "filter",
      "condition": {
        "type": "within_range",
        "indicator_a": {"type": "time"},
        "range_start": "09:45",
        "range_end": "15:30"
      }
    },
    {
      "name": "RSI Oversold Buy",
      "scope": "per_ticker",
      "action": "buy",
      "condition": {
        "type": "crosses_above",
        "indicator_a": {"type": "rsi", "length": 14},
        "threshold": 30.0
      }
    },
    {
      "name": "RSI Overbought Sell",
      "scope": "per_ticker",
      "action": "sell",
      "condition": {
        "type": "crosses_below",
        "indicator_a": {"type": "rsi", "length": 14},
        "threshold": 70.0
      }
    }
  ]
}
```

**Pros**: Works well in ranging/calm markets
**Cons**: Can miss big trends

---

### Strategy 3: Trend + Momentum Confirmation

Multi-indicator strategy requiring trend alignment and momentum.

```json
{
  "name": "Trend Momentum Strategy",
  "version": "1.0.0",
  "description": "Trend following with momentum confirmation",
  "tickers": ["SPY", "QQQ", "IWM"],
  "rules": [
    {
      "name": "VIX Filter",
      "scope": "global",
      "action": "filter",
      "condition": {
        "type": "less_than",
        "indicator_a": {"type": "vix"},
        "threshold": 28.0
      }
    },
    {
      "name": "VIX Slope Filter",
      "scope": "global",
      "action": "filter",
      "condition": {
        "type": "slope_below",
        "indicator_a": {"type": "vix"},
        "threshold": 0.5,
        "lookback_periods": 5
      }
    },
    {
      "name": "Trend Confirmation Buy",
      "scope": "per_ticker",
      "action": "buy",
      "condition": {
        "type": "crosses_above",
        "indicator_a": {"type": "ema", "length": 9},
        "indicator_b": {"type": "ema", "length": 21}
      }
    },
    {
      "name": "Trend Break Sell",
      "scope": "per_ticker",
      "action": "sell",
      "condition": {
        "type": "crosses_below",
        "indicator_a": {"type": "ema", "length": 9},
        "indicator_b": {"type": "ema", "length": 21}
      }
    }
  ]
}
```

---

### Strategy 4: Volatility Breakout

Trade breakouts from Bollinger Band squeezes.

```json
{
  "name": "Volatility Breakout",
  "version": "1.0.0",
  "description": "Buy breakouts above Bollinger upper band",
  "tickers": ["SPY"],
  "rules": [
    {
      "name": "Time Filter",
      "scope": "global",
      "action": "filter",
      "condition": {
        "type": "within_range",
        "indicator_a": {"type": "time"},
        "range_start": "09:35",
        "range_end": "12:00"
      }
    },
    {
      "name": "Breakout Buy",
      "scope": "per_ticker",
      "action": "buy",
      "condition": {
        "type": "crosses_above",
        "indicator_a": {"type": "price"},
        "indicator_b": {"type": "bollinger_upper", "length": 20, "std_dev": 2.0}
      }
    },
    {
      "name": "Mean Reversion Sell",
      "scope": "per_ticker",
      "action": "sell",
      "condition": {
        "type": "crosses_below",
        "indicator_a": {"type": "price"},
        "indicator_b": {"type": "sma", "length": 20}
      }
    }
  ]
}
```

---

## Best Practices

### 1. Always Use Filters

**Never trade without at least one filter rule.**

Recommended filters:
- **VIX Filter**: `VIX < 25` prevents trading in high volatility
- **Time Filter**: Avoid first 15 minutes and last 30 minutes
- **Trend Filter**: Only trade with the larger trend

### 2. Start Simple

Begin with 2-3 rules maximum:
1. One filter rule
2. One entry rule
3. One exit rule

Add complexity only after the simple version is profitable.

### 3. Match Strategy to Market Conditions

| Market Type | Recommended Strategy |
|------------|---------------------|
| Trending | EMA crossover, breakouts |
| Ranging | RSI mean reversion |
| High Volatility | Avoid or use tight stops |
| Low Volatility | Breakout strategies |

### 4. Test Before Trading Live

1. **Backtest**: Run historical tests with sample data
2. **Paper Trade**: Test with simulated money first
3. **Small Size**: Start with minimal position sizes
4. **Scale Up**: Gradually increase size as confidence grows

### 5. Use Appropriate Timeframes

| Timeframe | Best For | Update Frequency |
|-----------|----------|-----------------|
| 1 minute | Scalping | Very fast signals |
| 5 minute | Day trading | Standard intraday |
| 15 minute | Swing day | Fewer false signals |
| 1 hour | Swing trading | Position trades |
| Daily | Position trading | Long-term trends |

### 6. Mind the Market Hours

**Best trading hours (US Eastern):**
- 9:45 AM - 11:30 AM (morning momentum)
- 2:00 PM - 3:30 PM (afternoon continuation)

**Avoid:**
- 9:30 AM - 9:45 AM (opening volatility)
- 3:45 PM - 4:00 PM (closing chaos)

---

## Advanced Techniques

### Multi-Timeframe Analysis

Combine indicators from different timeframes:

```json
{
  "condition": {
    "type": "greater_than",
    "indicator_a": {"type": "ema", "length": 9, "timeframe": "5m"},
    "indicator_b": {"type": "ema", "length": 50, "timeframe": "15m"}
  }
}
```

This buys when short-term 5-minute EMA is above longer-term 15-minute EMA.

### Slope-Based Entries

Use slope conditions to confirm trend strength:

```json
{
  "condition": {
    "type": "slope_above",
    "indicator_a": {"type": "ema", "length": 21},
    "threshold": 0.05,
    "lookback_periods": 5
  }
}
```

This confirms the EMA is rising at a minimum rate.

### Compound Strategies

Create strategies that require multiple conditions by using multiple filter rules. All filter rules must be true for trades to execute.

### Dynamic Position Sizing

While position sizing is handled by the risk manager, you can adjust risk based on:
- ATR for volatility-adjusted stops
- VIX for market-wide risk scaling

---

## Troubleshooting Strategies

### Too Many Trades

**Problem**: Strategy is overtrading
**Solutions**:
- Add time filters to reduce trading windows
- Increase indicator lengths for smoother signals
- Add confirmation rules (RSI > 50 for buys)

### Too Few Trades

**Problem**: Strategy rarely triggers
**Solutions**:
- Loosen filter conditions (VIX < 30 instead of < 20)
- Reduce indicator lengths for faster signals
- Remove unnecessary confirmation rules

### Poor Performance

**Problem**: Strategy losing money
**Solutions**:
1. Check if strategy matches market conditions
2. Add or tighten filters
3. Review entry/exit timing
4. Backtest on different time periods
5. Consider inverse conditions

### Strategy Not Applying

**Problem**: Changes not taking effect
**Solutions**:
1. Check for validation errors (see logs)
2. Verify JSON syntax
3. Ensure hot-reload signal is working
4. Restart bot if necessary

---

## Summary

Creating effective strategies requires:

1. **Understanding** indicators and conditions
2. **Filtering** for favorable market conditions
3. **Testing** thoroughly before live trading
4. **Starting** simple and adding complexity gradually
5. **Monitoring** and adjusting based on results

Remember: **No strategy works in all market conditions.** The best approach is to have multiple strategies for different market regimes and use filters to activate the appropriate one.

---

*Happy Trading! 📈*
