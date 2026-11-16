package strategy

import (
	"strings"
	"sync"
	"time"
	"tws_traderbot/backend/models"
)

// OHLCVBar represents a single completed or forming candlestick bar.
type OHLCVBar struct {
	Timestamp int64   `json:"timestamp"` // Unix epoch seconds
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
}

// Bar is an alias to OHLCVBar for convenience.
type Bar = OHLCVBar

// BarSeriesSnapshot represents an immutable point-in-time snapshot of all OHLCV series.
type BarSeriesSnapshot struct {
	Time   []int64
	Open   []float64
	High   []float64
	Low    []float64
	Close  []float64
	Volume []float64
}

// BarFrame manages a single timeframe's contiguous OHLCV arrays, bars, and memoized indicator cache.
type BarFrame struct {
	mu         sync.RWMutex
	version    uint64
	Symbol     string
	Timeframe  models.TimeframeUnit
	Bars       []OHLCVBar
	Time       []int64
	Open       []float64
	High       []float64
	Low        []float64
	Close      []float64
	Volume     []float64
	indicators map[string][]float64 // memoized indicator cache
}

// NewBarFrame allocates an empty BarFrame with initialized cache.
// Optionally accepts a symbol string as the second argument.
func NewBarFrame(tf models.TimeframeUnit, symbol ...string) *BarFrame {
	sym := ""
	if len(symbol) > 0 {
		sym = symbol[0]
	}
	return &BarFrame{
		Symbol:     sym,
		Timeframe:  tf,
		Bars:       make([]OHLCVBar, 0),
		Time:       make([]int64, 0),
		Open:       make([]float64, 0),
		High:       make([]float64, 0),
		Low:        make([]float64, 0),
		Close:      make([]float64, 0),
		Volume:     make([]float64, 0),
		indicators: make(map[string][]float64),
	}
}

// Len returns the bar count under read-lock.
func (bf *BarFrame) Len() int {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	return len(bf.Bars)
}

// BarCount returns the bar count under read-lock (alias to Len).
func (bf *BarFrame) BarCount() int {
	return bf.Len()
}

// AppendBar appends a bar and atomically invalidates the indicator cache.
func (bf *BarFrame) AppendBar(bar OHLCVBar) {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	bf.version++
	bf.Bars = append(bf.Bars, bar)
	bf.Time = append(bf.Time, bar.Timestamp)
	bf.Open = append(bf.Open, bar.Open)
	bf.High = append(bf.High, bar.High)
	bf.Low = append(bf.Low, bar.Low)
	bf.Close = append(bf.Close, bar.Close)
	bf.Volume = append(bf.Volume, bar.Volume)

	// Invalidate memoized cache on new bar arrival
	bf.indicators = make(map[string][]float64)
}

// AddBar appends a bar with individual field values and invalidates the cache.
func (bf *BarFrame) AddBar(t int64, o, h, l, c, v float64) {
	bf.AppendBar(OHLCVBar{
		Timestamp: t,
		Open:      o,
		High:      h,
		Low:       l,
		Close:     c,
		Volume:    v,
	})
}

// UpdateLastBar updates the in-flight real-time bar in place and clears the cache.
func (bf *BarFrame) UpdateLastBar(bar OHLCVBar) {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	bf.version++
	n := len(bf.Bars)
	if n == 0 {
		bf.Bars = append(bf.Bars, bar)
		bf.Time = append(bf.Time, bar.Timestamp)
		bf.Open = append(bf.Open, bar.Open)
		bf.High = append(bf.High, bar.High)
		bf.Low = append(bf.Low, bar.Low)
		bf.Close = append(bf.Close, bar.Close)
		bf.Volume = append(bf.Volume, bar.Volume)
	} else {
		bf.Bars[n-1] = bar
		bf.Time[n-1] = bar.Timestamp
		bf.Open[n-1] = bar.Open
		bf.High[n-1] = bar.High
		bf.Low[n-1] = bar.Low
		bf.Close[n-1] = bar.Close
		bf.Volume[n-1] = bar.Volume
	}

	// Invalidate memoized cache
	bf.indicators = make(map[string][]float64)
}

// UpdateLast updates the latest bar with individual values and clears the cache.
func (bf *BarFrame) UpdateLast(o, h, l, c, v float64) {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	bf.version++
	n := len(bf.Bars)
	if n == 0 {
		return
	}
	t := bf.Time[n-1]
	bf.Bars[n-1] = OHLCVBar{Timestamp: t, Open: o, High: h, Low: l, Close: c, Volume: v}
	bf.Open[n-1] = o
	bf.High[n-1] = h
	bf.Low[n-1] = l
	bf.Close[n-1] = c
	bf.Volume[n-1] = v

	bf.indicators = make(map[string][]float64)
}

// Snapshot returns an immutable point-in-time copy of all OHLCV series under a single read-lock.
func (bf *BarFrame) Snapshot() BarSeriesSnapshot {
	bf.mu.RLock()
	defer bf.mu.RUnlock()

	n := len(bf.Bars)
	t := make([]int64, n)
	o := make([]float64, n)
	h := make([]float64, n)
	l := make([]float64, n)
	c := make([]float64, n)
	v := make([]float64, n)

	copy(t, bf.Time)
	copy(o, bf.Open)
	copy(h, bf.High)
	copy(l, bf.Low)
	copy(c, bf.Close)
	copy(v, bf.Volume)

	return BarSeriesSnapshot{
		Time:   t,
		Open:   o,
		High:   h,
		Low:    l,
		Close:  c,
		Volume: v,
	}
}

// GetOHLCV returns a point-in-time copy of all OHLCV series (alias to Snapshot).
func (bf *BarFrame) GetOHLCV() BarSeriesSnapshot {
	return bf.Snapshot()
}

// GetSeries returns a defensively copied slice of the requested OHLCV or derived series.
func (bf *BarFrame) GetSeries(source string) []float64 {
	bf.mu.RLock()
	defer bf.mu.RUnlock()

	n := len(bf.Bars)
	out := make([]float64, n)

	switch strings.ToLower(strings.TrimSpace(source)) {
	case "open":
		copy(out, bf.Open)
	case "high":
		copy(out, bf.High)
	case "low":
		copy(out, bf.Low)
	case "volume", "vol":
		copy(out, bf.Volume)
	case "hl2":
		for i := 0; i < n; i++ {
			out[i] = (bf.High[i] + bf.Low[i]) / 2.0
		}
	case "hlc3":
		for i := 0; i < n; i++ {
			out[i] = (bf.High[i] + bf.Low[i] + bf.Close[i]) / 3.0
		}
	case "ohlc4":
		for i := 0; i < n; i++ {
			out[i] = (bf.Open[i] + bf.High[i] + bf.Low[i] + bf.Close[i]) / 4.0
		}
	case "close":
		fallthrough
	default:
		copy(out, bf.Close)
	}
	return out
}

// GetBars returns a defensively copied slice of all bars under RLock.
func (bf *BarFrame) GetBars() []OHLCVBar {
	bf.mu.RLock()
	defer bf.mu.RUnlock()

	res := make([]OHLCVBar, len(bf.Bars))
	copy(res, bf.Bars)
	return res
}

// GetLatestBar returns the most recent bar under RLock.
func (bf *BarFrame) GetLatestBar() (OHLCVBar, bool) {
	bf.mu.RLock()
	defer bf.mu.RUnlock()

	if len(bf.Bars) == 0 {
		return OHLCVBar{}, false
	}
	return bf.Bars[len(bf.Bars)-1], true
}

// GetIndicator retrieves a memoized series slice under RLock with defensive copy.
func (bf *BarFrame) GetIndicator(key string) ([]float64, bool) {
	bf.mu.RLock()
	defer bf.mu.RUnlock()

	val, exists := bf.indicators[key]
	if !exists {
		return nil, false
	}
	res := make([]float64, len(val))
	copy(res, val)
	return res, true
}

// SetIndicator stores a calculated series slice into cache under exclusive Lock with defensive copy.
func (bf *BarFrame) SetIndicator(key string, series []float64) {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	cpy := make([]float64, len(series))
	copy(cpy, series)
	bf.indicators[key] = cpy
}

// GetOrComputeIndicator implements race-free double-checked locking with computation outside locks.
func (bf *BarFrame) GetOrComputeIndicator(key string, computeFn func(bars []OHLCVBar) []float64) []float64 {
	// Step 1: Fast path under RLock
	bf.mu.RLock()
	if val, ok := bf.indicators[key]; ok {
		res := make([]float64, len(val))
		copy(res, val)
		bf.mu.RUnlock()
		return res
	}
	ver := bf.version
	// Copy bars for safe out-of-lock computation
	barsCopy := make([]OHLCVBar, len(bf.Bars))
	copy(barsCopy, bf.Bars)
	bf.mu.RUnlock()

	// Step 2: Compute outside locks to avoid lock contention
	computed := computeFn(barsCopy)

	// Step 3: Exclusive Lock for cache write with version and second check
	bf.mu.Lock()
	defer bf.mu.Unlock()

	if bf.version != ver {
		// Version changed during out-of-lock computation; return computed slice without caching
		res := make([]float64, len(computed))
		copy(res, computed)
		return res
	}

	if val, ok := bf.indicators[key]; ok {
		res := make([]float64, len(val))
		copy(res, val)
		return res
	}

	stored := make([]float64, len(computed))
	copy(stored, computed)
	bf.indicators[key] = stored

	res := make([]float64, len(computed))
	copy(res, computed)
	return res
}

// ClearCache flushes the memoized indicator cache under exclusive Lock.
func (bf *BarFrame) ClearCache() {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	bf.version++
	bf.indicators = make(map[string][]float64)
}

// -----------------------------------------------------------------------------
// BarFrameBundle (Hierarchical Multi-Timeframe Bundle)
// -----------------------------------------------------------------------------

// BarFrameBundle manages multi-timeframe bar frames across symbols.
type BarFrameBundle struct {
	mu      sync.RWMutex
	Symbol  string                                                // primary symbol if single-instrument bundle
	frames  map[string]map[models.TimeframeUnit]*BarFrame         // symbol -> timeframe -> BarFrame
	Frames  map[models.TimeframeUnit]*BarFrame                    // convenience map for primary symbol
}

// NewBarFrameBundle allocates a bundle, optionally initialized for a primary symbol.
func NewBarFrameBundle(symbol ...string) *BarFrameBundle {
	sym := ""
	if len(symbol) > 0 {
		sym = symbol[0]
	}
	b := &BarFrameBundle{
		Symbol: sym,
		frames: make(map[string]map[models.TimeframeUnit]*BarFrame),
		Frames: make(map[models.TimeframeUnit]*BarFrame),
	}
	if sym != "" {
		b.frames[sym] = b.Frames
	}
	return b
}

// GetFrame retrieves the BarFrame for a symbol and timeframe under RLock.
// If symbol is omitted or empty, b.Symbol is used.
func (b *BarFrameBundle) GetFrame(symbol string, tf models.TimeframeUnit) *BarFrame {
	b.mu.RLock()
	defer b.mu.RUnlock()

	targetSym := symbol
	if targetSym == "" {
		targetSym = b.Symbol
	}
	symFrames, ok := b.frames[targetSym]
	if !ok {
		return nil
	}
	return symFrames[tf]
}

// GetOrSetFrame retrieves or initializes a BarFrame for the symbol and timeframe.
func (b *BarFrameBundle) GetOrSetFrame(symbol string, tf models.TimeframeUnit) *BarFrame {
	targetSym := symbol
	if targetSym == "" {
		targetSym = b.Symbol
	}

	b.mu.RLock()
	if symFrames, ok := b.frames[targetSym]; ok {
		if frame, exists := symFrames[tf]; exists {
			b.mu.RUnlock()
			return frame
		}
	}
	b.mu.RUnlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	symFrames, ok := b.frames[targetSym]
	if !ok {
		symFrames = make(map[models.TimeframeUnit]*BarFrame)
		b.frames[targetSym] = symFrames
		if targetSym == b.Symbol {
			b.Frames = symFrames
		}
	}
	frame, exists := symFrames[tf]
	if !exists {
		frame = NewBarFrame(tf, targetSym)
		symFrames[tf] = frame
	}
	return frame
}

// AddBar appends a bar to the appropriate symbol/timeframe frame.
func (b *BarFrameBundle) AddBar(symbol string, tf models.TimeframeUnit, t int64, o, h, l, c, v float64) {
	frame := b.GetOrSetFrame(symbol, tf)
	frame.AddBar(t, o, h, l, c, v)
}

// AppendBar appends an OHLCVBar to the specified symbol and timeframe.
func (b *BarFrameBundle) AppendBar(symbol string, tf models.TimeframeUnit, bar OHLCVBar) {
	frame := b.GetOrSetFrame(symbol, tf)
	frame.AppendBar(bar)
}

// ClearCache clears indicator caches for a symbol (or all symbols if symbol is empty).
func (b *BarFrameBundle) ClearCache(symbol ...string) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(symbol) > 0 && symbol[0] != "" {
		target := symbol[0]
		if symFrames, ok := b.frames[target]; ok {
			for _, frame := range symFrames {
				frame.ClearCache()
			}
		}
		return
	}

	// Clear all symbols
	for _, symFrames := range b.frames {
		for _, frame := range symFrames {
			frame.ClearCache()
		}
	}
}

// -----------------------------------------------------------------------------
// Standalone IndicatorCache
// -----------------------------------------------------------------------------

// IndicatorCache holds memoized calculations keyed by string.
type IndicatorCache struct {
	mu    sync.RWMutex
	cache map[string][]float64
}

// NewIndicatorCache creates an empty thread-safe indicator cache.
func NewIndicatorCache() *IndicatorCache {
	return &IndicatorCache{
		cache: make(map[string][]float64),
	}
}

// Get retrieves a cached slice if present with defensive copy.
func (c *IndicatorCache) Get(key string) ([]float64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	val, ok := c.cache[key]
	if !ok {
		return nil, false
	}
	res := make([]float64, len(val))
	copy(res, val)
	return res, true
}

// Set stores a calculated slice into the cache with defensive copy.
func (c *IndicatorCache) Set(key string, val []float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	stored := make([]float64, len(val))
	copy(stored, val)
	c.cache[key] = stored
}

// Clear purges the indicator cache.
func (c *IndicatorCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string][]float64)
}

// Now returns the current Unix timestamp in seconds.
func Now() int64 {
	return time.Now().Unix()
}
