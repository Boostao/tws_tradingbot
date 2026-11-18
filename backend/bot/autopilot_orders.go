package bot

import (
	"fmt"
	"time"

	"tws_traderbot/backend/models"
	"github.com/scmhub/ibapi"
)

// Constants for autopilot order limits
const (
	MaxQtyPerOrder   = 1000
	MaxNotionalCap   = 100000.0
	MaxOrdersPerRun  = 50
	MaxOpenPositions = 10
)

// PlaceAutopilotOrder acts as the sole chokepoint for autopilot order placement.
func (c *AutopilotController) PlaceAutopilotOrder(orderId int64, contract *ibapi.Contract, action string, qty int, price float64, symbol string, decisionKey string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 0. Deduplication
	if decisionKey != "" && c.IsDuplicate(decisionKey) {
		return fmt.Errorf("autopilot order placement denied: duplicate decision %s", decisionKey)
	}

	fmt.Printf("DEBUG: PlaceAutopilotOrder: mode=%d, dryRun=%v, clientNil=%v, symbol=%s, key=%s\n", c.mode, c.engine.IsDryRun(), c.client == nil, symbol, decisionKey)

	// 1. Check autopilotExecMode
	if c.mode == ModePaperLive && c.client == nil {
		return fmt.Errorf("autopilot order placement denied: client not initialized")
	}

	// For non-PaperLive, allow only DryRun
	if c.mode != ModePaperLive && !c.engine.IsDryRun() {
		return fmt.Errorf("autopilot order placement denied: system not in PaperLive mode or DryRun")
	}

	// 2. ValidatePaperSafety
	if c.mode == ModePaperLive && !c.ValidatePaperSafety() {
		fmt.Printf("DEBUG: Autopilot denied: safety validation failed\n")
		return fmt.Errorf("autopilot order placement denied: safety validation failed")
	}

	// 3. Fail closed on missing/stale prices or invalid equity/sizing
	if price <= 0 {
		return fmt.Errorf("autopilot order placement denied: missing or stale price %f", price)
	}
	if qty <= 0 || qty > MaxQtyPerOrder {
		return fmt.Errorf("autopilot order placement denied: invalid quantity %d", qty)
	}
	if float64(qty)*price > MaxNotionalCap {
		return fmt.Errorf("autopilot order placement denied: notional cap exceeded")
	}

	// 4. Enforce max_orders_per_run
	if c.ordersPlaced >= MaxOrdersPerRun {
		fmt.Printf("DEBUG: Autopilot denied: max orders %d\n", c.ordersPlaced)
		return fmt.Errorf("autopilot order placement denied: max orders per run reached")
	}
	
	// 5. Enforce max_open_positions
	positions := c.engine.GetOpenPositions()
	if len(positions) >= MaxOpenPositions {
		fmt.Printf("DEBUG: Autopilot denied: positions %d\n", len(positions))
		return fmt.Errorf("autopilot order placement denied: max open positions reached")
	}

	// 6. Set OrderRef
	order := ibapi.NewOrder()
	order.OrderID = orderId
	order.Action = action
	order.TotalQuantity = ibapi.StringToDecimal(fmt.Sprintf("%d", qty))
	order.OrderType = "MKT" // Simplified for now
	order.OrderRef = "AUTOPILOT:" + c.runID
	order.Transmit = true
	// Ensure bracket orders can be linked if required
	if decisionKey != "" {
		order.OCAGroup = "OCA_" + decisionKey
		order.OCAType = 1
	}

	// 7. Call PlaceOrder
	if !c.engine.IsDryRun() {
		c.client.PlaceOrder(orderId, contract, order)
		c.ordersPlaced++
		c.activeOrders[orderId] = true
	} else {
		// Record simulated fill if dry run
		fill := &models.TradeRecord{
			OrderID:   orderId,
			Symbol:    contract.Symbol,
			Action:    action,
			Quantity:  qty,
			Price:     price,
			Status:    "SIMULATED_FILL",
			Mode:      "dry_run",
			CreatedAt: time.Now().UTC(),
		}
		if decisionKey != "" {
			fill.OCAGroup = "OCA_" + decisionKey
		}
		c.engine.RecordFill(fill)
	}
	return nil
}

// CancelAutopilotOrder acts as the sole chokepoint for autopilot order cancellation.
func (c *AutopilotController) CancelAutopilotOrder(orderId int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client == nil {
		return fmt.Errorf("autopilot order cancellation denied: client not initialized")
	}

	// Fail-safe cancellation
	c.client.CancelOrder(orderId, ibapi.OrderCancel{})
	delete(c.activeOrders, orderId)
	return nil
}
