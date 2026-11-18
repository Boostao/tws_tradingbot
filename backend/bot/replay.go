package bot

import (
	"fmt"

	"github.com/scmhub/ibapi"
)

// ReplayRuntime provides an execution environment for simulation without a live TWS connection.
type ReplayRuntime struct {
	engine *Engine
	sink   OrderSink
}

// NewReplayRuntime constructs a new ReplayRuntime for pure simulation.
// It ensures that no live TWS client is connected to guarantee simulation purity.
func NewReplayRuntime(engine *Engine, client interface{}, simSink OrderSink) (*ReplayRuntime, error) {
	if client != nil {
		return nil, fmt.Errorf("simulation error: client must be nil for pure simulation runtime")
	}

	return &ReplayRuntime{
		engine: engine,
		sink:   simSink,
	}, nil
}

// RecordOpenOrder simulates an OpenOrder callback.
func (r *ReplayRuntime) RecordOpenOrder(orderID int64, contract *ibapi.Contract, order *ibapi.Order, orderState *ibapi.OrderState) {
	r.sink.OpenOrder(orderID, contract, order, orderState)
}

// RecordOrderStatus simulates an OrderStatus callback.
func (r *ReplayRuntime) RecordOrderStatus(orderID int64, status string, filled ibapi.Decimal, remaining ibapi.Decimal, avgFillPrice float64, permID int64, parentID int64, lastFillPrice float64, clientID int64, whyHeld string, mktCapPrice float64) {
	r.sink.OrderStatus(orderID, status, filled, remaining, avgFillPrice, permID, parentID, lastFillPrice, clientID, whyHeld, mktCapPrice)
}

// RecordExecDetails simulates an ExecDetails callback.
func (r *ReplayRuntime) RecordExecDetails(reqID int64, contract *ibapi.Contract, execution *ibapi.Execution) {
	r.sink.ExecDetails(reqID, contract, execution)
}

// RecordCommissionAndFeesReport simulates a CommissionAndFeesReport callback.
func (r *ReplayRuntime) RecordCommissionAndFeesReport(commissionReport *ibapi.CommissionAndFeesReport) {
	r.sink.CommissionAndFeesReport(commissionReport)
}
