package bot

import (
	"testing"

	"github.com/scmhub/ibapi"
)

// MockOrderSink implements OrderSink for testing.
type MockOrderSink struct {
	openOrders []int64
	statuses   []string
}

func (m *MockOrderSink) OpenOrder(orderID int64, contract *ibapi.Contract, order *ibapi.Order, orderState *ibapi.OrderState) {
	m.openOrders = append(m.openOrders, orderID)
}

func (m *MockOrderSink) OrderStatus(orderID int64, status string, filled ibapi.Decimal, remaining ibapi.Decimal, avgFillPrice float64, permID int64, parentID int64, lastFillPrice float64, clientID int64, whyHeld string, mktCapPrice float64) {
	m.statuses = append(m.statuses, status)
}

func (m *MockOrderSink) ExecDetails(reqID int64, contract *ibapi.Contract, execution *ibapi.Execution) {}
func (m *MockOrderSink) CommissionAndFeesReport(commissionReport *ibapi.CommissionAndFeesReport) {}

func TestNewReplayRuntime(t *testing.T) {
	engine := &Engine{}
	simSink := &MockOrderSink{}

	// Test with client != nil (should fail)
	_, err := NewReplayRuntime(engine, "not-nil", simSink)
	if err == nil {
		t.Error("Expected error when client is not nil, got nil")
	}

	// Test with client == nil (should succeed)
	_, err = NewReplayRuntime(engine, nil, simSink)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestReplayRuntime_Callbacks(t *testing.T) {
	engine := &Engine{}
	simSink := &MockOrderSink{}
	runtime, _ := NewReplayRuntime(engine, nil, simSink)

	// Test RecordOpenOrder
	runtime.RecordOpenOrder(1, nil, nil, nil)
	if len(simSink.openOrders) != 1 || simSink.openOrders[0] != 1 {
		t.Errorf("Expected 1 open order, got %d", len(simSink.openOrders))
	}

	// Test RecordOrderStatus
	runtime.RecordOrderStatus(1, "Filled", ibapi.StringToDecimal("10"), ibapi.StringToDecimal("0"), 100.0, 0, 0, 100.0, 0, "", 0)
	if len(simSink.statuses) != 1 || simSink.statuses[0] != "Filled" {
		t.Errorf("Expected 1 status, got %d", len(simSink.statuses))
	}
}
