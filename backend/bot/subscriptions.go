package bot

import (
	"fmt"
	"sync"
	"time"

	"github.com/scmhub/ibapi"
)

// SubscriptionManager handles market data subscription lifecycle, including retries and backoff for failed requests.
type SubscriptionManager struct {
	wrapper            *IBWrapper
	bars               map[int64][]*ibapi.Bar
	mu                 sync.RWMutex
	retryMu            sync.Mutex
	retryInProgress    bool
	lastRetryTime      time.Time
}

// NewSubscriptionManager initializes a new subscription manager.
func NewSubscriptionManager(wrapper *IBWrapper) *SubscriptionManager {
	return &SubscriptionManager{
		wrapper: wrapper,
		bars:    make(map[int64][]*ibapi.Bar),
	}
}

// UpdateBar stores incoming bars and triggers evaluation on closed-bar transition.
func (sm *SubscriptionManager) UpdateBar(reqID int64, bar *ibapi.Bar) {
	sm.mu.Lock()
	bars := sm.bars[reqID]
	var lastBar *ibapi.Bar
	if len(bars) > 0 {
		lastBar = bars[len(bars)-1]
	}
	sm.bars[reqID] = append(bars, bar)
	sm.mu.Unlock()

	// Check for closed-bar transition (date change)
	if lastBar != nil && lastBar.Date != bar.Date {
		// Evaluate using closed bars
		symbol := fmt.Sprintf("SYM_%d", reqID)
		
		// Create snapshot
		sm.mu.RLock()
		snapshot := sm.bars[reqID]
		sm.mu.RUnlock()
		
		// Evaluate Tick (snapshot-based)
		// Convert snapshot (bars) to market data map
		closePrices := make([]float64, len(snapshot))
		openPrices := make([]float64, len(snapshot))
		for i, b := range snapshot {
			closePrices[i] = b.Close
			openPrices[i] = b.Open
		}
		
		normMap := map[string][]float64{
			"Close":      closePrices,
			"Open":       openPrices,
			"indicatorA": closePrices,
		}
		
		if sm.wrapper.engine != nil {
			actions := sm.wrapper.engine.EvaluateTick(symbol, normMap)
			if len(actions) > 0 {
				for _, action := range actions {
					sm.wrapper.ExecuteAction(symbol, string(action))
				}
			}
		}
	}
}

// HandleSubscriptionError manages retries for failed market data subscriptions.
func (sm *SubscriptionManager) HandleSubscriptionError(errCode int64, errString string) {
	if sm == nil || sm.wrapper == nil {
		return
	}
	if errCode == 10089 {
		sm.retryMu.Lock()
		if sm.retryInProgress || time.Since(sm.lastRetryTime) < 30*time.Second {
			sm.retryMu.Unlock()
			return
		}
		sm.retryInProgress = true
		sm.retryMu.Unlock()

		go func() {
			defer func() {
				sm.retryMu.Lock()
				sm.retryInProgress = false
				sm.lastRetryTime = time.Now()
				sm.retryMu.Unlock()
			}()
			
			// Backoff: 5, 10, 20 seconds
			backoffs := []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second}
			for _, d := range backoffs {
				time.Sleep(d)
				sm.ResubscribeAll()
			}
		}()
	}
}

// ResubscribeAll iterates over tracked subscriptions and re-requests them.
func (sm *SubscriptionManager) ResubscribeAll() {
	if sm == nil || sm.wrapper == nil {
		fmt.Println("SubscriptionManager or wrapper is nil, cannot resubscribe")
		return
	}
	sm.wrapper.metaMu.RLock()
	defer sm.wrapper.metaMu.RUnlock()

	client := sm.wrapper.client
	if client == nil || !sm.wrapper.connected.Load() {
		return
	}

	for reqID, meta := range sm.wrapper.subscriptionMeta {
		fmt.Printf("Re-subscribing: %s (ReqID: %d)\n", meta.Symbol, reqID)
		client.ReqMarketDataType(4) // Request delayed market data
		client.ReqMktData(reqID, meta.Contract, "", false, false, nil)
	}
}
