"""
TWS Data Provider Module

Provides real market data from Interactive Brokers TWS/Gateway using the ibapi.
Supports:
- Historical bar data for backtesting
- Watchlist fetching from TWS favorites
- Real-time market data subscriptions
- Contract search and resolution
"""

import logging
import threading
import time
from dataclasses import dataclass, field
from datetime import datetime, timedelta, date, timezone
from enum import Enum
from pathlib import Path
from queue import Queue, Empty
from typing import Any, Dict, List, Optional, Callable
import pandas as pd

# IB API imports
try:
    from ibapi.client import EClient
    from ibapi.wrapper import EWrapper
    from ibapi.contract import Contract
    from ibapi.common import BarData, TickerId, TickAttrib
    from ibapi.order import Order
    from ibapi.scanner import ScannerSubscription
    from ibapi.execution import ExecutionFilter
    IBAPI_AVAILABLE = True
except ImportError:
    IBAPI_AVAILABLE = False
    EClient = object
    EWrapper = object
    ExecutionFilter = object

from src.config.settings import get_settings

logger = logging.getLogger(__name__)


class TWSDuration(Enum):
    """Standard durations for historical data requests."""
    SECONDS_30 = "30 S"
    MINUTES_1 = "60 S"
    MINUTES_5 = "300 S"
    MINUTES_15 = "900 S"
    MINUTES_30 = "1800 S"
    HOUR_1 = "3600 S"
    HOURS_4 = "14400 S"
    DAY_1 = "1 D"
    DAYS_2 = "2 D"
    WEEK_1 = "1 W"
    WEEKS_2 = "2 W"
    MONTH_1 = "1 M"
    MONTHS_3 = "3 M"
    MONTHS_6 = "6 M"
    YEAR_1 = "1 Y"
    YEARS_2 = "2 Y"


class TWSBarSize(Enum):
    """Valid bar sizes for historical data."""
    SECS_1 = "1 secs"
    SECS_5 = "5 secs"
    SECS_10 = "10 secs"
    SECS_15 = "15 secs"
    SECS_30 = "30 secs"
    MIN_1 = "1 min"
    MIN_2 = "2 mins"
    MIN_3 = "3 mins"
    MIN_5 = "5 mins"
    MIN_10 = "10 mins"
    MIN_15 = "15 mins"
    MIN_20 = "20 mins"
    MIN_30 = "30 mins"
    HOUR_1 = "1 hour"
    HOUR_2 = "2 hours"
    HOUR_3 = "3 hours"
    HOUR_4 = "4 hours"
    HOUR_8 = "8 hours"
    DAY_1 = "1 day"
    WEEK_1 = "1 week"
    MONTH_1 = "1 month"


@dataclass
class WatchlistItem:
    """Represents an item from a TWS watchlist."""
    symbol: str
    sec_type: str = "STK"
    exchange: str = "SMART"
    currency: str = "USD"
    name: str = ""
    
    def to_contract(self) -> "Contract":
        """Convert to IB Contract."""
        contract = Contract()
        contract.symbol = self.symbol
        contract.secType = self.sec_type
        contract.exchange = self.exchange
        contract.currency = self.currency
        return contract


@dataclass
class HistoricalDataRequest:
    """Request for historical data."""
    req_id: int
    symbol: str
    contract: Any
    end_datetime: str
    duration: str
    bar_size: str
    what_to_show: str = "TRADES"
    use_rth: bool = True
    completed: bool = False
    bars: List[Dict] = field(default_factory=list)
    error: Optional[str] = None


class TWSDataWrapper(EWrapper):
    """
    IB API Wrapper for receiving data callbacks.
    
    Handles callbacks from TWS for:
    - Historical data bars
    - Contract details
    - Account updates
    - Error messages
    - Scanner results (watchlists)
    """
    
    def __init__(self):
        super().__init__()
        self._historical_data: Dict[int, HistoricalDataRequest] = {}
        self._contracts: Dict[int, List[Any]] = {}
        self._matching_symbols: Dict[int, List[Dict]] = {}  # For reqMatchingSymbols results
        self._watchlist_items: Dict[int, List[WatchlistItem]] = {}
        self._scanner_data: Dict[int, List[Dict]] = {}
        self._positions: List[Dict] = []
        self._account_values: Dict[str, Any] = {}
        self._errors: Dict[int, str] = {}
        self._next_valid_id: Optional[int] = None
        self._connected = False
        self._managed_accounts: List[str] = []

        # Real-time market data
        self._market_data: Dict[int, Dict[str, Any]] = {}
        self._market_data_events: Dict[int, threading.Event] = {}
        self._market_data_symbols: Dict[int, str] = {}
        self._market_data_lock = threading.Lock()
        self._market_data_permission_errors: set[str] = set()

        # Order tracking
        self._orders: Dict[int, Dict[str, Any]] = {}
        self._order_events: Dict[int, threading.Event] = {}
        self._order_lock = threading.Lock()
        self._open_orders_event = threading.Event()
        self._positions_event = threading.Event()

        # Account updates / portfolio
        self._portfolio_positions: List[Dict[str, Any]] = []
        self._portfolio_positions_by_symbol: Dict[str, Dict[str, Any]] = {}
        self._position_entry_times: Dict[str, str] = {}
        self._account_update_event = threading.Event()

        # Executions
        self._executions: List[Dict[str, Any]] = []
        self._executions_event = threading.Event()
        
        # Event queues for synchronization
        self._connection_event = threading.Event()
        self._data_events: Dict[int, threading.Event] = {}
    
    def nextValidId(self, orderId: int):
        """Callback when connection is established and we get our starting order ID."""
        self._next_valid_id = orderId
        self._connected = True
        self._connection_event.set()
        logger.info(f"TWS Connected. Next valid order ID: {orderId}")
    
    def managedAccounts(self, accountsList: str):
        """Callback with list of managed accounts."""
        self._managed_accounts = accountsList.split(",")
        logger.info(f"Managed accounts: {self._managed_accounts}")
    
    def error(self, reqId: TickerId, *args):
        """Handle error messages from TWS across ibapi signature variants."""
        errorCode: Optional[int]
        errorString: Optional[str]
        advancedOrderRejectJson = ""

        # ibapi signatures observed:
        # (reqId, errorCode, errorString)
        # (reqId, errorCode, errorString, advancedOrderRejectJson)
        # (reqId, errorTime, errorCode, errorString, advancedOrderRejectJson)
        if len(args) == 2:
            errorCode, errorString = args
        elif len(args) == 3:
            errorCode, errorString, advancedOrderRejectJson = args
        elif len(args) >= 4:
            _, errorCode, errorString, advancedOrderRejectJson = args[:4]
        else:
            logger.error("TWS error callback received unexpected arguments")
            return
        # Some error codes are informational
        if errorCode in (2104, 2106, 2158, 2119, 2174):  # Market data/connection info
            if errorCode == 2174:
                 logger.warning(f"TWS Warning [{errorCode}]: {errorString}")
            else:
                 logger.debug(f"TWS Info [{errorCode}]: {errorString}")
        elif errorCode == 2100:  # Account update unsubscribed (noisy)
            logger.debug(f"TWS Info [{errorCode}] reqId={reqId}: {errorString}")
            return
        elif errorCode == 10089:  # Market data subscription required
            symbol = self._market_data_symbols.get(reqId)
            if symbol:
                self._market_data_permission_errors.add(symbol.upper())
            if reqId in self._market_data_events:
                self._market_data_events[reqId].set()
            logger.warning(f"TWS Error [{errorCode}] reqId={reqId}: {errorString}")
            return
        elif errorCode == 300:  # Can't find EId with tickerId
            if reqId in self._market_data_events:
                self._market_data_events[reqId].set()
            logger.warning(f"TWS Error [{errorCode}] reqId={reqId}: {errorString}")
            return
        elif errorCode == 200:  # No security definition found
            logger.warning(f"Contract not found for reqId {reqId}: {errorString}")
            self._errors[reqId] = errorString
            if reqId in self._data_events:
                self._data_events[reqId].set()
        elif errorCode == 162:  # Historical data request cancelled
            logger.debug(f"Request {reqId} cancelled")
        elif errorCode == 504:  # Not connected
            logger.error("Not connected to TWS")
            self._connected = False
        else:
            logger.error(f"TWS Error [{errorCode}] reqId={reqId}: {errorString}")
            self._errors[reqId] = errorString
            if reqId in self._data_events:
                self._data_events[reqId].set()
            if reqId in self._market_data_events:
                self._market_data_events[reqId].set()
            if reqId in self._order_events:
                self._order_events[reqId].set()
    
    def connectionClosed(self):
        """Handle connection closed."""
        self._connected = False
        logger.warning("TWS Connection closed")
    
    # Historical Data Callbacks
    
    def historicalData(self, reqId: int, bar: BarData):
        """Receive a single historical bar."""
        if reqId in self._historical_data:
            self._historical_data[reqId].bars.append({
                "timestamp": bar.date,
                "open": bar.open,
                "high": bar.high,
                "low": bar.low,
                "close": bar.close,
                "volume": bar.volume,
                "wap": bar.wap,
                "barCount": bar.barCount,
            })
    
    def historicalDataEnd(self, reqId: int, start: str, end: str):
        """Historical data request completed."""
        if reqId in self._historical_data:
            self._historical_data[reqId].completed = True
            logger.debug(f"Historical data complete for reqId {reqId}: {start} to {end}")
        if reqId in self._data_events:
            self._data_events[reqId].set()
    
    # Contract Details Callbacks
    
    def contractDetails(self, reqId: int, contractDetails):
        """Receive contract details."""
        if reqId not in self._contracts:
            self._contracts[reqId] = []
        self._contracts[reqId].append(contractDetails)
    
    def contractDetailsEnd(self, reqId: int):
        """Contract details request completed."""
        logger.debug(f"Contract details complete for reqId {reqId}")
        if reqId in self._data_events:
            self._data_events[reqId].set()
    
    # Symbol Search Callbacks (reqMatchingSymbols)
    
    def symbolSamples(self, reqId: int, contractDescriptions):
        """
        Receive matching symbol samples from reqMatchingSymbols.
        
        This is the callback for fuzzy symbol search that matches
        the TWS symbol lookup dialog.
        """
        if reqId not in self._matching_symbols:
            self._matching_symbols[reqId] = []
        
        for desc in contractDescriptions:
            contract = desc.contract
            self._matching_symbols[reqId].append({
                "symbol": contract.symbol,
                "sec_type": contract.secType,
                "exchange": contract.primaryExchange,
                "currency": contract.currency,
                "name": desc.derivativeSecTypes[0] if desc.derivativeSecTypes else "",
                # derivativeSecTypes contains derivative types available
            })
        
        logger.debug(f"Received {len(contractDescriptions)} matching symbols for reqId {reqId}")
        if reqId in self._data_events:
            self._data_events[reqId].set()
    
    # Scanner (Watchlist) Callbacks
    
    def scannerData(self, reqId: int, rank: int, contractDetails, distance: str,
                    benchmark: str, projection: str, legsStr: str):
        """Receive scanner data row."""
        if reqId not in self._scanner_data:
            self._scanner_data[reqId] = []
        
        contract = contractDetails.contract
        self._scanner_data[reqId].append({
            "rank": rank,
            "symbol": contract.symbol,
            "sec_type": contract.secType,
            "exchange": contract.exchange,
            "currency": contract.currency,
            "name": contractDetails.longName if hasattr(contractDetails, 'longName') else "",
        })
    
    def scannerDataEnd(self, reqId: int):
        """Scanner data completed."""
        logger.debug(f"Scanner data complete for reqId {reqId}")
        if reqId in self._data_events:
            self._data_events[reqId].set()
    
    # Position Callbacks
    
    def position(self, account: str, contract, pos: float, avgCost: float):
        """Receive position update."""
        self._positions.append({
            "account": account,
            "symbol": contract.symbol,
            "sec_type": contract.secType,
            "position": pos,
            "avg_cost": avgCost,
        })
    
    def positionEnd(self):
        """Position updates complete."""
        logger.debug("Position updates complete")
        self._positions_event.set()
    
    # Account Callbacks
    
    def accountSummary(self, reqId: int, account: str, tag: str, value: str, currency: str):
        """Receive account summary value."""
        key = f"{account}_{tag}"
        self._account_values[key] = {
            "account": account,
            "tag": tag,
            "value": value,
            "currency": currency,
        }
    
    def accountSummaryEnd(self, reqId: int):
        """Account summary complete."""
        if reqId in self._data_events:
            self._data_events[reqId].set()

    def updatePortfolio(
        self,
        contract: Contract,
        position: float,
        marketPrice: float,
        marketValue: float,
        averageCost: float,
        unrealizedPNL: float,
        realizedPNL: float,
        accountName: str,
    ):
        """Receive portfolio updates from TWS account updates."""
        symbol = (contract.symbol or "").upper()
        if not symbol:
            return

        if position == 0:
            self._portfolio_positions_by_symbol.pop(symbol, None)
            self._position_entry_times.pop(symbol, None)
            return

        entry_time = self._position_entry_times.get(symbol)
        if not entry_time:
            entry_time = datetime.utcnow().isoformat()
            self._position_entry_times[symbol] = entry_time

        self._portfolio_positions_by_symbol[symbol] = {
            "account": accountName,
            "symbol": symbol,
            "sec_type": contract.secType,
            "position": position,
            "avg_cost": averageCost,
            "market_price": marketPrice,
            "market_value": marketValue,
            "unrealized_pnl": unrealizedPNL,
            "realized_pnl": realizedPNL,
            "entry_time": entry_time,
            "timestamp": datetime.utcnow().isoformat(),
        }

    def accountDownloadEnd(self, accountName: str):
        """Account update snapshot complete."""
        self._account_update_event.set()

    def execDetails(self, reqId: int, contract: Contract, execution):
        """Receive execution details."""
        try:
            exec_time = getattr(execution, "time", None)
            self._executions.append(
                {
                    "symbol": contract.symbol,
                    "sec_type": contract.secType,
                    "side": getattr(execution, "side", None),
                    "shares": getattr(execution, "shares", None),
                    "price": getattr(execution, "price", None),
                    "exec_id": getattr(execution, "execId", None),
                    "time": exec_time,
                }
            )
        except Exception:
            logger.exception("Failed to capture execution details")

    def execDetailsEnd(self, reqId: int):
        """Executions snapshot complete."""
        self._executions_event.set()

    # Order Callbacks

    def openOrder(self, orderId: int, contract: Contract, order: Order, orderState):
        """Handle open order info."""
        with self._order_lock:
            existing = self._orders.get(orderId, {})
            self._orders.setdefault(orderId, {})
            self._orders[orderId].update(
                {
                    "order_id": orderId,
                    "symbol": contract.symbol,
                    "action": order.action,
                    "quantity": getattr(order, "totalQuantity", None),
                    "order_type": getattr(order, "orderType", None),
                    "price": getattr(order, "lmtPrice", None),
                    "status": getattr(orderState, "status", None),
                    "submitted_time": existing.get("submitted_time")
                    or datetime.now().astimezone().isoformat(),
                }
            )

    def openOrderEnd(self):
        """Signal completion of open orders snapshot."""
        self._open_orders_event.set()

    def orderStatus(
        self,
        orderId: int,
        status: str,
        filled: float,
        remaining: float,
        avgFillPrice: float,
        permId: int,
        parentId: int,
        lastFillPrice: float,
        clientId: int,
        whyHeld: str,
        mktCapPrice: float,
    ):
        """Handle order status updates."""
        with self._order_lock:
            data = self._orders.setdefault(orderId, {"order_id": orderId})
            data.update(
                {
                    "status": status,
                    "filled": filled,
                    "remaining": remaining,
                    "avg_fill_price": avgFillPrice,
                    "last_fill_price": lastFillPrice,
                    "timestamp": datetime.utcnow(),
                }
            )
            if status in {"Submitted", "PreSubmitted"} and not data.get("submitted_time"):
                data["submitted_time"] = datetime.now().astimezone().isoformat()
        if orderId in self._order_events:
            self._order_events[orderId].set()

    # Real-time Market Data Callbacks

    def tickPrice(self, reqId: int, tickType: int, price: float, attrib: TickAttrib):
        """Receive real-time price updates."""
        with self._market_data_lock:
            data = self._market_data.setdefault(reqId, {"symbol": self._market_data_symbols.get(reqId)})
            data["timestamp"] = datetime.utcnow()

            price_map = {
                1: "bid",
                2: "ask",
                4: "last",
                6: "high",
                7: "low",
                9: "close",
                14: "open",
                66: "bid",
                67: "ask",
                68: "last",
                72: "high",
                73: "low",
                75: "close",
            }

            field = price_map.get(tickType)
            if field:
                data[field] = price

    def tickSize(self, reqId: int, tickType: int, size: int):
        """Receive real-time size updates."""
        with self._market_data_lock:
            data = self._market_data.setdefault(reqId, {"symbol": self._market_data_symbols.get(reqId)})
            data["timestamp"] = datetime.utcnow()

            size_map = {
                0: "bid_size",
                3: "ask_size",
                5: "last_size",
                8: "volume",
                69: "bid_size",
                70: "ask_size",
                71: "last_size",
                74: "volume",
            }

            field = size_map.get(tickType)
            if field:
                data[field] = size

    def tickSnapshotEnd(self, reqId: int):
        """Snapshot data complete."""
        if reqId in self._market_data_events:
            self._market_data_events[reqId].set()


class TWSDataClient(TWSDataWrapper, EClient):
    """
    Combined wrapper and client for TWS data operations.
    
    Handles the event-driven IB API with synchronous convenience methods.
    """
    
    def __init__(self):
        TWSDataWrapper.__init__(self)
        EClient.__init__(self, wrapper=self)
        
        self._req_id_counter = 1000
        self._order_id_counter: Optional[int] = None
        self._thread: Optional[threading.Thread] = None
        self._lock = threading.Lock()
    
    def _get_next_req_id(self) -> int:
        """Get next request ID in a thread-safe manner."""
        with self._lock:
            self._req_id_counter += 1
            return self._req_id_counter

    def _get_next_order_id(self, timeout: float = 5.0) -> Optional[int]:
        """Get next order ID from TWS in a thread-safe manner."""
        if self._order_id_counter is None:
            if not self._connection_event.wait(timeout=timeout):
                return None
            with self._lock:
                self._order_id_counter = self._next_valid_id or 1

        with self._lock:
            order_id = self._order_id_counter
            self._order_id_counter += 1
            return order_id
    
    def connect_and_run(
        self,
        host: str = "127.0.0.1",
        port: int = 7497,
        client_id: int = 10,
        timeout: float = 10.0,
    ) -> bool:
        """
        Connect to TWS and start the message processing thread.
        
        Args:
            host: TWS host address
            port: TWS port (7497 paper, 7496 live)
            client_id: Client ID (should be unique)
            timeout: Connection timeout in seconds
            
        Returns:
            True if connected successfully
        """
        if self._connected:
            logger.info("Already connected to TWS")
            return True
        
        try:
            logger.info(f"Connecting to TWS at {host}:{port} with client_id={client_id}")
            self.connect(host, port, client_id)
            
            # Start message processing thread
            self._thread = threading.Thread(target=self.run, daemon=True)
            self._thread.start()
            
            # Wait for connection confirmation
            if self._connection_event.wait(timeout=timeout):
                logger.info("Successfully connected to TWS")
                self._market_data_permission_errors.clear()
                return True
            else:
                logger.error("Connection timeout - is TWS running with API enabled?")
                self.disconnect()
                return False
                
        except Exception as e:
            logger.error(f"Failed to connect to TWS: {e}")
            return False
    
    def disconnect_client(self):
        """Disconnect from TWS."""
        if self._connected:
            self.disconnect()
            self._connected = False
            logger.info("Disconnected from TWS")
        if self._thread and self._thread.is_alive():
            self._thread.join(timeout=5)
        self._thread = None
    
    @property
    def is_connected(self) -> bool:
        """Check if connected to TWS."""
        return self._connected and self.isConnected()


class TWSDataProvider:
    """
    High-level data provider for TWS market data.
    
    Provides convenient methods for:
    - Fetching historical bar data
    - Getting watchlist symbols
    - Resolving contracts
    - Account information
    """
    
    def __init__(
        self,
        host: Optional[str] = None,
        port: Optional[int] = None,
        client_id: int = 10,
    ):
        """
        Initialize the TWS data provider.
        
        Args:
            host: TWS host (uses settings if None)
            port: TWS port (uses settings if None)
            client_id: Client ID for this connection
        """
        if not IBAPI_AVAILABLE:
            raise ImportError(
                "ibapi not installed (module name: ibapi). Install with: pip install nautilus-ibapi"
            )
        
        settings = get_settings()
        self.host = host or settings.ib.host
        self.port = port or settings.ib.port
        self.client_id = client_id
        
        self._client: Optional[TWSDataClient] = None
        self._market_data_subscriptions: Dict[str, int] = {}
    
    def connect(self, timeout: float = 10.0) -> bool:
        """
        Connect to TWS.
        
        Args:
            timeout: Connection timeout
            
        Returns:
            True if connected
        """
        if self._client and self._client.is_connected:
            return True
        
        self._client = TWSDataClient()
        return self._client.connect_and_run(
            host=self.host,
            port=self.port,
            client_id=self.client_id,
            timeout=timeout,
        )
    
    def disconnect(self):
        """Disconnect from TWS."""
        if self._client:
            try:
                for req_id in list(self._market_data_subscriptions.values()):
                    self._client.cancelMktData(req_id)
            except Exception:
                pass
            self._client.disconnect_client()
            self._client = None

    def reconnect(self, timeout: float = 10.0) -> bool:
        """Recycle the connection to TWS."""
        self.disconnect()
        return self.connect(timeout=timeout)
    
    def is_connected(self) -> bool:
        """
        Check connection status without attempting to connect.
        
        Returns:
            True if currently connected to TWS
        """
        return self._client is not None and self._client.is_connected
    
    def _ensure_connected(self) -> bool:
        """Ensure we're connected to TWS."""
        if not self.is_connected:
            return self.connect()
        return True
    
    def create_stock_contract(
        self,
        symbol: str,
        exchange: str = "SMART",
        currency: str = "USD",
    ) -> Contract:
        """
        Create a stock contract.
        
        Args:
            symbol: Stock symbol (e.g., "SPY", "AAPL")
            exchange: Exchange (default SMART for best routing)
            currency: Currency (default USD)
            
        Returns:
            IB Contract object
        """
        contract = Contract()
        contract.symbol = symbol
        contract.secType = "STK"
        contract.exchange = exchange
        contract.currency = currency
        return contract
    
    def create_index_contract(
        self,
        symbol: str,
        exchange: str = "CBOE",
    ) -> Contract:
        """
        Create an index contract (e.g., VIX).
        
        Args:
            symbol: Index symbol
            exchange: Exchange
            
        Returns:
            IB Contract object
        """
        contract = Contract()
        contract.symbol = symbol
        contract.secType = "IND"
        contract.exchange = exchange
        contract.currency = "USD"
        return contract

    def _build_market_data_contract(
        self,
        symbol: str,
        exchange: str = "SMART",
        currency: str = "USD",
    ) -> Contract:
        """Build a contract for market data requests."""
        if symbol.upper() == "VIX":
            return self.create_index_contract("VIX", exchange="CBOE")
        return self.create_stock_contract(symbol, exchange=exchange, currency=currency)

    def _build_order_contract(
        self,
        symbol: str,
        exchange: str = "SMART",
        currency: str = "USD",
    ) -> Contract:
        """Build a contract for order placement."""
        return self._build_market_data_contract(symbol, exchange=exchange, currency=currency)

    def place_order(
        self,
        symbol: str,
        action: str,
        quantity: int,
        order_type: str = "MKT",
        limit_price: Optional[float] = None,
        stop_price: Optional[float] = None,
        exchange: str = "SMART",
        currency: str = "USD",
        tif: str = "DAY",
        timeout: float = 5.0,
    ) -> Optional[int]:
        """
        Place a live order through TWS.

        Args:
            symbol: Ticker symbol
            action: BUY or SELL
            quantity: Number of shares
            order_type: MKT, LMT, STP, STP LMT
            limit_price: Limit price for LMT/STP LMT
            stop_price: Stop price for STP/STP LMT
            exchange: Exchange for the contract
            currency: Currency
            tif: Time in force
            timeout: Timeout waiting for order id

        Returns:
            Order ID if submitted, else None
        """
        if not self._ensure_connected():
            logger.error("Not connected to TWS")
            return None

        if snapshot and symbol.upper() in self._client._market_data_permission_errors:
            return None

        if quantity <= 0:
            logger.error("Order quantity must be positive")
            return None

        order_id = self._client._get_next_order_id(timeout=timeout)
        if order_id is None:
            logger.error("Failed to get valid order id")
            return None

        contract = self._build_order_contract(symbol, exchange=exchange, currency=currency)

        order = Order()
        order.action = action.upper()
        order.totalQuantity = int(quantity)
        order.orderType = order_type
        order.tif = tif

        if order_type == "LMT" and limit_price is not None:
            order.lmtPrice = float(limit_price)
        if order_type in ("STP", "STP LMT") and stop_price is not None:
            order.auxPrice = float(stop_price)
        if order_type == "STP LMT" and limit_price is not None:
            order.lmtPrice = float(limit_price)

        with self._client._order_lock:
            self._client._orders[order_id] = {
                "order_id": order_id,
                "symbol": symbol,
                "action": order.action,
                "quantity": order.totalQuantity,
                "order_type": order.orderType,
                "status": "SUBMITTED",
                "timestamp": datetime.utcnow(),
            }

        self._client.placeOrder(order_id, contract, order)
        logger.info(f"Order submitted: {order_id} {action} {quantity} {symbol} {order_type}")
        return order_id

    def cancel_order(self, order_id: int) -> bool:
        """Cancel an existing order."""
        if not self._client:
            return False
        try:
            self._client.cancelOrder(order_id)
            return True
        except Exception as e:
            logger.warning(f"Cancel order failed for {order_id}: {e}")
            return False

    def get_order_status(self, order_id: int) -> Optional[Dict[str, Any]]:
        """Get the latest cached order status for an order."""
        if not self._client:
            return None
        with self._client._order_lock:
            return self._client._orders.get(order_id)

    def get_open_orders(self, timeout: float = 5.0) -> List[Dict[str, Any]]:
        """Fetch open orders snapshot from TWS."""
        if not self._ensure_connected():
            return []

        with self._client._order_lock:
            prior_orders = self._client._orders.copy()
            self._client._orders = {}

        self._client._open_orders_event.clear()
        self._client.reqAllOpenOrders()

        self._client._open_orders_event.wait(timeout=timeout)

        with self._client._order_lock:
            for order_id, order in self._client._orders.items():
                if not order.get("submitted_time"):
                    prev = prior_orders.get(order_id)
                    if prev and prev.get("submitted_time"):
                        order["submitted_time"] = prev.get("submitted_time")
            return list(self._client._orders.values())

    def subscribe_market_data(
        self,
        symbol: str,
        snapshot: bool = False,
        exchange: str = "SMART",
        currency: str = "USD",
        timeout: float = 5.0,
        market_data_type: Optional[int] = None,
    ) -> Optional[Dict[str, Any]]:
        """
        Subscribe to market data for a symbol.

        Args:
            symbol: Ticker symbol
            snapshot: If True, request a one-time snapshot
            exchange: Exchange for the contract
            currency: Currency for the contract
            timeout: Snapshot timeout

        Returns:
            Snapshot dict if snapshot=True, else None
        """
        if not self._ensure_connected():
            logger.error("Not connected to TWS")
            return None

        req_id = self._client._get_next_req_id()
        contract = self._build_market_data_contract(symbol, exchange=exchange, currency=currency)

        self._client._market_data_symbols[req_id] = symbol
        self._client._market_data[req_id] = {"symbol": symbol, "timestamp": datetime.utcnow()}

        if snapshot:
            self._client._market_data_events[req_id] = threading.Event()

        if market_data_type is not None:
            try:
                self._client.reqMarketDataType(int(market_data_type))
            except Exception as exc:
                logger.debug(f"Failed to set market data type {market_data_type}: {exc}")

        self._client.reqMktData(
            reqId=req_id,
            contract=contract,
            genericTickList="",
            snapshot=snapshot,
            regulatorySnapshot=False,
            mktDataOptions=[],
        )

        if snapshot:
            if not self._client._market_data_events[req_id].wait(timeout=timeout):
                logger.warning(f"Snapshot timeout for {symbol}")
            self._client.cancelMktData(req_id)
            if market_data_type is not None and market_data_type != 1:
                try:
                    self._client.reqMarketDataType(1)
                except Exception as exc:
                    logger.debug(f"Failed to reset market data type: {exc}")
            return self._client._market_data.get(req_id)

        self._market_data_subscriptions[symbol] = req_id
        return None

    def has_market_data_permission_error(self, symbol: str) -> bool:
        if not self._client:
            return False
        return symbol.upper() in self._client._market_data_permission_errors

    def unsubscribe_market_data(self, symbol: str) -> bool:
        """Unsubscribe from streaming market data for a symbol."""
        if not self._client or symbol not in self._market_data_subscriptions:
            return False
        req_id = self._market_data_subscriptions.pop(symbol)
        try:
            self._client.cancelMktData(req_id)
            return True
        except Exception:
            return False

    def get_latest_market_data(self, symbol: str) -> Optional[Dict[str, Any]]:
        """Get the latest cached market data for a symbol."""
        if not self._client:
            return None
        req_id = self._market_data_subscriptions.get(symbol)
        if not req_id:
            return None
        return self._client._market_data.get(req_id)

    def get_market_data_snapshot(
        self,
        symbol: str,
        exchange: str = "SMART",
        currency: str = "USD",
        timeout: float = 5.0,
        market_data_type: Optional[int] = None,
    ) -> Optional[Dict[str, Any]]:
        """Fetch a one-time market data snapshot for a symbol."""
        return self.subscribe_market_data(
            symbol=symbol,
            snapshot=True,
            exchange=exchange,
            currency=currency,
            timeout=timeout,
            market_data_type=market_data_type,
        )
    
    def get_historical_data(
        self,
        symbol: str,
        duration: str = "1 M",
        bar_size: str = "5 mins",
        end_datetime: Optional[datetime] = None,
        what_to_show: str = "TRADES",
        use_rth: bool = True,
        timeout: float = 60.0,
    ) -> pd.DataFrame:
        """
        Fetch historical bar data for a symbol.
        
        Args:
            symbol: Stock symbol
            duration: Duration string (e.g., "1 D", "1 W", "1 M", "1 Y")
            bar_size: Bar size (e.g., "1 min", "5 mins", "1 hour", "1 day")
            end_datetime: End datetime (defaults to now)
            what_to_show: Data type (TRADES, MIDPOINT, BID, ASK, etc.)
            use_rth: Use regular trading hours only
            timeout: Request timeout
            
        Returns:
            DataFrame with OHLCV data
        """
        if not self._ensure_connected():
            logger.error("Not connected to TWS")
            return pd.DataFrame()
        
        req_id = self._client._get_next_req_id()
        
        # Create contract
        if symbol.upper() == "VIX":
            contract = self.create_index_contract("VIX")
        else:
            contract = self.create_stock_contract(symbol)
        
        # Format end datetime
        if end_datetime is None:
            end_datetime = datetime.now(timezone.utc)
        elif end_datetime.tzinfo is None:
            # Assume local if naive, convert to UTC
            end_datetime = end_datetime.astimezone(timezone.utc)
        else:
            end_datetime = end_datetime.astimezone(timezone.utc)
            
        end_str = end_datetime.strftime("%Y%m%d-%H:%M:%S")
        
        # Create request tracking
        request = HistoricalDataRequest(
            req_id=req_id,
            symbol=symbol,
            contract=contract,
            end_datetime=end_str,
            duration=duration,
            bar_size=bar_size,
            what_to_show=what_to_show,
            use_rth=use_rth,
        )
        self._client._historical_data[req_id] = request
        self._client._data_events[req_id] = threading.Event()
        
        logger.info(f"Requesting {duration} of {bar_size} bars for {symbol}")
        
        # Request historical data
        self._client.reqHistoricalData(
            reqId=req_id,
            contract=contract,
            endDateTime=end_str,
            durationStr=duration,
            barSizeSetting=bar_size,
            whatToShow=what_to_show,
            useRTH=1 if use_rth else 0,
            formatDate=1,
            keepUpToDate=False,
            chartOptions=[],
        )
        
        # Wait for completion
        if not self._client._data_events[req_id].wait(timeout=timeout):
            logger.error(f"Timeout waiting for historical data for {symbol}")
            return pd.DataFrame()
        
        # Check for errors
        if req_id in self._client._errors:
            logger.error(f"Error getting data for {symbol}: {self._client._errors[req_id]}")
            return pd.DataFrame()
        
        # Convert to DataFrame
        bars = request.bars
        if not bars:
            logger.warning(f"No bars received for {symbol}")
            return pd.DataFrame()
        
        df = pd.DataFrame(bars)
        
        # Parse timestamp with flexible handling for IB formats
        ts = df["timestamp"]
        parsed = None
        
        if ts.dtype.kind in {"i", "u", "f"}:
            parsed = pd.to_datetime(ts, unit="s", errors="coerce")
        else:
            # Handle IB format like "20260128 09:30:00 US/Eastern"
            # We slice the first 17 chars (YYYYMMDD HH:MM:SS) to ignore timezone for parsing
            # This avoids issues with non-standard zone names
            clean_ts = ts.astype(str).str.slice(0, 17)
            parsed = pd.to_datetime(clean_ts, format="%Y%m%d %H:%M:%S", errors="coerce")
            
            if parsed.isna().any():
                # Fallback to simple parsing (e.g. for "20260203" daily bars)
                parsed = parsed.fillna(pd.to_datetime(ts, errors="coerce"))

        if parsed.isna().any():
            logger.warning("Some timestamps could not be parsed for %s. Sample: %s", 
                         symbol, ts.iloc[0] if not ts.empty else "N/A")

        df["timestamp"] = parsed
        df = df.sort_values("timestamp").reset_index(drop=True)
        
        logger.info(f"Received {len(df)} bars for {symbol}")
        return df
    
    def get_multiple_historical_data(
        self,
        symbols: List[str],
        duration: str = "1 M",
        bar_size: str = "5 mins",
        end_datetime: Optional[datetime] = None,
        **kwargs,
    ) -> Dict[str, pd.DataFrame]:
        """
        Fetch historical data for multiple symbols.
        
        Args:
            symbols: List of symbols
            duration: Duration string
            bar_size: Bar size string
            end_datetime: End datetime
            **kwargs: Additional arguments passed to get_historical_data
            
        Returns:
            Dict mapping symbol to DataFrame
        """
        result = {}
        
        for symbol in symbols:
            try:
                df = self.get_historical_data(
                    symbol=symbol,
                    duration=duration,
                    bar_size=bar_size,
                    end_datetime=end_datetime,
                    **kwargs,
                )
                if not df.empty:
                    result[symbol] = df
                
                # Small delay to avoid pacing violations
                time.sleep(0.5)
                
            except Exception as e:
                logger.error(f"Error fetching data for {symbol}: {e}")
        
        return result
    
    def search_contracts(
        self,
        pattern: str,
        sec_type: str = "STK",
        timeout: float = 10.0,
    ) -> List[Dict]:
        """
        Search for contracts matching a pattern.
        
        Args:
            pattern: Symbol pattern to search
            sec_type: Security type
            timeout: Request timeout
            
        Returns:
            List of matching contracts
        """
        if not self._ensure_connected():
            return []
        
        req_id = self._client._get_next_req_id()
        self._client._contracts[req_id] = []
        self._client._data_events[req_id] = threading.Event()
        
        # Create contract for search
        contract = Contract()
        contract.symbol = pattern
        contract.secType = sec_type
        contract.currency = "USD"
        
        self._client.reqContractDetails(req_id, contract)
        
        if not self._client._data_events[req_id].wait(timeout=timeout):
            logger.warning(f"Timeout searching for contracts: {pattern}")
            return []
        
        contracts = self._client._contracts.get(req_id, [])
        return [
            {
                "symbol": c.contract.symbol,
                "sec_type": c.contract.secType,
                "exchange": c.contract.exchange,
                "currency": c.contract.currency,
                "name": c.longName if hasattr(c, 'longName') else "",
            }
            for c in contracts
        ]
    
    def search_symbols(
        self,
        pattern: str,
        timeout: float = 3.0,
    ) -> List[Dict]:
        """
        Search for symbols matching a pattern using reqMatchingSymbols.
        
        This performs a fuzzy search similar to TWS's symbol lookup dialog,
        returning multiple matching symbols with company names.
        
        Args:
            pattern: Symbol pattern to search (e.g., "FLEX", "APP", "MICR")
            timeout: Request timeout
            
        Returns:
            List of matching symbol dicts with symbol, name, sec_type, exchange
        """
        if not self._ensure_connected():
            return []
        
        if not pattern or len(pattern) < 1:
            return []
        
        req_id = self._client._get_next_req_id()
        self._client._matching_symbols[req_id] = []
        self._client._data_events[req_id] = threading.Event()
        
        logger.debug(f"Searching for symbols matching: {pattern}")
        
        # Use reqMatchingSymbols for fuzzy search
        self._client.reqMatchingSymbols(req_id, pattern)
        
        if not self._client._data_events[req_id].wait(timeout=timeout):
            logger.warning(f"Timeout searching for symbols: {pattern}")
            # Return whatever we have so far
        
        results = self._client._matching_symbols.get(req_id, [])
        
        # Clean up
        if req_id in self._client._matching_symbols:
            del self._client._matching_symbols[req_id]
        if req_id in self._client._data_events:
            del self._client._data_events[req_id]
        
        return results

    def get_portfolio_positions(self, timeout: float = 10.0, account: str = "") -> List[Dict]:
        """
        Get portfolio positions with market price and unrealized PnL from TWS account updates.

        Args:
            timeout: Request timeout
            account: Account ID (empty for default/first managed account)

        Returns:
            List of portfolio position dicts
        """
        if not self._ensure_connected() or self._client is None:
            return []

        if not account:
            managed = self._client._managed_accounts
            account = managed[0] if managed else ""

        self._client._portfolio_positions = []
        self._client._portfolio_positions_by_symbol = {}
        self._client._account_update_event.clear()

        try:
            self._client.reqAccountUpdates(True, account)
        except Exception as exc:
            logger.warning(f"Failed to request account updates: {exc}")
            return []

        self._client._account_update_event.wait(timeout=timeout)

        try:
            self._client.reqAccountUpdates(False, account)
        except Exception:
            pass

        positions = list(self._client._portfolio_positions_by_symbol.values())
        self._client._portfolio_positions = positions
        return positions

    def get_executions(
        self,
        timeout: float = 10.0,
        account: str = "",
        since: Optional[datetime] = None,
    ) -> List[Dict]:
        """
        Get recent executions for entry time mapping.

        Args:
            timeout: Request timeout
            account: Account ID (empty for default/first managed account)
            since: Optional lower-bound time for executions

        Returns:
            List of execution dicts
        """
        if not self._ensure_connected() or self._client is None:
            return []

        if not account:
            managed = self._client._managed_accounts
            account = managed[0] if managed else ""

        self._client._executions = []
        self._client._executions_event.clear()

        filt = ExecutionFilter()
        if account:
            filt.acctCode = account
        if since:
            filt.time = since.strftime("%Y%m%d-%H:%M:%S")

        req_id = self._client._get_next_req_id()
        try:
            self._client.reqExecutions(req_id, filt)
        except Exception as exc:
            logger.warning(f"Failed to request executions: {exc}")
            return []

        self._client._executions_event.wait(timeout=timeout)
        return self._client._executions.copy()
    
    def get_positions(self, timeout: float = 10.0) -> List[Dict]:
        """
        Get current positions.
        
        Returns:
            List of position dicts
        """
        if not self._ensure_connected():
            return []
        
        self._client._positions = []
        self._client._positions_event.clear()
        self._client.reqPositions()

        self._client._positions_event.wait(timeout=timeout)
        try:
            self._client.cancelPositions()
        except Exception:
            pass

        return self._client._positions.copy()
    
    def get_account_summary(
        self,
        account: str = "",
        tags: str = "NetLiquidation,TotalCashValue,GrossPositionValue",
        timeout: float = 10.0,
    ) -> Dict[str, Any]:
        """
        Get account summary.
        
        Args:
            account: Account ID (empty for all)
            tags: Comma-separated list of tags
            timeout: Request timeout
            
        Returns:
            Dict with account values
        """
        if not self._ensure_connected():
            return {}
        
        req_id = self._client._get_next_req_id()
        self._client._data_events[req_id] = threading.Event()
        self._client._account_values = {}
        
        self._client.reqAccountSummary(req_id, "All", tags)
        
        self._client._data_events[req_id].wait(timeout=timeout)
        
        # Cancel the subscription
        self._client.cancelAccountSummary(req_id)
        
        return self._client._account_values.copy()
    
    def get_watchlist_symbols(self) -> List[str]:
        """
        Get symbols from TWS positions and local watchlist file.
        
        Note: The IB API does NOT expose TWS Favorites/Watchlists directly.
        This is a TWS UI feature that is not accessible via the API.
        
        This method returns symbols from:
        1. Current portfolio positions (if connected to TWS)
        2. Local watchlist file (config/watchlist.txt)
        3. Default symbols as fallback
        
        To sync with TWS Favorites, manually update config/watchlist.txt.
        
        Returns:
            List of unique symbol strings
        """
        symbols = set()
        
        # 1. Try to get symbols from current TWS positions
        if self.is_connected():
            try:
                positions = self.get_positions(timeout=5.0)
                for pos in positions:
                    if pos.get("symbol"):
                        symbols.add(pos["symbol"].upper())
                logger.debug(f"Got {len(symbols)} symbols from TWS positions")
            except Exception as e:
                logger.debug(f"Could not fetch positions: {e}")
        
        # 2. Load symbols from local watchlist file
        watchlist_file = Path(__file__).parent.parent.parent / "config" / "watchlist.txt"
        if watchlist_file.exists():
            try:
                with open(watchlist_file, "r") as f:
                    for line in f:
                        line = line.strip()
                        if line and not line.startswith("#"):
                            ticker = line.split(":", 1)[0].strip()
                            if ticker:
                                symbols.add(ticker.upper())
                logger.debug(f"Loaded watchlist from {watchlist_file}")
            except Exception as e:
                logger.warning(f"Error loading watchlist file: {e}")
        
        # 3. If still empty, use default symbols
        if not symbols:
            default_symbols = [
                # Major ETFs
                "SPY", "QQQ", "IWM", "DIA", "VTI",
                # Sector ETFs
                "XLF", "XLE", "XLK", "XLV", "XLI",
                # Tech Giants
                "AAPL", "MSFT", "GOOGL", "AMZN", "NVDA", "META", "TSLA",
                # Financial
                "JPM", "BAC", "GS", "MS",
                # Volatility
                "VIX",
            ]
            symbols = set(default_symbols)
        
        # Return as sorted list
        return sorted(list(symbols))
    
    def validate_symbols(
        self,
        symbols: List[str],
        timeout: float = 30.0,
    ) -> Dict[str, bool]:
        """
        Validate that symbols exist and are tradeable.
        
        Args:
            symbols: List of symbols to validate
            timeout: Total timeout for validation
            
        Returns:
            Dict mapping symbol to validity
        """
        if not self._ensure_connected():
            return {s: False for s in symbols}
        
        result = {}
        per_symbol_timeout = timeout / len(symbols) if symbols else timeout
        
        for symbol in symbols:
            try:
                contracts = self.search_contracts(
                    symbol,
                    timeout=per_symbol_timeout,
                )
                # Check if we found an exact match
                result[symbol] = any(
                    c["symbol"].upper() == symbol.upper()
                    for c in contracts
                )
            except Exception as e:
                logger.warning(f"Error validating {symbol}: {e}")
                result[symbol] = False
        
        return result


# Singleton instance for UI use
_tws_provider: Optional[TWSDataProvider] = None


def get_tws_provider() -> TWSDataProvider:
    """Get or create the global TWS data provider instance."""
    global _tws_provider
    if _tws_provider is None:
        settings = get_settings()
        # Keep the API sidecar on a separate client id band to avoid collisions.
        default_client_id = settings.ib.client_id + 66
        _tws_provider = TWSDataProvider(client_id=default_client_id)
    return _tws_provider


def reset_tws_provider():
    """Reset the global TWS provider instance."""
    global _tws_provider
    if _tws_provider is not None:
        _tws_provider.disconnect()
        _tws_provider = None
