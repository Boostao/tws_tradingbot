from __future__ import annotations

import logging
import threading
import time
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Any, Dict, Optional

import pandas as pd

try:
    from ibapi.client import EClient
    from ibapi.common import BarData, TickAttrib, TickerId
    from ibapi.contract import Contract
    from ibapi.execution import ExecutionFilter
    from ibapi.order import Order
    from ibapi.wrapper import EWrapper

    IBAPI_AVAILABLE = True
except ImportError:
    IBAPI_AVAILABLE = False
    EClient = object
    EWrapper = object
    Contract = Any
    Order = Any
    BarData = Any
    TickAttrib = Any
    TickerId = Any
    ExecutionFilter = Any

from src.config.settings import get_settings


logger = logging.getLogger(__name__)


@dataclass
class HistoricalDataRequest:
    req_id: int
    symbol: str
    contract: Any
    end_datetime: str
    duration: str
    bar_size: str
    what_to_show: str = "TRADES"
    use_rth: bool = True
    completed: bool = False
    bars: list[dict[str, Any]] = field(default_factory=list)
    error: str | None = None


class TWSDataWrapper(EWrapper):
    def __init__(self) -> None:
        super().__init__()
        self._historical_data: dict[int, HistoricalDataRequest] = {}
        self._account_values: dict[str, Any] = {}
        self._errors: dict[int, str] = {}
        self._next_valid_id: int | None = None
        self._connected = False
        self._managed_accounts: list[str] = []
        self._orders: dict[int, dict[str, Any]] = {}
        self._order_lock = threading.Lock()
        self._open_orders_event = threading.Event()
        self._portfolio_positions_by_symbol: dict[str, dict[str, Any]] = {}
        self._positions_event = threading.Event()
        self._account_update_event = threading.Event()
        self._connection_event = threading.Event()
        self._data_events: dict[int, threading.Event] = {}
        self._market_data: dict[int, dict[str, Any]] = {}
        self._market_data_symbols: dict[int, str] = {}
        self._market_data_events: dict[int, threading.Event] = {}
        self._market_data_lock = threading.Lock()
        self._market_data_permission_errors: set[str] = set()
        self._executions: list[dict[str, Any]] = []
        self._executions_event = threading.Event()

    def nextValidId(self, orderId: int) -> None:
        self._next_valid_id = orderId
        self._connected = True
        self._connection_event.set()
        logger.info("TWS connected. Next valid order ID: %s", orderId)

    def managedAccounts(self, accountsList: str) -> None:
        self._managed_accounts = [account for account in accountsList.split(",") if account]

    def error(self, reqId: TickerId, *args) -> None:
        error_code: int | None
        error_string: str | None
        if len(args) == 2:
            error_code, error_string = args
        elif len(args) == 3:
            error_code, error_string, _ = args
        elif len(args) >= 4:
            _, error_code, error_string, _ = args[:4]
        else:
            logger.error("Unexpected TWS error callback arguments")
            return

        if error_code in (2100, 2104, 2106, 2119, 2158, 2174):
            logger.debug("TWS info [%s] reqId=%s: %s", error_code, reqId, error_string)
            return

        if error_code == 10089:
            symbol = self._market_data_symbols.get(reqId)
            if symbol:
                self._market_data_permission_errors.add(symbol.upper())
            if reqId in self._market_data_events:
                self._market_data_events[reqId].set()
            logger.warning("TWS market-data permission error [%s] %s", error_code, error_string)
            return

        if error_code == 504:
            self._connected = False

        self._errors[int(reqId)] = str(error_string)
        if reqId in self._data_events:
            self._data_events[reqId].set()
        if reqId in self._market_data_events:
            self._market_data_events[reqId].set()
        logger.error("TWS error [%s] reqId=%s: %s", error_code, reqId, error_string)

    def connectionClosed(self) -> None:
        self._connected = False
        logger.warning("TWS connection closed")

    def historicalData(self, reqId: int, bar: BarData) -> None:
        if reqId not in self._historical_data:
            return
        self._historical_data[reqId].bars.append(
            {
                "timestamp": bar.date,
                "open": bar.open,
                "high": bar.high,
                "low": bar.low,
                "close": bar.close,
                "volume": bar.volume,
                "wap": getattr(bar, "wap", None),
                "barCount": getattr(bar, "barCount", None),
            }
        )

    def historicalDataEnd(self, reqId: int, _start: str, _end: str) -> None:
        request = self._historical_data.get(reqId)
        if request is not None:
            request.completed = True
        if reqId in self._data_events:
            self._data_events[reqId].set()

    def accountSummary(self, reqId: int, account: str, tag: str, value: str, currency: str) -> None:
        self._account_values[f"{account}_{tag}"] = {
            "account": account,
            "tag": tag,
            "value": value,
            "currency": currency,
        }

    def accountSummaryEnd(self, reqId: int) -> None:
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
    ) -> None:
        symbol = (contract.symbol or "").upper()
        if not symbol:
            return
        if position == 0:
            self._portfolio_positions_by_symbol.pop(symbol, None)
            return
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
            "timestamp": datetime.now(timezone.utc).isoformat(),
        }

    def accountDownloadEnd(self, _accountName: str) -> None:
        self._account_update_event.set()

    def openOrder(self, orderId: int, contract: Contract, order: Order, orderState) -> None:
        with self._order_lock:
            current = self._orders.setdefault(orderId, {})
            current.update(
                {
                    "order_id": orderId,
                    "symbol": contract.symbol,
                    "action": getattr(order, "action", None),
                    "quantity": getattr(order, "totalQuantity", None),
                    "order_type": getattr(order, "orderType", None),
                    "price": getattr(order, "lmtPrice", None),
                    "stop_price": getattr(order, "auxPrice", None),
                    "status": getattr(orderState, "status", None),
                }
            )

    def openOrderEnd(self) -> None:
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
    ) -> None:
        del permId, parentId, clientId, whyHeld, mktCapPrice
        with self._order_lock:
            current = self._orders.setdefault(orderId, {"order_id": orderId})
            current.update(
                {
                    "status": status,
                    "filled": filled,
                    "remaining": remaining,
                    "avg_fill_price": avgFillPrice,
                    "last_fill_price": lastFillPrice,
                }
            )

    def execDetails(self, reqId: int, contract: Contract, execution) -> None:
        del reqId
        self._executions.append(
            {
                "symbol": contract.symbol,
                "sec_type": contract.secType,
                "side": getattr(execution, "side", None),
                "shares": getattr(execution, "shares", None),
                "price": getattr(execution, "price", None),
                "exec_id": getattr(execution, "execId", None),
                "time": getattr(execution, "time", None),
            }
        )

    def execDetailsEnd(self, reqId: int) -> None:
        del reqId
        self._executions_event.set()

    def tickPrice(self, reqId: int, tickType: int, price: float, attrib: TickAttrib) -> None:
        del attrib
        with self._market_data_lock:
            data = self._market_data.setdefault(reqId, {"symbol": self._market_data_symbols.get(reqId)})
            data["timestamp"] = datetime.now(timezone.utc)
            price_map = {1: "bid", 2: "ask", 4: "last", 6: "high", 7: "low", 9: "close", 14: "open"}
            field = price_map.get(tickType)
            if field:
                data[field] = price

    def tickSize(self, reqId: int, tickType: int, size: int) -> None:
        with self._market_data_lock:
            data = self._market_data.setdefault(reqId, {"symbol": self._market_data_symbols.get(reqId)})
            data["timestamp"] = datetime.now(timezone.utc)
            size_map = {0: "bid_size", 3: "ask_size", 5: "last_size", 8: "volume"}
            field = size_map.get(tickType)
            if field:
                data[field] = size

    def tickSnapshotEnd(self, reqId: int) -> None:
        if reqId in self._market_data_events:
            self._market_data_events[reqId].set()


class TWSDataClient(TWSDataWrapper, EClient):
    def __init__(self) -> None:
        TWSDataWrapper.__init__(self)
        EClient.__init__(self, wrapper=self)
        self._req_id_counter = 1000
        self._order_id_counter: int | None = None
        self._thread: threading.Thread | None = None
        self._lock = threading.Lock()

    def _get_next_req_id(self) -> int:
        with self._lock:
            self._req_id_counter += 1
            return self._req_id_counter

    def _get_next_order_id(self, timeout: float = 5.0) -> int | None:
        if self._order_id_counter is None:
            if not self._connection_event.wait(timeout=timeout):
                return None
            with self._lock:
                self._order_id_counter = self._next_valid_id or 1
        with self._lock:
            order_id = self._order_id_counter
            self._order_id_counter += 1
            return order_id

    def connect_and_run(self, host: str, port: int, client_id: int, timeout: float = 10.0) -> bool:
        if self._connected:
            return True
        try:
            self.connect(host, port, client_id)
            self._thread = threading.Thread(target=self.run, daemon=True)
            self._thread.start()
            if self._connection_event.wait(timeout=timeout):
                self._market_data_permission_errors.clear()
                return True
            self.disconnect()
            return False
        except Exception:
            logger.exception("Failed to connect to TWS")
            return False

    def disconnect_client(self) -> None:
        if self._connected:
            self.disconnect()
            self._connected = False
        if self._thread and self._thread.is_alive():
            self._thread.join(timeout=5.0)
        self._thread = None

    @property
    def is_connected(self) -> bool:
        return self._connected and self.isConnected()


class TWSDataProvider:
    def __init__(self, host: str | None = None, port: int | None = None, client_id: int = 10) -> None:
        if not IBAPI_AVAILABLE:
            raise ImportError("ibapi is not installed")
        settings = get_settings()
        self.host = host or settings.ib.host
        self.port = port or settings.ib.port
        self.client_id = client_id
        self._client: TWSDataClient | None = None

    def connect(self, timeout: float = 10.0) -> bool:
        if self._client and self._client.is_connected:
            return True
        self._client = TWSDataClient()
        return self._client.connect_and_run(self.host, self.port, self.client_id, timeout=timeout)

    def disconnect(self) -> None:
        if self._client is not None:
            self._client.disconnect_client()
            self._client = None

    def is_connected(self) -> bool:
        return self._client is not None and self._client.is_connected

    def _ensure_connected(self) -> bool:
        return self.is_connected() or self.connect()

    def create_stock_contract(self, symbol: str, exchange: str = "SMART", currency: str = "USD") -> Contract:
        contract = Contract()
        contract.symbol = symbol
        contract.secType = "STK"
        contract.exchange = exchange
        contract.currency = currency
        return contract

    def create_index_contract(self, symbol: str, exchange: str = "CBOE") -> Contract:
        contract = Contract()
        contract.symbol = symbol
        contract.secType = "IND"
        contract.exchange = exchange
        contract.currency = "USD"
        return contract

    def _build_contract(self, symbol: str, exchange: str = "SMART", currency: str = "USD") -> Contract:
        if symbol.upper() == "VIX":
            return self.create_index_contract(symbol, exchange="CBOE")
        return self.create_stock_contract(symbol, exchange=exchange, currency=currency)

    def get_historical_data(
        self,
        symbol: str,
        exchange: str = "SMART",
        duration: str = "5 D",
        bar_size: str = "5 mins",
        end_datetime: datetime | None = None,
        what_to_show: str = "TRADES",
        use_rth: bool = True,
        timeout: float = 60.0,
    ) -> pd.DataFrame:
        if not self._ensure_connected() or self._client is None:
            return pd.DataFrame()
        req_id = self._client._get_next_req_id()
        if end_datetime is None:
            end_datetime = datetime.now(timezone.utc)
        elif end_datetime.tzinfo is None:
            end_datetime = end_datetime.astimezone(timezone.utc)
        else:
            end_datetime = end_datetime.astimezone(timezone.utc)
        request = HistoricalDataRequest(
            req_id=req_id,
            symbol=symbol,
            contract=self._build_contract(symbol, exchange=exchange),
            end_datetime=end_datetime.strftime("%Y%m%d-%H:%M:%S"),
            duration=duration,
            bar_size=bar_size,
            what_to_show=what_to_show,
            use_rth=use_rth,
        )
        self._client._historical_data[req_id] = request
        self._client._data_events[req_id] = threading.Event()
        self._client.reqHistoricalData(
            reqId=req_id,
            contract=request.contract,
            endDateTime=request.end_datetime,
            durationStr=duration,
            barSizeSetting=bar_size,
            whatToShow=what_to_show,
            useRTH=1 if use_rth else 0,
            formatDate=1,
            keepUpToDate=False,
            chartOptions=[],
        )
        if not self._client._data_events[req_id].wait(timeout=timeout):
            logger.error("Timeout waiting for historical data for %s", symbol)
            return pd.DataFrame()
        if req_id in self._client._errors:
            logger.error("Historical data error for %s: %s", symbol, self._client._errors[req_id])
            return pd.DataFrame()
        if not request.bars:
            return pd.DataFrame()
        frame = pd.DataFrame(request.bars)
        timestamps = frame["timestamp"]
        parsed = None
        if timestamps.dtype.kind in {"i", "u", "f"}:
            parsed = pd.to_datetime(timestamps, unit="s", errors="coerce")
        else:
            clean = timestamps.astype(str).str.slice(0, 17)
            parsed = pd.to_datetime(clean, format="%Y%m%d %H:%M:%S", errors="coerce")
            if parsed.isna().any():
                parsed = parsed.fillna(pd.to_datetime(timestamps, errors="coerce"))
        frame["timestamp"] = parsed
        return frame.sort_values("timestamp").reset_index(drop=True)

    def get_account_summary(
        self,
        account: str = "",
        tags: str = "NetLiquidation,TotalCashValue,GrossPositionValue",
        timeout: float = 10.0,
    ) -> Dict[str, Any]:
        if not self._ensure_connected() or self._client is None:
            return {}
        req_id = self._client._get_next_req_id()
        self._client._account_values = {}
        self._client._data_events[req_id] = threading.Event()
        self._client.reqAccountSummary(req_id, account or "All", tags)
        self._client._data_events[req_id].wait(timeout=timeout)
        try:
            self._client.cancelAccountSummary(req_id)
        except Exception:
            pass
        return self._client._account_values.copy()

    def get_portfolio_positions(self, timeout: float = 10.0, account: str = "") -> list[dict[str, Any]]:
        if not self._ensure_connected() or self._client is None:
            return []
        if not account:
            managed_accounts = self._client._managed_accounts
            account = managed_accounts[0] if managed_accounts else ""
        self._client._portfolio_positions_by_symbol = {}
        self._client._account_update_event.clear()
        try:
            self._client.reqAccountUpdates(True, account)
        except Exception:
            logger.exception("Failed to request account updates")
            return []
        self._client._account_update_event.wait(timeout=timeout)
        try:
            self._client.reqAccountUpdates(False, account)
        except Exception:
            pass
        return list(self._client._portfolio_positions_by_symbol.values())

    def get_open_orders(self, timeout: float = 5.0) -> list[dict[str, Any]]:
        if not self._ensure_connected() or self._client is None:
            return []
        self._client._open_orders_event.clear()
        with self._client._order_lock:
            self._client._orders = {}
        self._client.reqAllOpenOrders()
        self._client._open_orders_event.wait(timeout=timeout)
        with self._client._order_lock:
            return list(self._client._orders.values())

    def get_executions(
        self,
        timeout: float = 10.0,
        account: str = "",
        since: datetime | None = None,
    ) -> list[dict[str, Any]]:
        if not self._ensure_connected() or self._client is None:
            return []
        if not account:
            managed_accounts = self._client._managed_accounts
            account = managed_accounts[0] if managed_accounts else ""
        self._client._executions = []
        self._client._executions_event.clear()
        execution_filter = ExecutionFilter()
        if account:
            execution_filter.acctCode = account
        if since is not None:
            execution_filter.time = since.strftime("%Y%m%d-%H:%M:%S")
        req_id = self._client._get_next_req_id()
        self._client.reqExecutions(req_id, execution_filter)
        self._client._executions_event.wait(timeout=timeout)
        return self._client._executions.copy()

    def place_order(
        self,
        symbol: str,
        action: str,
        quantity: int,
        order_type: str = "MKT",
        exchange: str = "SMART",
        currency: str = "USD",
        limit_price: float | None = None,
        stop_price: float | None = None,
        tif: str = "DAY",
        timeout: float = 5.0,
    ) -> int | None:
        if not self._ensure_connected() or self._client is None:
            return None
        if quantity <= 0:
            return None
        order_id = self._client._get_next_order_id(timeout=timeout)
        if order_id is None:
            return None
        contract = self._build_contract(symbol, exchange=exchange, currency=currency)
        order = Order()
        order.action = action.upper()
        order.totalQuantity = int(quantity)
        order.orderType = order_type
        order.tif = tif
        if limit_price is not None:
            order.lmtPrice = float(limit_price)
        if stop_price is not None:
            order.auxPrice = float(stop_price)
        with self._client._order_lock:
            self._client._orders[order_id] = {
                "order_id": order_id,
                "symbol": symbol,
                "action": order.action,
                "quantity": order.totalQuantity,
                "order_type": order.orderType,
                "status": "SUBMITTED",
            }
        self._client.placeOrder(order_id, contract, order)
        logger.info("Submitted order %s %s %s %s", order_id, order.action, quantity, symbol)
        return order_id

    def place_bracket_order(
        self,
        symbol: str,
        action: str,
        quantity: int,
        stop_loss_price: float,
        take_profit_price: float,
        exchange: str = "SMART",
        currency: str = "USD",
        tif: str = "DAY",
        timeout: float = 5.0,
    ) -> list[int]:
        if not self._ensure_connected() or self._client is None:
            return []
        if quantity <= 0:
            return []
        parent_id = self._client._get_next_order_id(timeout=timeout)
        take_profit_id = self._client._get_next_order_id(timeout=timeout)
        stop_loss_id = self._client._get_next_order_id(timeout=timeout)
        if parent_id is None or take_profit_id is None or stop_loss_id is None:
            return []

        contract = self._build_contract(symbol, exchange=exchange, currency=currency)
        entry_action = action.upper()
        exit_action = "SELL" if entry_action == "BUY" else "BUY"

        parent = Order()
        parent.orderId = parent_id
        parent.action = entry_action
        parent.totalQuantity = int(quantity)
        parent.orderType = "MKT"
        parent.tif = tif
        parent.transmit = False

        take_profit = Order()
        take_profit.orderId = take_profit_id
        take_profit.action = exit_action
        take_profit.totalQuantity = int(quantity)
        take_profit.orderType = "LMT"
        take_profit.lmtPrice = float(take_profit_price)
        take_profit.parentId = parent_id
        take_profit.tif = tif
        take_profit.transmit = False

        stop_loss = Order()
        stop_loss.orderId = stop_loss_id
        stop_loss.action = exit_action
        stop_loss.totalQuantity = int(quantity)
        stop_loss.orderType = "STP"
        stop_loss.auxPrice = float(stop_loss_price)
        stop_loss.parentId = parent_id
        stop_loss.tif = tif
        stop_loss.transmit = True

        with self._client._order_lock:
            self._client._orders[parent_id] = {"order_id": parent_id, "symbol": symbol, "action": parent.action, "quantity": quantity, "order_type": parent.orderType, "status": "SUBMITTED"}
            self._client._orders[take_profit_id] = {"order_id": take_profit_id, "symbol": symbol, "action": take_profit.action, "quantity": quantity, "order_type": take_profit.orderType, "status": "SUBMITTED"}
            self._client._orders[stop_loss_id] = {"order_id": stop_loss_id, "symbol": symbol, "action": stop_loss.action, "quantity": quantity, "order_type": stop_loss.orderType, "status": "SUBMITTED"}

        self._client.placeOrder(parent_id, contract, parent)
        self._client.placeOrder(take_profit_id, contract, take_profit)
        self._client.placeOrder(stop_loss_id, contract, stop_loss)
        logger.info("Submitted bracket order parent=%s tp=%s sl=%s symbol=%s", parent_id, take_profit_id, stop_loss_id, symbol)
        return [parent_id, take_profit_id, stop_loss_id]


def check_ibapi_available() -> bool:
    return IBAPI_AVAILABLE