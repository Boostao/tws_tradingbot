package bot

import (
	"fmt"
	"math"
	"strings"

	"tws_traderbot/backend/models"

	"github.com/scmhub/ibapi"
)

// CalculatePositionSize computes position size in shares based on 3 modes:
// 1. "shares" (fixed share count)
// 2. "dollar" / "cash" ($ notional value / price)
// 3. "percent" / "%" (% of portfolio equity / price)
func CalculatePositionSize(item *models.WatchlistItem, price float64, totalEquity float64) (float64, error) {
	if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
		return 0, fmt.Errorf("invalid market price: %v (must be positive finite number)", price)
	}

	if item == nil {
		return 0, fmt.Errorf("watchlist item is nil")
	}

	mode := "shares"
	if strings.TrimSpace(item.SizeMode) != "" {
		mode = strings.ToLower(strings.TrimSpace(item.SizeMode))
	}
	val := item.SizeValue

	var shares float64

	switch mode {
	case "shares", "# shares", "share", "qty", "fixed":
		if val < 1 || math.IsNaN(val) || math.IsInf(val, 0) {
			return 0, fmt.Errorf("invalid share quantity: %v (must be >= 1)", val)
		}
		shares = math.Floor(val)

	case "dollar", "$", "notional", "$ notional", "cash":
		if val <= 0 || math.IsNaN(val) || math.IsInf(val, 0) {
			return 0, fmt.Errorf("invalid notional dollar value: %v (must be > 0)", val)
		}
		shares = math.Floor(val / price)
		if shares < 1 || math.IsNaN(shares) || math.IsInf(shares, 0) {
			return 0, fmt.Errorf("notional size $%.2f insufficient for share price $%.2f (calculated < 1 share)", val, price)
		}

	case "percent", "%", "% portfolio", "portfolio":
		if totalEquity <= 0 || math.IsNaN(totalEquity) || math.IsInf(totalEquity, 0) {
			return 0, fmt.Errorf("invalid portfolio equity: %v (must be > 0)", totalEquity)
		}
		if val <= 0 || val > 100 || math.IsNaN(val) || math.IsInf(val, 0) {
			return 0, fmt.Errorf("invalid portfolio percentage: %v%% (must be between 0 and 100)", val)
		}
		targetNotional := totalEquity * (val / 100.0)
		shares = math.Floor(targetNotional / price)
		if shares < 1 || math.IsNaN(shares) || math.IsInf(shares, 0) {
			return 0, fmt.Errorf("allocated notional $%.2f (%.1f%% of $%.2f) insufficient for share price $%.2f (calculated < 1 share)",
				targetNotional, val, totalEquity, price)
		}

	default:
		// Default fallback to fixed shares
		if val < 1 || math.IsNaN(val) || math.IsInf(val, 0) {
			return 0, fmt.Errorf("invalid share quantity: %v (must be >= 1)", val)
		}
		shares = math.Floor(val)
	}

	// Universal post-calculation guard: ensure shares is positive, non-NaN, and finite
	if shares <= 0 || math.IsNaN(shares) || math.IsInf(shares, 0) {
		return 0, fmt.Errorf("calculated share size is non-positive or non-finite: %v", shares)
	}

	return shares, nil
}

// BracketOrderBundle contains the parent order and both OCA bracket child orders.
type BracketOrderBundle struct {
	Parent     *ibapi.Order
	TakeProfit *ibapi.Order
	StopLoss   *ibapi.Order
	OCAGroup   string
}

// CreateBracketOrders generates an IBKR bracket order bundle:
// 1. Parent order (Transmit = false)
// 2. Take Profit child order (OrderType: LMT, Action: opposite, Transmit = false, OCAGroup, OCAType = 1)
// 3. Stop Loss child order (OrderType: STP, Action: opposite, Transmit = true, OCAGroup, OCAType = 1)
func CreateBracketOrders(
	parentOrderID, tpOrderID, slOrderID int64,
	action string,
	qty int,
	orderType string,
	parentPrice float64,
	takeProfitPct float64,
	stopLossPct float64,
	ocaGroup string,
) (*BracketOrderBundle, error) {
	if qty <= 0 {
		return nil, fmt.Errorf("invalid order quantity: %d (must be >= 1)", qty)
	}
	if parentPrice <= 0 || math.IsNaN(parentPrice) || math.IsInf(parentPrice, 0) {
		return nil, fmt.Errorf("invalid parent price: %v (must be positive finite number)", parentPrice)
	}

	upperAction := strings.ToUpper(strings.TrimSpace(action))
	if upperAction != "BUY" && upperAction != "SELL" {
		return nil, fmt.Errorf("invalid action: %q (must be BUY or SELL)", action)
	}

	oppositeAction := "SELL"
	if upperAction == "SELL" {
		oppositeAction = "BUY"
	}

	if ocaGroup == "" {
		ocaGroup = fmt.Sprintf("OCA_%d", parentOrderID)
	}

	orderQty := ibapi.StringToDecimal(fmt.Sprintf("%d", qty))

	// 1. Parent order
	parent := ibapi.NewOrder()
	parent.OrderID = parentOrderID
	parent.Action = upperAction
	parent.TotalQuantity = orderQty
	parent.TIF = "DAY"
	parent.Transmit = false

	upperType := strings.ToUpper(strings.TrimSpace(orderType))
	if upperType == "LMT" || upperType == "LIMIT" {
		parent.OrderType = "LMT"
		parent.LmtPrice = roundPrice(parentPrice)
	} else {
		parent.OrderType = "MKT"
	}

	// 2. Take Profit Child Order (Limit)
	var tpPrice float64
	if upperAction == "BUY" {
		tpPrice = roundPrice(parentPrice * (1.0 + math.Abs(takeProfitPct)/100.0))
	} else {
		tpPrice = roundPrice(parentPrice * (1.0 - math.Abs(takeProfitPct)/100.0))
	}

	tp := ibapi.NewOrder()
	tp.OrderID = tpOrderID
	tp.ParentID = parentOrderID
	tp.Action = oppositeAction
	tp.OrderType = "LMT"
	tp.TotalQuantity = orderQty
	tp.LmtPrice = tpPrice
	tp.TIF = "GTC"
	tp.OCAGroup = ocaGroup
	tp.OCAType = 1 // 1 = CANCEL_WITH_BLOCK
	tp.Transmit = false

	// 3. Stop Loss Child Order (Stop)
	var slPrice float64
	if upperAction == "BUY" {
		slPrice = roundPrice(parentPrice * (1.0 - math.Abs(stopLossPct)/100.0))
	} else {
		slPrice = roundPrice(parentPrice * (1.0 + math.Abs(stopLossPct)/100.0))
	}

	if slPrice <= 0 || math.IsNaN(slPrice) || math.IsInf(slPrice, 0) || tpPrice <= 0 || math.IsNaN(tpPrice) || math.IsInf(tpPrice, 0) {
		return nil, fmt.Errorf("calculated bracket price is non-positive or non-finite: sl=%.2f tp=%.2f", slPrice, tpPrice)
	}

	sl := ibapi.NewOrder()
	sl.OrderID = slOrderID
	sl.ParentID = parentOrderID
	sl.Action = oppositeAction
	sl.OrderType = "STP"
	sl.TotalQuantity = orderQty
	sl.AuxPrice = slPrice
	sl.TIF = "GTC"
	sl.OCAGroup = ocaGroup
	sl.OCAType = 1 // 1 = CANCEL_WITH_BLOCK
	sl.Transmit = true // Last child transmits the entire bracket

	return &BracketOrderBundle{
		Parent:     parent,
		TakeProfit: tp,
		StopLoss:   sl,
		OCAGroup:   ocaGroup,
	}, nil
}

func roundPrice(p float64) float64 {
	return math.Round(p*100.0) / 100.0
}
