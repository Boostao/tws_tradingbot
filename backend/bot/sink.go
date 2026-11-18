package bot

import (
	"github.com/scmhub/ibapi"
)

// OrderSink defines the interface for synthetic order callbacks in simulation.
type OrderSink interface {
	OpenOrder(orderID int64, contract *ibapi.Contract, order *ibapi.Order, orderState *ibapi.OrderState)
	OrderStatus(orderID int64, status string, filled ibapi.Decimal, remaining ibapi.Decimal, avgFillPrice float64, permID int64, parentID int64, lastFillPrice float64, clientID int64, whyHeld string, mktCapPrice float64)
	ExecDetails(reqID int64, contract *ibapi.Contract, execution *ibapi.Execution)
	CommissionAndFeesReport(commissionReport *ibapi.CommissionAndFeesReport)
}
