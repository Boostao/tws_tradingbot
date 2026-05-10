package bot

import (
	"fmt"
	"time"

	"github.com/scmhub/ibapi"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type IBKRClient struct {
	client  *ibapi.EClient
	wrapper *IBWrapper
}

// IBWrapper embeds ibapi.Wrapper to provide callbacks.
type IBWrapper struct {
	ibapi.Wrapper
	engine       *Engine
	client       *ibapi.EClient
	marketData   map[string][]float64
	currentReqID int64
}

func NewIBKRClient(engine *Engine) *IBKRClient {
	wrapper := &IBWrapper{
		engine:     engine,
		marketData: make(map[string][]float64),
	}
	client := ibapi.NewEClient(wrapper)
	wrapper.client = client

	return &IBKRClient{
		client:  client,
		wrapper: wrapper,
	}
}

// Error override to suppress meaningless IBKR "OK" farm connection logs
func (w *IBWrapper) Error(reqID int64, errTime int64, errCode int64, errString string, advancedOrderRejectJson string) {
	// 2104, 2106, 2158 are just "data farm is OK" informational messages
	if errCode == 2104 || errCode == 2106 || errCode == 2158 {
		return
	}
	fmt.Printf("IBKR Error (ReqID: %d, Code: %d): %s\n", reqID, errCode, errString)
}

// Connect starts the TCP socket to TWS dynamically using arguments.
func (c *IBKRClient) Connect(host string, port int, clientID int) error {
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

	time.Sleep(1 * time.Second)
	fmt.Printf("Connected to TWS locally on %s:%d\n", host, port)
	return nil
}

func (c *IBKRClient) Disconnect() {
	if c.client != nil {
		c.client.Disconnect()
		fmt.Println("Disconnected from TWS")
	}
}

// Override HistoricalData to parse incoming bars into pure float arrays for our mathematical Engine.
func (w *IBWrapper) HistoricalData(reqID int64, bar *ibapi.Bar) {
	w.marketData[fmt.Sprintf("Close_%d", reqID)] = append(w.marketData[fmt.Sprintf("Close_%d", reqID)], bar.Close)
	w.marketData[fmt.Sprintf("Open_%d", reqID)] = append(w.marketData[fmt.Sprintf("Open_%d", reqID)], bar.Open)
	w.marketData[fmt.Sprintf("High_%d", reqID)] = append(w.marketData[fmt.Sprintf("High_%d", reqID)], bar.High)
	w.marketData[fmt.Sprintf("Low_%d", reqID)] = append(w.marketData[fmt.Sprintf("Low_%d", reqID)], bar.Low)
}

// Override HistoricalDataEnd to trigger Engine Evaluation
func (w *IBWrapper) HistoricalDataEnd(reqID int64, startDateStr string, endDateStr string) {
	fmt.Printf("IBKR: Finished streaming historical data block for %d. Starting evaluation...\n", reqID)

	if w.engine != nil && w.engine.IsRunning {
		symbol := fmt.Sprintf("SYM_%d", reqID) // In reality map reqID -> symbol

		normMap := map[string][]float64{}
		normMap["Close"] = w.marketData[fmt.Sprintf("Close_%d", reqID)]
		normMap["Open"] = w.marketData[fmt.Sprintf("Open_%d", reqID)]

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

// ExecuteAction constructs and places a market order via the IBKR connection
func (w *IBWrapper) ExecuteAction(symbol string, actionType string) {
	if w.engine == nil { return } // Safety
	contract := &ibapi.Contract{
		Symbol:       symbol,
		SecType:      "STK",
		Exchange:     "SMART",
		Currency:     "USD",
	}

	// Figure out order type based on ActionType
	actionMap := map[string]string{
		"BUY":  "BUY",
		"SELL": "SELL",
		// Add "SHORT" / "COVER" handling if needed based on models
	}

	ibAction, exists := actionMap[actionType]
	if !exists { return } // unhandled action

	// Build the Order
	order := &ibapi.Order{
		Action:        ibAction,
		OrderType:     "MKT",
		TotalQuantity: ibapi.StringToDecimal("100"), // Fixed 100 size for testing - this should be dynamic!
		TIF:           "DAY",
		Transmit:      true,
	}

	w.currentReqID++
	orderId := w.currentReqID

	fmt.Printf("Executing %s order for %s (OrderID: %d)\n", ibAction, symbol, orderId)
	
	// Actually send the order to IBKR
	w.client.PlaceOrder(orderId, contract, order)

	// Emit the execution out to Svelte!
	if w.engine != nil && w.engine.Ctx != nil {
		msg := fmt.Sprintf("[%s] EXECUTING %s %d %s @ MKT (OrderID: %d)", time.Now().Format("15:04:05"), ibAction, order.TotalQuantity.Int(), symbol, orderId)
		// We hook into Wails runtime to emit this directly to the ledger
		runtime.EventsEmit(w.engine.Ctx, "ledger_entry", msg)
	}
}
