package strategy_test

import (
	"math"
	"testing"

	"tws_traderbot/backend/strategy"
)

func TestHeikinAshi(t *testing.T) {
	// Feature 23: Heikin-Ashi
	// Empty check
	haO, haH, haL, haC := strategy.ComputeHeikinAshi(nil, nil, nil, nil)
	if haO != nil {
		t.Fatalf("expected nil for empty input")
	}

	// Single bar
	open := []float64{100.0}
	high := []float64{105.0}
	low := []float64{98.0}
	close := []float64{102.0}

	haO, haH, haL, haC = strategy.ComputeHeikinAshi(open, high, low, close)
	if len(haO) != 1 {
		t.Fatalf("expected 1 HA bar")
	}
	expectedOpen0 := (100.0 + 102.0) / 2.0
	expectedClose0 := (100.0 + 105.0 + 98.0 + 102.0) / 4.0
	if haO[0] != expectedOpen0 || haC[0] != expectedClose0 {
		t.Errorf("HA bar 0 mismatch: open=%v close=%v", haO[0], haC[0])
	}
	if haH[0] != 105.0 || haL[0] != 98.0 {
		t.Errorf("HA bar 0 high/low mismatch: high=%v low=%v", haH[0], haL[0])
	}

	// 3-bar sequence
	open = []float64{100, 102, 106}
	high = []float64{105, 108, 110}
	low = []float64{98, 101, 105}
	close = []float64{102, 107, 109}

	haO, haH, haL, haC = strategy.ComputeHeikinAshi(open, high, low, close)
	for i := range haO {
		if haH[i] < haO[i] || haH[i] < haC[i] {
			t.Errorf("bar %d: HA High %v must be >= Open %v and Close %v", i, haH[i], haO[i], haC[i])
		}
		if haL[i] > haO[i] || haL[i] > haC[i] {
			t.Errorf("bar %d: HA Low %v must be <= Open %v and Close %v", i, haL[i], haO[i], haC[i])
		}
	}
}

func TestRVOL(t *testing.T) {
	// Feature 24: RVOL
	volumes := []float64{100, 100, 100, 100, 250}
	rvol := strategy.ComputeRVOL(volumes, 4)

	// Indices 0..3 are warmup -> NaN
	for i := 0; i < 4; i++ {
		if !math.IsNaN(rvol[i]) {
			t.Errorf("expected NaN for warmup index %d, got %v", i, rvol[i])
		}
	}

	// Index 4: prior 4-bar SMA is (100+100+100+100)/4 = 100. Volume = 250 -> RVOL = 2.5
	if math.Abs(rvol[4]-2.5) > 1e-6 {
		t.Errorf("expected RVOL 2.5, got %v", rvol[4])
	}

	// Zero prior volume guard
	zeroVols := []float64{0, 0, 0, 0, 100}
	zeroRvol := strategy.ComputeRVOL(zeroVols, 4)
	if zeroRvol[4] != 1.0 && zeroRvol[4] != 0.0 {
		t.Errorf("expected safe fallback for 0 baseline volume, got %v", zeroRvol[4])
	}

	// Zero current volume
	zeroCurr := []float64{100, 100, 100, 100, 0}
	rvolCurr := strategy.ComputeRVOL(zeroCurr, 4)
	if rvolCurr[4] != 0.0 {
		t.Errorf("expected 0.0 for zero current volume, got %v", rvolCurr[4])
	}
}

func TestOBV(t *testing.T) {
	// Feature 25: OBV
	closes := []float64{10, 12, 11, 11, 15}
	vols := []float64{100, 200, 150, 50, 300}
	obv := strategy.ComputeOBV(closes, vols)

	if len(obv) != 5 {
		t.Fatalf("expected 5 bars")
	}
	if obv[0] != 100 { // first bar
		t.Errorf("expected obv[0]=100, got %v", obv[0])
	}
	if obv[1] != 300 { // 12 > 10 -> 100 + 200 = 300
		t.Errorf("expected obv[1]=300, got %v", obv[1])
	}
	if obv[2] != 150 { // 11 < 12 -> 300 - 150 = 150
		t.Errorf("expected obv[2]=150, got %v", obv[2])
	}
	if obv[3] != 150 { // 11 == 11 -> flat continuity = 150
		t.Errorf("expected obv[3]=150, got %v", obv[3])
	}
	if obv[4] != 450 { // 15 > 11 -> 150 + 300 = 450
		t.Errorf("expected obv[4]=450, got %v", obv[4])
	}
}

func TestMACD(t *testing.T) {
	// Feature 26: MACD (12, 26, 9)
	n := 40
	closes := make([]float64, n)
	for i := 0; i < n; i++ {
		closes[i] = 100.0 + float64(i)
	}

	macdLine, signalLine, hist := strategy.ComputeMACD(closes, 12, 26, 9)
	if len(macdLine) != n || len(signalLine) != n || len(hist) != n {
		t.Fatalf("lengths mismatch")
	}

	// Warmup indices (< 25) for macdLine should be NaN
	if !math.IsNaN(macdLine[24]) {
		t.Errorf("expected NaN for MACD line index 24")
	}
	// In an upward trend, fast EMA > slow EMA -> MACD line > 0
	if macdLine[39] <= 0 {
		t.Errorf("expected positive MACD line in uptrend, got %v", macdLine[39])
	}
	if !math.IsNaN(signalLine[39]) && !math.IsNaN(hist[39]) {
		if math.Abs((macdLine[39]-signalLine[39])-hist[39]) > 1e-6 {
			t.Errorf("expected hist = macd - signal, got %v vs %v - %v", hist[39], macdLine[39], signalLine[39])
		}
	}
}

func TestRSI(t *testing.T) {
	// Feature 27: RSI
	n := 30
	// Monotonically rising -> RSI should approach 100.0
	rising := make([]float64, n)
	for i := 0; i < n; i++ {
		rising[i] = float64(10 + i)
	}
	rsiRising := strategy.ComputeRSI(rising, 14)
	if math.Abs(rsiRising[29]-100.0) > 1e-6 {
		t.Errorf("expected RSI 100 for monotonic rise, got %v", rsiRising[29])
	}

	// Monotonically falling -> RSI should be 0.0
	falling := make([]float64, n)
	for i := 0; i < n; i++ {
		falling[i] = float64(50 - i)
	}
	rsiFalling := strategy.ComputeRSI(falling, 14)
	if math.Abs(rsiFalling[29]-0.0) > 1e-6 {
		t.Errorf("expected RSI 0 for monotonic drop, got %v", rsiFalling[29])
	}

	// Perfectly flat -> RSI should be 50.0
	flat := make([]float64, n)
	for i := 0; i < n; i++ {
		flat[i] = 100.0
	}
	rsiFlat := strategy.ComputeRSI(flat, 14)
	if math.Abs(rsiFlat[29]-50.0) > 1e-6 {
		t.Errorf("expected RSI 50.0 for flat price, got %v", rsiFlat[29])
	}
}

func TestBollingerBands(t *testing.T) {
	// Feature 28: Bollinger Bands
	n := 25
	closes := make([]float64, n)
	for i := 0; i < n; i++ {
		closes[i] = 100.0 + float64(i%5)
	}

	basis, upper, lower, bw, pctB := strategy.ComputeBollingerBands(closes, 20, 2.0)
	if len(basis) != n {
		t.Fatalf("length mismatch")
	}

	last := n - 1
	if math.IsNaN(pctB[last]) {
		t.Errorf("expected non-NaN %%B")
	}
	if upper[last] <= basis[last] || lower[last] >= basis[last] {
		t.Errorf("expected upper > basis > lower, got u=%v b=%v l=%v", upper[last], basis[last], lower[last])
	}
	if bw[last] <= 0 {
		t.Errorf("expected positive bandwidth, got %v", bw[last])
	}

	// Flat price: sigma = 0 -> upper = lower = basis, %B = 0.5, bandwidth = 0.0
	flatCloses := make([]float64, n)
	for i := 0; i < n; i++ {
		flatCloses[i] = 50.0
	}
	fBasis, fUpper, fLower, fBw, fPctB := strategy.ComputeBollingerBands(flatCloses, 20, 2.0)
	if fUpper[last] != 50.0 || fLower[last] != 50.0 || fBasis[last] != 50.0 {
		t.Errorf("expected flat bands at 50.0")
	}
	if fBw[last] != 0.0 {
		t.Errorf("expected 0.0 bandwidth for flat price, got %v", fBw[last])
	}
	if fPctB[last] != 0.5 {
		t.Errorf("expected 0.5 %%B for flat price, got %v", fPctB[last])
	}
}

func TestStochastic(t *testing.T) {
	// Feature 29: Stochastic Oscillator (15, 5, 5)
	n := 30
	high := make([]float64, n)
	low := make([]float64, n)
	close := make([]float64, n)

	for i := 0; i < n; i++ {
		high[i] = 110.0 + float64(i)
		low[i] = 90.0 + float64(i)
		close[i] = 100.0 + float64(i)
	}

	kLine, dLine := strategy.ComputeStochastic(high, low, close, 15, 5, 5)
	last := n - 1
	if math.IsNaN(kLine[last]) || math.IsNaN(dLine[last]) {
		t.Fatalf("expected valid stochastic values at bar %d", last)
	}
	if kLine[last] < 0 || kLine[last] > 100 {
		t.Errorf("kLine out of [0, 100] bounds: %v", kLine[last])
	}

	// Flat price: HH == LL -> midpoint 50.0
	for i := 0; i < n; i++ {
		high[i] = 100.0
		low[i] = 100.0
		close[i] = 100.0
	}
	kFlat, dFlat := strategy.ComputeStochastic(high, low, close, 15, 5, 5)
	if kFlat[last] != 50.0 || dFlat[last] != 50.0 {
		t.Errorf("expected 50.0 for flat price stochastic, got k=%v d=%v", kFlat[last], dFlat[last])
	}

	// Length guard tests
	kEmpty, dEmpty := strategy.ComputeStochastic(nil, nil, nil, 15, 5, 5)
	if kEmpty != nil || dEmpty != nil {
		t.Errorf("expected nil for empty stochastic inputs")
	}

	kMismatch, dMismatch := strategy.ComputeStochastic([]float64{10, 20}, []float64{5}, []float64{8, 15}, 15, 5, 5)
	if kMismatch != nil || dMismatch != nil {
		t.Errorf("expected nil for mismatched stochastic input lengths")
	}
}

func TestWilliamsAlligator(t *testing.T) {
	// Feature 30: Williams Alligator
	n := 35
	high := make([]float64, n)
	low := make([]float64, n)
	for i := 0; i < n; i++ {
		high[i] = 105.0 + float64(i)
		low[i] = 95.0 + float64(i)
	}

	jaw, teeth, lips := strategy.ComputeWilliamsAlligator(high, low)
	if len(jaw) != n || len(teeth) != n || len(lips) != n {
		t.Fatalf("length mismatch")
	}

	// Jaw has 8-bar shift, teeth 5-bar shift, lips 3-bar shift
	if !math.IsNaN(jaw[7]) {
		t.Errorf("expected jaw to be NaN for index < 8")
	}
	if !math.IsNaN(teeth[4]) {
		t.Errorf("expected teeth to be NaN for index < 5")
	}
	if !math.IsNaN(lips[2]) {
		t.Errorf("expected lips to be NaN for index < 3")
	}
	// At index 34, all lines should be non-NaN
	if math.IsNaN(jaw[34]) || math.IsNaN(teeth[34]) || math.IsNaN(lips[34]) {
		t.Errorf("expected valid alligator lines at bar 34")
	}
}

func TestGuppyMMA(t *testing.T) {
	// Feature 31: Guppy MMA
	n := 220
	closes := make([]float64, n)
	for i := 0; i < n; i++ {
		closes[i] = 100.0 + float64(i)*0.5
	}

	fMin, fMax, sMin, sMax, anchor := strategy.ComputeGuppyMMA(closes)
	last := n - 1

	if fMin[last] > fMax[last] {
		t.Errorf("fastMin %v should be <= fastMax %v", fMin[last], fMax[last])
	}
	if sMin[last] > sMax[last] {
		t.Errorf("slowMin %v should be <= slowMax %v", sMin[last], sMax[last])
	}
	// In uptrend, fast cluster > slow cluster > anchor EMA 200
	if fMin[last] < sMax[last] {
		t.Errorf("expected fast cluster above slow cluster in steady uptrend")
	}
	if math.IsNaN(anchor[last]) || anchor[last] <= 0 {
		t.Errorf("expected valid anchor EMA 200 at bar %d", last)
	}
}

func TestATR(t *testing.T) {
	// Feature 32: ATR & Volatility %
	n := 20
	high := make([]float64, n)
	low := make([]float64, n)
	close := make([]float64, n)

	for i := 0; i < n; i++ {
		high[i] = 105.0
		low[i] = 95.0
		close[i] = 100.0
	}

	tr, atr, atrPct := strategy.ComputeATR(high, low, close, 14)
	if tr[0] != 10.0 {
		t.Errorf("expected TR[0] = 10, got %v", tr[0])
	}

	last := n - 1
	if math.IsNaN(atr[last]) || math.Abs(atr[last]-10.0) > 1e-4 {
		t.Errorf("expected ATR ~10.0, got %v", atr[last])
	}
	// Close = 100, ATR = 10 -> ATR % = 10.0%
	if math.Abs(atrPct[last]-10.0) > 1e-4 {
		t.Errorf("expected ATR %% ~10.0, got %v", atrPct[last])
	}
}

func TestRelativeStrength(t *testing.T) {
	// Feature 33: Relative Strength vs SPX Benchmark
	n := 110
	ticker := make([]float64, n)
	spx := make([]float64, n)

	for i := 0; i < n; i++ {
		ticker[i] = 100.0 * math.Pow(1.02, float64(i)) // +2% per bar
		spx[i] = 100.0 * math.Pow(1.01, float64(i))    // +1% per bar
	}

	rs := strategy.ComputeRelativeStrength(ticker, spx, 100)
	last := n - 1
	// Ticker outperformed SPX -> RS > 100.0
	if rs[last] <= 100.0 {
		t.Errorf("expected outperformance RS > 100.0, got %v", rs[last])
	}

	// Self-benchmarking returns 100.0
	rsSelf := strategy.ComputeRelativeStrength(ticker, ticker, 100)
	if math.Abs(rsSelf[last]-100.0) > 1e-6 {
		t.Errorf("expected RS 100.0 for self benchmark, got %v", rsSelf[last])
	}

	// Boundary: n == lookback (e.g. exactly 100 bars) returns neutral 100.0 baseline, NOT NaN
	rs100 := strategy.ComputeRelativeStrength(ticker[:100], spx[:100], 100)
	if len(rs100) != 100 || rs100[99] != 100.0 || math.IsNaN(rs100[99]) {
		t.Errorf("expected RS 100.0 neutral baseline at n == lookback, got %v", rs100[99])
	}
}

func TestHigh52W(t *testing.T) {
	// Feature 35: Screener 2.0 52-Week High
	n := 300
	highs := make([]float64, n)
	for i := 0; i < n; i++ {
		highs[i] = 100.0 + float64(i%250)
	}

	h52 := strategy.ComputeHigh52W(highs, 252)
	last := n - 1
	if math.IsNaN(h52[last]) || h52[last] < 340.0 {
		t.Errorf("expected valid 52W high, got %v", h52[last])
	}
}
