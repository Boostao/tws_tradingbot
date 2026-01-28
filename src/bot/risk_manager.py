"""
Risk Manager Module

Enforces trading limits and risk controls.
"""

from typing import Dict, Optional
import time

from src.config import config
from src.utils import get_logger

logger = get_logger(__name__)


class RiskManager:
    """Manages trading risk and enforces limits."""

    def __init__(self):
        self.max_position_size = config.get('risk.max_position_size', 0.1)  # 10% of capital
        self.max_daily_loss = config.get('risk.max_daily_loss', 0.05)  # 5% daily loss
        self.starting_capital = config.get('bot.starting_capital', 100000)
        self.daily_pnl = 0.0
        self.last_reset = time.time()
        self.positions: Dict[str, Dict] = {}  # symbol -> position details

    def can_buy(self, symbol: str) -> bool:
        """Check if buying is allowed for the symbol."""
        try:
            # Check daily loss limit
            if self.daily_pnl < -self.starting_capital * self.max_daily_loss:
                logger.warning(f"Daily loss limit reached: ${self.daily_pnl:.2f}")
                return False

            # Check position size limit
            current_position = self.positions.get(symbol, {}).get('quantity', 0)
            if current_position > 0:
                logger.info(f"Already have position in {symbol}")
                return False

            # Check total exposure
            total_exposure = sum(pos.get('value', 0) for pos in self.positions.values())
            if total_exposure >= self.starting_capital * 0.8:  # 80% max exposure
                logger.warning("Maximum exposure reached")
                return False

            return True

        except Exception as e:
            logger.error(f"Error checking buy permission for {symbol}: {e}")
            return False

    def record_buy(self, symbol: str, quantity: int, price: float):
        """Record a buy transaction."""
        try:
            value = quantity * price
            self.positions[symbol] = {
                'quantity': quantity,
                'avg_price': price,
                'value': value,
                'timestamp': time.time()
            }
            logger.info(f"Recorded buy: {quantity} {symbol} @ ${price:.2f}")
        except Exception as e:
            logger.error(f"Error recording buy for {symbol}: {e}")

    def record_sell(self, symbol: str, quantity: int):
        """Record a sell transaction."""
        try:
            if symbol in self.positions:
                position = self.positions[symbol]
                avg_price = position['avg_price']
                sell_value = quantity * avg_price  # Simplified, should use actual sell price
                cost_basis = position['quantity'] * avg_price
                pnl = sell_value - cost_basis

                self.daily_pnl += pnl
                del self.positions[symbol]

                logger.info(f"Recorded sell: {quantity} {symbol}, PnL: ${pnl:.2f}")
            else:
                logger.warning(f"No position found for {symbol} to sell")
        except Exception as e:
            logger.error(f"Error recording sell for {symbol}: {e}")

    def get_positions(self) -> Dict[str, Dict]:
        """Get all current positions."""
        return self.positions.copy()

    def get_daily_pnl(self) -> float:
        """Get current daily P&L."""
        return self.daily_pnl

    def reset_daily_pnl(self):
        """Reset daily P&L (call at market open)."""
        self.daily_pnl = 0.0
        self.last_reset = time.time()
        logger.info("Daily P&L reset")

    def check_stop_loss(self, symbol: str, current_price: float) -> bool:
        """Check if stop loss should be triggered."""
        try:
            if symbol in self.positions:
                position = self.positions[symbol]
                avg_price = position['avg_price']
                stop_loss_pct = config.get('risk.stop_loss_pct', 0.05)  # 5% stop loss

                if current_price <= avg_price * (1 - stop_loss_pct):
                    logger.warning(f"Stop loss triggered for {symbol}: ${current_price:.2f} <= ${avg_price * (1 - stop_loss_pct):.2f}")
                    return True
            return False
        except Exception as e:
            logger.error(f"Error checking stop loss for {symbol}: {e}")
            return False


# Global instance
risk_manager = RiskManager()