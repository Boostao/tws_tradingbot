import threading
import time
from typing import Dict, Any, Optional, Callable
from ibapi.client import EClient
from ibapi.wrapper import EWrapper
from ibapi.contract import Contract
from ibapi.common import TickerId, BarData
from ibapi.account_summary_tags import AccountSummaryTags
from src.config import config
from src.utils import get_logger

logger = get_logger(__name__)


class TWSWrapper(EWrapper):
    """Custom wrapper for TWS API events."""

    def __init__(self):
        super().__init__()
        self.client = None
        self.connected_event = threading.Event()
        self.disconnected_event = threading.Event()
        self.error_event = threading.Event()
        self.market_data: Dict[TickerId, Dict[str, Any]] = {}
        self.account_summary: Dict[str, Any] = {}
        self._market_data_lock = threading.Lock()
        self._account_lock = threading.Lock()

    def nextValidId(self, orderId: int):
        """Called when connection is established."""
        logger.info(f"TWS connection established. Next valid order ID: {orderId}")
        self.connected_event.set()

    def connectionClosed(self):
        """Called when connection is closed."""
        logger.warning("TWS connection closed")
        self.connected_event.clear()
        self.disconnected_event.set()

    def error(self, reqId: TickerId, errorCode: int, errorString: str, advancedOrderRejectJson: str = ""):
        """Handle errors."""
        logger.error(f"TWS Error - ReqID: {reqId}, Code: {errorCode}, Message: {errorString}")
        if errorCode in [502, 504]:  # Connection lost
            self.connected_event.clear()
            self.error_event.set()

    def managedAccounts(self, accountsList: str):
        """Called with the list of managed accounts."""
        logger.info(f"Managed accounts: {accountsList}")

    def tickPrice(self, reqId: TickerId, tickType: int, price: float, attrib):
        """Handle price ticks."""
        with self._market_data_lock:
            if reqId not in self.market_data:
                self.market_data[reqId] = {}
            self.market_data[reqId]['price'] = price
            self.market_data[reqId]['tick_type'] = tickType

    def tickSize(self, reqId: TickerId, tickType: int, size: int):
        """Handle size ticks."""
        with self._market_data_lock:
            if reqId not in self.market_data:
                self.market_data[reqId] = {}
            self.market_data[reqId]['size'] = size

    def accountSummary(self, reqId: int, account: str, tag: str, value: str, currency: str):
        """Handle account summary data."""
        with self._account_lock:
            if account not in self.account_summary:
                self.account_summary[account] = {}
            self.account_summary[account][tag] = value

    def accountSummaryEnd(self, reqId: int):
        """End of account summary."""
        logger.info("Account summary received")


class TWSDataProvider:
    """Thread-safe TWS API connection wrapper."""

    def __init__(self, config_obj):
        self.config = config_obj
        self.host = self.config.get('tws.host', '127.0.0.1')
        self.port = self.config.get('tws.port', 7497)
        self.client_id = self.config.get('tws.client_id', 1)
        self.timeout = self.config.get('tws.timeout', 30)

        self.wrapper = TWSWrapper()
        self.client = EClient(self.wrapper)
        self.wrapper.client = self.client

        self._lock = threading.RLock()  # Reentrant lock for thread-safety
        self._api_thread = None
        self._reconnect_thread = None
        self._stop_reconnect = threading.Event()

        # Start reconnection monitor
        self._start_reconnect_monitor()

    def _start_reconnect_monitor(self):
        """Start background thread to monitor and maintain connection."""
        self._reconnect_thread = threading.Thread(target=self._reconnect_loop, daemon=True)
        self._reconnect_thread.start()

    def _reconnect_loop(self):
        """Background loop to handle reconnection."""
        while not self._stop_reconnect.is_set():
            with self._lock:
                if not self.is_connected():
                    logger.info("Attempting to reconnect to TWS...")
                    try:
                        self._connect_internal()
                    except Exception as e:
                        logger.error(f"Reconnection failed: {e}")
            time.sleep(5)  # Check every 5 seconds

    def _connect_internal(self):
        """Internal connection method."""
        if self._api_thread and self._api_thread.is_alive():
            return  # Already running
        self.client.connect(self.host, self.port, self.client_id)
        self._api_thread = threading.Thread(target=self.client.run, daemon=True)
        self._api_thread.start()
        # Wait for connection
        if self.wrapper.connected_event.wait(timeout=self.timeout):
            logger.info("Successfully connected to TWS")
        else:
            logger.error("Connection timeout")
            self.client.disconnect()

    def connect(self) -> bool:
        """Connect to TWS. Returns True if successful."""
        with self._lock:
            if self.is_connected():
                return True
            try:
                self._connect_internal()
                return self.is_connected()
            except Exception as e:
                logger.error(f"Connection failed: {e}")
                return False

    def disconnect(self):
        """Disconnect from TWS."""
        with self._lock:
            if self.is_connected():
                self.client.disconnect()
                self.wrapper.connected_event.clear()
                if self._api_thread:
                    self._api_thread.join(timeout=5)

    def is_connected(self) -> bool:
        """Check if connected to TWS."""
        return self.wrapper.connected_event.is_set()

    def get_market_data(self, symbol: str, sec_type: str = "STK", exchange: str = "SMART", currency: str = "USD") -> Optional[Dict[str, Any]]:
        """Fetch market data for a symbol."""
        with self._lock:
            if not self.is_connected():
                logger.error("Not connected to TWS")
                return None

            contract = Contract()
            contract.symbol = symbol
            contract.secType = sec_type
            contract.exchange = exchange
            contract.currency = currency

            req_id = self._get_next_req_id()
            self.client.reqMktData(req_id, contract, "", False, False, [])

            # Wait for data (simple implementation, in production use proper async)
            time.sleep(1)  # Allow time for data to arrive

            with self.wrapper._market_data_lock:
                data = self.wrapper.market_data.get(req_id)
                if data:
                    self.client.cancelMktData(req_id)
                    del self.wrapper.market_data[req_id]
                return data

    def get_account_summary(self, account: str = "All") -> Optional[Dict[str, Any]]:
        """Fetch account summary."""
        with self._lock:
            if not self.is_connected():
                logger.error("Not connected to TWS")
                return None

            req_id = self._get_next_req_id()
            tags = AccountSummaryTags.AllTags
            self.client.reqAccountSummary(req_id, "All", tags)

            # Wait for data
            time.sleep(2)

            with self.wrapper._account_lock:
                summary = self.wrapper.account_summary.get(account)
                if summary:
                    self.client.cancelAccountSummary(req_id)
                return summary

    def _get_next_req_id(self) -> int:
        """Get next request ID. In production, maintain a counter."""
        return int(time.time() * 1000) % 1000000

    def shutdown(self):
        """Shutdown the provider."""
        self._stop_reconnect.set()
        if self._reconnect_thread:
            self._reconnect_thread.join(timeout=5)
        self.disconnect()
        if self._api_thread:
            self._api_thread.join(timeout=5)


# Global instance
data_provider = TWSDataProvider(config)