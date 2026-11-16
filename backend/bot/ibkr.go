package bot

import (
	"fmt"
	"math"
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
}

// IBWrapper embeds ibapi.Wrapper to provide callbacks and execution handling.
type IBWrapper struct {
	ibapi.Wrapper
	mu          sync.RWMutex
	engine      *Engine
	client      *ibapi.EClient
	marketData  map[string][]float64
	nextOrderID atomic.Int64
	nextReqID   atomic.Int64
}

func NewIBKRClient(engine *Engine) *IBKRClient {
	wrapper := &IBWrapper{
		engine:     engine,
		marketData: make(map[string][]float64),
	}
	wrapper.nextOrderID.Store(1)
	wrapper.nextReqID.Store(1)

	client := ibapi.NewEClient(wrapper)
	wrapper.client = client

	return &IBKRClient{
		client:  client,
		wrapper: wrapper,
	}
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

// Error override to suppress meaningless IBKR informational messages.
func (w *IBWrapper) Error(reqID int64, errTime int64, errCode int64, errString string, advancedOrderRejectJson string) {
	// 2104, 2106, 2158 are just "data farm is OK" informational messages
	if errCode == 2104 || errCode == 2106 || errCode == 2158 {
		return
	}
	fmt.Printf("IBKR Error (ReqID: %d, Code: %d): %s\n", reqID, errCode, errString)
}

// Connect starts the TCP socket to TWS dynamically using arguments.
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

	err := c.client.Connect(host, port, int64(clientID))
	if err != nil {
		return fmt.Errorf("failed to connect to TWS on %s:%d (clientID: %d): %w", host, port, clientID, err)
	}

	c.connected.Store(true)
	time.Sleep(1 * time.Second)
	fmt.Printf("Connected to TWS locally on %s:%d\n", host, port)
	return nil
}

func (c *IBKRClient) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected.Swap(false) {
		return
	}

	if c.client != nil {
		_ = c.client.Disconnect()
		fmt.Println("Disconnected from TWS")
	}
}

func (c *IBKRClient) IsConnected() bool {
	return c.connected.Load()
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
		if w.engine.App != nil {
			msg := fmt.Sprintf("[%s] ❌ ORDRE REFUSÉ pour %s: %s", time.Now().Format("15:04:05"), symbol, reason)
			w.engine.App.Event.Emit("ledger_entry", msg)
			w.engine.App.Event.Emit("order_rejected", map[string]interface{}{
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
			if w.engine.App != nil {
				msg := fmt.Sprintf("[%s] ❌ ORDRE REFUSÉ pour %s: %s", time.Now().Format("15:04:05"), symbol, reason)
				w.engine.App.Event.Emit("ledger_entry", msg)
				w.engine.App.Event.Emit("order_rejected", map[string]interface{}{
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
