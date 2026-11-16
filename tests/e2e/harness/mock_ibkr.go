package harness

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/scmhub/ibapi"
)

// CapturedOrder records an order intercepted by MockIBClient without sending to live TWS.
type CapturedOrder struct {
	OrderID      int64
	Contract     *ibapi.Contract
	Order        *ibapi.Order
	Status       string
	SubmittedAt  time.Time
	FilledAt     time.Time
	FillPrice    float64
	RejectReason string
}

// MockIBClient provides a deterministic, thread-safe mock of Interactive Brokers TWS socket.
// It generates monotonic atomic order IDs, intercepts orders, and simulates execution callbacks
// entirely offline without network I/O or port bindings.
type MockIBClient struct {
	mu                sync.RWMutex
	connected         bool
	host              string
	port              int
	clientID          int
	serverVersion     int
	nextOrderID       atomic.Int64
	accounts          []string
	placedOrders      []CapturedOrder
	subscribedSymbols []string
	lastErrorCode     atomic.Int64
	lastErrorMsg      string
}

// NewMockIBClient creates and initializes a MockIBClient.
func NewMockIBClient() *MockIBClient {
	c := &MockIBClient{
		connected:         true,
		host:              "127.0.0.1",
		port:              7497,
		clientID:          1,
		serverVersion:     226,
		accounts:          []string{"DUO185814"},
		placedOrders:      make([]CapturedOrder, 0),
		subscribedSymbols: make([]string, 0),
	}
	c.nextOrderID.Store(100) // Default starting ID
	return c
}

// Connect simulates socket connection to TWS.
func (m *MockIBClient) Connect(host string, port int, clientID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.host = host
	m.port = port
	m.clientID = clientID
	m.connected = true
	return nil
}

// Disconnect simulates socket disconnection.
func (m *MockIBClient) Disconnect() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = false
}

// IsConnected returns whether the mock client is currently connected.
func (m *MockIBClient) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected
}

// GetPort returns the configured socket port.
func (m *MockIBClient) GetPort() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.port
}

// GetAccount returns the primary configured paper account.
func (m *MockIBClient) GetAccount() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.accounts) > 0 {
		return m.accounts[0]
	}
	return ""
}

// SimulateError simulates an incoming IBKR error callback.
func (m *MockIBClient) SimulateError(reqID int64, code int64, msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastErrorCode.Store(code)
	m.lastErrorMsg = msg
}

// GetLastErrorCode returns the most recent simulated error code.
func (m *MockIBClient) GetLastErrorCode() int64 {
	return m.lastErrorCode.Load()
}

// GetNextOrderID atomically retrieves and increments the next valid order ID.
func (m *MockIBClient) GetNextOrderID() (int64, error) {
	if !m.IsConnected() {
		return 0, fmt.Errorf("order ID sequence not initialized: broker disconnected")
	}
	id := m.nextOrderID.Add(1) - 1
	if id <= 0 {
		return 0, fmt.Errorf("order ID sequence not initialized: id <= 0")
	}
	return id, nil
}

// SetNextOrderID updates the sequence counter if newID > current.
func (m *MockIBClient) SetNextOrderID(newID int64) {
	for {
		curr := m.nextOrderID.Load()
		if newID <= curr {
			return
		}
		if m.nextOrderID.CompareAndSwap(curr, newID) {
			return
		}
	}
}

// ResetNextOrderID forcibly sets the sequence counter for testing unready/overflow boundaries.
func (m *MockIBClient) ResetNextOrderID(newID int64) {
	m.nextOrderID.Store(newID)
}

// PlaceOrder intercepts and records an order submission.
func (m *MockIBClient) PlaceOrder(orderID int64, contract *ibapi.Contract, order *ibapi.Order) {
	m.mu.Lock()
	defer m.mu.Unlock()

	captured := CapturedOrder{
		OrderID:     orderID,
		Contract:    contract,
		Order:       order,
		Status:      "Submitted",
		SubmittedAt: time.Now(),
	}
	m.placedOrders = append(m.placedOrders, captured)
}

// GetPlacedOrders returns a copy of all intercepted orders.
func (m *MockIBClient) GetPlacedOrders() []CapturedOrder {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]CapturedOrder, len(m.placedOrders))
	copy(res, m.placedOrders)
	return res
}

// ClearPlacedOrders clears intercepted orders list.
func (m *MockIBClient) ClearPlacedOrders() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.placedOrders = make([]CapturedOrder, 0)
}

// SubscribeMarketData records market data subscription for symbol.
func (m *MockIBClient) SubscribeMarketData(symbol string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscribedSymbols = append(m.subscribedSymbols, symbol)
}

// GetSubscribedSymbols returns all subscribed symbol names.
func (m *MockIBClient) GetSubscribedSymbols() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]string, len(m.subscribedSymbols))
	copy(res, m.subscribedSymbols)
	return res
}

// SimulateFill marks an intercepted order as filled at specified price.
func (m *MockIBClient) SimulateFill(orderID int64, price float64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.placedOrders {
		if m.placedOrders[i].OrderID == orderID {
			m.placedOrders[i].Status = "Filled"
			m.placedOrders[i].FilledAt = time.Now()
			m.placedOrders[i].FillPrice = price
			return true
		}
	}
	return false
}

// SimulateReject marks an intercepted order as rejected.
func (m *MockIBClient) SimulateReject(orderID int64, reason string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.placedOrders {
		if m.placedOrders[i].OrderID == orderID {
			m.placedOrders[i].Status = "Rejected"
			m.placedOrders[i].RejectReason = reason
			return true
		}
	}
	return false
}
