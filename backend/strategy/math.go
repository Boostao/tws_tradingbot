package strategy

import "math"

// Simple math tools for technical indicators over arrays

func SMA(data []float64, period int) []float64 {
	out := make([]float64, len(data))
	for i := range data {
		if i < period-1 {
			out[i] = math.NaN()
			continue
		}
		sum := 0.0
		for j := 0; j < period; j++ {
			sum += data[i-j]
		}
		out[i] = sum / float64(period)
	}
	return out
}

func EMA(data []float64, period int) []float64 {
	out := make([]float64, len(data))
	alpha := 2.0 / (float64(period) + 1.0)
	
	// Init with SMA
	sum := 0.0
	for i := 0; i < len(data); i++ {
		if i < period-1 {
			out[i] = math.NaN()
			sum += data[i]
		} else if i == period-1 {
			sum += data[i]
			out[i] = sum / float64(period)
		} else {
			out[i] = (data[i] - out[i-1]) * alpha + out[i-1]
		}
	}
	return out
}
