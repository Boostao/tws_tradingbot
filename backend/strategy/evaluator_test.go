package strategy

import (
	"math"
	"testing"
	"tws_traderbot/backend/models"
)

func TestSMA(t *testing.T) {
	data := []float64{10, 20, 30, 40, 50}
	sma := SMA(data, 3)

	if !math.IsNaN(sma[0]) || !math.IsNaN(sma[1]) {
		t.Errorf("Expected leading NaNs")
	}

	if sma[2] != 20.0 { // (10+20+30)/3
		t.Errorf("Expected SMA 20, got %v", sma[2])
	}
	if sma[3] != 30.0 { // (20+30+40)/3
		t.Errorf("Expected SMA 30, got %v", sma[3])
	}
}

func TestCrossAbove(t *testing.T) {
	seriesA := []float64{40, 45, 55}
	seriesB := []float64{50, 50, 50}

	// 45 <= 50, but 55 > 50 -> Crosses above!
	if !CrossAbove(seriesA, seriesB) {
		t.Errorf("Expected true for cross above")
	}

	seriesC := []float64{60, 60, 60}
	if CrossAbove(seriesA, seriesC) {
		t.Errorf("Expected false, never crossed")
	}
}

func TestEvaluateCondition(t *testing.T) {
	cond := &models.Condition{
		Type:            models.ConditionSlopeBelow,
		Threshold:       0.0,
		LookbackPeriods: 2,
	}

	// Vix sloping down
	data := map[string][]float64{
		"indicatorA": {25, 20, 18},
	}
	
	result := EvaluateConditions(cond, data)
	if !result {
		t.Errorf("Expected VIX to trigger down-slope filter")
	}
}
