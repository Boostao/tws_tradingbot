package strategy

import (
	"math"
	"testing"
	"time"

	"tws_traderbot/backend/models"
)

func TestEvaluateConditions_AllOperators(t *testing.T) {
	// 1. Cross Above (series vs series)
	condCross := &models.Condition{Type: models.ConditionCrossAbove}
	dataCross := map[string][]float64{
		"indicatorA": {10, 20},
		"indicatorB": {15, 15},
	}
	if !EvaluateConditions(condCross, dataCross) {
		t.Errorf("expected CrossAbove series true")
	}

	// Cross Above (series vs scalar)
	condCrossScalar := &models.Condition{Type: models.ConditionCrossAbove, Threshold: 50.0}
	dataScalar := map[string][]float64{
		"indicatorA": {45, 55},
	}
	if !EvaluateConditions(condCrossScalar, dataScalar) {
		t.Errorf("expected CrossAboveScalar true")
	}

	// 2. Cross Below (series vs series)
	condCrossBelow := &models.Condition{Type: models.ConditionCrossBelow}
	dataCrossBelow := map[string][]float64{
		"indicatorA": {20, 10},
		"indicatorB": {15, 15},
	}
	if !EvaluateConditions(condCrossBelow, dataCrossBelow) {
		t.Errorf("expected CrossBelow series true")
	}

	// Cross Below (series vs scalar)
	condCrossBelowScalar := &models.Condition{Type: models.ConditionCrossBelow, Threshold: 30.0}
	dataBelowScalar := map[string][]float64{
		"indicatorA": {35, 25},
	}
	if !EvaluateConditions(condCrossBelowScalar, dataBelowScalar) {
		t.Errorf("expected CrossBelowScalar true")
	}

	// 3. Greater Than
	condGreater := &models.Condition{Type: models.ConditionGreater, Threshold: 100.0}
	if !EvaluateConditions(condGreater, map[string][]float64{"indicatorA": {105.0}}) {
		t.Errorf("expected Greater true")
	}
	if EvaluateConditions(condGreater, map[string][]float64{"indicatorA": {95.0}}) {
		t.Errorf("expected Greater false")
	}

	// Greater Than (indicator B)
	condGreaterB := &models.Condition{Type: models.ConditionGreater}
	if !EvaluateConditions(condGreaterB, map[string][]float64{"indicatorA": {110}, "indicatorB": {100}}) {
		t.Errorf("expected Greater indicatorB true")
	}

	// 4. Less Than
	condLess := &models.Condition{Type: models.ConditionLess, Threshold: 50.0}
	if !EvaluateConditions(condLess, map[string][]float64{"indicatorA": {45.0}}) {
		t.Errorf("expected Less true")
	}
	if EvaluateConditions(condLess, map[string][]float64{"indicatorA": {55.0}}) {
		t.Errorf("expected Less false")
	}

	// Less Than (indicator B)
	condLessB := &models.Condition{Type: models.ConditionLess}
	if !EvaluateConditions(condLessB, map[string][]float64{"indicatorA": {80}, "indicatorB": {90}}) {
		t.Errorf("expected Less indicatorB true")
	}

	// 5. Slope Above
	condSlopeUp := &models.Condition{Type: models.ConditionSlopeAbove, Threshold: 0.5, LookbackPeriods: 2}
	if !EvaluateConditions(condSlopeUp, map[string][]float64{"indicatorA": {10, 12, 15}}) { // slope = (15-10)/2 = 2.5 > 0.5
		t.Errorf("expected SlopeAbove true")
	}

	// 6. Slope Below
	condSlopeDown := &models.Condition{Type: models.ConditionSlopeBelow, Threshold: 0.0, LookbackPeriods: 2}
	if !EvaluateConditions(condSlopeDown, map[string][]float64{"indicatorA": {25, 20, 18}}) {
		t.Errorf("expected SlopeBelow true")
	}

	// 7. Within Range (value)
	condWithin := &models.Condition{Type: models.ConditionWithin}
	if !EvaluateConditions(condWithin, map[string][]float64{"indicatorA": {50.0}}) {
		t.Errorf("expected WithinRange 50.0 true")
	}
	if EvaluateConditions(condWithin, map[string][]float64{"indicatorA": {20.0}}) {
		t.Errorf("expected WithinRange 20.0 false")
	}

	// Within Range (time)
	nowStr := time.Now().Format("15:04")
	condTime := &models.Condition{Type: models.ConditionWithin, RangeStart: "00:00", RangeEnd: "23:59"}
	if !EvaluateConditions(condTime, map[string][]float64{}) {
		t.Errorf("expected current time %s to fall in 00:00..23:59", nowStr)
	}
}

func TestEvaluateConditions_FailClosed(t *testing.T) {
	// Nil condition
	if EvaluateConditions(nil, map[string][]float64{"indicatorA": {100}}) {
		t.Errorf("expected fail-closed on nil condition")
	}

	// Empty data map
	if EvaluateConditions(&models.Condition{Type: models.ConditionGreater}, map[string][]float64{}) {
		t.Errorf("expected fail-closed on empty data")
	}

	// NaN inputs
	nanData := map[string][]float64{
		"indicatorA": {math.NaN(), math.NaN()},
		"indicatorB": {10, 20},
	}
	if EvaluateConditions(&models.Condition{Type: models.ConditionCrossAbove}, nanData) {
		t.Errorf("expected fail-closed on NaN CrossAbove")
	}
	if EvaluateConditions(&models.Condition{Type: models.ConditionGreater, Threshold: 50.0}, map[string][]float64{"indicatorA": {math.NaN()}}) {
		t.Errorf("expected fail-closed on NaN Greater")
	}
	if EvaluateConditions(&models.Condition{Type: models.ConditionLess, Threshold: 50.0}, map[string][]float64{"indicatorA": {math.NaN()}}) {
		t.Errorf("expected fail-closed on NaN Less")
	}

	// Insufficient lookback for slope
	if EvaluateConditions(&models.Condition{Type: models.ConditionSlopeAbove, LookbackPeriods: 5}, map[string][]float64{"indicatorA": {1, 2}}) {
		t.Errorf("expected fail-closed on insufficient lookback")
	}

	// NaN lookback for slope: SlopeAbove -0.5 and SlopeBelow 0.5 must fail closed (false)
	nanSlopeData := map[string][]float64{"indicatorA": {math.NaN(), math.NaN(), math.NaN()}}
	if EvaluateConditions(&models.Condition{Type: models.ConditionSlopeAbove, Threshold: -0.5, LookbackPeriods: 2}, nanSlopeData) {
		t.Errorf("expected fail-closed false on NaN SlopeAbove")
	}
	if EvaluateConditions(&models.Condition{Type: models.ConditionSlopeBelow, Threshold: 0.5, LookbackPeriods: 2}, nanSlopeData) {
		t.Errorf("expected fail-closed false on NaN SlopeBelow")
	}

	// Direct Slope() function check
	sVal := Slope([]float64{math.NaN(), 10.0}, 1)
	if !math.IsNaN(sVal) {
		t.Errorf("expected Slope to return NaN on NaN input, got %v", sVal)
	}

	// IndicatorB required by condition model but missing or empty in data
	dummyB := &models.Indicator{Type: models.IndicatorPrice}
	condGreaterBRequired := &models.Condition{
		Type:       models.ConditionGreater,
		IndicatorA: &models.Indicator{Type: models.IndicatorPrice},
		IndicatorB: dummyB,
		Threshold:  0.0,
	}
	if EvaluateConditions(condGreaterBRequired, map[string][]float64{"indicatorA": {150.0}}) {
		t.Errorf("expected fail-closed on missing IndicatorB in data")
	}
	if EvaluateConditions(condGreaterBRequired, map[string][]float64{"indicatorA": {150.0}, "indicatorB": {}}) {
		t.Errorf("expected fail-closed on empty IndicatorB slice in data")
	}

	condCrossBRequired := &models.Condition{
		Type:       models.ConditionCrossAbove,
		IndicatorA: &models.Indicator{Type: models.IndicatorPrice},
		IndicatorB: dummyB,
		Threshold:  50.0,
	}
	if EvaluateConditions(condCrossBRequired, map[string][]float64{"indicatorA": {45.0, 55.0}}) {
		t.Errorf("expected fail-closed on missing IndicatorB for CrossAbove (never fall back to scalar)")
	}

	condLessBRequired := &models.Condition{
		Type:       models.ConditionLess,
		IndicatorA: &models.Indicator{Type: models.IndicatorPrice},
		IndicatorB: dummyB,
		Threshold:  200.0,
	}
	if EvaluateConditions(condLessBRequired, map[string][]float64{"indicatorA": {150.0}}) {
		t.Errorf("expected fail-closed on missing IndicatorB for ConditionLess")
	}
}

func TestDynamicEvaluator_MultiTimeframe(t *testing.T) {
	bundle := NewBarFrameBundle("AAPL.NASDAQ")

	// Add 35 daily bars with a clean moving average crossover on the last bar
	for i := 0; i < 35; i++ {
		p := 100.0 - float64(i)*0.8 // downward trend so EMA 10 < EMA 30
		if i == 34 {
			p = 250.0 // sudden massive jump to cross EMA 10 above EMA 30 on the final bar
		}
		bundle.AddBar("AAPL.NASDAQ", models.TimeframeD1, int64(i*86400), p-1, p+2, p-2, p, 1000)
	}

	strat := &models.Strategy{
		ID:   "test_strat",
		Name: "Test Golden Cross Strategy",
		Rules: []models.Rule{
			{
				ID:       "rule_ema_cross",
				Name:     "EMA 10/30 Crossover",
				Scope:    models.ScopeGlobal,
				Action:   models.ActionBuy,
				Enabled:  true,
				Priority: 10,
				Condition: &models.Condition{
					Type: models.ConditionCrossAbove,
					IndicatorA: &models.Indicator{
						Type:      models.IndicatorEMA,
						Timeframe: models.TimeframeD1,
						Period:    10,
					},
					IndicatorB: &models.Indicator{
						Type:      models.IndicatorEMA,
						Timeframe: models.TimeframeD1,
						Period:    30,
					},
				},
			},
			{
				ID:       "rule_disabled",
				Name:     "Disabled Rule",
				Scope:    models.ScopeGlobal,
				Action:   models.ActionSell,
				Enabled:  false,
				Priority: 5,
				Condition: &models.Condition{
					Type: models.ConditionGreater,
					IndicatorA: &models.Indicator{
						Type:      models.IndicatorPrice,
						Timeframe: models.TimeframeD1,
					},
					Threshold: 10.0,
				},
			},
		},
	}

	signals := EvaluateRules(strat, bundle, "AAPL.NASDAQ")
	if len(signals) == 0 {
		t.Fatalf("expected at least one signal to fire")
	}
	if signals[0].RuleID != "rule_ema_cross" || signals[0].Action != models.ActionBuy {
		t.Errorf("unexpected signal: %+v", signals[0])
	}
}

func TestEvaluateRule_MissingIndicatorBFailClosed(t *testing.T) {
	bundle := NewBarFrameBundle()
	frameAAPL := bundle.GetOrSetFrame("AAPL", models.TimeframeD1)
	frameAAPL.AppendBar(OHLCVBar{Timestamp: 1000, Close: 150.0})

	msft := "MSFT"
	rule := &models.Rule{
		ID:      "rule_missing_b",
		Name:    "Test Missing B",
		Enabled: true,
		Condition: &models.Condition{
			Type: models.ConditionGreater,
			IndicatorA: &models.Indicator{
				Type:      models.IndicatorPrice,
				Timeframe: models.TimeframeD1,
			},
			IndicatorB: &models.Indicator{
				Type:      models.IndicatorPrice,
				Timeframe: models.TimeframeD1,
				Symbol:    &msft, // MSFT has 0 bars in bundle!
			},
			Threshold: 0.0,
		},
	}

	fired := EvaluateRule(rule, bundle, "AAPL", time.Now())
	if fired {
		t.Fatalf("expected EvaluateRule to fail closed when IndicatorB has 0 bars, but rule fired true")
	}
}

func TestEvaluateRules_ScopePerTickerAndIndicatorVIX(t *testing.T) {
	bundle := NewBarFrameBundle()
	bundle.AddBar("AAPL", models.TimeframeD1, 1000, 148, 152, 147, 150.0, 1000)
	bundle.AddBar("VIX", models.TimeframeD1, 1000, 16, 17, 15, 15.5, 0)

	strat := &models.Strategy{
		ID:   "strat_per_ticker_vix",
		Name: "Per-Ticker + VIX Filter",
		Rules: []models.Rule{
			{
				ID:      "r_vix_filter",
				Name:    "VIX Calm Regime",
				Scope:   models.ScopeGlobal,
				Action:  models.ActionFilter,
				Enabled: true,
				Condition: &models.Condition{
					Type:      models.ConditionLess,
					Threshold: 20.0,
					IndicatorA: &models.Indicator{
						Type:      models.IndicatorVIX, // Symbol nil -> must resolve on VIX, not AAPL!
						Timeframe: models.TimeframeD1,
					},
				},
			},
			{
				ID:      "r_per_ticker",
				Name:    "Per-Ticker Breakout Rule",
				Scope:   models.ScopePerTicker, // Must evaluate on AAPL even though Name != "AAPL"
				Action:  models.ActionBuy,
				Enabled: true,
				Condition: &models.Condition{
					Type:      models.ConditionGreater,
					Threshold: 100.0,
					IndicatorA: &models.Indicator{
						Type:      models.IndicatorPrice,
						Timeframe: models.TimeframeD1,
					},
				},
			},
		},
	}

	signals := EvaluateRules(strat, bundle, "AAPL")
	if len(signals) != 2 {
		t.Fatalf("expected 2 signals (VIX filter + per-ticker buy), got %d: %+v", len(signals), signals)
	}
}
