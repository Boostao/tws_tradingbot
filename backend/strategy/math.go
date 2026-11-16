package strategy

import (
	"math"
	"strings"
)

// SMA calculates Simple Moving Average with NaN leading padding and robust handling of leading NaNs.
func SMA(data []float64, period int) []float64 {
	n := len(data)
	out := make([]float64, n)
	if n < period || period <= 0 {
		for i := range out {
			out[i] = math.NaN()
		}
		return out
	}

	for i := 0; i < n; i++ {
		if i < period-1 {
			out[i] = math.NaN()
			continue
		}
		sum := 0.0
		hasNaN := false
		for j := 0; j < period; j++ {
			val := data[i-j]
			if math.IsNaN(val) {
				hasNaN = true
				break
			}
			sum += val
		}
		if hasNaN {
			out[i] = math.NaN()
		} else {
			out[i] = sum / float64(period)
		}
	}
	return out
}

// EMA calculates Exponential Moving Average with SMA seed and NaN padding, robust to leading NaNs.
func EMA(data []float64, period int) []float64 {
	n := len(data)
	out := make([]float64, n)
	for i := range out {
		out[i] = math.NaN()
	}
	if n < period || period <= 0 {
		return out
	}

	start := -1
	for i, v := range data {
		if !math.IsNaN(v) {
			start = i
			break
		}
	}
	if start == -1 || n-start < period {
		return out
	}

	alpha := 2.0 / (float64(period) + 1.0)
	sum := 0.0
	for i := start; i < start+period; i++ {
		if math.IsNaN(data[i]) {
			return out
		}
		sum += data[i]
	}
	seedIdx := start + period - 1
	out[seedIdx] = sum / float64(period)

	for i := seedIdx + 1; i < n; i++ {
		if math.IsNaN(data[i]) || math.IsNaN(out[i-1]) {
			out[i] = math.NaN()
		} else {
			out[i] = (data[i]-out[i-1])*alpha + out[i-1]
		}
	}
	return out
}

// WilderRMA calculates Wilder's Smoothed Moving Average (SMMA with alpha = 1/period).
// Used by RSI, ATR, and Williams Alligator, robust to leading NaNs.
func WilderRMA(data []float64, period int) []float64 {
	n := len(data)
	out := make([]float64, n)
	for i := range out {
		out[i] = math.NaN()
	}
	if n < period || period <= 0 {
		return out
	}

	start := -1
	for i, v := range data {
		if !math.IsNaN(v) {
			start = i
			break
		}
	}
	if start == -1 || n-start < period {
		return out
	}

	sum := 0.0
	for i := start; i < start+period; i++ {
		if math.IsNaN(data[i]) {
			return out
		}
		sum += data[i]
	}
	seedIdx := start + period - 1
	out[seedIdx] = sum / float64(period)

	for i := seedIdx + 1; i < n; i++ {
		if math.IsNaN(data[i]) || math.IsNaN(out[i-1]) {
			out[i] = math.NaN()
		} else {
			out[i] = (out[i-1]*float64(period-1) + data[i]) / float64(period)
		}
	}
	return out
}

// StdDev calculates rolling sample standard deviation over period.
func StdDev(data []float64, period int, sma []float64) []float64 {
	n := len(data)
	out := make([]float64, n)
	if n < period || period <= 0 {
		for i := range out {
			out[i] = math.NaN()
		}
		return out
	}

	if len(sma) != n {
		sma = SMA(data, period)
	}

	for i := 0; i < n; i++ {
		if i < period-1 || math.IsNaN(sma[i]) {
			out[i] = math.NaN()
			continue
		}
		sumSq := 0.0
		mean := sma[i]
		valid := true
		for j := 0; j < period; j++ {
			val := data[i-j]
			if math.IsNaN(val) {
				valid = false
				break
			}
			diff := val - mean
			sumSq += diff * diff
		}
		if !valid {
			out[i] = math.NaN()
		} else {
			out[i] = math.Sqrt(sumSq / float64(period))
		}
	}
	return out
}

// Slope calculates linear slope over lookback bars: (last - prev) / lookback.
func Slope(series []float64, lookback int) float64 {
	n := len(series)
	if n <= lookback || lookback <= 0 {
		return math.NaN()
	}
	last := series[n-1]
	prev := series[n-1-lookback]
	if math.IsNaN(last) || math.IsNaN(prev) {
		return math.NaN()
	}
	return (last - prev) / float64(lookback)
}

// CrossAbove checks if seriesA crossed above seriesB on the most recent bar.
func CrossAbove(seriesA, seriesB []float64) bool {
	nA, nB := len(seriesA), len(seriesB)
	if nA < 2 || nB < 2 {
		return false
	}
	lastA, prevA := seriesA[nA-1], seriesA[nA-2]
	lastB, prevB := seriesB[nB-1], seriesB[nB-2]

	if math.IsNaN(lastA) || math.IsNaN(prevA) || math.IsNaN(lastB) || math.IsNaN(prevB) {
		return false
	}
	return prevA <= prevB && lastA > lastB
}

// CrossAboveScalar checks if series crossed above a constant scalar threshold.
func CrossAboveScalar(series []float64, threshold float64) bool {
	n := len(series)
	if n < 2 || math.IsNaN(threshold) {
		return false
	}
	last, prev := series[n-1], series[n-2]
	if math.IsNaN(last) || math.IsNaN(prev) {
		return false
	}
	return prev <= threshold && last > threshold
}

// CrossBelow checks if seriesA crossed below seriesB on the most recent bar.
func CrossBelow(seriesA, seriesB []float64) bool {
	nA, nB := len(seriesA), len(seriesB)
	if nA < 2 || nB < 2 {
		return false
	}
	lastA, prevA := seriesA[nA-1], seriesA[nA-2]
	lastB, prevB := seriesB[nB-1], seriesB[nB-2]

	if math.IsNaN(lastA) || math.IsNaN(prevA) || math.IsNaN(lastB) || math.IsNaN(prevB) {
		return false
	}
	return prevA >= prevB && lastA < lastB
}

// CrossBelowScalar checks if series crossed below a constant scalar threshold.
func CrossBelowScalar(series []float64, threshold float64) bool {
	n := len(series)
	if n < 2 || math.IsNaN(threshold) {
		return false
	}
	last, prev := series[n-1], series[n-2]
	if math.IsNaN(last) || math.IsNaN(prev) {
		return false
	}
	return prev >= threshold && last < threshold
}

// WithinRange checks whether timeStr ("HH:MM") falls within [rangeStart, rangeEnd].
// Handles overnight wrap-around (e.g. 22:00 to 06:00).
func WithinRange(timeStr, rangeStart, rangeEnd string) bool {
	if rangeStart == "" || rangeEnd == "" {
		return true
	}
	cur := strings.TrimSpace(timeStr)
	start := strings.TrimSpace(rangeStart)
	end := strings.TrimSpace(rangeEnd)

	if start <= end {
		return cur >= start && cur <= end
	}
	// Overnight wrap (e.g. 22:00 to 06:00)
	return cur >= start || cur <= end
}
