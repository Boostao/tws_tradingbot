package strategy

import (
	"math"
)

// -----------------------------------------------------------------------------
// 1. Heikin-Ashi (Feature 23)
// -----------------------------------------------------------------------------

// ComputeHeikinAshi computes smoothed HA candlestick series from price slices.
func ComputeHeikinAshi(open, high, low, close []float64) (haOpen, haHigh, haLow, haClose []float64) {
	n := len(close)
	if n == 0 || len(open) != n || len(high) != n || len(low) != n {
		return nil, nil, nil, nil
	}
	haOpen = make([]float64, n)
	haHigh = make([]float64, n)
	haLow = make([]float64, n)
	haClose = make([]float64, n)

	// First bar
	haClose[0] = (open[0] + high[0] + low[0] + close[0]) / 4.0
	haOpen[0] = (open[0] + close[0]) / 2.0
	haHigh[0] = high[0]
	haLow[0] = low[0]

	// Subsequent bars
	for i := 1; i < n; i++ {
		haClose[i] = (open[i] + high[i] + low[i] + close[i]) / 4.0
		haOpen[i] = (haOpen[i-1] + haClose[i-1]) / 2.0
		haHigh[i] = math.Max(high[i], math.Max(haOpen[i], haClose[i]))
		haLow[i] = math.Min(low[i], math.Min(haOpen[i], haClose[i]))
	}
	return haOpen, haHigh, haLow, haClose
}

// CalculateHeikinAshi computes smoothed HA candlestick series from OHLCVBar slice.
func CalculateHeikinAshi(bars []OHLCVBar) []OHLCVBar {
	n := len(bars)
	if n == 0 {
		return []OHLCVBar{}
	}
	ha := make([]OHLCVBar, n)

	ha[0] = OHLCVBar{
		Timestamp: bars[0].Timestamp,
		Open:      (bars[0].Open + bars[0].Close) / 2.0,
		Close:     (bars[0].Open + bars[0].High + bars[0].Low + bars[0].Close) / 4.0,
		High:      bars[0].High,
		Low:       bars[0].Low,
		Volume:    bars[0].Volume,
	}

	for i := 1; i < n; i++ {
		b := bars[i]
		haClose := (b.Open + b.High + b.Low + b.Close) / 4.0
		haOpen := (ha[i-1].Open + ha[i-1].Close) / 2.0
		haHigh := math.Max(b.High, math.Max(haOpen, haClose))
		haLow := math.Min(b.Low, math.Min(haOpen, haClose))

		ha[i] = OHLCVBar{
			Timestamp: b.Timestamp,
			Open:      haOpen,
			High:      haHigh,
			Low:       haLow,
			Close:     haClose,
			Volume:    b.Volume,
		}
	}
	return ha
}

// -----------------------------------------------------------------------------
// 2. RVOL — Relative Volume (Feature 24)
// -----------------------------------------------------------------------------

// ComputeRVOL calculates Relative Volume compared to the baseline SMA of prior N periods.
func ComputeRVOL(volume []float64, period int) []float64 {
	n := len(volume)
	out := make([]float64, n)
	if period <= 0 {
		period = 100 // Cobalt baseline
	}
	smaVol := SMA(volume, period)

	for i := range volume {
		if i < period {
			out[i] = math.NaN()
		} else {
			priorSMA := smaVol[i-1]
			if math.IsNaN(priorSMA) || priorSMA <= 0 {
				out[i] = 1.0 // Safe fallback
			} else {
				vol := volume[i]
				if vol <= 0 {
					out[i] = 0.0
				} else {
					out[i] = vol / priorSMA
				}
			}
		}
	}
	return out
}

// CalculateRVOL is an alias to ComputeRVOL.
func CalculateRVOL(volumes []float64, period int) []float64 {
	return ComputeRVOL(volumes, period)
}

// -----------------------------------------------------------------------------
// 3. OBV — On-Balance Volume (Feature 25)
// -----------------------------------------------------------------------------

// ComputeOBV calculates cumulative On-Balance Volume.
func ComputeOBV(close, volume []float64) []float64 {
	n := len(close)
	if n == 0 || len(volume) != n {
		return make([]float64, n)
	}
	obv := make([]float64, n)
	obv[0] = volume[0]

	for i := 1; i < n; i++ {
		if close[i] > close[i-1] {
			obv[i] = obv[i-1] + volume[i]
		} else if close[i] < close[i-1] {
			obv[i] = obv[i-1] - volume[i]
		} else {
			obv[i] = obv[i-1]
		}
	}
	return obv
}

// CalculateOBV is an alias to ComputeOBV.
func CalculateOBV(closes []float64, volumes []float64) []float64 {
	return ComputeOBV(closes, volumes)
}

// -----------------------------------------------------------------------------
// 4. MACD (Feature 26)
// -----------------------------------------------------------------------------

// ComputeMACD calculates MACD Line, Signal Line, and Histogram (default 12, 26, 9).
func ComputeMACD(close []float64, fast, slow, signal int) (macdLine, signalLine, histogram []float64) {
	n := len(close)
	if fast <= 0 {
		fast = 12
	}
	if slow <= 0 {
		slow = 26
	}
	if signal <= 0 {
		signal = 9
	}
	if n == 0 {
		return nil, nil, nil
	}

	fastEMA := EMA(close, fast)
	slowEMA := EMA(close, slow)

	macdLine = make([]float64, n)
	for i := 0; i < n; i++ {
		if math.IsNaN(fastEMA[i]) || math.IsNaN(slowEMA[i]) {
			macdLine[i] = math.NaN()
		} else {
			macdLine[i] = fastEMA[i] - slowEMA[i]
		}
	}

	signalLine = EMA(macdLine, signal)
	histogram = make([]float64, n)
	for i := 0; i < n; i++ {
		if math.IsNaN(macdLine[i]) || math.IsNaN(signalLine[i]) {
			histogram[i] = math.NaN()
		} else {
			histogram[i] = macdLine[i] - signalLine[i]
		}
	}
	return macdLine, signalLine, histogram
}

// CalculateMACD is an alias to ComputeMACD.
func CalculateMACD(closes []float64, fast, slow, signal int) (macdLine, signalLine, histogram []float64) {
	return ComputeMACD(closes, fast, slow, signal)
}

// -----------------------------------------------------------------------------
// 5. RSI — Wilder's Relative Strength Index (Feature 27)
// -----------------------------------------------------------------------------

// ComputeRSI calculates Wilder's 14-period RSI in range [0, 100].
func ComputeRSI(close []float64, period int) []float64 {
	n := len(close)
	if period <= 0 {
		period = 14
	}
	out := make([]float64, n)
	if n <= period {
		for i := range out {
			out[i] = math.NaN()
		}
		return out
	}

	gains := make([]float64, n)
	losses := make([]float64, n)
	for i := 1; i < n; i++ {
		diff := close[i] - close[i-1]
		if diff > 0 {
			gains[i] = diff
		} else {
			losses[i] = -diff
		}
	}

	avgGain := 0.0
	avgLoss := 0.0
	for i := 1; i <= period; i++ {
		avgGain += gains[i]
		avgLoss += losses[i]
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	for i := 0; i < period; i++ {
		out[i] = math.NaN()
	}

	if avgLoss == 0 {
		if avgGain == 0 {
			out[period] = 50.0
		} else {
			out[period] = 100.0
		}
	} else {
		rs := avgGain / avgLoss
		out[period] = 100.0 - (100.0 / (1.0 + rs))
	}

	for i := period + 1; i < n; i++ {
		avgGain = (avgGain*float64(period-1) + gains[i]) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + losses[i]) / float64(period)

		if avgLoss == 0 {
			if avgGain == 0 {
				out[i] = 50.0
			} else {
				out[i] = 100.0
			}
		} else {
			rs := avgGain / avgLoss
			rsi := 100.0 - (100.0 / (1.0 + rs))
			if rsi < 0.0 {
				rsi = 0.0
			} else if rsi > 100.0 {
				rsi = 100.0
			}
			out[i] = rsi
		}
	}
	return out
}

// CalculateRSI is an alias to ComputeRSI.
func CalculateRSI(closes []float64, period int) []float64 {
	return ComputeRSI(closes, period)
}

// -----------------------------------------------------------------------------
// 6. Bollinger Bands (Feature 28)
// -----------------------------------------------------------------------------

// ComputeBollingerBands calculates Basis (middle SMA), Upper, Lower, Bandwidth, and %B (20, 2σ).
func ComputeBollingerBands(close []float64, period int, numStdDev float64) (basis, upper, lower, bandwidth, percentB []float64) {
	n := len(close)
	if period <= 0 {
		period = 20
	}
	if numStdDev <= 0 {
		numStdDev = 2.0
	}

	basis = SMA(close, period)
	stdDev := StdDev(close, period, basis)

	upper = make([]float64, n)
	lower = make([]float64, n)
	bandwidth = make([]float64, n)
	percentB = make([]float64, n)

	for i := 0; i < n; i++ {
		if math.IsNaN(basis[i]) || math.IsNaN(stdDev[i]) {
			upper[i] = math.NaN()
			lower[i] = math.NaN()
			bandwidth[i] = math.NaN()
			percentB[i] = math.NaN()
			continue
		}

		upper[i] = basis[i] + numStdDev*stdDev[i]
		lower[i] = basis[i] - numStdDev*stdDev[i]

		if basis[i] != 0 {
			bandwidth[i] = (upper[i] - lower[i]) / basis[i]
		} else {
			bandwidth[i] = 0.0
		}

		spread := upper[i] - lower[i]
		if spread != 0 {
			percentB[i] = (close[i] - lower[i]) / spread
		} else {
			percentB[i] = 0.5
		}
	}
	return basis, upper, lower, bandwidth, percentB
}

// CalculateBollingerBands calculates Middle, Upper, and Lower Bollinger Bands.
func CalculateBollingerBands(closes []float64, period int, mult float64) (middle, upper, lower []float64) {
	b, u, l, _, _ := ComputeBollingerBands(closes, period, mult)
	return b, u, l
}

// -----------------------------------------------------------------------------
// 7. Stochastic Oscillator (Feature 29)
// -----------------------------------------------------------------------------

// ComputeStochastic calculates %K and %D lines (default 15, 5, 5).
func ComputeStochastic(high, low, close []float64, kLength, kSmooth, dSmooth int) (kLine, dLine []float64) {
	n := len(close)
	if n == 0 || len(high) != n || len(low) != n {
		return nil, nil
	}
	if kLength <= 0 {
		kLength = 15
	}
	if kSmooth <= 0 {
		kSmooth = 5
	}
	if dSmooth <= 0 {
		dSmooth = 5
	}

	rawK := make([]float64, n)
	for i := 0; i < n; i++ {
		if i < kLength-1 {
			rawK[i] = math.NaN()
			continue
		}
		highest := high[i]
		lowest := low[i]
		for j := 0; j < kLength; j++ {
			if high[i-j] > highest {
				highest = high[i-j]
			}
			if low[i-j] < lowest {
				lowest = low[i-j]
			}
		}
		diff := highest - lowest
		if diff == 0 {
			rawK[i] = 50.0 // Midpoint fallback on flat range
		} else {
			k := ((close[i] - lowest) / diff) * 100.0
			if k < 0 {
				k = 0
			} else if k > 100 {
				k = 100
			}
			rawK[i] = k
		}
	}

	kLine = SMA(rawK, kSmooth)
	dLine = SMA(kLine, dSmooth)
	return kLine, dLine
}

// CalculateStochastic is an alias to ComputeStochastic.
func CalculateStochastic(highs, lows, closes []float64, kPeriod, dPeriod int) (kLine, dLine []float64) {
	return ComputeStochastic(highs, lows, closes, kPeriod, dPeriod, dPeriod)
}

// -----------------------------------------------------------------------------
// 8. Williams Alligator (Feature 30)
// -----------------------------------------------------------------------------

// ComputeWilliamsAlligator calculates Jaw (13/8), Teeth (8/5), and Lips (5/3) SMMA lines on median price.
func ComputeWilliamsAlligator(high, low []float64) (jaw, teeth, lips []float64) {
	n := len(high)
	if n == 0 || len(low) != n {
		return nil, nil, nil
	}

	medianPrice := make([]float64, n)
	for i := 0; i < n; i++ {
		medianPrice[i] = (high[i] + low[i]) / 2.0
	}

	jawSMMA := WilderRMA(medianPrice, 13)
	teethSMMA := WilderRMA(medianPrice, 8)
	lipsSMMA := WilderRMA(medianPrice, 5)

	jaw = make([]float64, n)
	teeth = make([]float64, n)
	lips = make([]float64, n)

	// Offset backward to simulate the forward-shifted lines on the current bar
	for i := 0; i < n; i++ {
		if i >= 8 {
			jaw[i] = jawSMMA[i-8]
		} else {
			jaw[i] = math.NaN()
		}
		if i >= 5 {
			teeth[i] = teethSMMA[i-5]
		} else {
			teeth[i] = math.NaN()
		}
		if i >= 3 {
			lips[i] = lipsSMMA[i-3]
		} else {
			lips[i] = math.NaN()
		}
	}
	return jaw, teeth, lips
}

// CalculateAlligator is an alias to ComputeWilliamsAlligator.
func CalculateAlligator(highs, lows []float64) (jaw, teeth, lips []float64) {
	return ComputeWilliamsAlligator(highs, lows)
}

// -----------------------------------------------------------------------------
// 9. Guppy Multiple Moving Average — Guppy MMA (Feature 31)
// -----------------------------------------------------------------------------

// ComputeGuppyMMA calculates Fast & Slow EMA clusters and Anchor EMA 200 on weekly/daily frame.
func ComputeGuppyMMA(close []float64) (fastMin, fastMax, slowMin, slowMax, anchorEMA []float64) {
	n := len(close)
	f1 := EMA(close, 3)
	f2 := EMA(close, 5)
	f3 := EMA(close, 8)
	f4 := EMA(close, 10)
	f5 := EMA(close, 12)
	f6 := EMA(close, 15)

	s1 := EMA(close, 30)
	s2 := EMA(close, 35)
	s3 := EMA(close, 40)
	s4 := EMA(close, 45)
	s5 := EMA(close, 50)
	s6 := EMA(close, 60)

	anchorEMA = EMA(close, 200)

	fastMin = make([]float64, n)
	fastMax = make([]float64, n)
	slowMin = make([]float64, n)
	slowMax = make([]float64, n)

	for i := 0; i < n; i++ {
		if math.IsNaN(f6[i]) {
			fastMin[i] = math.NaN()
			fastMax[i] = math.NaN()
		} else {
			minF := math.Min(f1[i], math.Min(f2[i], math.Min(f3[i], math.Min(f4[i], math.Min(f5[i], f6[i])))))
			maxF := math.Max(f1[i], math.Max(f2[i], math.Max(f3[i], math.Max(f4[i], math.Max(f5[i], f6[i])))))
			fastMin[i] = minF
			fastMax[i] = maxF
		}

		if math.IsNaN(s6[i]) {
			slowMin[i] = math.NaN()
			slowMax[i] = math.NaN()
		} else {
			minS := math.Min(s1[i], math.Min(s2[i], math.Min(s3[i], math.Min(s4[i], math.Min(s5[i], s6[i])))))
			maxS := math.Max(s1[i], math.Max(s2[i], math.Max(s3[i], math.Max(s4[i], math.Max(s5[i], s6[i])))))
			slowMin[i] = minS
			slowMax[i] = maxS
		}
	}
	return fastMin, fastMax, slowMin, slowMax, anchorEMA
}

// CalculateGuppy computes 6 fast and 6 slow EMAs plus Anchor EMA 200.
func CalculateGuppy(closes []float64) (fast, slow [][]float64, anchor []float64) {
	fastPeriods := []int{3, 5, 8, 10, 12, 15}
	slowPeriods := []int{30, 35, 40, 45, 50, 60}

	fast = make([][]float64, len(fastPeriods))
	for i, p := range fastPeriods {
		fast[i] = EMA(closes, p)
	}

	slow = make([][]float64, len(slowPeriods))
	for i, p := range slowPeriods {
		slow[i] = EMA(closes, p)
	}

	anchor = EMA(closes, 200)
	return fast, slow, anchor
}

// -----------------------------------------------------------------------------
// 10. ATR & Volatility % (Feature 32)
// -----------------------------------------------------------------------------

// ComputeATR calculates True Range, ATR (14-period Wilder RMA), and ATR % of price.
func ComputeATR(high, low, close []float64, period int) (tr, atr, atrPct []float64) {
	n := len(close)
	if period <= 0 {
		period = 14
	}
	if n == 0 || len(high) != n || len(low) != n {
		return nil, nil, nil
	}

	tr = make([]float64, n)
	tr[0] = high[0] - low[0]

	for i := 1; i < n; i++ {
		hMinusL := high[i] - low[i]
		hMinusPrevC := math.Abs(high[i] - close[i-1])
		lMinusPrevC := math.Abs(low[i] - close[i-1])
		tr[i] = math.Max(hMinusL, math.Max(hMinusPrevC, lMinusPrevC))
	}

	atr = WilderRMA(tr, period)
	atrPct = make([]float64, n)

	for i := 0; i < n; i++ {
		if math.IsNaN(atr[i]) || close[i] <= 0 {
			atrPct[i] = math.NaN()
		} else {
			atrPct[i] = (atr[i] / close[i]) * 100.0
		}
	}
	return tr, atr, atrPct
}

// CalculateATR computes ATR and ATR % from OHLCVBar slice.
func CalculateATR(bars []OHLCVBar, period int) (atr []float64, atrPct []float64) {
	n := len(bars)
	if n == 0 {
		return nil, nil
	}
	high := make([]float64, n)
	low := make([]float64, n)
	close := make([]float64, n)
	for i, b := range bars {
		high[i] = b.High
		low[i] = b.Low
		close[i] = b.Close
	}
	_, a, ap := ComputeATR(high, low, close, period)
	return a, ap
}

// -----------------------------------------------------------------------------
// 11. Relative Strength vs SPX Benchmark (Feature 33)
// -----------------------------------------------------------------------------

// ComputeRelativeStrength calculates 100-bar outperformance ratio vs SPX benchmark.
// Returns neutral 100.0 baseline when benchmark is missing or sparse.
func ComputeRelativeStrength(tickerClose, benchClose []float64, lookback int) []float64 {
	n := len(tickerClose)
	out := make([]float64, n)
	if lookback <= 0 {
		lookback = 100
	}
	if n <= lookback || len(benchClose) != n {
		for i := range out {
			out[i] = 100.0 // Neutral benchmark ratio
		}
		return out
	}

	for i := 0; i < n; i++ {
		if i < lookback {
			out[i] = math.NaN()
			continue
		}
		tBase := tickerClose[i-lookback]
		bBase := benchClose[i-lookback]
		if tBase <= 0 || bBase <= 0 || benchClose[i] <= 0 {
			out[i] = 100.0
			continue
		}
		tPerf := tickerClose[i] / tBase
		bPerf := benchClose[i] / bBase
		if bPerf <= 0 {
			out[i] = 100.0
		} else {
			out[i] = (tPerf / bPerf) * 100.0
		}
	}
	return out
}

// CalculateRelativeStrength is an alias to ComputeRelativeStrength.
func CalculateRelativeStrength(stockCloses, spxCloses []float64, lookback int) []float64 {
	return ComputeRelativeStrength(stockCloses, spxCloses, lookback)
}

// -----------------------------------------------------------------------------
// 12. Screener 2.0 — 52-Week High (Feature 35)
// -----------------------------------------------------------------------------

// ComputeHigh52W calculates 252-day rolling highest high for Screener 2.0.
func ComputeHigh52W(high []float64, lookback int) []float64 {
	n := len(high)
	if lookback <= 0 {
		lookback = 252 // Standard trading days in 52 weeks
	}
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		if i < lookback-1 {
			out[i] = math.NaN()
			continue
		}
		highest := high[i]
		for j := 0; j < lookback; j++ {
			if high[i-j] > highest {
				highest = high[i-j]
			}
		}
		out[i] = highest
	}
	return out
}
