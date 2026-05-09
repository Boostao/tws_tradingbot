package bot

import (
	"fmt"
	"time"

	"github.com/scmhub/ibapi"
)

type IBKRClient struct {
	client  *ibapi.EClient
	wrapper *IBWrapper
}

// IBWrapper embeds ibapi.Wrapper to provide callbacks.
type IBWrapper struct {
	ibapi.Wrapper
	engine       *Engine
	marketData   map[string][]float64
	currentReqID int64
}

func NewIBKRClient() *IBKRClient {
	wrapper := &IBWrapper{
		marketData: make(map[string][]float64),
	}
	client := ibapi.NewEClient(wrapper)

	return &IBKRClient{
		client:  client,
		wrapper: wrapper,
	}
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
		symbol := fmt.Sprintf("SYM_%d", reqID)

		normMap := map[string][]float64{}
		normMap["Close"] = w.marketData[fmt.Sprintf("Close_%d", reqID)]
		normMap["Open"] = w.marketData[fmt.Sprintf("Open_%d", reqID)]

		actions := w.engine.EvaluateTick(symbol, normMap)

		if len(actions) > 0 {
			fmt.Printf(">>> TRADING RULES TRIGGERED for %s: %v\n", symbol, actions)
		} else {
			fmt.Printf(">>> No rules triggered for %s.\n", symbol)
		}
	}
}
