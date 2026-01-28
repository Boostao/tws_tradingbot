# Trading Bot Project Planning

## Project Overview
Develop a sophisticated trading system using the **Nautilus Trader** framework for TWS integration. The project consists of two main components managed via a unified Streamlit interface: a **Live Trading Bot (TB)** execution service and a **Rule-Based Trading Strategy UI (RBTSUI)** for effortless strategy creation, backtesting, and deployment.

## Key Requirements

### 1. Framework & Core
- **Trading Engine**: Nautilus Trader (Live Node & Backtest Node).
- **Broker Interface**: Interactive Brokers TWS via Nautilus IB integration.
- **UI Framework**: Streamlit (Themed like TradingView - Dark mode with sharp accents).
- **Architecture**: Separated Logic (Nautilus) and UI (Streamlit), communicating via configuration files or shared state.

### 2. Streamlit UI Structure
The application will feature two distinct tabs/modes:

#### A. Live Service (Trading Bot Monitor)
- Monitor the running Nautilus Trader instance.
- Display live PnL, active orders, positions, and logs.
- Stop/Restart controls.
- Indicator tracking.

#### B. Rule-Based Trading Strategy UI (RBTSUI)
- **Visual Strategy Builder**: Create rules without coding.
- **Rule Scope**:
    - **Global**: Applied once per cycle (e.g., "VIX Slope < -0.25").
    - **Per-Ticker**: Applied to each asset (e.g., "EMA Crossover").
- **Backtesting**:
    - Select tickers and date range.
    - Set initial capital (default 10k).
    - Run backtest on historical data.
    - Display equity curve and performance metrics.
- **Rule Validation**: Display a graph next to each rule to help user verify when the rule last evaluated to TRUE.
- **Persistence**: Import/Export strategies as JSON.
- **Deployment**: "Apply Strategy" button to dynamically update the Live Bot behavior.

### 3. Rule Engine Logic
The strategy engine must interpret dynamic rules defined in the UI. A rule generally follows this structure:
> **Curve A** has **Property** relative to **Curve B / Value** in the **Past X periods**.

**Supported Indicators (Curves):**
- EMA (Exponential Moving Average) `T-x` (Length) in `P` (Period: 1m, 5m, 1h, etc.)
- VIX (External Market Data)
- Time (Regular Trading Hours)
- Asset Allocation (1/N of capital)

**Example Rules:**
1.  **(Global) VIX Slope**: VIX EMA(9, 5min) slope < -0.25 over past 30 min.
2.  **(Global) Market Hours**: Current time is within RTH.
3.  **(Global/Ticker) Allocation**: Order size = Total Equity / N tickers (max).
4.  **(Per-Ticker) Crossover**: EMA(9, 5min) > EMA(21, 5min) (Bullish Cross).

## Technical Architecture

### Component Diagram
```
[User] <-> [Streamlit App]
                |
                +-> [RBTSUI Tab]
                |       |-> Strategy Builder (JSON Generator)
                |       |-> Backtest Engine (Nautilus BacktestNode)
                |       |-> Visualizer (Plotly)
                |
                +-> [Live Monitor Tab]
                        |-> Reads State/Logs
                        |-> Sentinel/Control (Start/Stop)

[Storage] <-> [Strategy Registry (JSON Files)]
                        ^
                        | (Load/Reload)
                        v
[Live Service] -> [Nautilus Trader Node]
                |-> [Dynamic Strategy Loader]
                |       |-> Parses JSON Rules
                |       |-> Converts to Nautilus Logic
                |-> [IB TWS Connection]
```

### Data Flow for Deployment
1.  **Design**: User creates rules in RBTSUI.
2.  **Save/Export**: Strategy saved as `strategy_config.json`.
3.  **Apply**: User clicks "Apply to Bot".
4.  **Reload**: Live Bot detects change (or is signaled), loads new JSON, reconstructs the logic, and restarts the strategy loop.

## Directory Structure
```
tws_traderbot/
├── src/
│   ├── bot/
│   │   ├── __init__.py
│   │   ├── live_runner.py       # Entry point for Live Nautilus Node
│   │   ├── backtest_runner.py   # Entry point for Backtest Nautilus Node
│   │   ├── adapter.py           # TWS/Nautilus setup
│   │   └── strategy/
│   │       ├── __init__.py
│   │       ├── base.py
│   │       ├── dynamic.py       # The core Strategy class that interprets JSON rules
│   │       └── rules/           # Rule parsing logic
│   │           ├── conditions.py
│   │           └── indicators.py
│   ├── ui/
│   │   ├── __init__.py
│   │   ├── main.py              # Main Streamlit App
│   │   ├── styles.py            # Dark theme CSS
│   │   ├── tabs/
│   │   │   ├── monitoring.py    # Live tab
│   │   │   └── strategy.py      # RBTSUI tab
│   │   └── components/
│   │       ├── rule_builder.py
│   │       └── charts.py
│   ├── config/
│   │   ├── settings.py
│   │   └── active_strategy.json # Current deployed strategy
│   └── utils/
│       ├── indicators.py
│       └── data_loader.py       # Historical data fetcher for backtests
├── strategies/                  # Saved JSON strategies
├── logs/
├── tests/
├── requirements.txt
└── README.md
```

## Implementation Plan

### Phase 1: Foundation & Nautilus Setup
- Set up `nautilus_trader` environment.
- Create a basic Nautilus Strategy class that can accept parameters.
- Establish TWS connection via Nautilus.
- Create Streamlit skeleton with "Dark TradingView" theme.

### Phase 2: Dynamic Strategy Engine (The Brain)
- Design the JSON schema for strategies.
- Implement the `DynamicStrategy` class in Nautilus that:
    - Parses the JSON.
    - Initializes necessary indicators (EMA, VIX, etc.) dynamically.
    - Evaluates rules on every bar/tick.
- Implement specific Rule logic (Slope, Crossover, Thresholds).

### Phase 3: RBTSUI - Builder & Backtester
- Build the UI forms to construct rules.
- Integrate Plotly to visualize "Last True" events for rules.
- Implement the Backtest button:
    - Spawns a `BacktestNode`.
    - Runs the `DynamicStrategy` with selected config.
    - Returns results to UI.

### Phase 4: Live Service Integration
- Implement the Live Monitor tab.
- Build the "Apply" mechanism (Save JSON -> Restart Bot/Reload Strategy).
- Ensure safe reloading (cancel open orders vs keep positions).

### Phase 5: Refinement
- Add Position Sizing logic (Rule 3).
- Implement VIX fetcher (as a data feed).
- Logging and Error handling.

## Risk Management & Safety
- **Allocation Limits**: Strict 1/N allocation enforcement.
- **Loss Limits**: Global max daily loss stop.
- **Fail-Safe**: Bot must handle invalid JSON configs gracefully (fall back to default safe mode).
- **Execution**: Limit orders preferred over Market orders.

## Technologies
- **Core**: Python, Nautilus Trader
- **UI**: Streamlit, Plotly (Charts)
- **Data**: Interactive Brokers (Live), CSV/Parquet (Backtesting)
- **Utils**: Pandas, NumPy
