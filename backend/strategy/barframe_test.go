package strategy_test

import (
	"sync"
	"testing"
	"time"

	"tws_traderbot/backend/models"
	"tws_traderbot/backend/strategy"
)

func TestTimeframeMappings(t *testing.T) {
	// Feature 20: 9 canonical Cobalt intervals
	all := models.AllTimeframes()
	if len(all) != 9 {
		t.Fatalf("expected 9 timeframes, got %d", len(all))
	}

	expectedBarSizes := map[models.TimeframeUnit]string{
		models.TimeframeM1:  "1 min",
		models.TimeframeM5:  "5 mins",
		models.TimeframeM15: "15 mins",
		models.TimeframeM30: "30 mins",
		models.TimeframeH1:  "1 hour",
		models.TimeframeH4:  "4 hours",
		models.TimeframeD1:  "1 day",
		models.TimeframeW1:  "1 week",
		models.TimeframeMN1: "1 month",
	}

	for tf, expectedSize := range expectedBarSizes {
		if tf.ToIBKRBarSize() != expectedSize {
			t.Errorf("expected %s for %s, got %s", expectedSize, tf, tf.ToIBKRBarSize())
		}
		if tf.DefaultIBKRDuration() == "" {
			t.Errorf("expected non-empty default duration for %s", tf)
		}
	}

	// Parsing tests
	parsed, err := models.ParseTimeframe("15m")
	if err != nil || parsed != models.TimeframeM15 {
		t.Errorf("failed to parse 15m: %v", err)
	}

	parsed, err = models.ParseTimeframe("1d")
	if err != nil || parsed != models.TimeframeD1 {
		t.Errorf("failed to parse 1d: %v", err)
	}

	_, err = models.ParseTimeframe("invalid_tf")
	if err == nil {
		t.Errorf("expected error for invalid timeframe string")
	}
}

func TestBarFrame_AppendAndSeries(t *testing.T) {
	// Feature 21: BarFrame OHLCV ingestion & series extraction
	bf := strategy.NewBarFrame(models.TimeframeD1, "AAPL.NASDAQ")
	if bf.Len() != 0 || bf.BarCount() != 0 {
		t.Fatalf("expected empty BarFrame")
	}

	if _, ok := bf.GetLatestBar(); ok {
		t.Fatalf("expected GetLatestBar on empty frame to return false")
	}

	baseTime := time.Now().Unix()
	for i := 0; i < 10; i++ {
		p := float64(100 + i)
		bf.AddBar(baseTime+int64(i*86400), p-1, p+2, p-2, p, 1000+float64(i*100))
	}

	if bf.Len() != 10 {
		t.Fatalf("expected 10 bars, got %d", bf.Len())
	}

	latest, ok := bf.GetLatestBar()
	if !ok || latest.Close != 109.0 {
		t.Fatalf("expected latest close 109.0, got %v", latest.Close)
	}

	closes := bf.GetSeries("close")
	if len(closes) != 10 || closes[9] != 109.0 {
		t.Fatalf("unexpected closes slice: %v", closes)
	}

	highs := bf.GetSeries("high")
	if len(highs) != 10 || highs[9] != 111.0 {
		t.Fatalf("unexpected highs slice: %v", highs)
	}

	lows := bf.GetSeries("low")
	if len(lows) != 10 || lows[9] != 107.0 {
		t.Fatalf("unexpected lows slice: %v", lows)
	}

	opens := bf.GetSeries("open")
	if len(opens) != 10 || opens[9] != 108.0 {
		t.Fatalf("unexpected opens slice: %v", opens)
	}

	vols := bf.GetSeries("volume")
	if len(vols) != 10 || vols[9] != 1900.0 {
		t.Fatalf("unexpected volumes slice: %v", vols)
	}

	hl2 := bf.GetSeries("hl2")
	if hl2[9] != (111.0+107.0)/2.0 {
		t.Fatalf("unexpected hl2: %v", hl2[9])
	}

	hlc3 := bf.GetSeries("hlc3")
	if hlc3[9] != (111.0+107.0+109.0)/3.0 {
		t.Fatalf("unexpected hlc3: %v", hlc3[9])
	}

	ohlc4 := bf.GetSeries("ohlc4")
	if ohlc4[9] != (108.0+111.0+107.0+109.0)/4.0 {
		t.Fatalf("unexpected ohlc4: %v", ohlc4[9])
	}
}

func TestBarFrame_UpdateLastBar(t *testing.T) {
	bf := strategy.NewBarFrame(models.TimeframeM5, "MSFT.NASDAQ")
	bf.AddBar(1000, 50, 55, 49, 52, 500)
	bf.SetIndicator("test_key", []float64{1.0})

	// Updating last bar should purge indicator cache
	bf.UpdateLast(50, 56, 48, 54, 700)
	latest, _ := bf.GetLatestBar()
	if latest.Close != 54 || latest.High != 56 || latest.Volume != 700 {
		t.Fatalf("failed to update last bar in place: %+v", latest)
	}

	if _, hit := bf.GetIndicator("test_key"); hit {
		t.Fatalf("cache should be purged after UpdateLast")
	}

	// Append bar should also purge indicator cache
	bf.SetIndicator("test_key_2", []float64{2.0})
	bf.AppendBar(strategy.OHLCVBar{Timestamp: 1300, Open: 54, High: 58, Low: 53, Close: 57, Volume: 800})
	if _, hit := bf.GetIndicator("test_key_2"); hit {
		t.Fatalf("cache should be purged after AppendBar")
	}
}

func TestBarFrameBundle_MultiTimeframe(t *testing.T) {
	bundle := strategy.NewBarFrameBundle("AAPL.NASDAQ")

	// Ingest 1d and 5m bars
	bundle.AddBar("AAPL.NASDAQ", models.TimeframeD1, 1000, 150, 155, 149, 154, 10000)
	bundle.AddBar("AAPL.NASDAQ", models.TimeframeM5, 1000, 152, 153, 151, 152.5, 2000)

	f1d := bundle.GetFrame("AAPL.NASDAQ", models.TimeframeD1)
	if f1d == nil || f1d.Len() != 1 {
		t.Fatalf("expected 1D frame with 1 bar")
	}

	f5m := bundle.GetFrame("AAPL.NASDAQ", models.TimeframeM5)
	if f5m == nil || f5m.Len() != 1 {
		t.Fatalf("expected 5M frame with 1 bar")
	}

	f1h := bundle.GetFrame("AAPL.NASDAQ", models.TimeframeH1)
	if f1h != nil {
		t.Fatalf("expected nil for unpopulated 1H frame")
	}

	// ClearCache on bundle
	f1d.SetIndicator("cached_sma", []float64{154.0})
	bundle.ClearCache("AAPL.NASDAQ")
	if _, hit := f1d.GetIndicator("cached_sma"); hit {
		t.Fatalf("expected cache to be cleared by bundle.ClearCache")
	}
}

func TestBarFrame_ConcurrentDoubleCheckedLocking(t *testing.T) {
	// Feature 22: Race-safe memoized indicator cache under concurrency
	bf := strategy.NewBarFrame(models.TimeframeD1, "SPY.ARCA")
	for i := 0; i < 50; i++ {
		p := float64(400 + i)
		bf.AddBar(int64(i*86400), p, p+1, p-1, p+0.5, 5000)
	}

	var wg sync.WaitGroup
	computeCount := int64(0)
	var muCount sync.Mutex

	computeFn := func(bars []strategy.OHLCVBar) []float64 {
		muCount.Lock()
		computeCount++
		muCount.Unlock()
		time.Sleep(1 * time.Millisecond) // simulate compute latency
		res := make([]float64, len(bars))
		for i, b := range bars {
			res[i] = b.Close * 2.0
		}
		return res
	}

	// Concurrently invoke GetOrComputeIndicator across 30 goroutines with same key
	key := "SPY_double_close"
	for g := 0; g < 30; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			series := bf.GetOrComputeIndicator(key, computeFn)
			if len(series) != 50 {
				t.Errorf("expected 50 elements, got %d", len(series))
			}
		}()
	}
	wg.Wait()

	// Verify that defensive copying prevents external slice mutation
	cached1, hit := bf.GetIndicator(key)
	if !hit {
		t.Fatalf("expected cache hit")
	}
	cached1[0] = 999999.0 // mutate local slice

	cached2, _ := bf.GetIndicator(key)
	if cached2[0] == 999999.0 {
		t.Fatalf("data race hazard: cache returned mutable pointer to internal backing array!")
	}
}

func TestStandaloneIndicatorCache(t *testing.T) {
	cache := strategy.NewIndicatorCache()
	if _, hit := cache.Get("missing"); hit {
		t.Fatalf("expected miss")
	}

	vals := []float64{10, 20, 30}
	cache.Set("test", vals)

	got, hit := cache.Get("test")
	if !hit || len(got) != 3 || got[1] != 20 {
		t.Fatalf("unexpected cache get: %v", got)
	}

	cache.Clear()
	if _, hit := cache.Get("test"); hit {
		t.Fatalf("expected miss after clear")
	}
}

func TestBarFrame_Snapshot(t *testing.T) {
	bf := strategy.NewBarFrame(models.TimeframeD1, "MSFT.NASDAQ")
	for i := 0; i < 5; i++ {
		p := float64(300 + i)
		bf.AddBar(int64(1000+i*60), p, p+2, p-2, p+1, 500)
	}

	snap := bf.Snapshot()
	if len(snap.Close) != 5 || len(snap.High) != 5 || len(snap.Low) != 5 || len(snap.Open) != 5 || len(snap.Volume) != 5 || len(snap.Time) != 5 {
		t.Fatalf("unexpected snapshot lengths: %v", snap)
	}

	// Appending to bf must not change snap
	bf.AddBar(int64(2000), 310, 312, 308, 311, 600)
	if len(snap.Close) != 5 {
		t.Fatalf("snapshot length changed after AppendBar: %d", len(snap.Close))
	}

	// GetOHLCV alias returns same lengths
	snap2 := bf.GetOHLCV()
	if len(snap2.Close) != 6 {
		t.Fatalf("expected GetOHLCV to have 6 bars, got %d", len(snap2.Close))
	}
}

func TestBarFrame_VersionInvalidationDuringCompute(t *testing.T) {
	bf := strategy.NewBarFrame(models.TimeframeD1, "NVDA.NASDAQ")
	for i := 0; i < 10; i++ {
		bf.AddBar(int64(i*60), 100, 105, 95, 102, 1000)
	}

	computeStarted := make(chan struct{})
	barAdded := make(chan struct{})

	computeFn := func(bars []strategy.OHLCVBar) []float64 {
		close(computeStarted)
		<-barAdded // wait for concurrent bar append to increment version
		res := make([]float64, len(bars))
		for i := range bars {
			res[i] = 42.0
		}
		return res
	}

	var computed []float64
	done := make(chan struct{})
	go func() {
		computed = bf.GetOrComputeIndicator("test_versioning", computeFn)
		close(done)
	}()

	<-computeStarted
	// Concurrent bar addition increments version and invalidates cache
	bf.AddBar(int64(600), 103, 107, 101, 106, 1200)
	close(barAdded)
	<-done

	// The computed slice was returned directly
	if len(computed) != 10 {
		t.Fatalf("expected computed length 10, got %d", len(computed))
	}

	// Cache must NOT have been poisoned with the stale 10-length series
	if _, hit := bf.GetIndicator("test_versioning"); hit {
		t.Fatalf("expected cache miss because version changed during computation")
	}
}
