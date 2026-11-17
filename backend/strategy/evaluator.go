package strategy

import (
	"fmt"
	"math"
	"time"
	"tws_traderbot/backend/models"
)

var easternLoc = loadEasternLocation()

func loadEasternLocation() *time.Location {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.FixedZone("EST", -5*3600)
	}
	return loc
}

// -----------------------------------------------------------------------------
// 1. Backward-Compatible Flat Map Evaluator (Feature 34)
// -----------------------------------------------------------------------------

// EvaluateConditions evaluates a Condition model against flat series data.
// Preserves exact signature: func EvaluateConditions(c *models.Condition, data map[string][]float64) bool
func EvaluateConditions(c *models.Condition, data map[string][]float64) bool {
	if c == nil {
		return false
	}
	if c.Type == models.ConditionWithin && c.RangeStart != "" && c.RangeEnd != "" {
		if c.IndicatorB != nil {
			return false
		}
		return WithinRange(time.Now().In(easternLoc).Format("15:04"), c.RangeStart, c.RangeEnd)
	}
	if len(data) == 0 {
		return false
	}

	// Fail closed if IndicatorB is required by the condition model but missing or empty in data
	if c.IndicatorB != nil {
		b, okB := data["indicatorB"]
		if !okB || len(b) == 0 {
			return false
		}
	}

	switch c.Type {
	case models.ConditionCrossAbove, models.ConditionCrossesAbove:
		a, okA := data["indicatorA"]
		b, okB := data["indicatorB"]
		if okA && okB {
			return CrossAbove(a, b)
		}
		if c.IndicatorB != nil {
			return false
		}
		if okA && len(a) >= 2 {
			return CrossAboveScalar(a, c.Threshold)
		}

	case models.ConditionCrossBelow, models.ConditionCrossesBelow:
		a, okA := data["indicatorA"]
		b, okB := data["indicatorB"]
		if okA && okB {
			return CrossBelow(a, b)
		}
		if c.IndicatorB != nil {
			return false
		}
		if okA && len(a) >= 2 {
			return CrossBelowScalar(a, c.Threshold)
		}

	case models.ConditionGreater:
		a, okA := data["indicatorA"]
		if !okA || len(a) == 0 {
			return false
		}
		last := a[len(a)-1]
		if math.IsNaN(last) {
			return false
		}
		thresh := c.Threshold
		if b, okB := data["indicatorB"]; okB && len(b) > 0 {
			lastB := b[len(b)-1]
			if math.IsNaN(lastB) {
				return false
			}
			thresh = lastB
		} else if c.IndicatorB != nil {
			return false
		}
		return last > thresh

	case models.ConditionLess:
		a, okA := data["indicatorA"]
		if !okA || len(a) == 0 {
			return false
		}
		last := a[len(a)-1]
		if math.IsNaN(last) {
			return false
		}
		thresh := c.Threshold
		if b, okB := data["indicatorB"]; okB && len(b) > 0 {
			lastB := b[len(b)-1]
			if math.IsNaN(lastB) {
				return false
			}
			thresh = lastB
		} else if c.IndicatorB != nil {
			return false
		}
		return last < thresh

	case models.ConditionSlopeAbove:
		if c.IndicatorB != nil {
			return false
		}
		a, okA := data["indicatorA"]
		if !okA || len(a) <= c.LookbackPeriods || c.LookbackPeriods <= 0 {
			return false
		}
		slope := Slope(a, c.LookbackPeriods)
		if math.IsNaN(slope) {
			return false
		}
		return slope > c.Threshold

	case models.ConditionSlopeBelow:
		if c.IndicatorB != nil {
			return false
		}
		a, okA := data["indicatorA"]
		if !okA || len(a) <= c.LookbackPeriods || c.LookbackPeriods <= 0 {
			return false
		}
		slope := Slope(a, c.LookbackPeriods)
		if math.IsNaN(slope) {
			return false
		}
		return slope < c.Threshold

	case models.ConditionWithin:
		if c.IndicatorB != nil {
			return false
		}
		if c.RangeStart != "" && c.RangeEnd != "" {
			return WithinRange(time.Now().In(easternLoc).Format("15:04"), c.RangeStart, c.RangeEnd)
		}
		a, okA := data["indicatorA"]
		if okA && len(a) > 0 {
			last := a[len(a)-1]
			if math.IsNaN(last) {
				return false
			}
			return last >= 30.0 && last <= 70.0
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// 2. Dynamic Multi-Timeframe Evaluator (Features 20-34)
// -----------------------------------------------------------------------------

// ResolveIndicatorSeries calculates or retrieves an indicator from the memoized BarFrame cache.
func ResolveIndicatorSeries(bundle *BarFrameBundle, symbol string, ind *models.Indicator, defaultTf models.TimeframeUnit) []float64 {
	if ind == nil || bundle == nil {
		return nil
	}

	targetSymbol := symbol
	if ind.Symbol != nil && *ind.Symbol != "" {
		targetSymbol = *ind.Symbol
	} else if ind.Type == models.IndicatorVIX {
		targetSymbol = "VIX"
	}

	tf := ind.GetTimeframe(defaultTf)
	frame := bundle.GetFrame(targetSymbol, tf)
	if (frame == nil || frame.Len() == 0) && ind.Type == models.IndicatorVIX && (ind.Symbol == nil || *ind.Symbol == "") {
		if alt := bundle.GetFrame("VIX.TVC", tf); alt != nil && alt.Len() > 0 {
			targetSymbol = "VIX.TVC"
			frame = alt
		}
	}
	if frame == nil || frame.Len() == 0 {
		return nil
	}

	cacheKey := ind.CacheKey(targetSymbol)
	if cached, hit := frame.GetIndicator(cacheKey); hit {
		return cached
	}

	var series []float64
	comp := ind.GetComponent()
	period := ind.GetPeriod(14)
	source := ind.GetSource()

	switch ind.Type {
	case models.IndicatorPrice:
		series = frame.GetSeries(source)

	case models.IndicatorVolume:
		series = frame.GetSeries("volume")

	case models.IndicatorSMA:
		src := frame.GetSeries(source)
		series = SMA(src, period)

	case models.IndicatorEMA:
		src := frame.GetSeries(source)
		series = EMA(src, period)

	case models.IndicatorRSI:
		src := frame.GetSeries(source)
		series = ComputeRSI(src, period)

	case models.IndicatorMACD:
		src := frame.GetSeries(source)
		fast := 12
		slow := 26
		sig := 9
		if ind.Params != nil {
			if f, ok := ind.Params["fast"].(float64); ok && f > 0 {
				fast = int(f)
			}
			if s, ok := ind.Params["slow"].(float64); ok && s > 0 {
				slow = int(s)
			}
			if sg, ok := ind.Params["signal"].(float64); ok && sg > 0 {
				sig = int(sg)
			}
		}
		macdLine, signalLine, hist := ComputeMACD(src, fast, slow, sig)
		frame.SetIndicator(fmt.Sprintf("%s:%s:macd:macd", targetSymbol, tf), macdLine)
		frame.SetIndicator(fmt.Sprintf("%s:%s:macd:signal", targetSymbol, tf), signalLine)
		frame.SetIndicator(fmt.Sprintf("%s:%s:macd:histogram", targetSymbol, tf), hist)
		switch comp {
		case "signal":
			series = signalLine
		case "histogram", "hist":
			series = hist
		case "macd", "line":
			fallthrough
		default:
			series = macdLine
		}

	case models.IndicatorBollinger:
		src := frame.GetSeries(source)
		numStd := 2.0
		if ind.Params != nil {
			if m, ok := ind.Params["multiplier"].(float64); ok && m > 0 {
				numStd = m
			}
			if s, ok := ind.Params["std_dev"].(float64); ok && s > 0 {
				numStd = s
			}
		}
		basis, upper, lower, bw, pctB := ComputeBollingerBands(src, period, numStd)
		frame.SetIndicator(fmt.Sprintf("%s:%s:bollinger:basis", targetSymbol, tf), basis)
		frame.SetIndicator(fmt.Sprintf("%s:%s:bollinger:upper", targetSymbol, tf), upper)
		frame.SetIndicator(fmt.Sprintf("%s:%s:bollinger:lower", targetSymbol, tf), lower)
		frame.SetIndicator(fmt.Sprintf("%s:%s:bollinger:bandwidth", targetSymbol, tf), bw)
		frame.SetIndicator(fmt.Sprintf("%s:%s:bollinger:percent_b", targetSymbol, tf), pctB)
		switch comp {
		case "upper":
			series = upper
		case "lower":
			series = lower
		case "bandwidth":
			series = bw
		case "percent_b", "pct_b":
			series = pctB
		case "basis":
			fallthrough
		default:
			series = basis
		}

	case models.IndicatorStochastic:
		kLen := 15
		kSm := 5
		dSm := 5
		if ind.Params != nil {
			if k, ok := ind.Params["k_length"].(float64); ok && k > 0 {
				kLen = int(k)
			}
			if ks, ok := ind.Params["k_smooth"].(float64); ok && ks > 0 {
				kSm = int(ks)
			}
			if ds, ok := ind.Params["d_smooth"].(float64); ok && ds > 0 {
				dSm = int(ds)
			}
		}
		snap := frame.Snapshot()
		k, d := ComputeStochastic(snap.High, snap.Low, snap.Close, kLen, kSm, dSm)
		frame.SetIndicator(fmt.Sprintf("%s:%s:stochastic:k", targetSymbol, tf), k)
		frame.SetIndicator(fmt.Sprintf("%s:%s:stochastic:d", targetSymbol, tf), d)
		if comp == "d" {
			series = d
		} else {
			series = k
		}

	case models.IndicatorOBV:
		snap := frame.Snapshot()
		series = ComputeOBV(snap.Close, snap.Volume)

	case models.IndicatorAlligator:
		snap := frame.Snapshot()
		jaw, teeth, lips := ComputeWilliamsAlligator(snap.High, snap.Low)
		frame.SetIndicator(fmt.Sprintf("%s:%s:alligator:jaw", targetSymbol, tf), jaw)
		frame.SetIndicator(fmt.Sprintf("%s:%s:alligator:teeth", targetSymbol, tf), teeth)
		frame.SetIndicator(fmt.Sprintf("%s:%s:alligator:lips", targetSymbol, tf), lips)
		switch comp {
		case "teeth":
			series = teeth
		case "lips":
			series = lips
		case "jaw":
			fallthrough
		default:
			series = jaw
		}

	case models.IndicatorHeikinAshi:
		snap := frame.Snapshot()
		haOpen, haHigh, haLow, haClose := ComputeHeikinAshi(snap.Open, snap.High, snap.Low, snap.Close)
		frame.SetIndicator(fmt.Sprintf("%s:%s:heikin_ashi:open", targetSymbol, tf), haOpen)
		frame.SetIndicator(fmt.Sprintf("%s:%s:heikin_ashi:high", targetSymbol, tf), haHigh)
		frame.SetIndicator(fmt.Sprintf("%s:%s:heikin_ashi:low", targetSymbol, tf), haLow)
		frame.SetIndicator(fmt.Sprintf("%s:%s:heikin_ashi:close", targetSymbol, tf), haClose)
		switch comp {
		case "open":
			series = haOpen
		case "high":
			series = haHigh
		case "low":
			series = haLow
		case "close":
			fallthrough
		default:
			series = haClose
		}

	case models.IndicatorRVOL:
		series = ComputeRVOL(frame.GetSeries("volume"), period)

	case models.IndicatorGuppy:
		fMin, fMax, sMin, sMax, anc := ComputeGuppyMMA(frame.GetSeries("close"))
		switch comp {
		case "fast_min":
			series = fMin
		case "fast_max":
			series = fMax
		case "slow_min":
			series = sMin
		case "slow_max":
			series = sMax
		case "anchor", "ema200":
			series = anc
		default:
			series = fMax
		}

	case models.IndicatorATR:
		snap := frame.Snapshot()
		tr, atr, atrPct := ComputeATR(snap.High, snap.Low, snap.Close, period)
		frame.SetIndicator(fmt.Sprintf("%s:%s:atr:tr", targetSymbol, tf), tr)
		frame.SetIndicator(fmt.Sprintf("%s:%s:atr:atr", targetSymbol, tf), atr)
		frame.SetIndicator(fmt.Sprintf("%s:%s:atr:atr_pct", targetSymbol, tf), atrPct)
		if comp == "atr_pct" || comp == "pct" {
			series = atrPct
		} else {
			series = atr
		}

	case models.IndicatorRelativeStrength, models.IndicatorRelativePerformance:
		benchFrame := bundle.GetFrame("SPX", tf)
		if benchFrame == nil {
			benchFrame = bundle.GetFrame("SPY", tf)
		}
		var benchCloses []float64
		if benchFrame != nil {
			benchCloses = benchFrame.GetSeries("close")
		}
		series = ComputeRelativeStrength(frame.GetSeries("close"), benchCloses, period)

	case models.IndicatorHigh52W:
		series = ComputeHigh52W(frame.GetSeries("high"), 252)

	default:
		series = frame.GetSeries(source)
	}

	if len(series) > 0 {
		frame.SetIndicator(cacheKey, series)
	}
	return series
}

// EvaluateRule dynamically evaluates a single Rule against a BarFrameBundle.
func EvaluateRule(rule *models.Rule, bundle *BarFrameBundle, symbol string, now time.Time) bool {
	if rule == nil || !rule.Enabled || rule.Condition == nil {
		return false
	}

	cond := rule.Condition

	// Special case: Time-of-day condition without market data
	if cond.Type == models.ConditionWithin && cond.RangeStart != "" && cond.RangeEnd != "" {
		timeStr := now.In(easternLoc).Format("15:04")
		return WithinRange(timeStr, cond.RangeStart, cond.RangeEnd)
	}

	seriesA := ResolveIndicatorSeries(bundle, symbol, cond.IndicatorA, models.TimeframeD1)
	if len(seriesA) == 0 {
		return false
	}

	var seriesB []float64
	if cond.IndicatorB != nil {
		seriesB = ResolveIndicatorSeries(bundle, symbol, cond.IndicatorB, models.TimeframeD1)
		if len(seriesB) == 0 {
			return false
		}
	}

	switch cond.Type {
	case models.ConditionCrossAbove, models.ConditionCrossesAbove:
		if len(seriesB) > 0 {
			return CrossAbove(seriesA, seriesB)
		}
		return CrossAboveScalar(seriesA, cond.Threshold)

	case models.ConditionCrossBelow, models.ConditionCrossesBelow:
		if len(seriesB) > 0 {
			return CrossBelow(seriesA, seriesB)
		}
		return CrossBelowScalar(seriesA, cond.Threshold)

	case models.ConditionGreater:
		lastA := seriesA[len(seriesA)-1]
		if math.IsNaN(lastA) {
			return false
		}
		thresh := cond.Threshold
		if len(seriesB) > 0 {
			lastB := seriesB[len(seriesB)-1]
			if math.IsNaN(lastB) {
				return false
			}
			thresh = lastB
		}
		return lastA > thresh

	case models.ConditionLess:
		lastA := seriesA[len(seriesA)-1]
		if math.IsNaN(lastA) {
			return false
		}
		thresh := cond.Threshold
		if len(seriesB) > 0 {
			lastB := seriesB[len(seriesB)-1]
			if math.IsNaN(lastB) {
				return false
			}
			thresh = lastB
		}
		return lastA < thresh

	case models.ConditionSlopeAbove:
		if len(seriesA) <= cond.LookbackPeriods || cond.LookbackPeriods <= 0 {
			return false
		}
		slope := Slope(seriesA, cond.LookbackPeriods)
		if math.IsNaN(slope) {
			return false
		}
		return slope > cond.Threshold

	case models.ConditionSlopeBelow:
		if len(seriesA) <= cond.LookbackPeriods || cond.LookbackPeriods <= 0 {
			return false
		}
		slope := Slope(seriesA, cond.LookbackPeriods)
		if math.IsNaN(slope) {
			return false
		}
		return slope < cond.Threshold

	case models.ConditionWithin:
		lastA := seriesA[len(seriesA)-1]
		if math.IsNaN(lastA) {
			return false
		}
		return lastA >= 30.0 && lastA <= 70.0
	}

	return false
}

// EvaluateRules evaluates all enabled rules of a strategy against a BarFrameBundle and returns fired signals (Feature 34).
func EvaluateRules(strat *models.Strategy, bundle *BarFrameBundle, symbol string) []models.RuleSignal {
	return EvaluateRulesAt(strat, bundle, symbol, time.Now())
}

// EvaluateRulesAt evaluates all enabled rules of a strategy against a BarFrameBundle at the specified timestamp.
// Optional aliases allow ScopeSymbol rules (e.g., bare "NVDA") to match InstrumentID-keyed bundles (e.g., "NVDA.NASDAQ").
func EvaluateRulesAt(strat *models.Strategy, bundle *BarFrameBundle, symbol string, now time.Time, aliases ...string) []models.RuleSignal {
	if strat == nil || bundle == nil {
		return nil
	}

	var signals []models.RuleSignal

	for _, rule := range strat.Rules {
		if !rule.Enabled {
			continue
		}

		if rule.Scope == models.ScopeSymbol {
			matched := rule.Name == symbol
			if !matched {
				for _, alias := range aliases {
					if alias != "" && rule.Name == alias {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}

		active := EvaluateRule(&rule, bundle, symbol, now)
		if active {
			signals = append(signals, models.RuleSignal{
				RuleID:    rule.ID,
				RuleName:  rule.Name,
				Symbol:    symbol,
				Action:    rule.Action,
				Active:    true,
				Priority:  rule.Priority,
				Timestamp: now.Unix(),
				Message:   fmt.Sprintf("[%s] Rule '%s' fired %s on %s", now.Format("15:04:05"), rule.Name, rule.Action, symbol),
			})
		}
	}
	return signals
}
