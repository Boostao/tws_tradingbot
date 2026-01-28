# TWS Trader Bot

A sophisticated rule-based trading bot built with [Nautilus Trader](https://nautilustrader.io/) framework, integrating with Interactive Brokers' Trader Workstation (TWS). Features a TradingView-inspired dark theme Streamlit UI for live monitoring and visual strategy building.

![Python 3.10+](https://img.shields.io/badge/python-3.10+-blue.svg)
![Nautilus Trader](https://img.shields.io/badge/nautilus--trader-1.180.0+-green.svg)
![License](https://img.shields.io/badge/license-MIT-blue.svg)

## 🚀 Features

### Trading Engine
- **Rule-Based Strategies**: Create complex trading strategies using visual rule builder
- **Hot-Reload**: Update strategies on-the-fly without restarting the bot
- **Risk Management**: Built-in position sizing and risk controls
- **Multiple Timeframes**: Support for 1m, 5m, 15m, 1h, and daily bars
- **Backtesting**: Test strategies against historical data before going live

### Strategy Builder
- **Visual Rule Editor**: Drag-and-drop style rule creation
- **15+ Indicators**: EMA, SMA, RSI, MACD, Bollinger Bands, ATR, VIX, and more
- **10+ Condition Types**: Crosses above/below, greater/less than, within range, slope analysis
- **Global Filters**: VIX-based and time-based filtering for all trades
- **Per-Ticker Rules**: Individual entry/exit rules per symbol

### User Interface (Streamlit)
- **Live Trading Tab**: Real-time position monitoring, P&L tracking, trade history
- **Strategy Builder Tab**: Visual strategy creation with mini-charts
- **TradingView Theme**: Professional dark theme matching TradingView's design
- **Auto-Refresh**: Live data updates every second during market hours

## 📋 Prerequisites

- Python 3.10 or higher
- Interactive Brokers account (Paper or Live)
- TWS (Trader Workstation) or IB Gateway installed
- TWS API enabled (see setup guide)

## 🔧 Installation

### 1. Clone the Repository
```bash
git clone <repository-url>
cd tws_traderbot
```

### 2. Create Virtual Environment
```bash
python -m venv venv
source venv/bin/activate  # Linux/Mac
# or
.\venv\Scripts\activate   # Windows
```

### 3. Install Dependencies
```bash
pip install -r requirements.txt
```

### 4. Set Up TWS/IB Gateway

**Important**: Before running the bot, you must configure TWS for API access.

1. Open TWS and log in
2. Go to **File → Global Configuration → API → Settings**
3. Enable the following:
   - ✅ Enable ActiveX and Socket Clients
   - ✅ Allow connections from localhost only
   - ✅ Read-Only API (for testing)
4. Note the **Socket Port** (default: 7497 for paper, 7496 for live)
5. Click **Apply** and restart TWS

See [TWS_SETUP_GUIDE.md](../TWS_SETUP_GUIDE.md) for detailed instructions.

### 5. Configure Environment
```bash
cp config/environment/.env.example config/environment/.env
```

Edit `.env` with your settings:
```env
TWS_HOST=127.0.0.1
TWS_PORT=7497          # 7497 for paper, 7496 for live
TWS_CLIENT_ID=1
ACCOUNT_ID=your_account_id
```

## 🎯 Quick Start

### Running the UI
```bash
./run_ui.sh
# or
streamlit run src/ui/main.py
```

The UI will open at `http://localhost:8501`

### Running the Trading Bot
```bash
./run_bot.sh
# or
python -m src.bot.live_runner
```

## 📊 Creating Your First Strategy

### Using the Strategy Builder UI

1. **Open the UI** and navigate to the **Strategy Builder** tab
2. **Add a Global Filter** (optional):
   - Click "Add Rule"
   - Select Scope: "Global"
   - Select Action: "Filter"
   - Configure condition (e.g., VIX < 25)
3. **Add Entry Rules**:
   - Click "Add Rule"
   - Select Scope: "Per Ticker"
   - Select Action: "Buy"
   - Configure condition (e.g., EMA(9) crosses above EMA(21))
4. **Add Exit Rules**:
   - Same process but with Action: "Sell"
5. **Set Tickers**:
   - Add symbols like SPY, QQQ, IWM
6. **Save Strategy**:
   - Click "Save Strategy"
   - Enter a name and version

### Example Strategy JSON

Strategies are stored as JSON files in `strategies/`:

```json
{
  "id": "ema-crossover-v1",
  "name": "EMA Crossover Strategy",
  "version": "1.0.0",
  "description": "Simple EMA crossover with VIX filter",
  "tickers": ["SPY", "QQQ"],
  "rules": [
    {
      "id": "vix-filter",
      "name": "VIX Filter",
      "scope": "global",
      "action": "filter",
      "condition": {
        "type": "less_than",
        "indicator_a": {"type": "vix"},
        "threshold": 25.0
      }
    },
    {
      "id": "ema-cross-buy",
      "name": "EMA Crossover Buy",
      "scope": "per_ticker",
      "action": "buy",
      "condition": {
        "type": "crosses_above",
        "indicator_a": {"type": "ema", "length": 9},
        "indicator_b": {"type": "ema", "length": 21}
      }
    }
  ]
}
```

## 🔄 Hot-Reload Workflow

1. **Start the bot** with `./run_bot.sh`
2. **Open the UI** with `./run_ui.sh`
3. **Edit strategy** in the Strategy Builder tab
4. **Click "Apply"** to hot-reload the strategy
5. The bot will pick up changes within seconds

## 🧪 Testing

### Run All Tests
```bash
pytest
```

### Run Unit Tests Only
```bash
pytest tests/unit/
```

### Run Integration Tests
```bash
pytest tests/integration/
```

### Run with Coverage
```bash
pytest --cov=src --cov-report=html
```

## 📁 Project Structure

```
tws_traderbot/
├── config/
│   ├── default.yaml           # Default configuration
│   └── environment/           # Environment-specific configs
├── data/
│   └── sample/                # Sample market data for testing
├── docs/
│   └── strategy_guide.md      # Comprehensive strategy documentation
├── logs/                      # Application logs
├── src/
│   ├── bot/
│   │   ├── backtest_runner.py # Backtesting engine
│   │   ├── live_runner.py     # Live trading with hot-reload
│   │   ├── state.py           # Bot state management
│   │   └── strategy/
│   │       ├── base.py        # DynamicRuleStrategy
│   │       ├── rules/
│   │       │   ├── engine.py    # Rule evaluation engine
│   │       │   ├── evaluator.py # History evaluation
│   │       │   ├── models.py    # Pydantic data models
│   │       │   └── serialization.py
│   │       └── validator.py   # Strategy validation
│   ├── config/
│   │   ├── settings.py        # Configuration loading
│   │   └── validation.py      # Config validation
│   ├── ui/
│   │   ├── main.py            # Streamlit entry point
│   │   ├── components/
│   │   │   ├── rule_chart.py  # Mini-charts for rules
│   │   │   └── rule_display.py
│   │   └── tabs/
│   │       ├── monitoring.py  # Live trading tab
│   │       └── strategy.py    # Strategy builder tab
│   └── utils/
│       ├── indicators.py      # Technical indicators
│       ├── logger.py          # Enhanced logging
│       └── market_hours.py    # Market hours utilities
├── strategies/                # Saved strategy JSON files
├── tests/
│   ├── unit/                  # Unit tests
│   ├── integration/           # Integration tests
│   └── fixtures/              # Test fixtures
├── run_bot.sh                 # Start trading bot
├── run_ui.sh                  # Start Streamlit UI
├── requirements.txt           # Python dependencies
└── README.md                  # This file
```

## ⚙️ Configuration

### default.yaml
```yaml
trading:
  initial_capital: 100000
  max_position_size: 0.1        # 10% of capital per position
  stop_loss_percent: 2.0
  take_profit_percent: 4.0

tws:
  host: "127.0.0.1"
  port: 7497
  client_id: 1

logging:
  level: INFO
  file: logs/trading.log
```

### Environment Variables
| Variable | Description | Default |
|----------|-------------|---------|
| `TWS_HOST` | TWS hostname | 127.0.0.1 |
| `TWS_PORT` | TWS API port | 7497 |
| `TWS_CLIENT_ID` | TWS client ID | 1 |
| `ACCOUNT_ID` | IB account ID | - |
| `LOG_LEVEL` | Logging level | INFO |

## 🔧 Troubleshooting

### Common Issues

**Bot can't connect to TWS**
- Ensure TWS is running and logged in
- Check API is enabled in TWS settings
- Verify port number matches (7497 paper, 7496 live)
- Make sure no other clients are using the same client ID

**Strategy not loading**
- Check JSON syntax in strategy file
- Validate with `python -c "from src.bot.strategy.rules.serialization import load_strategy; load_strategy('path/to/strategy.json')"`
- Check logs for validation errors

**UI not showing live data**
- Ensure bot is running
- Check that state file exists in `.state/`
- Verify market is open (US Eastern time)

**Hot-reload not working**
- Ensure `.reload_signal` file is being created
- Check bot logs for reload messages
- Verify strategy validates before saving

### Getting Help

1. Check the [Strategy Guide](docs/strategy_guide.md)
2. Review logs in `logs/` directory
3. Run tests to verify installation: `pytest`
4. Open an issue on GitHub

## 📈 Performance Tips

- **Start with Paper Trading**: Always test strategies on paper account first
- **Use Filters**: Add VIX and time filters to avoid choppy markets
- **Monitor Logs**: Check `logs/trading.log` for real-time information
- **Backtest First**: Run backtests before deploying strategies live

## 🛡️ Risk Disclaimer

**Trading involves substantial risk of loss and is not suitable for all investors.**

This software is provided for educational and informational purposes only. It is not financial advice. Always:
- Start with paper trading
- Never trade with money you can't afford to lose
- Understand the strategies you're using
- Monitor your positions actively

## 📄 License

MIT License - see LICENSE file for details.

## 🤝 Contributing

Contributions are welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Add tests for new features
4. Submit a pull request

## 📚 Resources

- [Nautilus Trader Documentation](https://nautilustrader.io/docs/)
- [Interactive Brokers API](https://interactivebrokers.github.io/tws-api/)
- [Streamlit Documentation](https://docs.streamlit.io/)
- [TradingView](https://www.tradingview.com/) - UI design inspiration