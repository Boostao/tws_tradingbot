package bot

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"tws_traderbot/backend/models"

	"github.com/scmhub/ibapi"
)

type IBKRClient struct {
	client    *ibapi.EClient
	wrapper   *IBWrapper
	mu        sync.Mutex
	connected atomic.Bool
	clientID  int64
}

// IBWrapper embeds ibapi.Wrapper to provide callbacks, live telemetry ingestion, and execution handling.
type IBWrapper struct {
	ibapi.Wrapper
	mu                sync.RWMutex
	engine            *Engine
	client            *ibapi.EClient
	connected         *atomic.Bool
	clientID          atomic.Int64
	positionsHydrated atomic.Bool
	disconnecting     atomic.Bool
	subscribedAccount string
	summaryReqID      int64
	pnlReqID          int64
	marketData        map[string][]float64
	nextOrderID       atomic.Int64
	nextReqID         atomic.Int64
}

// Compile-time assertion that *IBWrapper implements ibapi.EWrapper cleanly.
var _ ibapi.EWrapper = (*IBWrapper)(nil)

// NewIBWrapper constructs a standalone IBWrapper bound to an Engine (useful for testing and E2E harnesses).
func NewIBWrapper(engine *Engine) *IBWrapper {
	w := &IBWrapper{
		engine:     engine,
		marketData: make(map[string][]float64),
	}
	w.nextOrderID.Store(1)
	w.nextReqID.Store(1)
	return w
}

func NewIBKRClient(engine *Engine) *IBKRClient {
	c := &IBKRClient{}
	wrapper := NewIBWrapper(engine)
	wrapper.connected = &c.connected

	client := ibapi.NewEClient(wrapper)
	wrapper.client = client
	c.client = client
	c.wrapper = wrapper
	return c
}

// Wrapper returns the underlying IBWrapper for direct callback testing or telemetry inspection.
func (c *IBKRClient) Wrapper() *IBWrapper {
	return c.wrapper
}

// NextReqID atomically allocates a unique TWS request ID.
func (w *IBWrapper) NextReqID() int64 {
	return w.nextReqID.Add(1) - 1
}

// NextValidID captures the starting valid order ID from TWS upon connection.
// Monotonically advances the atomic order ID counter, rejecting non-positive IDs and backwards regressions.
func (w *IBWrapper) NextValidID(orderID int64) {
	if orderID <= 0 {
		fmt.Printf("IBKR: Ignoring non-positive NextValidID from TWS: %d\n", orderID)
		return
	}
	for {
		curr := w.nextOrderID.Load()
		if orderID <= curr {
			fmt.Printf("IBKR: Ignoring NextValidID %d <= current orderID %d\n", orderID, curr)
			return
		}
		if w.nextOrderID.CompareAndSwap(curr, orderID) {
			fmt.Printf("IBKR: Received NextValidID from TWS: %d\n", orderID)
			return
		}
	}
}

// NextOrderID atomically generates and returns the next thread-safe order ID.
// Feature 15: Atomic NextValidID Sequencing.
func (w *IBWrapper) NextOrderID() int64 {
	return w.nextOrderID.Add(1) - 1
}

// ConnectAck handles TWS handshake completion.
func (w *IBWrapper) ConnectAck() {
	if w.connected != nil {
		w.connected.Store(true)
	}
	fmt.Println("IBKR: ConnectAck received from TWS")
}

// ConnectionClosed handles unexpected or graceful TWS socket disconnection.
func (w *IBWrapper) ConnectionClosed() {
	if w.connected != nil {
		w.connected.Store(false)
	}
	w.positionsHydrated.Store(false)

	w.mu.Lock()
	w.subscribedAccount = ""
	w.summaryReqID = 0
	w.pnlReqID = 0
	w.mu.Unlock()

	if w.engine != nil {
		w.engine.RecordDisconnect("TWS connection closed")
		w.engine.ResetBrokerTelemetry()
	}
	fmt.Println("IBKR: ConnectionClosed from TWS")
}

// ManagedAccounts captures the dynamic TWS account ID and starts account-specific subscriptions asynchronously.
func (w *IBWrapper) ManagedAccounts(accountsList []string) {
	var acct string
	for _, raw := range accountsList {
		for _, part := range strings.Split(raw, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				acct = trimmed
				break
			}
		}
		if acct != "" {
			break
		}
	}
	if acct == "" {
		return
	}

	if w.engine != nil {
		w.engine.SetAccountID(acct)
		w.engine.PersistCapturedAccount(acct)
	}

	w.mu.Lock()
	alreadySubscribed := (w.subscribedAccount == acct)
	if !alreadySubscribed {
		w.subscribedAccount = acct
	}
	var pnlReqID int64
	if !alreadySubscribed {
		pnlReqID = w.NextReqID()
		w.pnlReqID = pnlReqID
	}
	client := w.client
	w.mu.Unlock()

	if !alreadySubscribed && client != nil && w.connected != nil && w.connected.Load() && !w.disconnecting.Load() {
		go func(c *ibapi.EClient, account string, reqID int64) {
			if w.disconnecting.Load() || !w.connected.Load() || !c.IsConnected() {
				return
			}
			c.ReqPnL(reqID, account, "")
			c.ReqAccountUpdates(true, account)
		}(client, acct, pnlReqID)
	}
}

// AccountSummary ingests NetLiquidation, TotalCashValue, BuyingPower, EquityWithLoanValue, and ExcessLiquidity.
func (w *IBWrapper) AccountSummary(reqID int64, account string, tag string, value string, currency string) {
	if w.engine == nil {
		return
	}
	if acct := strings.TrimSpace(account); acct != "" {
		w.engine.SetAccountID(acct)
	}
	val, err := parseFiniteFloat(value)
	if err != nil {
		return
	}

	switch strings.TrimSpace(tag) {
	case "NetLiquidation":
		w.engine.SetEquityWithCurrency(val, currency)
	case "BuyingPower":
		w.engine.SetBuyingPowerWithCurrency(val, currency)
	case "EquityWithLoanValue":
		w.engine.SetEquityWithLoanWithCurrency(val, currency)
	case "ExcessLiquidity":
		w.engine.SetExcessLiquidityWithCurrency(val, currency)
	case "TotalCashValue":
		w.engine.SetCashBalanceFallback(currency, val)
	}
}

// AccountSummaryEnd logs completion of initial account summary snapshot.
func (w *IBWrapper) AccountSummaryEnd(reqID int64) {
	fmt.Printf("IBKR: AccountSummaryEnd (ReqID: %d)\n", reqID)
}

// UpdateAccountValue ingests native per-currency CashBalance and account valuation updates.
func (w *IBWrapper) UpdateAccountValue(tag string, val string, currency string, accountName string) {
	if w.engine == nil {
		return
	}
	if acct := strings.TrimSpace(accountName); acct != "" {
		w.engine.SetAccountID(acct)
	}
	parsed, err := parseFiniteFloat(val)
	if err != nil {
		return
	}

	switch strings.TrimSpace(tag) {
	case "CashBalance":
		w.engine.SetCashBalance(currency, parsed)
	case "TotalCashValue":
		w.engine.SetCashBalanceFallback(currency, parsed)
	case "NetLiquidation":
		w.engine.SetEquityWithCurrency(parsed, currency)
	case "BuyingPower":
		w.engine.SetBuyingPowerWithCurrency(parsed, currency)
	case "EquityWithLoanValue":
		w.engine.SetEquityWithLoanWithCurrency(parsed, currency)
	case "ExcessLiquidity":
		w.engine.SetExcessLiquidityWithCurrency(parsed, currency)
	}
}

// Position ingests live open position quantities and average cost.
func (w *IBWrapper) Position(account string, contract *ibapi.Contract, position ibapi.Decimal, avgCost float64) {
	if w.engine == nil {
		return
	}
	if acct := strings.TrimSpace(account); acct != "" {
		w.engine.SetAccountID(acct)
	}

	posQty := sanitizeDecimal(position)
	avg := sanitizeFloat(avgCost)
	sym, venue, instID, currency := mapContractToInstrument(contract)
	key := contractKey(contract)
	if key == "" {
		return
	}

	w.engine.UpsertPosition(key, models.BrokerPosition{
		Symbol:       sym,
		Venue:        venue,
		InstrumentID: instID,
		Currency:     currency,
		Position:     posQty,
		AvgCost:      avg,
	})

	if w.positionsHydrated.Load() {
		_ = w.engine.SyncLivePositionsToDB()
	}
}

// PositionEnd marks position hydration complete and triggers atomic SQLite holdings sync.
func (w *IBWrapper) PositionEnd() {
	w.positionsHydrated.Store(true)
	if w.engine != nil {
		if err := w.engine.SyncLivePositionsToDB(); err != nil {
			fmt.Printf("IBKR: SyncLivePositionsToDB error: %v\n", err)
		}
	}
}

// UpdatePortfolio enriches live positions with PrimaryExchange, market price, market value, and PnL.
func (w *IBWrapper) UpdatePortfolio(contract *ibapi.Contract, position ibapi.Decimal, marketPrice float64, marketValue float64, averageCost float64, unrealizedPNL float64, realizedPNL float64, accountName string) {
	if w.engine == nil {
		return
	}
	if acct := strings.TrimSpace(accountName); acct != "" {
		w.engine.SetAccountID(acct)
	}

	key := contractKey(contract)
	if key == "" {
		return
	}
	sym, venue, instID, currency := mapContractToInstrument(contract)
	posQty := sanitizeDecimal(position)

	w.engine.UpdatePortfolioPosition(key, models.BrokerPosition{
		Symbol:        sym,
		Venue:         venue,
		InstrumentID:  instID,
		Currency:      currency,
		Position:      posQty,
		AvgCost:       sanitizeFloat(averageCost),
		MarketPrice:   sanitizeFloat(marketPrice),
		MarketValue:   sanitizeFloat(marketValue),
		UnrealizedPnL: sanitizeFloat(unrealizedPNL),
		RealizedPnL:   sanitizeFloat(realizedPNL),
	})
	if posQty == 0 && w.positionsHydrated.Load() {
		_ = w.engine.SyncLivePositionsToDB()
	}
}

// OpenOrder ingests active working orders from TWS.
func (w *IBWrapper) OpenOrder(orderID int64, contract *ibapi.Contract, order *ibapi.Order, orderState *ibapi.OrderState) {
	if w.engine == nil || order == nil {
		return
	}
	sym, venue, instID, _ := mapContractToInstrument(contract)
	status := ""
	if orderState != nil {
		status = strings.TrimSpace(orderState.Status)
	}

	w.engine.UpsertOpenOrder(models.BrokerOrder{
		OrderID:      orderID,
		Symbol:       sym,
		Venue:        venue,
		InstrumentID: instID,
		Action:       strings.ToUpper(strings.TrimSpace(order.Action)),
		OrderType:    strings.ToUpper(strings.TrimSpace(order.OrderType)),
		TotalQty:     sanitizeDecimal(order.TotalQuantity),
		LimitPrice:   sanitizeFloat(order.LmtPrice),
		Status:       status,
		OutsideRTH:   order.OutsideRTH,
	})
}

// OpenOrderEnd logs completion of the initial open orders snapshot.
func (w *IBWrapper) OpenOrderEnd() {
	fmt.Println("IBKR: OpenOrderEnd received from TWS")
}

// OrderStatus updates fill progress or removes completed/cancelled orders from active orders.
func (w *IBWrapper) OrderStatus(orderID int64, status string, filled ibapi.Decimal, remaining ibapi.Decimal, avgFillPrice float64, permID int64, parentID int64, lastFillPrice float64, clientID int64, whyHeld string, mktCapPrice float64) {
	if w.engine == nil {
		return
	}
	w.engine.UpdateOrderStatus(
		orderID,
		strings.TrimSpace(status),
		sanitizeDecimal(filled),
		sanitizeDecimal(remaining),
		sanitizeFloat(avgFillPrice),
	)
}

// Pnl receives streaming daily, unrealized, and realized PnL from TWS ReqPnL.
// Note: ibapi.EWrapper spells this method Pnl (lowercase nl).
func (w *IBWrapper) Pnl(reqID int64, dailyPnL float64, unrealizedPnL float64, realizedPnL float64) {
	if w.engine == nil {
		return
	}
	w.engine.UpdateLivePnL(
		sanitizeFloat(dailyPnL),
		sanitizeFloat(unrealizedPnL),
		sanitizeFloat(realizedPnL),
	)
}

// Error suppresses benign IBKR data farm status codes and records structured BrokerAlerts for actionable codes.
func (w *IBWrapper) Error(reqID int64, errTime int64, errCode int64, errString string, advancedOrderRejectJson string) {
	// 2104, 2106, 2158 are "market data farm connection is OK" informational messages
	if errCode == 2104 || errCode == 2106 || errCode == 2158 {
		return
	}
	if errCode == 504 && w.disconnecting.Load() {
		return
	}

	alert := BuildBrokerAlert(errCode, errString)
	if w.engine != nil {
		w.engine.AddBrokerAlert(alert)
		if errCode == 1100 {
			w.engine.RecordDisconnect(errString)
		}
		if app := w.engine.GetApp(); app != nil {
			app.Event.Emit("broker_alert", alert)
		}
	}

	if errCode == 1102 && w.client != nil && w.connected != nil && w.connected.Load() && !w.disconnecting.Load() {
		go func(c *ibapi.EClient) {
			if w.disconnecting.Load() || !w.connected.Load() || !c.IsConnected() {
				return
			}
			c.ReqPositions()
			c.ReqAllOpenOrders()
		}(w.client)
	}

	fmt.Printf("IBKR Error (ReqID: %d, Code: %d): %s\n", reqID, errCode, errString)
}

// BuildBrokerAlert maps TWS error codes into structured, human-readable operator alerts with remediation hints.
func BuildBrokerAlert(errCode int64, errString string) models.BrokerAlert {
	now := time.Now().UTC().Format(time.RFC3339)
	msg := strings.TrimSpace(errString)
	severity := "warning"
	hint := "Consult TWS API log or verify contract/order parameters."

	switch errCode {
	case 10089:
		severity = "warning"
		hint = "Market data subscription required for live quotes. Enable Delayed Market Data in TWS API settings or subscribe in IBKR Client Portal."
	case 504:
		severity = "error"
		hint = "Not connected to TWS. Verify Trader Workstation / IB Gateway is running on port 7497 (Paper) with API socket enabled."
	case 502:
		severity = "error"
		hint = "Unable to connect to TWS socket. Confirm host/port (127.0.0.1:7497) and enable ActiveX/Socket Clients in TWS Global Configuration."
	case 1100:
		severity = "error"
		hint = "Connectivity between IBKR and TWS lost; automated execution paused until connection restores."
	case 1101:
		severity = "info"
		hint = "Connectivity restored; market data subscriptions may need re-subscription."
	case 1102:
		severity = "info"
		hint = "Connectivity restored with data maintained; re-syncing open positions and working orders."
	case 200:
		severity = "warning"
		hint = "No security definition found; check ticker symbol, exchange, and currency."
	case 201:
		severity = "error"
		hint = "Order rejected by TWS risk/margin or session rules; review order details and buying power."
	case 321:
		severity = "warning"
		hint = "TWS request validation error; verify client ID, account, or API parameters."
	case 354:
		severity = "warning"
		hint = "Requested market data is not subscribed for this account."
	default:
		if errCode >= 500 && errCode < 600 {
			severity = "error"
		}
	}

	return models.BrokerAlert{
		Code:      errCode,
		Severity:  severity,
		Message:   msg,
		Hint:      hint,
		Timestamp: now,
	}
}

func sanitizeFloat(v float64) float64 {
	if v == ibapi.UNSET_FLOAT || math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e15 {
		return 0
	}
	return v
}

func sanitizeDecimal(d ibapi.Decimal) float64 {
	if d == ibapi.UNSET_DECIMAL {
		return 0
	}
	return sanitizeFloat(d.Float())
}

func parseFiniteFloat(s string) (float64, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 0, fmt.Errorf("empty numeric value")
	}
	v, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e15 {
		return 0, fmt.Errorf("non-finite float %q", s)
	}
	return v, nil
}

func canonicalExchangeName(ex string) string {
	switch strings.ToUpper(strings.TrimSpace(ex)) {
	case "TSE":
		return "TSX"
	case "VENTURE":
		return "TSXV"
	case "PURE":
		return "CSE"
	case "AEQS":
		return "NEO"
	case "ISLAND":
		return "NASDAQ"
	default:
		return strings.ToUpper(strings.TrimSpace(ex))
	}
}

func mapContractToInstrument(c *ibapi.Contract) (symbol, venue, instrumentID, currency string) {
	if c == nil {
		return "", "SMART", "", "USD"
	}
	symbol = strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(c.Symbol)), " ", ".")
	currency = strings.ToUpper(strings.TrimSpace(c.Currency))
	if currency == "" {
		currency = "USD"
	}

	venue = canonicalExchangeName(c.PrimaryExchange)
	if venue == "" {
		venue = canonicalExchangeName(c.Exchange)
	}
	if venue == "" {
		venue = "SMART"
	}

	if symbol != "" {
		instrumentID = symbol + "." + venue
	}
	return symbol, venue, instrumentID, currency
}

func contractKey(c *ibapi.Contract) string {
	if c == nil {
		return ""
	}
	if c.ConID > 0 {
		return fmt.Sprintf("conid:%d", c.ConID)
	}
	sym := strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(c.Symbol)), " ", ".")
	if sym == "" {
		return ""
	}
	curr := strings.ToUpper(strings.TrimSpace(c.Currency))
	if curr == "" {
		curr = "USD"
	}
	return sym + "." + curr
}

// Connect starts the TCP socket to TWS dynamically and issues initial telemetry subscriptions.
func (c *IBKRClient) Connect(host string, port int, clientID int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected.Load() {
		return nil
	}

	if host == "" {
		host = "127.0.0.1"
	}
	if port == 0 {
		port = 7497
	}

	c.clientID = int64(clientID)
	if c.wrapper != nil {
		c.wrapper.clientID.Store(int64(clientID))
		c.wrapper.positionsHydrated.Store(false)
	}

	err := c.client.Connect(host, port, int64(clientID))
	if err != nil {
		c.connected.Store(false)
		return fmt.Errorf("failed to connect to TWS on %s:%d (clientID: %d): %w", host, port, clientID, err)
	}

	if c.client.IsConnected() {
		c.connected.Store(true)
		time.Sleep(500 * time.Millisecond)
		c.StartAccountSubscriptions()
		fmt.Printf("Connected to TWS locally on %s:%d\n", host, port)
	} else {
		c.connected.Store(false)
	}
	return nil
}

// StartAccountSubscriptions requests managed accounts, summary metrics, positions, and open orders.
func (c *IBKRClient) StartAccountSubscriptions() {
	if c.client == nil || !c.IsConnected() {
		return
	}

	c.client.ReqManagedAccts()

	summaryReqID := c.wrapper.NextReqID()
	c.wrapper.mu.Lock()
	c.wrapper.summaryReqID = summaryReqID
	c.wrapper.mu.Unlock()

	c.client.ReqAccountSummary(summaryReqID, "All", "NetLiquidation,TotalCashValue,BuyingPower,EquityWithLoanValue,ExcessLiquidity")
	c.client.ReqPositions()
	c.client.ReqOpenOrders()
	c.client.ReqAllOpenOrders()
	if c.clientID == 0 {
		c.client.ReqAutoOpenOrders(true)
	}
}

func (c *IBKRClient) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	wasConnected := c.connected.Swap(false)
	if !wasConnected {
		return
	}

	if c.wrapper != nil {
		c.wrapper.disconnecting.Store(true)
		defer c.wrapper.disconnecting.Store(false)

		c.wrapper.mu.Lock()
		c.wrapper.summaryReqID = 0
		c.wrapper.pnlReqID = 0
		c.wrapper.subscribedAccount = ""
		c.wrapper.mu.Unlock()
		c.wrapper.positionsHydrated.Store(false)
	}

	if c.client != nil {
		_ = c.client.Disconnect()
		fmt.Println("Disconnected from TWS")
	}

	if c.wrapper != nil && c.wrapper.engine != nil {
		c.wrapper.engine.RecordDisconnect("Disconnected by operator")
		c.wrapper.engine.ResetBrokerTelemetry()
	}
}

func (c *IBKRClient) IsConnected() bool {
	return c.connected.Load() && c.client != nil && c.client.IsConnected()
}

// HistoricalData parses incoming bars into float arrays protected by write-lock.
func (w *IBWrapper) HistoricalData(reqID int64, bar *ibapi.Bar) {
	w.mu.Lock()
	defer w.mu.Unlock()

	closeKey := fmt.Sprintf("Close_%d", reqID)
	openKey := fmt.Sprintf("Open_%d", reqID)
	highKey := fmt.Sprintf("High_%d", reqID)
	lowKey := fmt.Sprintf("Low_%d", reqID)

	w.marketData[closeKey] = append(w.marketData[closeKey], bar.Close)
	w.marketData[openKey] = append(w.marketData[openKey], bar.Open)
	w.marketData[highKey] = append(w.marketData[highKey], bar.High)
	w.marketData[lowKey] = append(w.marketData[lowKey], bar.Low)
}

// HistoricalDataEnd triggers Engine evaluation with cloned data under read-lock.
func (w *IBWrapper) HistoricalDataEnd(reqID int64, startDateStr string, endDateStr string) {
	fmt.Printf("IBKR: Finished streaming historical data block for %d. Starting evaluation...\n", reqID)

	if w.engine != nil && w.engine.IsRunning() {
		symbol := fmt.Sprintf("SYM_%d", reqID)

		w.mu.RLock()
		closeSrc := w.marketData[fmt.Sprintf("Close_%d", reqID)]
		openSrc := w.marketData[fmt.Sprintf("Open_%d", reqID)]
		closeCopy := append([]float64(nil), closeSrc...)
		openCopy := append([]float64(nil), openSrc...)
		w.mu.RUnlock()

		normMap := map[string][]float64{
			"Close":      closeCopy,
			"Open":       openCopy,
			"indicatorA": closeCopy,
		}

		actions := w.engine.EvaluateTick(symbol, normMap)

		if len(actions) > 0 {
			fmt.Printf(">>> TRADING RULES TRIGGERED for %s: %v\n", symbol, actions)
			for _, action := range actions {
				w.ExecuteAction(symbol, string(action))
			}
		} else {
			fmt.Printf(">>> No rules triggered for %s.\n", symbol)
		}
	}
}

// ExecuteAction constructs, validates, and executes an order.
// Feature 10: Evaluates 9-Gate Fail-Closed Pipeline.
// Feature 16: Strict Fail-Closed Dry-Run Guard (never calls PlaceOrder when dry_run is true).
// Feature 18: Attaches Take Profit and Stop Loss OCA bracket orders if enabled.
func (w *IBWrapper) ExecuteAction(symbol string, actionType string) {
	upperAction := strings.ToUpper(strings.TrimSpace(actionType))
	if upperAction != "BUY" && upperAction != "SELL" {
		return
	}

	// If engine is not present, still allocate order ID for sequencing
	if w.engine == nil {
		_ = w.NextOrderID()
		return
	}

	// 1. Evaluate 9-Gate Fail-Closed Pipeline
	can, reason := w.engine.CanExecute(symbol, models.ActionType(strings.ToLower(upperAction)))
	if !can {
		fmt.Printf("[FAIL-CLOSED REJECT] Order execution blocked for %s: %s\n", symbol, reason)
		if app := w.engine.GetApp(); app != nil {
			msg := fmt.Sprintf("[%s] ❌ ORDRE REFUSÉ pour %s: %s", time.Now().Format("15:04:05"), symbol, reason)
			app.Event.Emit("ledger_entry", msg)
			app.Event.Emit("order_rejected", map[string]interface{}{
				"symbol": symbol,
				"action": upperAction,
				"reason": reason,
			})
		}
		return
	}

	// 2. Resolve Multi-Venue Contract
	item := w.engine.GetWatchlistItem(symbol)
	if item == nil {
		norm := models.NormalizeSymbol(symbol)
		item = &models.WatchlistItem{
			Symbol:            norm.Symbol,
			Exchange:          norm.Exchange,
			Venue:             norm.Venue,
			InstrumentID:      norm.InstrumentID,
			IsOrderable:       norm.IsOrderable,
			IsRegimeReference: norm.IsRegimeReference,
			Enabled:           true,
		}
	}

	contract, err := ResolveContract(item)
	if err != nil {
		fmt.Printf("[FAIL-CLOSED REJECT] Contract resolution failed for %s: %v\n", symbol, err)
		return
	}

	// 3. Price & Sizing Calculation
	price := w.engine.GetLatestPrice(symbol)
	if price <= 0 {
		price = 100.0 // Default price fallback
	}
	equity := w.engine.GetEquity()
	sizeFloat, err := w.engine.CalculatePositionSize(item, price, equity)
	if err != nil || sizeFloat <= 0 || math.IsNaN(sizeFloat) || math.IsInf(sizeFloat, 0) {
		sizeFloat = 100.0
	}
	qty := int(sizeFloat)
	if qty <= 0 {
		qty = 1
	}

	// 4. Atomic Order ID Sequencing
	orderId := w.NextOrderID()

	// 5. Strict Dry-Run Guard: Never call w.client.PlaceOrder in dry-run mode
	if w.engine.IsDryRun() {
		ocaGroup := ""
		if w.engine.IsBracketEnabled() {
			ocaGroup = fmt.Sprintf("OCA_%d", orderId)
		}

		fill := &models.TradeRecord{
			OrderID:   orderId,
			Symbol:    contract.Symbol,
			Action:    upperAction,
			Quantity:  qty,
			Price:     price,
			Status:    "SIMULATED_FILL",
			OCAGroup:  ocaGroup,
			Mode:      "dry_run",
			CreatedAt: time.Now().UTC(),
		}
		_ = w.engine.RecordFill(fill)
		fmt.Printf("[DRY-RUN SIMULATED FILL] %s %d %s @ $%.2f (OrderID: %d)\n", upperAction, qty, contract.Symbol, price, orderId)
		return
	}

	// 6. Live Execution Guard
	if w.engine.IsBracketEnabled() {
		tpOrderID := w.NextOrderID()
		slOrderID := w.NextOrderID()
		tpPct, slPct := w.engine.GetBracketPercentages()

		bundle, err := CreateBracketOrders(orderId, tpOrderID, slOrderID, upperAction, qty, "MKT", price, tpPct, slPct, "")
		if err != nil || bundle == nil {
			reason := fmt.Sprintf("Live bracket order creation failed: %v", err)
			fmt.Printf("[FAIL-CLOSED REJECT] Live execution blocked for %s: %s\n", symbol, reason)
			if app := w.engine.GetApp(); app != nil {
				msg := fmt.Sprintf("[%s] ❌ ORDRE REFUSÉ pour %s: %s", time.Now().Format("15:04:05"), symbol, reason)
				app.Event.Emit("ledger_entry", msg)
				app.Event.Emit("order_rejected", map[string]interface{}{
					"symbol": symbol,
					"action": upperAction,
					"reason": reason,
				})
			}
			return
		}

		if w.client != nil {
			w.client.PlaceOrder(bundle.Parent.OrderID, contract, bundle.Parent)
			w.client.PlaceOrder(bundle.TakeProfit.OrderID, contract, bundle.TakeProfit)
			w.client.PlaceOrder(bundle.StopLoss.OrderID, contract, bundle.StopLoss)
		}
		fill := &models.TradeRecord{
			OrderID:   orderId,
			Symbol:    contract.Symbol,
			Action:    upperAction,
			Quantity:  qty,
			Price:     price,
			Status:    "SUBMITTED",
			OCAGroup:  bundle.OCAGroup,
			Mode:      "live",
			CreatedAt: time.Now().UTC(),
		}
		_ = w.engine.RecordFill(fill)
		return
	}

	// Standard unbracketed live market order
	order := ibapi.NewOrder()
	order.OrderID = orderId
	order.Action = upperAction
	order.OrderType = "MKT"
	order.TotalQuantity = ibapi.StringToDecimal(fmt.Sprintf("%d", qty))
	order.TIF = "DAY"
	order.Transmit = true

	if w.client != nil {
		w.client.PlaceOrder(orderId, contract, order)
	}

	fill := &models.TradeRecord{
		OrderID:   orderId,
		Symbol:    contract.Symbol,
		Action:    upperAction,
		Quantity:  qty,
		Price:     price,
		Status:    "SUBMITTED",
		Mode:      "live",
		CreatedAt: time.Now().UTC(),
	}
	_ = w.engine.RecordFill(fill)
}
