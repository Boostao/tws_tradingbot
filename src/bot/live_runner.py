"""
Live Trading Runner Module

Main entry point for running the trading bot in live mode with
Interactive Brokers via TWS/IB Gateway.

Usage:
    python -m src.bot.live_runner [--config CONFIG_PATH] [--strategy STRATEGY_PATH]
    
    or via the run_bot.sh script:
    
    ./run_bot.sh
"""

import argparse
import logging
from datetime import datetime, timezone
import signal
import sys
import time
import threading
from pathlib import Path
from typing import Optional, List

from src.config.settings import Settings, load_config
from src.bot.adapter import IBAdapter, IBConnectionConfig, check_ib_connection, NAUTILUS_IB_AVAILABLE
from src.bot.strategy.base import (
    DynamicRuleStrategy,
    DynamicRuleStrategyConfig,
    NautilusDynamicRuleStrategy,
    NautilusDynamicRuleStrategyConfig,
    NAUTILUS_AVAILABLE,
)
from src.bot.strategy.rules.serialization import load_strategy
from src.bot.tws_data_provider import TWSDataProvider
from src.utils.logger import setup_logging
from src.utils.notifications import NotificationManager
from src.bot.state import (
    read_state,
    write_stop_signal,
    write_emergency_stop,
    check_start_command,
    clear_start_command,
    check_stop_signal,
    clear_stop_signals,
    BotStatus,
    update_state as update_bot_state_file,
)


logger = logging.getLogger(__name__)


# Reload signal file path
RELOAD_SIGNAL_FILE = Path(__file__).parent.parent.parent / "config" / ".reload_signal"


class LiveTradingRunner:
    """
    Main runner for live trading operations.
    
    Handles:
    - Configuration loading
    - IB connection verification
    - Strategy initialization
    - Trading node lifecycle
    - Graceful shutdown
    """
    
    def __init__(
        self,
        config_path: Optional[str] = None,
        strategy_path: Optional[str] = None,
    ):
        """
        Initialize the live trading runner.
        
        Args:
            config_path: Path to config YAML file (uses default if None)
            strategy_path: Path to strategy JSON file (uses default if None)
        """
        # Load settings using ConfigLoader
        self.settings = load_config()
        
        # Strategy path
        default_strategy_path = self.settings.app.active_strategy_path
        self.strategy_path = strategy_path or str(
            Path(__file__).parent.parent.parent / default_strategy_path
        )
        
        # Components
        self.adapter: Optional[IBAdapter] = None
        self.strategy: Optional[DynamicRuleStrategy] = None
        self.node: Optional[object] = None
        self._tws_provider: Optional[TWSDataProvider] = None
        self._notifier = NotificationManager(self.settings)
        
        # State
        self._running = False
        self._shutdown_requested = False
        
        # Setup signal handlers
        signal.signal(signal.SIGINT, self._signal_handler)
        signal.signal(signal.SIGTERM, self._signal_handler)
        
        # Reload state
        self._reload_lock = threading.Lock()
        self._reload_requested = False
        self._reload_watcher_thread: Optional[threading.Thread] = None
        
        logger.info("LiveTradingRunner initialized")
    
    def _signal_handler(self, signum: int, frame) -> None:
        """Handle shutdown signals."""
        signal_name = signal.Signals(signum).name
        logger.info(f"Received {signal_name}, initiating graceful shutdown...")
        self._shutdown_requested = True
    
    def _start_reload_watcher(self) -> None:
        """Start the reload signal file watcher thread."""
        self._reload_watcher_thread = threading.Thread(
            target=self._reload_watcher_loop,
            name="ReloadWatcher",
            daemon=True,
        )
        self._reload_watcher_thread.start()
        logger.info("Reload watcher thread started")
    
    def _reload_watcher_loop(self) -> None:
        """
        Background thread that watches for reload signal file.
        
        Checks for .reload_signal file every 0.5 seconds.
        When found, sets _reload_requested flag and removes the file.
        """
        logger.debug(f"Watching for reload signal at: {RELOAD_SIGNAL_FILE}")
        
        while not self._shutdown_requested:
            try:
                if RELOAD_SIGNAL_FILE.exists():
                    logger.info("Reload signal detected!")
                    
                    # Set reload flag thread-safely
                    with self._reload_lock:
                        self._reload_requested = True
                    
                    # Remove the signal file
                    try:
                        RELOAD_SIGNAL_FILE.unlink()
                        logger.debug("Reload signal file removed")
                    except OSError as e:
                        logger.warning(f"Could not remove reload signal file: {e}")
                    
            except Exception as e:
                logger.warning(f"Error in reload watcher: {e}")
            
            # Sleep before next check
            time.sleep(0.5)
        
        logger.debug("Reload watcher thread stopping")
    
    def _check_and_handle_reload(self) -> bool:
        """
        Check if a reload is requested and handle it.
        
        Thread-safe check and reset of reload flag.
        
        Returns:
            True if reload was handled, False otherwise
        """
        reload_needed = False
        
        with self._reload_lock:
            if self._reload_requested:
                reload_needed = True
                self._reload_requested = False
        
        if reload_needed:
            return self._reload_strategy()
        
        return False
    
    def _reload_strategy(self) -> bool:
        """
        Reload the strategy configuration from disk.
        
        This performs a thread-safe hot-reload of the strategy:
        1. Stop current strategy
        2. Load new strategy config
        3. Create new strategy instance
        4. Start new strategy
        
        Returns:
            True if reload successful, False otherwise
        """
        logger.info("=" * 40)
        logger.info("HOT-RELOADING STRATEGY")
        logger.info("=" * 40)
        
        try:
            # Check if strategy file exists
            strategy_path = Path(self.strategy_path)
            if not strategy_path.exists():
                logger.error(f"Strategy file not found: {strategy_path}")
                return False
            
            # Stop current strategy
            if self.strategy:
                logger.info("Stopping current strategy...")
                self.strategy.on_stop()
            
            # Create new strategy config
            strategy_model = None
            try:
                strategy_model = load_strategy(strategy_path)
            except Exception as e:
                logger.warning(f"Failed to load strategy tickers: {e}")
            
            instruments = self._get_target_instruments(strategy_model)
            
            formatted_instruments = []
            for ticker in instruments:
                if "." not in ticker:
                    formatted_instruments.append(f"{ticker}.ARCA")
                else:
                    formatted_instruments.append(ticker)

            if strategy_model:
                extra_symbols = self._get_required_symbols(strategy_model)
                for symbol in extra_symbols:
                    if symbol == "VIX":
                        formatted_instruments.append("VIX.CBOE")
                    elif "." not in symbol:
                        formatted_instruments.append(f"{symbol}.ARCA")
                    else:
                        formatted_instruments.append(symbol)
                formatted_instruments = sorted(set(formatted_instruments))
            
            max_position_per_instrument = self.settings.get("risk.max_position_size")
            if max_position_per_instrument is None:
                if strategy_model and strategy_model.initial_capital:
                    max_position_per_instrument = strategy_model.initial_capital * self.settings.risk.max_position_pct
                else:
                    max_position_per_instrument = 10000.0

            strategy_config = DynamicRuleStrategyConfig(
                strategy_id="live_dynamic_strategy",
                strategy_config_path=self.strategy_path,
                instruments=formatted_instruments,
                max_position_per_instrument=max_position_per_instrument,
                use_equal_allocation=True,
            )
            
            # Create new strategy instance
            self.strategy = DynamicRuleStrategy(strategy_config)

            if self._tws_provider:
                self.strategy.set_order_handlers(
                    submit_buy=self._submit_buy_order,
                    submit_sell=self._submit_sell_order,
                    cancel_order=self._cancel_order,
                )
            
            # Start new strategy
            logger.info("Starting new strategy...")
            self.strategy.on_start()
            
            logger.info("Strategy hot-reload complete!")
            logger.info(f"Loaded strategy: {strategy_config.strategy_id}")
            logger.info(f"From file: {self.strategy_path}")
            
            return True
            
        except Exception as e:
            logger.error(f"Strategy reload failed: {e}", exc_info=True)
            return False
    
    def _get_target_instruments(self, strategy_model) -> List[str]:
        """
        Determine which instruments to trade.
        Priority:
        1. Watchlist file (config/watchlist.txt)
        2. Empty list (no tickers)
        """
        # 1. Watchlist
        watchlist_path = Path(__file__).parent.parent.parent / "config" / "watchlist.txt"
        if watchlist_path.exists():
            try:
                tickers = []
                with open(watchlist_path, "r") as f:
                    for line in f:
                        line = line.strip()
                        if not line: continue
                        # Parse TICKER:MARKET format
                        if ":" in line:
                            tickers.append(line.split(":")[0])
                        else:
                            tickers.append(line)
                if tickers:
                    logger.info(f"Using {len(tickers)} instruments from watchlist")
                    return tickers
            except Exception as e:
                logger.warning(f"Failed to read watchlist: {e}")
                
        # 2. Default to empty (bot will sit idle until watchlist is populated)
        return []

    def check_prerequisites(self) -> bool:
        """
        Check all prerequisites before starting.
        
        Returns:
            True if all checks pass
        """
        logger.info("Checking prerequisites...")
        
        # Check 1: Nautilus Trader and IB adapter available
        if not NAUTILUS_AVAILABLE:
            logger.warning(
                "Nautilus Trader not installed. "
                "Running in simulation mode. "
                "Install with: pip install nautilus_trader"
            )
            # Continue anyway - we can run in simulation mode
        elif not NAUTILUS_IB_AVAILABLE:
            logger.warning(
                "Nautilus Trader IB adapter not available (ibapi compatibility issue). "
                "Running in simulation mode."
            )
            # Continue anyway - we can run in simulation mode
        
        # Check 2: Strategy file exists
        strategy_path = Path(self.strategy_path)
        if not strategy_path.exists():
            logger.error(f"Strategy file not found: {strategy_path}")
            logger.info("Create a strategy using the UI or provide a valid strategy JSON file")
            return False
        
        # Check 3: IB Connection
        ib_config = self.settings.ib
        logger.info(f"Checking IB connection: {ib_config.host}:{ib_config.port}")
        
        connection_result = check_ib_connection(
            host=ib_config.host,
            port=ib_config.port,
            timeout=ib_config.timeout,
        )
        
        if not connection_result["connected"]:
            logger.error(f"IB connection failed: {connection_result['message']}")
            logger.info("Please ensure TWS or IB Gateway is running and API connections are enabled")
            return False
        
        logger.info("All prerequisites passed")
        return True
    
    def setup(self) -> bool:
        """
        Set up the trading components.
        
        Returns:
            True if setup successful
        """
        logger.info("Setting up trading components...")
        
        try:
            # Create IB adapter
            self.adapter = IBAdapter(self.settings)
            
            # Determine instruments from strategy, watchlist or settings
            strategy_model = None
            try:
                strategy_model = load_strategy(Path(self.strategy_path))
            except Exception as e:
                logger.warning(f"Failed to load strategy tickers: {e}")
            
            instruments = self._get_target_instruments(strategy_model)
            
            # Format instruments for IB (add venue)
            formatted_instruments = []
            for ticker in instruments:
                if "." not in ticker:
                    # Add default venue
                    formatted_instruments.append(f"{ticker}.ARCA")
                else:
                    formatted_instruments.append(ticker)

            if strategy_model:
                extra_symbols = self._get_required_symbols(strategy_model)
                for symbol in extra_symbols:
                    if symbol == "VIX":
                        formatted_instruments.append("VIX.CBOE")
                    elif "." not in symbol:
                        formatted_instruments.append(f"{symbol}.ARCA")
                    else:
                        formatted_instruments.append(symbol)
                formatted_instruments = sorted(set(formatted_instruments))
            
            # Create strategy
            max_position_per_instrument = self.settings.get("risk.max_position_size")
            if max_position_per_instrument is None:
                if strategy_model and strategy_model.initial_capital:
                    max_position_per_instrument = strategy_model.initial_capital * self.settings.risk.max_position_pct
                else:
                    max_position_per_instrument = 10000.0

            if NAUTILUS_AVAILABLE and NAUTILUS_IB_AVAILABLE:
                strategy_config = NautilusDynamicRuleStrategyConfig(
                    strategy_config_path=self.strategy_path,
                    instruments=formatted_instruments,
                    max_position_per_instrument=max_position_per_instrument,
                    use_equal_allocation=True,
                )
                self.strategy = NautilusDynamicRuleStrategy(strategy_config)
            else:
                strategy_config = DynamicRuleStrategyConfig(
                    strategy_id="live_dynamic_strategy",
                    strategy_config_path=self.strategy_path,
                    instruments=formatted_instruments,
                    max_position_per_instrument=max_position_per_instrument,
                    use_equal_allocation=True,
                )
                self.strategy = DynamicRuleStrategy(strategy_config)

                self._tws_provider = TWSDataProvider(
                    host=self.settings.ib.host,
                    port=self.settings.ib.port,
                    client_id=self.settings.ib.client_id + 1,
                )
                if not self._tws_provider.connect():
                    logger.error("Failed to connect to TWS for order execution")
                    return False
                self.strategy.set_order_handlers(
                    submit_buy=self._submit_buy_order,
                    submit_sell=self._submit_sell_order,
                    cancel_order=self._cancel_order,
                )
                self.strategy.set_tws_provider(self._tws_provider)
            
            logger.info(f"Strategy created: {strategy_config.strategy_id}")
            logger.info(f"Trading instruments: {formatted_instruments}")
            logger.info(f"Strategy file: {self.strategy_path}")
            
            return True
            
        except Exception as e:
            logger.error(f"Setup failed: {e}")
            return False
    
    def _update_heartbeat(self):
        """Update the last_heartbeat timestamp in the state file."""
        try:
            state = read_state()
            state.last_heartbeat = datetime.now(timezone.utc).isoformat()
            if state.status == BotStatus.STOPPED.value:
                # Also update last_update if stopped, so the UI knows we're alive
                # but not necessarily processing market data
                state.last_update = state.last_heartbeat
            update_bot_state_file(state)
        except Exception:
            pass

    def run(self) -> int:
        """
        Run the live trading bot.
        
        Returns:
            Exit code (0 for success, non-zero for errors)
        """
        logger.info("=" * 60)
        logger.info("STARTING LIVE TRADING BOT (IDLE MODE)")
        logger.info("=" * 60)
        
        # Log configuration
        ib_config = self.settings.ib
        logger.info(f"Trading Mode: {ib_config.trading_mode.upper()}")
        logger.info(f"IB Connection: {ib_config.host}:{ib_config.port}")
        logger.info(f"Account: {ib_config.account or 'Default'}")

        self._notifier.notify_status("⏸️ Bot waiting for start", ib_config.trading_mode.upper())
        self._notifier.start_command_listener(self._handle_command)
        
        # Initially update status to STOPPED
        try:
            state = read_state()
            state.status = BotStatus.STOPPED.value
            update_bot_state_file(state)
        except Exception:
            pass

        # Main Control Loop
        exit_code = 0
        self._running = False
        
        try:
            while not self._shutdown_requested:
                # 1. IDLE STATE: Wait for START command
                if not self._running:
                    if check_start_command():
                        logger.info("🟢 Start command received")
                        clear_stop_signals() # Clear any residual stops
                        clear_start_command()
                        
                        # Check prerequisites before starting
                        if not self.check_prerequisites():
                            logger.error("Prerequisites check failed - falling back to STOPPED")
                            self._notifier.notify_error("Prerequisites check failed")
                            # Update state back to stopped
                            try:
                                state = read_state()
                                state.status = BotStatus.STOPPED.value
                                update_bot_state_file(state)
                            except: pass
                            time.sleep(1) # prevent busy loop
                            continue
                            
                        # Setup components
                        if not self.setup():
                            logger.error("Setup failed - falling back to STOPPED")
                            self._notifier.notify_error("Setup failed")
                            time.sleep(1)
                            continue
                            
                        # ENTER RUNNING STATE
                        self._running = True
                        self._notifier.notify_status("✅ Bot started", ib_config.trading_mode.upper())
                        
                    else:
                        # Still idling
                        # Periodically update state to indicate liveness
                        self._update_heartbeat()
                        
                        time.sleep(1.0)
                        continue

                # 2. RUNNING STATE
                # If we are here, self._running is True
                try:
                    if NAUTILUS_AVAILABLE and NAUTILUS_IB_AVAILABLE and self.adapter:
                        # Full Nautilus Trader mode with IB
                        logger.info("Starting Nautilus Trader node...")
                        self._start_reload_watcher()
                        self._run_nautilus_mode()
                    else:
                        # Simulation mode (for development/testing or when IB adapter unavailable)
                        logger.info("Running in SIMULATION mode")
                        self._run_simulation_mode()
                    
                    # If execution returns here, it means we stopped (gracefully or due to error)
                    # We should return to IDLE unless shutdown requested
                    logger.info("Trading session ended, returning to IDLE state")
                    self._running = False
                    
                    # Update status to STOPPED
                    try:
                        state = read_state()
                        state.status = BotStatus.STOPPED.value
                        update_bot_state_file(state)
                    except Exception:
                        pass
                    
                    self._notifier.notify_status("🛑 Bot stopped", self.settings.ib.trading_mode.upper())

                except Exception as e:
                    logger.error(f"Error during trading execution: {e}", exc_info=True)
                    self._notifier.notify_error(str(e))
                    self._running = False # Force stop
                    time.sleep(5.0) # Backoff before allowing restart
            
        except KeyboardInterrupt:
            logger.info("Keyboard interrupt received")
        except Exception as e:
            logger.error(f"Error during main loop: {e}", exc_info=True)
            exit_code = 1
        finally:
            self._notifier.stop_command_listener()
            self.shutdown()
        
        return exit_code
    
    def _run_simulation_mode(self) -> None:
        """
        Run in simulation mode without Nautilus Trader.
        
        This is useful for:
        - Development without IB connection
        - Testing strategy logic
        - CI/CD environments
        
        Supports hot-reloading of strategy via reload signal file.
        """
        logger.info("Simulation mode: Strategy running with simulated data")
        
        # Start the reload watcher thread
        self._start_reload_watcher()
        
        # Start the strategy
        self.strategy.on_start()
        
        # Simulation loop
        import random
        from datetime import datetime
        
        base_price = 450.0  # Simulated SPY price
        
        # We check check_stop_signal from LiveRunner explicitly
        # In case the strategy's self-check doesn't propagate up quickly enough
        while not self._shutdown_requested:
            # Check for strategy reload
            self._check_and_handle_reload()
            
            # Check for manual stop signal
            if check_stop_signal():
                logger.info("STOP signal received in simulation loop")
                break
            
            # Generate simulated bar data
            for instrument_id in self.strategy.config.instruments:
                # Random price movement
                change = random.gauss(0, 0.5)
                current_price = base_price + change
                
                simulated_bar = {
                    "instrument_id": instrument_id,
                    "timestamp": datetime.now(),
                    "open": current_price - 0.1,
                    "high": current_price + 0.2,
                    "low": current_price - 0.2,
                    "close": current_price,
                    "volume": random.randint(1000, 10000),
                }
                
                self.strategy.on_bar(simulated_bar)
                base_price = current_price
            
            # Update heartbeat during simulation
            self._update_heartbeat()
            
            # Sleep between iterations
            time.sleep(1.0)
        
        # Stop the strategy
        self.strategy.on_stop()
    
    def _run_nautilus_mode(self) -> None:
        """
        Run in Nautilus Trader mode with hot-reload support.
        """
        import threading
        
        while not self._shutdown_requested:
            # Create and start node
            self.node = self.adapter.create_trading_node(
                strategy=self.strategy,
                instruments=self.strategy.config.instruments,
            )
            
            if not self.node:
                logger.error("Failed to create trading node")
                break
            
            # Run node in a separate thread
            node_thread = threading.Thread(target=self.node.run, name="NautilusNode")
            node_thread.start()
            
            # Wait for node to finish or reload
            while node_thread.is_alive() and not self._shutdown_requested:
                # Check for reload
                if self._check_and_handle_reload():
                    logger.info("Reload detected, stopping current node...")
                    # Stop the node (assuming it has a stop method)
                    try:
                        self.node.dispose()
                    except Exception as e:
                        logger.warning(f"Error disposing node: {e}")
                    break
                
                # Check for STOP signal
                if check_stop_signal():
                    logger.info("STOP signal received in Nautilus node loop")
                    try:
                        self.node.dispose()
                    except Exception as e:
                        logger.warning(f"Error disposing node: {e}")
                    break

                # Update heartbeat
                self._update_heartbeat()

                time.sleep(1.0)
            
            # Wait for thread to finish
            node_thread.join(timeout=5.0)
            if node_thread.is_alive():
                logger.warning("Node thread did not stop gracefully")
            
            # If we received a stop signal, we should break the outer loop too
            # check_stop_signal might still return true if we didn't clear it (but we didn't clear it inside here)
            # However, run() loop clears it when entering START.
            # Here we just want to exit the function.
            if check_stop_signal():
                break

            # If stopped due to shutdown request, break
            if self._shutdown_requested:
                break
                
            # If node stopped for other reasons (e.g. error, or finished backtest logic?), break
            # Unless we want auto-restart logic here?
            break
        
        logger.info("Nautilus mode stopped")
    
    def shutdown(self) -> None:
        """Gracefully shutdown all components."""
        logger.info("Shutting down...")
        self._running = False
        
        if self.strategy:
            self.strategy.on_stop()
        
        if self.node:
            try:
                self.node.dispose()
            except Exception as e:
                logger.warning(f"Error disposing node: {e}")

        if self._tws_provider:
            try:
                self._tws_provider.disconnect()
            except Exception as e:
                logger.warning(f"Error disconnecting TWS provider: {e}")
        
        logger.info("Shutdown complete")
        self._notifier.notify_status("🛑 Bot stopped", self.settings.ib.trading_mode.upper())

    def _handle_command(self, command: str) -> str:
        cmd = command.split()[0].lower()
        if cmd == "/status":
            state = read_state()
            status = state.status.value if hasattr(state.status, "value") else str(state.status)
            equity = f"${state.equity:,.2f}" if state.equity is not None else "N/A"
            pnl = f"${state.pnl:,.2f}" if state.pnl is not None else "N/A"
            return f"Status: {status}\nEquity: {equity}\nPnL: {pnl}"
        if cmd == "/stop":
            return "Stop signal sent" if write_stop_signal() else "Failed to send stop signal"
        if cmd in {"/force_exit", "/emergency_stop"}:
            return "Emergency stop sent" if write_emergency_stop() else "Failed to send emergency stop"
        return "Unknown command. Available: /status, /stop, /force_exit"

    def _submit_buy_order(self, instrument_id: str, quantity: float) -> Optional[int]:
        if not self._tws_provider:
            logger.error("TWS provider not initialized")
            return None
        symbol = instrument_id.split(".")[0]
        qty_int = max(1, int(quantity))
        order_id = self._tws_provider.place_order(symbol=symbol, action="BUY", quantity=qty_int)
        self._notifier.notify_order("BUY", symbol, qty_int, order_id)
        return order_id

    def _submit_sell_order(self, instrument_id: str, quantity: float) -> Optional[int]:
        if not self._tws_provider:
            logger.error("TWS provider not initialized")
            return None
        symbol = instrument_id.split(".")[0]
        qty_int = max(1, int(quantity))
        order_id = self._tws_provider.place_order(symbol=symbol, action="SELL", quantity=qty_int)
        self._notifier.notify_order("SELL", symbol, qty_int, order_id)
        return order_id

    def _cancel_order(self, order_id: str) -> None:
        if not self._tws_provider:
            logger.error("TWS provider not initialized")
            return
        try:
            self._tws_provider.cancel_order(int(order_id))
            self._notifier.notify(f"Canceled order {order_id}")
        except ValueError:
            logger.warning(f"Invalid order id: {order_id}")

    def _get_required_symbols(self, strategy_model) -> List[str]:
        """Collect symbol overrides (e.g., VIX) used by rule indicators."""
        required: List[str] = []
        for rule in strategy_model.rules:
            indicators = [rule.condition.indicator_a]
            if rule.condition.indicator_b:
                indicators.append(rule.condition.indicator_b)
            for indicator in indicators:
                indicator_type = indicator.type.value if hasattr(indicator.type, "value") else indicator.type
                if indicator_type == "vix":
                    required.append("VIX")
                if indicator.symbol:
                    required.append(indicator.symbol)
        return list({s for s in required if s})


def main() -> int:
    """
    Main entry point for the live trading runner.
    
    Returns:
        Exit code
    """
    # Parse arguments
    parser = argparse.ArgumentParser(
        description="Run the live trading bot with Interactive Brokers",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
    # Run with default configuration
    python -m src.bot.live_runner
    
    # Run with custom config
    python -m src.bot.live_runner --config config/production.yaml
    
    # Run with specific strategy
    python -m src.bot.live_runner --strategy config/my_strategy.json
    
    # Verbose logging
    python -m src.bot.live_runner --verbose
        """
    )
    
    parser.add_argument(
        "--config",
        "-c",
        type=str,
        help="Path to configuration YAML file",
        default=None,
    )
    
    parser.add_argument(
        "--strategy",
        "-s",
        type=str,
        help="Path to strategy JSON file",
        default=None,
    )
    
    parser.add_argument(
        "--verbose",
        "-v",
        action="store_true",
        help="Enable verbose logging",
    )
    
    parser.add_argument(
        "--check",
        action="store_true",
        help="Only check prerequisites, don't start trading",
    )
    
    args = parser.parse_args()
    
    # Setup logging
    log_level = logging.DEBUG if args.verbose else logging.INFO
    setup_logging(level=log_level)
    
    # Log startup info
    logger.info("=" * 60)
    logger.info("TWS TRADERBOT - Live Trading Runner")
    logger.info("=" * 60)
    
    # Create runner
    runner = LiveTradingRunner(
        config_path=args.config,
        strategy_path=args.strategy,
    )
    
    # Check-only mode
    if args.check:
        if runner.check_prerequisites():
            logger.info("All checks passed!")
            return 0
        else:
            logger.error("Some checks failed")
            return 1
    
    # Run the bot
    return runner.run()


if __name__ == "__main__":
    sys.exit(main())
