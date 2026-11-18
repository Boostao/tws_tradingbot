package bot

import (
	"math"
	"strings"
	"testing"
	"time"
	"tws_traderbot/backend/models"
)

// Helper to construct a fully armed engine for gate testing
func setupFullyArmedEngine() (*Engine, *models.WatchlistItem) {
	eng := NewEngine()
	eng.AutopilotController = eng.Autopilot
	eng.SetGlobalEnabled(true) // Gate 1: Master breaker ON
	// Gate 2: RiskBreakerTripped is false
	_ = eng.StartBot(&models.Strategy{ID: "strat_1"}) // Gate 3: Engine running, Gate 5: active slot assigned

	// Gate 4: Workspace enabled
	eng.CockpitState.Workspaces[0].Enabled = true
	eng.CockpitState.ActiveWorkspaceID = "achat"
	// Gate 5: Exactly 1 slot enabled
	eng.CockpitState.Workspaces[0].StrategySlots[0].Enabled = true
	eng.CockpitState.Workspaces[0].StrategySlots[0].StrategyID = "strat_1"

	// Gate 6 & 7: Watchlist item enabled & orderable
	item := &models.WatchlistItem{
		Symbol:            "AAPL",
		Exchange:          "SMART",
		Venue:             "SMART",
		InstrumentID:      "AAPL.SMART",
		Enabled:           true,
		IsOrderable:       true,
		IsRegimeReference: false,
		SizeMode:          "shares",
		SizeValue:         100.0,
	}
	eng.SetWatchlistItem(item)

	// Gate 8: Wednesday 10:30 AM ET (RTH)
	eng.SetTimeProvider(func() time.Time {
		return time.Date(2026, 11, 18, 10, 30, 0, 0, eng.sessionGate.Location())
	})

	// Gate 9: Price > 0
	eng.SetLatestPrice("AAPL", 150.0)

	return eng, item
}

func TestGate1_MasterCircuitBreaker(t *testing.T) {
	eng, _ := setupFullyArmedEngine()

	// Verify all gates pass initially
	can, reason := eng.CanExecute("AAPL", models.ActionBuy)
	if !can {
		t.Fatalf("expected all gates to pass, failed at: %s", reason)
	}

	// Trip Gate 1: Flip master circuit breaker to false
	eng.SetGlobalEnabled(false)
	can, reason = eng.CanExecute("AAPL", models.ActionBuy)
	if can {
		t.Errorf("Gate 1 failed: expected rejection when master circuit breaker is OFF")
	}
	if !strings.Contains(reason, "Gate 1") {
		t.Errorf("expected Gate 1 rejection reason, got: %s", reason)
	}
}

func TestGate2_DailyLossRiskBreaker(t *testing.T) {
	eng, _ := setupFullyArmedEngine()

	// Trip Gate 2: Manually trip RiskBreaker
	eng.RiskBreakerTripped.Store(true)
	can, reason := eng.CanExecute("AAPL", models.ActionBuy)
	if can {
		t.Errorf("Gate 2 failed: expected rejection when risk breaker is TRIPPED")
	}
	if !strings.Contains(reason, "Gate 2") {
		t.Errorf("expected Gate 2 rejection reason, got: %s", reason)
	}

	// Manual reset re-arms
	_ = eng.ResetCircuitBreaker()
	can, reason = eng.CanExecute("AAPL", models.ActionBuy)
	if !can {
		t.Errorf("expected re-armed engine to execute, failed: %s", reason)
	}
}

func TestGate3_EngineRunning(t *testing.T) {
	eng, _ := setupFullyArmedEngine()

	// Stop engine
	eng.StopBot()
	can, reason := eng.CanExecute("AAPL", models.ActionBuy)
	if can {
		t.Errorf("Gate 3 failed: expected rejection when engine is STOPPED")
	}
	if !strings.Contains(reason, "Gate 3") {
		t.Errorf("expected Gate 3 rejection reason, got: %s", reason)
	}
}

func TestGate4_ActiveWorkspaceEnabled(t *testing.T) {
	eng, _ := setupFullyArmedEngine()

	// Disable active workspace
	ws := eng.GetActiveWorkspace()
	ws.Enabled = false

	can, reason := eng.CanExecute("AAPL", models.ActionBuy)
	if can {
		t.Errorf("Gate 4 failed: expected rejection when active workspace is DISABLED")
	}
	if !strings.Contains(reason, "Gate 4") {
		t.Errorf("expected Gate 4 rejection reason, got: %s", reason)
	}
}

func TestGate5_ActiveStrategySlotAndConcurrency(t *testing.T) {
	eng, _ := setupFullyArmedEngine()

	// Case A: 0 active strategy slots
	eng.CockpitState.Workspaces[0].StrategySlots[0].Enabled = false
	can, reason := eng.CanExecute("AAPL", models.ActionBuy)
	if can {
		t.Errorf("Gate 5 failed: expected rejection when 0 strategy slots active")
	}
	if !strings.Contains(reason, "Gate 5") {
		t.Errorf("expected Gate 5 rejection reason, got: %s", reason)
	}

	// Case B: Slot enabled but StrategyID is empty
	eng.CockpitState.Workspaces[0].StrategySlots[0].Enabled = true
	eng.CockpitState.Workspaces[0].StrategySlots[0].StrategyID = ""
	can, reason = eng.CanExecute("AAPL", models.ActionBuy)
	if can {
		t.Errorf("Gate 5 failed: expected rejection when StrategyID is empty")
	}
	if !strings.Contains(reason, "Gate 5") {
		t.Errorf("expected Gate 5 rejection reason, got: %s", reason)
	}

	// Case C: Concurrency Violation: >1 strategy slot enabled simultaneously (violating 1/1 invariant)
	eng.CockpitState.Workspaces[0].StrategySlots[0].StrategyID = "strat_1"
	eng.CockpitState.Workspaces[0].StrategySlots[0].Enabled = true

	// Add second enabled slot
	eng.CockpitState.Workspaces[0].StrategySlots = append(eng.CockpitState.Workspaces[0].StrategySlots, models.CockpitStrategySlot{
		ID:         "slot_achat_2",
		StrategyID: "strat_2",
		Enabled:    true,
	})

	can, reason = eng.CanExecute("AAPL", models.ActionBuy)
	if can {
		t.Errorf("Gate 5 failed: expected rejection when multiple slots active (1/1 concurrency violation)")
	}
	if !strings.Contains(reason, "Gate 5") || !strings.Contains(reason, "Concurrency violation") {
		t.Errorf("expected concurrency violation reason, got: %s", reason)
	}
}

func TestGate6_WatchlistTickerEnabled(t *testing.T) {
	eng, item := setupFullyArmedEngine()

	// Case A: Ticker suspended (Enabled = false)
	item.Enabled = false
	can, reason := eng.CanExecute("AAPL", models.ActionBuy)
	if can {
		t.Errorf("Gate 6 failed: expected rejection when ticker is SUSPENDED")
	}
	if !strings.Contains(reason, "Gate 6") || !strings.Contains(reason, "SUSPENDED") {
		t.Errorf("expected SUSPENDED reason, got: %s", reason)
	}

	// Case B: Ticker not found in watchlist
	can, reason = eng.CanExecute("UNKNOWN_TICKER", models.ActionBuy)
	if can {
		t.Errorf("Gate 6 failed: expected rejection when ticker not found")
	}
	if !strings.Contains(reason, "Gate 6") || !strings.Contains(reason, "not found") {
		t.Errorf("expected not found reason, got: %s", reason)
	}
}

func TestGate7_OrderableEquityInvariant(t *testing.T) {
	eng, item := setupFullyArmedEngine()

	// Case A: Non-orderable
	item.IsOrderable = false
	can, reason := eng.CanExecute("AAPL", models.ActionBuy)
	if can {
		t.Errorf("Gate 7 failed: expected rejection when IsOrderable is false")
	}
	if !strings.Contains(reason, "Gate 7") {
		t.Errorf("expected Gate 7 rejection, got: %s", reason)
	}

	// Case B: Regime reference
	item.IsOrderable = true
	item.IsRegimeReference = true
	can, reason = eng.CanExecute("AAPL", models.ActionBuy)
	if can {
		t.Errorf("Gate 7 failed: expected rejection when IsRegimeReference is true")
	}
	if !strings.Contains(reason, "Gate 7") {
		t.Errorf("expected Gate 7 rejection, got: %s", reason)
	}
}

func TestGate8_SessionGating(t *testing.T) {
	eng, _ := setupFullyArmedEngine()

	// Case A: Outside RTH (Saturday weekend)
	eng.SetTimeProvider(func() time.Time {
		return time.Date(2026, 11, 21, 11, 0, 0, 0, eng.sessionGate.Location())
	})
	can, reason := eng.CanExecute("AAPL", models.ActionBuy)
	if can {
		t.Errorf("Gate 8 failed: expected rejection on weekend")
	}
	if !strings.Contains(reason, "Gate 8") || !strings.Contains(reason, "weekend") {
		t.Errorf("expected weekend rejection, got: %s", reason)
	}

	// Case B: Outside RTH (Wednesday 08:30 AM pre-market)
	eng.SetTimeProvider(func() time.Time {
		return time.Date(2026, 11, 18, 8, 30, 0, 0, eng.sessionGate.Location())
	})
	can, reason = eng.CanExecute("AAPL", models.ActionBuy)
	if can {
		t.Errorf("Gate 8 failed: expected rejection in pre-market")
	}
	if !strings.Contains(reason, "Gate 8") {
		t.Errorf("expected Gate 8 rejection, got: %s", reason)
	}
}

func TestGate9_PriceAndSizingValidity(t *testing.T) {
	eng, item := setupFullyArmedEngine()

	// Case A: Price <= 0
	eng.SetLatestPrice("AAPL", 0)
	can, reason := eng.CanExecute("AAPL", models.ActionBuy)
	if can {
		t.Errorf("Gate 9 failed: expected rejection when price is 0")
	}
	if !strings.Contains(reason, "Gate 9") || !strings.Contains(reason, "price") {
		t.Errorf("expected price rejection, got: %s", reason)
	}

	// Case B: Invalid Sizing (e.g. Dollar mode with insufficient notional)
	eng.SetLatestPrice("AAPL", 150.0)
	item.SizeMode = "dollar"
	item.SizeValue = 50.0 // $50 notional is insufficient for $150 share price (< 1 share)
	can, reason = eng.CanExecute("AAPL", models.ActionBuy)
	if can {
		t.Errorf("Gate 9 failed: expected rejection when calculated size is < 1 share")
	}
	if !strings.Contains(reason, "Gate 9") {
		t.Errorf("expected Gate 9 rejection, got: %s", reason)
	}
}

func TestFeature12_AutomatedDailyLossRiskBreakerTrip(t *testing.T) {
	eng, _ := setupFullyArmedEngine()
	eng.SetDailyLossLimit(1000.0) // $1,000 loss limit

	// PnL -$500: under limit -> does not trip
	eng.UpdatePnL(-300.0, -200.0)
	if eng.RiskBreakerTripped.Load() {
		t.Errorf("Risk breaker tripped prematurely at -$500 / $1000 limit")
	}
	if !eng.IsGlobalEnabled() {
		t.Errorf("GlobalEnabled should still be true at -$500")
	}

	// PnL -$1,050: breaches -$1,000 limit -> AUTOMATIC TRIP
	eng.UpdatePnL(-600.0, -450.0)
	if !eng.RiskBreakerTripped.Load() {
		t.Errorf("Risk breaker failed to trip automatically on breach -$1,050")
	}
	if eng.IsGlobalEnabled() {
		t.Errorf("GlobalEnabled should automatically flip to false upon breaker trip")
	}

	// Gate 1 and Gate 2 both reject
	can, reason := eng.CanExecute("AAPL", models.ActionBuy)
	if can {
		t.Errorf("expected engine to reject all executions when breaker is tripped")
	}
	t.Logf("Rejection after breaker trip: %s", reason)

	// Manual Reset attempt while actively breached must fail
	errReset := eng.ResetCircuitBreaker()
	if errReset == nil {
		t.Errorf("expected ResetCircuitBreaker to fail while loss limit is actively breached")
	}

	// PnL bounce does NOT auto-untrip:
	eng.UpdatePnL(-200.0, -100.0) // total -$300 recovers above -$1000 limit
	if !eng.RiskBreakerTripped.Load() {
		t.Errorf("Risk breaker should remain tripped across PnL bounce until manual reset")
	}

	// Manual Reset succeeds once PnL is no longer breached
	errReset = eng.ResetCircuitBreaker()
	if errReset != nil {
		t.Errorf("expected ResetCircuitBreaker to succeed after PnL recovery: %v", errReset)
	}
	if eng.RiskBreakerTripped.Load() {
		t.Errorf("Risk breaker should be cleared after ResetCircuitBreaker")
	}
	if !eng.IsGlobalEnabled() {
		t.Errorf("GlobalEnabled should be re-armed after ResetCircuitBreaker")
	}
}

func TestGate_ActionValidation(t *testing.T) {
	eng, _ := setupFullyArmedEngine()

	invalidActions := []models.ActionType{
		models.ActionAlert,
		models.ActionFilter,
		models.ActionType("HOLD"),
		models.ActionType(""),
		models.ActionType("CLOSE"),
		models.ActionType("SCALE"),
	}

	for _, act := range invalidActions {
		can, reason := eng.CanExecute("AAPL", act)
		if can {
			t.Errorf("expected rejection for invalid action %q, got can=true", act)
		}
		if !strings.Contains(reason, "Gate Rejected: Invalid or non-executable action") {
			t.Errorf("expected invalid action rejection reason for %q, got: %s", act, reason)
		}
	}

	// Valid actions (BUY, SELL) must pass
	for _, act := range []models.ActionType{models.ActionBuy, models.ActionSell, models.ActionType("buy"), models.ActionType("SELL")} {
		can, reason := eng.CanExecute("AAPL", act)
		if !can {
			t.Errorf("expected valid action %q to pass, failed with: %s", act, reason)
		}
	}
}

func TestGate9_NonFinitePrices(t *testing.T) {
	eng, _ := setupFullyArmedEngine()

	nonFinitePrices := []float64{
		math.NaN(),
		math.Inf(1),
		math.Inf(-1),
		0.0,
		-50.0,
	}

	for _, p := range nonFinitePrices {
		can, reason := eng.CanExecuteWithPrice("AAPL", models.ActionBuy, p)
		if can {
			t.Errorf("Gate 9 failed: expected rejection for price %v, got can=true", p)
		}
		if !strings.Contains(reason, "Gate 9 Rejected: Invalid market price for sizing") {
			t.Errorf("expected Gate 9 rejection reason for price %v, got: %s", p, reason)
		}
	}
}

func TestEvaluateTick_MasterBreakerLeakPrevention(t *testing.T) {
	eng, _ := setupFullyArmedEngine()

	// Configure a strategy with a triggering rule
	strat := &models.Strategy{
		ID:   "strat_leak_test",
		Name: "Breaker Test",
		Rules: []models.Rule{
			{
				Name:    "Always Trigger Buy",
				Scope:   models.ScopeGlobal,
				Action:  models.ActionBuy,
				Enabled: true,
				Condition: &models.Condition{
					Type:            models.ConditionSlopeBelow,
					Threshold:       100.0,
					LookbackPeriods: 2,
				},
			},
		},
	}
	eng.StopBot()
	if err := eng.StartBot(strat); err != nil {
		t.Fatalf("failed to start bot with test strategy: %v", err)
	}

	marketData := map[string][]float64{
		"Close":      {100.0, 95.0, 90.0},
		"indicatorA": {25.0, 20.0, 18.0},
	}

	// Breaker ON: should trigger
	actions := eng.EvaluateTick("AAPL", marketData)
	if len(actions) == 0 {
		t.Fatalf("expected action to trigger when breaker is ON")
	}

	// Flip Master Circuit Breaker OFF: must return nil immediately
	eng.SetGlobalEnabled(false)
	actions = eng.EvaluateTick("AAPL", marketData)
	if len(actions) != 0 {
		t.Errorf("expected nil actions when master circuit breaker is OFF, got: %v", actions)
	}

	// Trip Risk Breaker: must return nil immediately
	eng.SetGlobalEnabled(true)
	eng.RiskBreakerTripped.Store(true)
	actions = eng.EvaluateTick("AAPL", marketData)
	if len(actions) != 0 {
		t.Errorf("expected nil actions when risk breaker is TRIPPED, got: %v", actions)
	}
}

func TestGate4_WorkspaceResolution_FailClosed(t *testing.T) {
	eng, _ := setupFullyArmedEngine()

	// Corrupt active workspace ID to non-existent ID
	eng.CockpitState.ActiveWorkspaceID = "unknown_workspace_xyz"

	// GetActiveWorkspace must return nil
	ws := eng.GetActiveWorkspace()
	if ws != nil {
		t.Errorf("expected nil workspace for unknown ActiveWorkspaceID, got %+v", ws)
	}

	// Gate 4 must reject fail-closed
	can, reason := eng.CanExecute("AAPL", models.ActionBuy)
	if can {
		t.Errorf("Gate 4 failed: expected rejection when active workspace ID is unknown")
	}
	if !strings.Contains(reason, "Gate 4 Rejected: Active workspace is DISABLED") {
		t.Errorf("expected Gate 4 rejection reason, got: %s", reason)
	}
}
