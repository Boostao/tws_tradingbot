# Roadmap

## Phase 1: Enhanced Backtesting & Reporting
Current backtesting provides basic metrics. We will upgrade this to industry-standard reporting.

- [x] **Integration of QuantStats**: Replace/augment current backtest charts with a full "Tear Sheet" (Sharpe, Drawdown, Monthly Heatmap).
- [x] **Native Nautilus Backtesting**: Ensure the `backtest_runner.py` fully leverages `BacktestNode` for accurate spread/latency simulation.

## Phase 2: Remote Operations & Notifications
Move beyond "desktop monitoring" to true remote management.

- [ ] **Telegram/Discord Integration**:
    - Push notifications for Trades, PnL, and Errors.
    - Command handling (`/status`, `/stop`, `/force_exit`) for emergency control.
    - Telegram commands implemented; Discord remains notify-only.

## Phase 3: Strategy Optimization (Auto-Tuning)
Stop guessing indicator parameters.

- [x] **Hyperparameter Optimization (Optuna)**: 
    - Create an optimizer script that runs thousands of backtest variations.
    - Find optimal values for `rsi_period`, `stop_loss`, etc.

## Phase 4: AI & Advanced Signals
Prepare the bot for Machine Learning without rewriting the core.

- [x] **ML Signal Column**: Standardize a "Signal" indicator (0-1 float).
- [x] **External Model Loader**: Allow the bot to ingest predictions from external `.onnx` or `.joblib` models.

## Phase 5: Production Readiness
- [x] **Docker / Compose Setup**: Containerize the TWS Gateway + Bot pair for server deployment.
- [x] **PostgreSQL Support**: Option to switch from DuckDB to Postgres for multi-user/concurrent access.
