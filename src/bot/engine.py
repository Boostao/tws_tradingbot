"""
Trading Bot Engine

Main orchestration logic for the trading bot.
Handles the main loop, stock selection, and decision making.
"""

import time
import threading
from typing import List, Dict, Optional
from datetime import datetime, time as dt_time

from src.config import config
from src.utils import get_logger
from src.utils.market_hours import market_hours
from src.bot.data_provider import data_provider
from src.bot.strategy import buy_decision, sell_decision
from src.bot.order_manager import order_manager
from src.bot.risk_manager import risk_manager

logger = get_logger(__name__)


class TradingBotEngine:
    """Main trading bot engine."""

    def __init__(self):
        self.running = False
        self.thread: Optional[threading.Thread] = None
        self.stock_list: List[str] = []
        self.last_refresh = 0
        self.refresh_interval = config.get('bot.refresh_interval', 300)  # 5 minutes

    def start(self):
        """Start the trading bot."""
        if not self.running:
            self.running = True
            self.thread = threading.Thread(target=self._main_loop, daemon=True)
            self.thread.start()
            logger.info("Trading bot engine started")

    def stop(self):
        """Stop the trading bot."""
        if self.running:
            self.running = False
            if self.thread:
                self.thread.join(timeout=10)
            logger.info("Trading bot engine stopped")

    def is_running(self) -> bool:
        """Check if bot is running."""
        return self.running

    def _main_loop(self):
        """Main trading loop."""
        logger.info("Starting main trading loop")

        while self.running:
            try:
                # Check if market is open
                if not market_hours.is_market_open():
                    logger.info("Market is closed, sleeping...")
                    time.sleep(60)
                    continue

                # Refresh stock list periodically
                current_time = time.time()
                if current_time - self.last_refresh > self.refresh_interval:
                    self._refresh_stock_list()
                    self.last_refresh = current_time

                # Process each stock
                for symbol in self.stock_list:
                    self._process_stock(symbol)

                # Sleep for 1 minute
                time.sleep(60)

            except Exception as e:
                logger.error(f"Error in main loop: {e}")
                time.sleep(5)  # Brief pause before retry

    def _refresh_stock_list(self):
        """Refresh the list of stocks to trade."""
        try:
            # TODO: Implement wishlist retrieval from TWS
            # For now, use a placeholder list
            self.stock_list = config.get('bot.default_stocks', ['AAPL', 'GOOGL', 'MSFT'])
            logger.info(f"Refreshed stock list: {self.stock_list}")
        except Exception as e:
            logger.error(f"Error refreshing stock list: {e}")

    def _process_stock(self, symbol: str):
        """Process a single stock."""
        try:
            # Get current position
            position = order_manager.get_position(symbol)

            if position == 0:
                # No position, check buy decision
                if buy_decision(symbol):
                    self._execute_buy(symbol)
            else:
                # Have position, check sell decision
                if sell_decision(symbol):
                    self._execute_sell(symbol)

        except Exception as e:
            logger.error(f"Error processing {symbol}: {e}")

    def _execute_buy(self, symbol: str):
        """Execute a buy order."""
        try:
            # Check risk limits
            if not risk_manager.can_buy(symbol):
                logger.info(f"Risk manager blocked buy for {symbol}")
                return

            # Calculate quantity based on equal allocation
            capital = config.get('bot.starting_capital', 100000)
            num_stocks = len(self.stock_list)
            allocation = capital / num_stocks

            # Get current price
            market_data = data_provider.get_market_data(symbol)
            if not market_data or 'price' not in market_data:
                logger.warning(f"No price data for {symbol}")
                return

            price = market_data['price']
            quantity = int(allocation // price)

            if quantity <= 0:
                logger.info(f"Insufficient capital for {symbol} at ${price}")
                return

            # Place order
            success = order_manager.place_buy_order(symbol, quantity)
            if success:
                logger.info(f"Buy order placed: {quantity} shares of {symbol} at ~${price}")
                risk_manager.record_buy(symbol, quantity, price)
            else:
                logger.error(f"Failed to place buy order for {symbol}")

        except Exception as e:
            logger.error(f"Error executing buy for {symbol}: {e}")

    def _execute_sell(self, symbol: str):
        """Execute a sell order."""
        try:
            # Get current position
            position = order_manager.get_position(symbol)
            if position <= 0:
                logger.warning(f"No position to sell for {symbol}")
                return

            # Place sell order
            success = order_manager.place_sell_order(symbol, position)
            if success:
                logger.info(f"Sell order placed: {position} shares of {symbol}")
                risk_manager.record_sell(symbol, position)
            else:
                logger.error(f"Failed to place sell order for {symbol}")

        except Exception as e:
            logger.error(f"Error executing sell for {symbol}: {e}")


# Global engine instance
engine = TradingBotEngine()