//go:build testing

package bot

import (
	"testing"
	"github.com/scmhub/ibapi"
)

// MockWrapper implements ibapi.Wrapper for bypass testing
type MockWrapper struct {
	ibapi.Wrapper
	PlaceOrderCalled bool
}

func (w *MockWrapper) PlaceOrder(id int64, contract *ibapi.Contract, order *ibapi.Order) {
	w.PlaceOrderCalled = true
}

func TestAdversarialBypass_PlaceOrderDirectly(t *testing.T) {
	// 1. Setup
	mockWrapper := &MockWrapper{}
	client := ibapi.NewEClient(mockWrapper)
	ibapi.SetConnected(client)
	
	// Create an IBWrapper with a client
	wrapper := &IBWrapper{
		client: client,
	}
	
	// 2. Attack
	// Direct call to PlaceOrder on the client
	// This bypasses PlaceAutopilotOrder and its guards
	wrapper.client.PlaceOrder(12345, &ibapi.Contract{}, ibapi.NewOrder())
	
	// 3. Verify bypass
	// Direct client.PlaceOrder call does not trigger the wrapper's PlaceOrder
	t.Log("Adversarial bypass: PlaceOrder called directly, bypassing PlaceAutopilotOrder guards.")
}
