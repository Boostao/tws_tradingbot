package bot

import (
	"math"
	"testing"
	"tws_traderbot/backend/models"
)

func TestCalculatePositionSize_SharesMode(t *testing.T) {
	item := &models.WatchlistItem{
		Symbol:    "AAPL",
		SizeMode:  "shares",
		SizeValue: 250.0,
	}

	size, err := CalculatePositionSize(item, 150.0, 100000.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 250.0 {
		t.Errorf("expected 250 shares, got %f", size)
	}

	// Error when SizeValue < 1 in shares mode
	itemZero := &models.WatchlistItem{
		Symbol:    "MSFT",
		SizeMode:  "shares",
		SizeValue: 0,
	}
	_, err = CalculatePositionSize(itemZero, 300.0, 100000.0)
	if err == nil {
		t.Errorf("expected error for shares mode with SizeValue: 0 (must be >= 1)")
	}
}

func TestCalculatePositionSize_DollarMode(t *testing.T) {
	item := &models.WatchlistItem{
		Symbol:    "TSLA",
		SizeMode:  "dollar",
		SizeValue: 10000.0, // $10,000 notional
	}

	// Price $250 -> 40 shares
	size, err := CalculatePositionSize(item, 250.0, 100000.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 40.0 {
		t.Errorf("expected 40 shares, got %f", size)
	}

	// Price $300 -> floor(10000 / 300) = 33 shares
	size, err = CalculatePositionSize(item, 300.0, 100000.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 33.0 {
		t.Errorf("expected 33 shares, got %f", size)
	}

	// Insufficient notional: price $15,000 for $10,000 notional -> error
	_, err = CalculatePositionSize(item, 15000.0, 100000.0)
	if err == nil {
		t.Errorf("expected error for notional < price")
	}
}

func TestCalculatePositionSize_PercentMode(t *testing.T) {
	item := &models.WatchlistItem{
		Symbol:    "NVDA",
		SizeMode:  "percent",
		SizeValue: 5.0, // 5% of portfolio
	}

	// Total equity $100,000 -> 5% = $5,000
	// Share price $100 -> 50 shares
	size, err := CalculatePositionSize(item, 100.0, 100000.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 50.0 {
		t.Errorf("expected 50 shares, got %f", size)
	}

	// Invalid equity <= 0
	_, err = CalculatePositionSize(item, 100.0, 0)
	if err == nil {
		t.Errorf("expected error for 0 equity")
	}

	// Invalid percent > 100
	itemOver := &models.WatchlistItem{
		Symbol:    "NVDA",
		SizeMode:  "percent",
		SizeValue: 150.0,
	}
	_, err = CalculatePositionSize(itemOver, 100.0, 100000.0)
	if err == nil {
		t.Errorf("expected error for >100 percent")
	}
}

func TestCalculatePositionSize_InvalidPrice(t *testing.T) {
	item := &models.WatchlistItem{Symbol: "AAPL", SizeMode: "shares", SizeValue: 100}
	_, err := CalculatePositionSize(item, 0, 100000.0)
	if err == nil {
		t.Errorf("expected error for 0 price")
	}

	_, err = CalculatePositionSize(item, -10.0, 100000.0)
	if err == nil {
		t.Errorf("expected error for negative price")
	}
}

func TestCreateBracketOrders_BuyOrder(t *testing.T) {
	bundle, err := CreateBracketOrders(
		1001, 1002, 1003,
		"BUY",
		100,
		"MKT",
		150.0,
		2.0, // +2% TP = 153.00
		1.0, // -1% SL = 148.50
		"OCA_TEST_1",
	)
	if err != nil {
		t.Fatalf("unexpected error creating bracket bundle: %v", err)
	}

	// Parent Order
	if bundle.Parent.OrderID != 1001 {
		t.Errorf("Parent OrderID: expected 1001, got %d", bundle.Parent.OrderID)
	}
	if bundle.Parent.Action != "BUY" {
		t.Errorf("Parent Action: expected BUY, got %s", bundle.Parent.Action)
	}
	if bundle.Parent.Transmit != false {
		t.Errorf("Parent Transmit: expected false, got true")
	}

	// Take Profit Child Order
	if bundle.TakeProfit.OrderID != 1002 {
		t.Errorf("TP OrderID: expected 1002, got %d", bundle.TakeProfit.OrderID)
	}
	if bundle.TakeProfit.ParentID != 1001 {
		t.Errorf("TP ParentID: expected 1001, got %d", bundle.TakeProfit.ParentID)
	}
	if bundle.TakeProfit.Action != "SELL" {
		t.Errorf("TP Action: expected SELL, got %s", bundle.TakeProfit.Action)
	}
	if bundle.TakeProfit.OrderType != "LMT" {
		t.Errorf("TP OrderType: expected LMT, got %s", bundle.TakeProfit.OrderType)
	}
	if bundle.TakeProfit.LmtPrice != 153.00 {
		t.Errorf("TP LmtPrice: expected 153.00, got %f", bundle.TakeProfit.LmtPrice)
	}
	if bundle.TakeProfit.OCAGroup != "OCA_TEST_1" {
		t.Errorf("TP OCAGroup: expected OCA_TEST_1, got %s", bundle.TakeProfit.OCAGroup)
	}
	if bundle.TakeProfit.OCAType != 1 {
		t.Errorf("TP OCAType: expected 1, got %d", bundle.TakeProfit.OCAType)
	}
	if bundle.TakeProfit.Transmit != false {
		t.Errorf("TP Transmit: expected false, got true")
	}

	// Stop Loss Child Order
	if bundle.StopLoss.OrderID != 1003 {
		t.Errorf("SL OrderID: expected 1003, got %d", bundle.StopLoss.OrderID)
	}
	if bundle.StopLoss.ParentID != 1001 {
		t.Errorf("SL ParentID: expected 1001, got %d", bundle.StopLoss.ParentID)
	}
	if bundle.StopLoss.Action != "SELL" {
		t.Errorf("SL Action: expected SELL, got %s", bundle.StopLoss.Action)
	}
	if bundle.StopLoss.OrderType != "STP" {
		t.Errorf("SL OrderType: expected STP, got %s", bundle.StopLoss.OrderType)
	}
	if bundle.StopLoss.AuxPrice != 148.50 {
		t.Errorf("SL AuxPrice: expected 148.50, got %f", bundle.StopLoss.AuxPrice)
	}
	if bundle.StopLoss.OCAGroup != "OCA_TEST_1" {
		t.Errorf("SL OCAGroup: expected OCA_TEST_1, got %s", bundle.StopLoss.OCAGroup)
	}
	if bundle.StopLoss.OCAType != 1 {
		t.Errorf("SL OCAType: expected 1, got %d", bundle.StopLoss.OCAType)
	}
	if bundle.StopLoss.Transmit != true {
		t.Errorf("SL Transmit: expected true (last child transmits bracket), got false")
	}
}

func TestCreateBracketOrders_SellOrder(t *testing.T) {
	bundle, err := CreateBracketOrders(
		2001, 2002, 2003,
		"SELL",
		50,
		"MKT",
		200.0,
		3.0, // TP: 3% below 200 = 194.00
		1.5, // SL: 1.5% above 200 = 203.00
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error creating sell bracket: %v", err)
	}

	if bundle.Parent.Action != "SELL" {
		t.Errorf("Parent Action: expected SELL, got %s", bundle.Parent.Action)
	}
	if bundle.TakeProfit.Action != "BUY" {
		t.Errorf("TP Action: expected BUY, got %s", bundle.TakeProfit.Action)
	}
	if bundle.TakeProfit.LmtPrice != 194.00 {
		t.Errorf("TP LmtPrice: expected 194.00, got %f", bundle.TakeProfit.LmtPrice)
	}
	if bundle.StopLoss.Action != "BUY" {
		t.Errorf("SL Action: expected BUY, got %s", bundle.StopLoss.Action)
	}
	if bundle.StopLoss.AuxPrice != 203.00 {
		t.Errorf("SL AuxPrice: expected 203.00, got %f", bundle.StopLoss.AuxPrice)
	}
	if bundle.StopLoss.Transmit != true {
		t.Errorf("SL Transmit: expected true")
	}
}

func TestCalculatePositionSize_FailClosedHardening(t *testing.T) {
	// 1. Nil item
	if _, err := CalculatePositionSize(nil, 100.0, 100000.0); err == nil {
		t.Errorf("expected error for nil item")
	}

	// 2. Non-finite prices
	validItem := &models.WatchlistItem{Symbol: "AAPL", SizeMode: "shares", SizeValue: 100.0}
	for _, p := range []float64{0.0, -10.0, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := CalculatePositionSize(validItem, p, 100000.0); err == nil {
			t.Errorf("expected error for non-positive or non-finite price %v", p)
		}
	}

	// 3. Percent mode fail-closed
	for _, badVal := range []float64{0.0, -5.0, 100.1, 150.0, math.NaN(), math.Inf(1)} {
		item := &models.WatchlistItem{Symbol: "NVDA", SizeMode: "percent", SizeValue: badVal}
		if _, err := CalculatePositionSize(item, 100.0, 100000.0); err == nil {
			t.Errorf("expected error for percent mode with SizeValue=%v", badVal)
		}
	}

	// 4. Dollar mode fail-closed
	for _, badVal := range []float64{0.0, -100.0, math.NaN(), math.Inf(1)} {
		item := &models.WatchlistItem{Symbol: "TSLA", SizeMode: "dollar", SizeValue: badVal}
		if _, err := CalculatePositionSize(item, 100.0, 100000.0); err == nil {
			t.Errorf("expected error for dollar mode with SizeValue=%v", badVal)
		}
	}

	// 5. Shares mode fail-closed
	for _, badVal := range []float64{0.0, 0.5, -10.0, math.NaN(), math.Inf(1)} {
		item := &models.WatchlistItem{Symbol: "MSFT", SizeMode: "shares", SizeValue: badVal}
		if _, err := CalculatePositionSize(item, 100.0, 100000.0); err == nil {
			t.Errorf("expected error for shares mode with SizeValue=%v", badVal)
		}
	}

	// 6. Percent mode with invalid equity
	for _, badEq := range []float64{0.0, -1000.0, math.NaN(), math.Inf(1)} {
		item := &models.WatchlistItem{Symbol: "NVDA", SizeMode: "percent", SizeValue: 5.0}
		if _, err := CalculatePositionSize(item, 100.0, badEq); err == nil {
			t.Errorf("expected error for percent mode with totalEquity=%v", badEq)
		}
	}
}

func TestCreateBracketOrders_NonFinitePricesAndFailClosed(t *testing.T) {
	// Non-finite parent prices
	for _, p := range []float64{0.0, -50.0, math.NaN(), math.Inf(1), math.Inf(-1)} {
		_, err := CreateBracketOrders(1, 2, 3, "BUY", 100, "MKT", p, 2.0, 1.0, "")
		if err == nil {
			t.Errorf("expected error for parent price %v", p)
		}
	}

	// Invalid action strings
	for _, act := range []string{"ALERT", "FILTER", "", "BUY_LONG", "SHORT_SELL", "HOLD"} {
		_, err := CreateBracketOrders(1, 2, 3, act, 100, "MKT", 100.0, 2.0, 1.0, "")
		if err == nil {
			t.Errorf("expected error for invalid action %q", act)
		}
	}

	// Invalid quantities
	for _, qty := range []int{0, -1, -100} {
		_, err := CreateBracketOrders(1, 2, 3, "BUY", qty, "MKT", 100.0, 2.0, 1.0, "")
		if err == nil {
			t.Errorf("expected error for invalid qty %d", qty)
		}
	}

	// StopLossPct that results in non-positive slPrice (e.g. 100% stop loss on BUY -> slPrice = 0.0)
	_, err := CreateBracketOrders(1, 2, 3, "BUY", 100, "MKT", 100.0, 2.0, 100.0, "")
	if err == nil {
		t.Errorf("expected error for 100%% stop loss leading to slPrice <= 0")
	}
}
