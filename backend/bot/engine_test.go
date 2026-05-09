package bot

import (
	"testing"
	"tws_traderbot_go/backend/models"
)

func TestEngineEvaluation(t *testing.T) {
	eng := NewEngine()

	// 1. Shouldn't evaluate if not running
	actions := eng.EvaluateTick("SPY", nil)
	if len(actions) > 0 {
		t.Errorf("Should not execute if engine is off")
	}

	// 2. Load basic strategy
	strat := &models.Strategy{
		ID:   "s1",
		Name: "Test Trend",
		Rules: []models.Rule{
			{
				Name:  "VIX Slope Below",
				Scope: models.ScopeGlobal,
				Condition: &models.Condition{
					Type:            models.ConditionSlopeBelow,
					Threshold:       0.0,
					LookbackPeriods: 2,
				},
				Action:  models.ActionBuy,
				Enabled: true,
			},
		},
	}

	// 3. Start engine
	_ = eng.StartBot(strat)
	if !eng.IsRunning {
		t.Errorf("Engine should be running")
	}

	// Prepare data: steep negative slope
	marketData := map[string][]float64{
		"indicatorA": {25, 20, 18},
	}

	// Evaluate
	actions = eng.EvaluateTick("SPY", marketData)
	if len(actions) != 1 || actions[0] != models.ActionBuy {
		t.Errorf("Expected ActionBuy, got %v", actions)
	}

	eng.StopBot()
	if eng.IsRunning {
		t.Errorf("Engine should be stopped")
	}
}
