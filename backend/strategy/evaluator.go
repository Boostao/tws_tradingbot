package strategy

import (
	"tws_traderbot_go/backend/models"
)

func CrossAbove(seriesA, seriesB []float64) bool {
	if len(seriesA) < 2 || len(seriesB) < 2 {
		return false
	}
	last := len(seriesA) - 1
	return seriesA[last-1] <= seriesB[last-1] && seriesA[last] > seriesB[last]
}

func Slope(series []float64, lookback int) float64 {
	if len(series) < lookback || lookback <= 0 {
		return 0.0
	}
	last := len(series) - 1
	start := last - lookback
	return (series[last] - series[start]) / float64(lookback)
}

func EvaluateConditions(c *models.Condition, data map[string][]float64) bool {
	// A simple evaluator based on the ported python struct
	
	switch c.Type {
	case models.ConditionCrossAbove:
		// Needs full evaluation pipeline to calculate dynamically. 
		// Typically indicatorA and B would be pre-calculated in the data map.
		valA, okA := data["indicatorA"]
		valB, okB := data["indicatorB"]
		if okA && okB {
			return CrossAbove(valA, valB)
		}
	case models.ConditionGreater:
		valA, okA := data["indicatorA"]
		if okA && len(valA) > 0 {
			return valA[len(valA)-1] > c.Threshold
		}
	case models.ConditionSlopeBelow:
		valA, okA := data["indicatorA"]
		if okA && len(valA) > 0 {
			return Slope(valA, c.LookbackPeriods) < c.Threshold
		}
	}
	return false
}
