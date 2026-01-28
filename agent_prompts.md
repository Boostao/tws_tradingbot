# Agent Bootstrap Guide - TWS Trader Bot

> **Last Updated:** January 26, 2026  
> **Project Status:** ✅ Indicators expanded (MACD, BB, Stoch), Runtime Validated
> **Runtime Status:** Bot runs with Nautilus IB adapter, UI fully functional
> **Package Manager:** uv (virtual environment in `.venv/`)

---

## 🎯 Quick Start for Agents

### Project Location
```
/home/bruno/Work/Active project/cobalt/tws_traderbot/
```

### Prerequisites
```bash
# Install uv if not already installed
curl -LsSf https://astral.sh/uv/install.sh | sh

# Set up the project (creates .venv and installs dependencies)
cd /home/bruno/Work/Active\ project/cobalt/tws_traderbot && uv sync
```

### Run the Project
```bash
# Terminal 1: Start the Streamlit UI
cd /home/bruno/Work/Active\ project/cobalt/tws_traderbot && ./run_ui.sh
# Access at: http://localhost:8501

# Terminal 2: Start the Trading Bot
cd /home/bruno/Work/Active\ project/cobalt/tws_traderbot && ./run_bot.sh

# Or run directly with uv
cd /home/bruno/Work/Active\ project/cobalt/tws_traderbot && uv run python -m src.bot.live_runner
```

### Run Tests
```bash
cd /home/bruno/Work/Active\ project/cobalt/tws_traderbot && uv run pytest tests/ -v
# 102 tests should pass
```

---

## 📂 Project Architecture

### Directory Structure
```
tws_traderbot/
├── pyproject.toml                # Project config & dependencies (uv)
├── uv.lock                       # Locked dependencies
├── .venv/                        # Virtual environment (created by uv)
├── config/
│   ├── default.yaml              # Main configuration
│   ├── active_strategy.json      # Currently loaded strategy
│   └── environment/              # Environment-specific configs
├── data/
│   └── sample/                   # Sample market data (CSV files)
├── strategies/
│   └── example_strategy.json     # Example strategy template
├── src/
│   ├── bot/                      # Trading bot core
│   │   ├── adapter.py            # IB TWS adapter
│   │   ├── backtest_runner.py    # Backtesting engine
│   │   ├── live_runner.py        # Live trading entry point
│   │   ├── state.py              # Bot state management (JSON file-based)
│   │   └── strategy/
│   │       ├── base.py           # DynamicRuleStrategy (Nautilus integration)
│   │       ├── validator.py      # Strategy validation
│   │       └── rules/
│   │           ├── models.py     # Pydantic models (Rule, Condition, Strategy)
│   │           └── evaluator.py  # Rule evaluation engine
│   ├── config/
│   │   ├── settings.py           # Settings dataclass & load_config()
│   │   └── validation.py         # Config validation
│   ├── ui/                       # Streamlit UI
│   │   ├── main.py               # Entry point, sidebar
│   │   ├── styles.py             # TradingView dark theme CSS
│   │   ├── tabs/
│   │   │   ├── monitoring.py     # Live Trading tab
│   │   │   └── strategy.py       # Strategy Builder tab
│   │   └── components/
│   │       ├── rule_builder.py   # Visual rule editor
│   │       ├── rule_display.py   # Rule cards/display
│   │       ├── rule_chart.py     # Mini-chart with rule overlay
│   │       └── charts.py         # Equity curve, backtest charts
│   └── utils/
│       ├── indicators.py         # 15+ technical indicators
│       ├── logger.py             # Logging setup
│       └── market_hours.py       # Market hours utilities
├── tests/
│   ├── unit/                     # Unit tests
│   └── integration/              # Integration tests
├── .streamlit/
│   └── config.toml               # Streamlit configuration (telemetry disabled)
└── docs/
    └── strategy_guide.md         # Strategy creation guide
```

---

## 🔧 Key Components

### 1. Strategy System (Rules-Based)
**Files:** `src/bot/strategy/rules/models.py`, `src/bot/strategy/rules/evaluator.py`

Strategies are JSON files with this structure:
```json
{
  "name": "EMA Crossover",
  "description": "...",
  "version": "1.0.0",
  "global_filters": [...],       // VIX/time filters applied to ALL rules
  "rules": [                      // List of trading rules
    {
      "name": "MACD Crossover",
      "ticker": "SPY",
      "action": "BUY",
      "conditions": [
        {
          "left": {
            "type": "macd",
            "timeframe": "5m",
            "params": {
              "fast": 12,
              "slow": 26,
              "signal": 9
            },
            "component": "macd"
          },
          "operator": "crosses_above",
          "right": {
            "type": "macd",
            "timeframe": "5m",
             "params": {
              "fast": 12,
              "slow": 26,
              "signal": 9
            },
            "component": "signal"
          }
        }
      ],
      "enabled": true
    }
  ],
  "settings": {
    "position_size_type": "fixed_quantity",
    "position_size_value": 100,
    "max_positions": 5,
    "stop_loss_percent": 2.0,
    "take_profit_percent": 5.0
  }
}
```

**Available Indicators (20+):**
- Moving Averages: SMA, EMA
- Momentum: RSI, MACD, Stochastic
- Volatility: ATR, Bollinger Bands, VIX
- Volume: VWAP, OBV, Williams Alligator
- Price: Close, Open, High, Low
- Params: Supports dynamic parameters (e.g. `fast`, `slow`, `offset`) via `params` dict

**Tickers:**
- Strategy `tickers` are saved from the UI selection and used by the live runner when present

**Cross-symbol indicators:**
- `indicator.symbol` can reference another symbol (e.g., `VIX`) and will use that symbol's bars when available
- Backtest and live rule evaluation pass full `market_data` so cross-symbol comparisons work

**Live IB subscriptions:**
- Live runner includes indicator symbol overrides (including VIX) in the instruments list for IB data clients

**Condition Operators:**
- `crosses_above`, `crosses_below`
- `greater_than`, `less_than`, `equals`
- `slope_above`, `slope_below`
- `within_range` (Time-based validation)

**Position sizing:**
- Live runner uses `risk.max_position_size` if set; otherwise falls back to `strategy.initial_capital * risk.max_position_pct`

### 2. Nautilus Trader Integration
**File:** `src/bot/strategy/base.py`

- `NautilusDynamicRuleStrategy` extends Nautilus `Strategy` and wraps `DynamicRuleStrategy`
- Uses `msgspec.Struct` for config (required by Nautilus)
- Hot-reload support via file watching
- **Status:** Nautilus IB adapter supported via `nautilus_ibapi` and live runner uses `NautilusDynamicRuleStrategy`
- **Order Routing:** Market orders submitted through Nautilus `order_factory` when running in live mode

### 3. IB Adapter
**File:** `src/bot/adapter.py`

- `IBAdapter` class wraps ibapi connection
- `IBConnectionConfig` dataclass for connection settings
- `NAUTILUS_IB_AVAILABLE` flag indicates if full Nautilus IB integration works
- Falls back to direct ibapi when Nautilus adapter fails

### 4. State Management
**File:** `src/bot/state.py`

- JSON file-based state (`logs/bot_state.json`)
- `BotState` dataclass with: status, positions, orders, equity, P&L
- `read_state()` / `write_state()` functions
- UI reads state file for display updates

### 5. Streamlit UI
**Entry:** `src/ui/main.py`

**Tabs:**
1. **Live Trading** (`tabs/monitoring.py`): Connection status, metrics, positions, orders, logs, bot controls
2. **Strategy Builder** (`tabs/strategy.py`): Visual rule editor, backtesting, strategy import/export

**Styling:** Tokyo Night-inspired dark theme in `styles.py`
- Background: #1a1b26 (main), #16161e (cards)
- Text: #c0caf5 (primary), #a9b1d6 (secondary)
- Accents: #7aa2f7 (blue), #9ece6a (profit), #f7768e (loss)

---

## ⚠️ Known Issues & Limitations

### 1. Pydantic Deprecation Warnings
**Status:** Non-blocking (tests pass)  
**Cause:** Using class-based `Config` instead of `ConfigDict`  
**Files:** `src/bot/strategy/rules/models.py` lines 121, 185, 218

```python
# Current (deprecated):
class Config:
    extra = "forbid"

# Should migrate to:
model_config = ConfigDict(extra="forbid")
```

### 2. Backtest Engine (Nautilus Mode)
**Status:** Partial (native engine OK)  
**Note:** Nautilus engine runs with best-effort fill mapping; if mapping fails it falls back to native results

### 3. VIX Data Fetch
**Status:** Partial  
**Note:** `src/utils/data_loader.py` can fetch latest VIX via TWS provider when available; no external fallback feed is configured

### 4. Sample Data Files
**Location:** `data/sample/`  
**Contains:** SPY_5min.csv, QQQ_5min.csv, VIX_5min.csv, etc.  
**Note:** VIX file has a stray "20000" on line 464 (data artifact, not code issue)

---

## 🔄 Recent Session Work (January 14, 2026)

### Issues Fixed

1. **HTML `<span>` tags showing as text in metrics**
   - `st.metric()` doesn't render HTML
   - Fixed `format_currency()` in `styles.py` to return plain text
   - Added `format_currency_html()` for contexts needing HTML

2. **Truncated equity display in sidebar**
   - Added CSS in `styles.py`:
     ```css
     [data-testid="stMetricValue"] {
         white-space: nowrap;
         overflow: visible;
     }
     ```

3. **Streamlit telemetry warnings**
   - Created `.streamlit/config.toml` with `gatherUsageStats = false`

4. **Deprecated `use_container_width` warnings**
   - Replaced with `width="stretch"` in 5 UI files

5. **msgspec default_factory error**
   - Fixed in `src/bot/strategy/base.py`:
     ```python
     # Before:
     instruments: List[str] = ["SPY.ARCA"]
     # After:
     instruments: List[str] = msgspec.field(default_factory=lambda: ["SPY.ARCA"])
     ```

6. **Nautilus IB adapter graceful fallback**
   - Added `NAUTILUS_IB_AVAILABLE` check in `live_runner.py`
   - Bot now runs in simulation mode when adapter unavailable

---

## 🧪 Testing

### Test Structure
```
tests/
├── unit/
│   ├── test_indicators.py       # Technical indicator tests
│   ├── test_models.py           # Rule/Strategy model tests
│   ├── test_evaluator.py        # Rule evaluation tests
│   ├── test_strategy.py         # Strategy validation tests
│   └── test_settings.py         # Config loading tests
└── integration/
    ├── test_strategy_workflow.py # End-to-end strategy tests
    └── test_rule_evaluation.py   # Rule evaluation integration
```

### Run Specific Tests
```bash
# All tests
uv run pytest tests/ -v

# Unit tests only
uv run pytest tests/unit/ -v

# Integration tests only
uv run pytest tests/integration/ -v

# Specific file
uv run pytest tests/unit/test_evaluator.py -v

# With coverage
uv run pytest tests/ --cov=src --cov-report=html
```

---

## 📦 Package Management (uv)

```bash
# Initial setup - creates .venv and installs all dependencies
uv sync

# Add a new dependency
uv add <package-name>

# Add a dev dependency
uv add --dev <package-name>

# Update all dependencies
uv sync --upgrade

# Run any command in the virtual environment
uv run <command>

# Activate the virtual environment manually (optional)
source .venv/bin/activate
```

---

## 📋 Configuration Files

### config/default.yaml
```yaml
ib:
  host: "127.0.0.1"
  port: 7497              # 7497=paper, 7496=live
  client_id: 1
  account: ""             # Leave empty for default
  trading_mode: "paper"

app:
  log_level: "INFO"
  strategies_dir: "strategies"
  active_strategy_path: "config/active_strategy.json"

backtest:
  default_capital: 10000
  data_dir: "data/sample"
```

### .streamlit/config.toml
```toml
[browser]
gatherUsageStats = false

[server]
port = 8501
address = "localhost"

[theme]
base = "dark"
primaryColor = "#7aa2f7"
backgroundColor = "#1a1b26"
secondaryBackgroundColor = "#16161e"
textColor = "#c0caf5"
```

---

## 🛠️ Common Tasks

### Add a New Indicator
1. Add function to `src/utils/indicators.py`
2. Add to `IndicatorFactory` in `src/bot/strategy/rules/indicators.py`
3. Add to `IndicatorType` enum in `src/bot/strategy/rules/models.py`
4. Update UI dropdown in `src/ui/components/rule_builder.py`
5. Update strategy validation in `src/bot/strategy/validator.py`

### Add a New Condition Operator
1. Add to `OperatorType` enum in `models.py`
2. Implement evaluation in `evaluator.py` `_evaluate_operator()`
3. Update UI in `rule_builder.py`

### Modify Bot State
1. Update `BotState` dataclass in `src/bot/state.py`
2. Update `write_state()` calls in `live_runner.py`
3. Update UI displays in `tabs/monitoring.py`

---

## 📚 Key Imports Quick Reference

```python
# Configuration
from src.config.settings import Settings, load_config

# Strategy Models
from src.bot.strategy.rules.models import (
    Strategy, Rule, Condition,
    IndicatorType, OperatorType, ActionType, TimeFrame
)

# Rule Evaluation
from src.bot.strategy.rules.evaluator import RuleEvaluator

# Strategy Validation
from src.bot.strategy.validator import StrategyValidator

# Bot State
from src.bot.state import BotState, read_state, write_state

# IB Connection
from src.bot.adapter import IBAdapter, IBConnectionConfig, NAUTILUS_IB_AVAILABLE

# UI Styling
from src.ui.styles import COLORS, apply_theme, format_currency

# Indicators
from src.utils.indicators import calculate_ema, calculate_rsi, calculate_macd
```

---

## 🎯 Future Work / TODO

1. ~~**Fix Nautilus IB Adapter**~~ ✅ DONE - Use `nautilus_ibapi` package
2. **Migrate Pydantic models** - Use `ConfigDict` instead of class-based `Config`
3. **Add more tests** - Increase coverage for UI components
4. **Real-time data** - Integrate with live IB data feeds
5. **Order execution** - Implement actual order placement via IB
6. **Database** - Replace JSON file state with SQLite/PostgreSQL
7. **Authentication** - Add user auth for multi-user deployment

---

## 📞 Support Contacts

- **Framework:** [Nautilus Trader Docs](https://nautilustrader.io/)
- **IB API:** [Interactive Brokers API Docs](https://interactivebrokers.github.io/tws-api/)
- **Streamlit:** [Streamlit Docs](https://docs.streamlit.io/)
