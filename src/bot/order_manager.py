"""
Order Manager for TWS Trading Bot.
Handles placing buy/sell orders, tracking order status, and managing positions.
"""

import threading
import time
from typing import Dict, Any, Optional, List, Callable
from dataclasses import dataclass
from enum import Enum
from ibapi.client import EClient
from ibapi.wrapper import EWrapper
from ibapi.contract import Contract
from ibapi.order import Order
from ibapi.common import OrderId
from src.config import config
from src.utils import get_logger
from .data_provider import data_provider

logger = get_logger(__name__)


class OrderStatus(Enum):
    """Order status enumeration."""
    PENDING = "PendingSubmit"
    SUBMITTED = "Submitted"
    FILLED = "Filled"
    CANCELLED = "Cancelled"
    INACTIVE = "Inactive"
    UNKNOWN = "Unknown"


class OrderType(Enum):
    """Order type enumeration."""
    MARKET = "MKT"
    LIMIT = "LMT"
    STOP = "STP"
    STOP_LIMIT = "STP LMT"


@dataclass
class OrderInfo:
    """Order information structure."""
    order_id: int
    symbol: str
    action: str  # BUY or SELL
    quantity: int
    order_type: OrderType
    status: OrderStatus
    filled_quantity: int = 0
    avg_fill_price: float = 0.0
    timestamp: float = 0.0
    error_message: str = ""


@dataclass
class Position:
    """Position information structure."""
    symbol: str
    quantity: int
    avg_cost: float
    current_price: float = 0.0
    unrealized_pnl: float = 0.0


class OrderManager:
    """Manages order placement, tracking, and position management."""

    def __init__(self):
        self.data_provider = data_provider
        self.orders: Dict[int, OrderInfo] = {}
        self.positions: Dict[str, Position] = {}
        self._order_lock = threading.RLock()
        self._position_lock = threading.RLock()
        self._next_order_id = 1
        self._order_callbacks: Dict[int, Callable[[OrderInfo], None]] = {}

        # Extend the data provider's wrapper with order methods
        self._extend_wrapper()

    def _extend_wrapper(self):
        """Extend the data provider's wrapper with order-related methods."""
        wrapper = self.data_provider.wrapper

        # Store reference to order manager
        wrapper.order_manager = self

        # Override order-related methods
        original_next_valid_id = wrapper.nextValidId
        def next_valid_id(order_id: int):
            original_next_valid_id(order_id)
            with self._order_lock:
                self._next_order_id = order_id
                logger.info(f"Next valid order ID set to: {order_id}")
        wrapper.nextValidId = next_valid_id

        def order_status(order_id: OrderId, status: str, filled: float, remaining: float,
                        avg_fill_price: float, perm_id: int, parent_id: int, last_fill_price: float,
                        client_id: int, why_held: str, mkt_cap_price: float):
            """Handle order status updates."""
            with self._order_lock:
                if order_id in self.orders:
                    order = self.orders[order_id]
                    try:
                        order.status = OrderStatus(status)
                    except ValueError:
                        order.status = OrderStatus.UNKNOWN
                        logger.warning(f"Unknown order status: {status} for order {order_id}")

                    order.filled_quantity = int(filled)
                    if avg_fill_price > 0:
                        order.avg_fill_price = avg_fill_price

                    logger.info(f"Order {order_id} status: {status}, filled: {filled}/{order.quantity}")

                    # Call callback if registered
                    if order_id in self._order_callbacks:
                        try:
                            self._order_callbacks[order_id](order)
                        except Exception as e:
                            logger.error(f"Error in order callback for order {order_id}: {e}")

                    # Update positions if order is filled
                    if order.status == OrderStatus.FILLED:
                        self._update_position_from_order(order)

        wrapper.orderStatus = order_status

        def open_order(order_id: OrderId, contract: Contract, order: Order, order_state):
            """Handle open order information."""
            with self._order_lock:
                if order_id not in self.orders:
                    # Create order info from open order
                    try:
                        order_type = OrderType(order.orderType)
                    except ValueError:
                        order_type = OrderType.MARKET

                    order_info = OrderInfo(
                        order_id=order_id,
                        symbol=contract.symbol,
                        action=order.action,
                        quantity=order.totalQuantity,
                        order_type=order_type,
                        status=OrderStatus.SUBMITTED,
                        timestamp=time.time()
                    )
                    self.orders[order_id] = order_info
                    logger.info(f"Open order discovered: {order_id} - {order.action} {order.totalQuantity} {contract.symbol}")

        wrapper.openOrder = open_order

        def open_order_end():
            """End of open orders."""
            logger.debug("Open orders received")

        wrapper.openOrderEnd = open_order_end

        def position(account: str, contract: Contract, position: float, avg_cost: float):
            """Handle position updates."""
            with self._position_lock:
                symbol = contract.symbol
                if position != 0:
                    self.positions[symbol] = Position(
                        symbol=symbol,
                        quantity=int(position),
                        avg_cost=avg_cost
                    )
                elif symbol in self.positions:
                    del self.positions[symbol]
                logger.debug(f"Position update: {symbol} = {position} @ {avg_cost}")

        wrapper.position = position

        def position_end():
            """End of positions."""
            logger.debug("Positions received")

        wrapper.positionEnd = position_end

    def _update_position_from_order(self, order: OrderInfo):
        """Update positions based on filled order."""
        with self._position_lock:
            symbol = order.symbol
            if order.action.upper() == "BUY":
                if symbol in self.positions:
                    # Update existing position
                    pos = self.positions[symbol]
                    total_quantity = pos.quantity + order.filled_quantity
                    total_cost = (pos.quantity * pos.avg_cost) + (order.filled_quantity * order.avg_fill_price)
                    pos.quantity = total_quantity
                    pos.avg_cost = total_cost / total_quantity if total_quantity > 0 else 0
                else:
                    # New position
                    self.positions[symbol] = Position(
                        symbol=symbol,
                        quantity=order.filled_quantity,
                        avg_cost=order.avg_fill_price
                    )
            elif order.action.upper() == "SELL":
                if symbol in self.positions:
                    pos = self.positions[symbol]
                    pos.quantity -= order.filled_quantity
                    if pos.quantity <= 0:
                        del self.positions[symbol]
                    # Note: avg_cost remains the same for remaining shares

            logger.info(f"Position updated for {symbol}: {self.positions.get(symbol, 'Closed')}")

    def place_order(self, symbol: str, action: str, quantity: int, order_type: OrderType = OrderType.MARKET,
                   limit_price: Optional[float] = None, stop_price: Optional[float] = None,
                   callback: Optional[Callable[[OrderInfo], None]] = None) -> Optional[int]:
        """
        Place a buy or sell order.

        Args:
            symbol: Stock symbol
            action: "BUY" or "SELL"
            quantity: Number of shares
            order_type: Type of order
            limit_price: Limit price for limit orders
            stop_price: Stop price for stop orders
            callback: Optional callback function called on order status updates

        Returns:
            Order ID if successful, None otherwise
        """
        if not self.data_provider.is_connected():
            logger.error("Cannot place order: not connected to TWS")
            return None

        try:
            # Create contract
            contract = Contract()
            contract.symbol = symbol
            contract.secType = "STK"
            contract.exchange = "SMART"
            contract.currency = "USD"

            # Create order
            order = Order()
            order.action = action.upper()
            order.totalQuantity = quantity
            order.orderType = order_type.value

            if order_type == OrderType.LIMIT and limit_price:
                order.lmtPrice = limit_price
            elif order_type == OrderType.STOP and stop_price:
                order.auxPrice = stop_price
            elif order_type == OrderType.STOP_LIMIT and limit_price and stop_price:
                order.lmtPrice = limit_price
                order.auxPrice = stop_price

            # Get next order ID
            with self._order_lock:
                order_id = self._next_order_id
                self._next_order_id += 1

            # Create order info
            order_info = OrderInfo(
                order_id=order_id,
                symbol=symbol,
                action=action.upper(),
                quantity=quantity,
                order_type=order_type,
                status=OrderStatus.PENDING,
                timestamp=time.time()
            )

            with self._order_lock:
                self.orders[order_id] = order_info
                if callback:
                    self._order_callbacks[order_id] = callback

            # Place the order
            self.data_provider.client.placeOrder(order_id, contract, order)
            logger.info(f"Placed order: {order_id} - {action} {quantity} {symbol} @ {order_type.value}")

            return order_id

        except Exception as e:
            logger.error(f"Error placing order for {symbol}: {e}")
            return None

    def cancel_order(self, order_id: int) -> bool:
        """
        Cancel an open order.

        Args:
            order_id: Order ID to cancel

        Returns:
            True if cancellation request sent, False otherwise
        """
        if not self.data_provider.is_connected():
            logger.error("Cannot cancel order: not connected to TWS")
            return False

        with self._order_lock:
            if order_id not in self.orders:
                logger.warning(f"Order {order_id} not found")
                return False

            order = self.orders[order_id]
            if order.status in [OrderStatus.FILLED, OrderStatus.CANCELLED]:
                logger.warning(f"Cannot cancel order {order_id}: already {order.status.value}")
                return False

        try:
            self.data_provider.client.cancelOrder(order_id)
            logger.info(f"Cancel request sent for order {order_id}")
            return True
        except Exception as e:
            logger.error(f"Error cancelling order {order_id}: {e}")
            return False

    def get_order_status(self, order_id: int) -> Optional[OrderInfo]:
        """
        Get the status of an order.

        Args:
            order_id: Order ID

        Returns:
            OrderInfo if order exists, None otherwise
        """
        with self._order_lock:
            return self.orders.get(order_id)

    def get_all_orders(self) -> List[OrderInfo]:
        """Get all orders."""
        with self._order_lock:
            return list(self.orders.values())

    def get_open_orders(self) -> List[OrderInfo]:
        """Get all open (non-filled, non-cancelled) orders."""
        with self._order_lock:
            return [order for order in self.orders.values()
                   if order.status not in [OrderStatus.FILLED, OrderStatus.CANCELLED]]

    def get_positions(self) -> Dict[str, Position]:
        """Get current positions."""
        with self._position_lock:
            return self.positions.copy()

    def get_position(self, symbol: str) -> int:
        """Get position quantity for a symbol."""
        with self._position_lock:
            position = self.positions.get(symbol)
            return position.quantity if position else 0

    def update_positions(self):
        """Request updated positions from TWS."""
        if not self.data_provider.is_connected():
            logger.error("Cannot update positions: not connected to TWS")
            return

        try:
            self.data_provider.client.reqPositions()
            logger.debug("Requested position updates")
        except Exception as e:
            logger.error(f"Error requesting positions: {e}")

    def update_open_orders(self):
        """Request updated open orders from TWS."""
        if not self.data_provider.is_connected():
            logger.error("Cannot update orders: not connected to TWS")
            return

        try:
            self.data_provider.client.reqOpenOrders()
            logger.debug("Requested open order updates")
        except Exception as e:
            logger.error(f"Error requesting open orders: {e}")

    def get_position_value(self, symbol: str) -> Optional[float]:
        """
        Get the current market value of a position.

        Args:
            symbol: Stock symbol

        Returns:
            Market value if position exists, None otherwise
        """
        with self._position_lock:
            if symbol not in self.positions:
                return None

            position = self.positions[symbol]
            # Try to get current price from data provider
            market_data = self.data_provider.get_market_data(symbol)
            if market_data and 'price' in market_data:
                position.current_price = market_data['price']
                position.unrealized_pnl = (position.current_price - position.avg_cost) * position.quantity

            return position.current_price * position.quantity if position.current_price > 0 else None

    def get_total_portfolio_value(self) -> float:
        """Get total portfolio value including cash."""
        total_value = 0.0

        # Add position values
        for symbol in list(self.positions.keys()):
            value = self.get_position_value(symbol)
            if value:
                total_value += value

        # Add cash (would need account summary for this)
        # For now, return position values only
        return total_value

    def shutdown(self):
        """Shutdown the order manager."""
        # Cancel any open orders
        open_orders = self.get_open_orders()
        for order in open_orders:
            self.cancel_order(order.order_id)

        logger.info("Order manager shutdown complete")


# Global instance
order_manager = OrderManager()