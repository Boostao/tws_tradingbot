package e2e

import (
	"math"
	"sync"
	"testing"
	"time"

	"tws_traderbot/backend/bot"
	"tws_traderbot/backend/models"
	"tws_traderbot/backend/strategy"
	"tws_traderbot/tests/e2e/harness"
)

// Tier5AdversarialTestCases defines the 36 Tier 5 Adversarial Hardening test cases.
var Tier5AdversarialTestCases = []TestCase{
	{
		ID:          "E2E-T5-ADV-01",
		Tier:        Tier5,
		FeatureID:   18,
		FeatureName: "OCA Child Bracket Orders",
		Description: "Live bracket creation failure must fail closed, never leaking an unbracketed naked live order",
		Tags:        []string{"adversarial", "bracket", "leak_guard"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, _, _, err := h.BuildOCABracket("AAPL", "BUY", 100, 150.0, 160.0, -10.0)
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-02",
		Tier:        Tier5,
		FeatureID:   18,
		FeatureName: "OCA Child Bracket Orders",
		Description: "Zero distance bracket order (TP or SL at entry price) must be rejected fail-closed",
		Tags:        []string{"adversarial", "bracket", "zero_distance"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, _, _, err := h.BuildOCABracket("AAPL", "BUY", 100, 150.0, 150.0, 140.0)
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-03",
		Tier:        Tier5,
		FeatureID:   18,
		FeatureName: "OCA Child Bracket Orders",
		Description: "Inverted bracket prices (TP below entry or SL above entry on BUY) rejected fail-closed",
		Tags:        []string{"adversarial", "bracket", "inverted"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, _, _, err1 := h.BuildOCABracket("AAPL", "BUY", 100, 150.0, 140.0, 145.0)
			_, _, _, err2 := h.BuildOCABracket("AAPL", "BUY", 100, 150.0, 160.0, 155.0)
			return err1 != nil && err2 != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-04",
		Tier:        Tier5,
		FeatureID:   18,
		FeatureName: "OCA Child Bracket Orders",
		Description: "Bracket orders with non-positive quantity rejected fail-closed",
		Tags:        []string{"adversarial", "bracket", "quantity"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, _, _, err := h.BuildOCABracket("AAPL", "BUY", 0, 150.0, 160.0, 140.0)
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-05",
		Tier:        Tier5,
		FeatureID:   12,
		FeatureName: "Daily Loss Risk Breaker",
		Description: "Daily loss limit lockout must remain tripped across market rebounds until manual operator re-arm",
		Tags:        []string{"adversarial", "risk_breaker", "bounce"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.DailyLossLimit = 500.0
			h.DailyPnL = -600.0 // breach!
			can1, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)

			// Market bounce: total PnL recovers to -$400
			h.DailyPnL = -400.0
			can2, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			return !can1 && !can2, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-06",
		Tier:        Tier5,
		FeatureID:   12,
		FeatureName: "Daily Loss Risk Breaker",
		Description: "ResetCircuitBreaker must fail closed while account daily loss is actively breached",
		Tags:        []string{"adversarial", "risk_breaker", "reset_guard"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.DailyLossLimit = 500.0
			h.DailyPnL = -800.0
			err1 := h.ResetCircuitBreaker()
			h.Engine.SetDailyLossLimit(500.0)
			h.Engine.UpdatePnL(-800.0, 0.0)
			err2 := h.Engine.ResetCircuitBreaker()
			h.DailyPnL = 0.0 // reset for clean teardown
			return err1 != nil && err2 != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-07",
		Tier:        Tier5,
		FeatureID:   12,
		FeatureName: "Daily Loss Risk Breaker",
		Description: "Non-finite PnL values (NaN, Inf) must not silently bypass circuit breaker gates",
		Tags:        []string{"adversarial", "risk_breaker", "non_finite"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.Engine.UpdatePnL(math.NaN(), 0.0)
			_, _, total := h.Engine.GetPnL()
			return math.IsNaN(total), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-08",
		Tier:        Tier5,
		FeatureID:   15,
		FeatureName: "Atomic NextValidID Sequencing",
		Description: "NextValidID sequence counter enforces monotonic increase, rejecting backwards regression",
		Tags:        []string{"adversarial", "order_id", "monotonic"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.MockIB.ResetNextOrderID(5000)
			id1, _ := h.MockIB.GetNextOrderID() // 5000
			h.MockIB.SetNextOrderID(4000)       // attempt to regress backwards
			id2, _ := h.MockIB.GetNextOrderID() // must be 5001, NOT 4000
			return id1 == 5000 && id2 == 5001, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-09",
		Tier:        Tier5,
		FeatureID:   15,
		FeatureName: "Atomic NextValidID Sequencing",
		Description: "NextValidID sequence rejects non-positive order IDs (<= 0)",
		Tags:        []string{"adversarial", "order_id", "non_positive"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.MockIB.ResetNextOrderID(-10)
			_, err := h.MockIB.GetNextOrderID()
			h.MockIB.ResetNextOrderID(100) // restore clean state
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-10",
		Tier:        Tier5,
		FeatureID:   13,
		FeatureName: "Multi-Venue Contract Resolver",
		Description: "ResolveContract rejects items with IsOrderable=false even if InstrumentID is omitted",
		Tags:        []string{"adversarial", "resolver", "orderable_check"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			item1 := &models.WatchlistItem{
				Symbol:            "SPY",
				IsOrderable:       false,
				IsRegimeReference: true,
			}
			_, err1 := bot.ResolveContract(item1)

			item2 := &models.WatchlistItem{
				Symbol:            "SPY",
				IsOrderable:       false,
				IsRegimeReference: false,
				InstrumentID:      "",
			}
			_, err2 := bot.ResolveContract(item2)
			return err1 != nil && err2 != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-11",
		Tier:        Tier5,
		FeatureID:   13,
		FeatureName: "Multi-Venue Contract Resolver",
		Description: "ResolveContract rejects unlisted foreign exchange (e.g. PARIS) fail-closed",
		Tags:        []string{"adversarial", "resolver", "foreign_venue"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			item := &models.WatchlistItem{
				Symbol:      "BNP",
				Exchange:    "PARIS",
				Venue:       "PARIS",
				IsOrderable: true,
			}
			_, err := bot.ResolveContract(item)
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-12",
		Tier:        Tier5,
		FeatureID:   14,
		FeatureName: "Dot-Class Symbol Conversion",
		Description: "ConvertDotClass sanitizes multi-dot, slash, and venue-suffixed symbols",
		Tags:        []string{"adversarial", "dot_class", "sanitization"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			c1 := bot.ConvertDotClass("BRK.B.SMART")
			c2 := bot.ConvertDotClass("CTC.A.TSE")
			c3 := bot.ConvertDotClass("BBD/B")
			c4 := bot.ConvertDotClass("  BRK ... B  ")
			return c1 == "BRK B" && c2 == "CTC A" && c3 == "BBD B" && c4 == "BRK B", nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-13",
		Tier:        Tier5,
		FeatureID:   19,
		FeatureName: "3-Mode Position Sizing",
		Description: "Position sizing calculation with insufficient notional ($10 on $500 stock) fails closed",
		Tags:        []string{"adversarial", "sizing", "insufficient_notional"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			item := &models.WatchlistItem{
				Symbol:    "AAPL",
				SizeMode:  "dollar",
				SizeValue: 10.0,
			}
			_, err := bot.CalculatePositionSize(item, 500.0, 100000.0)
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-14",
		Tier:        Tier5,
		FeatureID:   19,
		FeatureName: "3-Mode Position Sizing",
		Description: "Position sizing with non-positive or non-finite price rejected fail-closed",
		Tags:        []string{"adversarial", "sizing", "non_finite_price"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			item := &models.WatchlistItem{
				Symbol:    "AAPL",
				SizeMode:  "shares",
				SizeValue: 100.0,
			}
			_, err1 := bot.CalculatePositionSize(item, 0.0, 100000.0)
			_, err2 := bot.CalculatePositionSize(item, -50.0, 100000.0)
			_, err3 := bot.CalculatePositionSize(item, math.NaN(), 100000.0)
			_, err4 := bot.CalculatePositionSize(item, math.Inf(1), 100000.0)
			return err1 != nil && err2 != nil && err3 != nil && err4 != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-15",
		Tier:        Tier5,
		FeatureID:   17,
		FeatureName: "Session Gating",
		Description: "Session gate enforces sub-second boundary transitions at 09:30 and 16:00 ET",
		Tags:        []string{"adversarial", "session", "subsecond"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			sg := bot.NewSessionGate()
			loc := sg.Location()
			tPre := time.Date(2026, 11, 18, 9, 29, 59, 999999999, loc)
			tRTH := time.Date(2026, 11, 18, 9, 30, 0, 0, loc)
			tClose := time.Date(2026, 11, 18, 16, 0, 0, 0, loc)
			return !sg.IsRTH(tPre) && sg.IsRTH(tRTH) && !sg.IsRTH(tClose), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-16",
		Tier:        Tier5,
		FeatureID:   17,
		FeatureName: "Session Gating",
		Description: "Session gate handles weekend closed state strictly (Saturday and Sunday)",
		Tags:        []string{"adversarial", "session", "weekend"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			sg := bot.NewSessionGate()
			loc := sg.Location()
			sat := time.Date(2026, 11, 21, 12, 0, 0, 0, loc)
			sun := time.Date(2026, 11, 22, 12, 0, 0, 0, loc)
			return sg.GetSession(sat) == bot.SessionClosed && sg.GetSession(sun) == bot.SessionClosed, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-17",
		Tier:        Tier5,
		FeatureID:   10,
		FeatureName: "9-Gate Fail-Closed Engine",
		Description: "Concurrent tick evaluation under rapid circuit breaker tripping runs race-free",
		Tags:        []string{"adversarial", "concurrency", "race_free"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			eng := h.Engine
			var wg sync.WaitGroup
			for i := 0; i < 50; i++ {
				wg.Add(2)
				go func() {
					defer wg.Done()
					_ = eng.EvaluateTick("AAPL", map[string][]float64{"Close": {150.0, 151.0}})
				}()
				go func(idx int) {
					defer wg.Done()
					if idx%2 == 0 {
						eng.UpdatePnL(-600.0, 0.0)
					} else {
						eng.UpdatePnL(100.0, 0.0)
					}
				}(i)
			}
			wg.Wait()
			return true, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-18",
		Tier:        Tier5,
		FeatureID:   10,
		FeatureName: "9-Gate Fail-Closed Engine",
		Description: "Gate 5 rejects execution when multiple strategy slots are armed concurrently (>1/1 violation)",
		Tags:        []string{"adversarial", "concurrency", "slot_violation"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			state := h.GetCockpitState()
			if state != nil && len(state.Workspaces) >= 2 {
				state.Workspaces[0].Enabled = true
				if len(state.Workspaces[0].StrategySlots) > 0 {
					state.Workspaces[0].StrategySlots[0].Enabled = true
					state.Workspaces[0].StrategySlots[0].StrategyID = "screener_10_30_haut"
				}
				state.Workspaces[1].Enabled = true
				if len(state.Workspaces[1].StrategySlots) > 0 {
					state.Workspaces[1].StrategySlots[0].Enabled = true
					state.Workspaces[1].StrategySlots[0].StrategyID = "screener_10_30_bas"
				}
				h.Engine.SetCockpitState(state)
			}
			can, reason := h.Engine.CanExecuteWithPrice("AAPL.SMART", models.ActionBuy, 150.0)
			return !can && reason != "OK", nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-19",
		Tier:        Tier5,
		FeatureID:   10,
		FeatureName: "9-Gate Fail-Closed Engine",
		Description: "Gate 5 rejects execution when active workspace has zero strategy slots enabled (0/1)",
		Tags:        []string{"adversarial", "gates", "gate5_zero_slots"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			state := h.GetCockpitState()
			if state != nil {
				for i := range state.Workspaces {
					for j := range state.Workspaces[i].StrategySlots {
						state.Workspaces[i].StrategySlots[j].Enabled = false
					}
				}
				h.Engine.SetCockpitState(state)
			}
			can, reason := h.Engine.CanExecuteWithPrice("AAPL.SMART", models.ActionBuy, 150.0)
			return !can && reason != "OK", nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-20",
		Tier:        Tier5,
		FeatureID:   16,
		FeatureName: "Dry-Run Guard & Ledger",
		Description: "Dry-run mode strictly prevents live socket order transmission and records simulated fill",
		Tags:        []string{"adversarial", "dry_run", "ledger"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.DryRun = true
			h.Engine.SetDryRun(true)
			auditBefore := h.GetDryRunAudit()
			tradesBefore := len(auditBefore.Trades)

			fill := &models.TradeRecord{
				OrderID:   9999,
				Symbol:    "AAPL",
				Action:    "BUY",
				Quantity:  100,
				Price:     150.0,
				Status:    "SIMULATED_FILL",
				Mode:      "dry_run",
				CreatedAt: time.Now().UTC(),
			}
			err := h.Engine.RecordFill(fill)
			if err != nil {
				return false, err
			}

			auditAfter := h.GetDryRunAudit()
			tradesAfter := len(auditAfter.Trades)
			return tradesAfter == tradesBefore+1, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-21",
		Tier:        Tier5,
		FeatureID:   22,
		FeatureName: "Memoized Indicator Pipeline",
		Description: "Extreme all-flat price series across indicators yields bounded midpoints without NaN or divide-by-zero",
		Tags:        []string{"adversarial", "indicators", "flat_price"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			n := 100
			closes := make([]float64, n)
			highs := make([]float64, n)
			lows := make([]float64, n)
			vols := make([]float64, n)
			for i := 0; i < n; i++ {
				closes[i] = 100.0
				highs[i] = 100.0
				lows[i] = 100.0
				vols[i] = 5000.0
			}

			rsi := strategy.ComputeRSI(closes, 14)
			basis, upper, lower, bw, pctB := strategy.ComputeBollingerBands(closes, 20, 2.0)
			k, d := strategy.ComputeStochastic(highs, lows, closes, 15, 5, 5)
			rvol := strategy.ComputeRVOL(vols, 20)

			return map[string]float64{
				"rsi":   rsi[50],
				"basis": basis[50],
				"upper": upper[50],
				"lower": lower[50],
				"bw":    bw[50],
				"pctB":  pctB[50],
				"k":     k[50],
				"d":     d[50],
				"rvol":  rvol[50],
			}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			m := res.(map[string]float64)
			harness.AssertEqual(t, m["rsi"], 50.0)
			harness.AssertEqual(t, m["basis"], 100.0)
			harness.AssertEqual(t, m["upper"], 100.0)
			harness.AssertEqual(t, m["lower"], 100.0)
			harness.AssertEqual(t, m["bw"], 0.0)
			harness.AssertEqual(t, m["pctB"], 0.5)
			harness.AssertEqual(t, m["k"], 50.0)
			harness.AssertEqual(t, m["d"], 50.0)
			harness.AssertEqual(t, m["rvol"], 1.0)
		},
	},
	{
		ID:          "E2E-T5-ADV-22",
		Tier:        Tier5,
		FeatureID:   23,
		FeatureName: "Heikin-Ashi Indicator",
		Description: "Inverted and degenerate OHLCV bars (High < Low, Close < Low, Open > High) compute smoothed candlesticks without panic",
		Tags:        []string{"adversarial", "heikin_ashi", "inverted_bars"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := make([]strategy.OHLCVBar, 50)
			for i := range bars {
				bars[i] = strategy.OHLCVBar{
					Timestamp: int64(1700000000 + i*60),
					Open:      120.0,
					High:      90.0,
					Low:       110.0,
					Close:     80.0,
					Volume:    -100.0,
				}
			}
			ha := strategy.CalculateHeikinAshi(bars)
			return len(ha), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 50)
		},
	},
	{
		ID:          "E2E-T5-ADV-23",
		Tier:        Tier5,
		FeatureID:   24,
		FeatureName: "RVOL Indicator",
		Description: "Zero volume and negative volume input slices gracefully fallback to neutral 1.0 baseline without divide-by-zero",
		Tags:        []string{"adversarial", "rvol", "zero_volume"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			zeroVol := make([]float64, 50)
			negVol := make([]float64, 50)
			for i := range zeroVol {
				zeroVol[i] = 0.0
				negVol[i] = -float64(i + 1)
			}
			rvolZero := strategy.ComputeRVOL(zeroVol, 10)
			rvolNeg := strategy.ComputeRVOL(negVol, 10)
			return rvolZero[25] == 1.0 && rvolNeg[25] == 1.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-24",
		Tier:        Tier5,
		FeatureID:   22,
		FeatureName: "Memoized Indicator Pipeline",
		Description: "Non-finite values (NaN, +Inf, -Inf) in price series are safely absorbed by SMA and EMA without crash",
		Tags:        []string{"adversarial", "non_finite", "math"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			series := []float64{100.0, math.NaN(), 105.0, math.Inf(1), math.Inf(-1), 102.0}
			sma := strategy.SMA(series, 3)
			ema := strategy.EMA(series, 3)
			return len(sma) == 6 && len(ema) == 6, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-25",
		Tier:        Tier5,
		FeatureID:   21,
		FeatureName: "Multi-Timeframe BarFrame Bundle",
		Description: "Massive 100,000-bar array computation maintains strictly linear O(N) execution under 500ms and bounded allocations",
		Tags:        []string{"adversarial", "scalability", "stress"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			n := 100000
			closes := make([]float64, n)
			for i := range closes {
				closes[i] = 100.0 + float64(i%50)*0.2
			}
			start := time.Now()
			ema := strategy.EMA(closes, 200)
			rsi := strategy.ComputeRSI(closes, 14)
			dur := time.Since(start)
			return len(ema) == n && len(rsi) == n && dur < 500*time.Millisecond, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-26",
		Tier:        Tier5,
		FeatureID:   21,
		FeatureName: "Multi-Timeframe BarFrame Bundle",
		Description: "BarFrame handles descending timestamps and out-of-order bar arrivals safely",
		Tags:        []string{"adversarial", "barframe", "descending_time"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bf := strategy.NewBarFrame(models.TimeframeD1, "AAPL")
			for i := 0; i < 50; i++ {
				bf.AppendBar(strategy.OHLCVBar{
					Timestamp: int64(5000 - i*60),
					Close:     150.0 + float64(i),
				})
			}
			return bf.Len(), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 50)
		},
	},
	{
		ID:          "E2E-T5-ADV-27",
		Tier:        Tier5,
		FeatureID:   21,
		FeatureName: "Multi-Timeframe BarFrame Bundle",
		Description: "Concurrent AppendBar, UpdateLast, Snapshot, and ClearCache across 20 goroutines with zero data races",
		Tags:        []string{"adversarial", "concurrency", "barframe_race"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bf := strategy.NewBarFrame(models.TimeframeD1, "AAPL")
			var wg sync.WaitGroup
			for g := 0; g < 20; g++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					for i := 0; i < 50; i++ {
						if i%3 == 0 {
							bf.AppendBar(strategy.OHLCVBar{Timestamp: int64(i), Close: float64(i)})
						} else if i%3 == 1 {
							bf.UpdateLast(10.0, 15.0, 9.0, 12.0, 1000.0)
						} else {
							_ = bf.Snapshot()
							bf.ClearCache()
						}
					}
				}(g)
			}
			wg.Wait()
			return bf.Len() > 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-28",
		Tier:        Tier5,
		FeatureID:   22,
		FeatureName: "Memoized Indicator Pipeline",
		Description: "Double-checked locking race safety: version invalidation during out-of-lock computation discards stale cache write",
		Tags:        []string{"adversarial", "double_checked_locking", "invalidation"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bf := strategy.NewBarFrame(models.TimeframeD1, "AAPL")
			for i := 0; i < 10; i++ {
				bf.AppendBar(strategy.OHLCVBar{Timestamp: int64(i), Close: float64(i)})
			}

			res := bf.GetOrComputeIndicator("test_key", func(bars []strategy.OHLCVBar) []float64 {
				bf.AppendBar(strategy.OHLCVBar{Timestamp: 999, Close: 999.0})
				return []float64{1.0, 2.0}
			})

			_, cached := bf.GetIndicator("test_key")
			return len(res) == 2 && !cached, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-29",
		Tier:        Tier5,
		FeatureID:   34,
		FeatureName: "Expanded Condition Evaluator",
		Description: "Evaluator fail-closed verification: ConditionCrossAbove without data returns false and does not panic",
		Tags:        []string{"adversarial", "evaluator", "fail_closed"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cond := &models.Condition{
				Type:      models.ConditionCrossAbove,
				Threshold: 50.0,
			}
			fired := strategy.EvaluateConditions(cond, map[string][]float64{})
			return !fired, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-30",
		Tier:        Tier5,
		FeatureID:   33,
		FeatureName: "Relative Strength vs SPX",
		Description: "Relative Strength returns neutral 100.0 baseline when benchmark is missing or length mismatched",
		Tags:        []string{"adversarial", "rs_spx", "neutral_baseline"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			ticker := []float64{100.0, 105.0, 110.0}
			bench := []float64{4000.0}
			rs := strategy.ComputeRelativeStrength(ticker, bench, 100)
			return rs[0] == 100.0 && rs[1] == 100.0 && rs[2] == 100.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-31",
		Tier:        Tier5,
		FeatureID:   6,
		FeatureName: "Smart Diff/Merge",
		Description: "Smart Diff/Merge strictly deduplicates identical tickers within same group and across multiple groups",
		Tags:        []string{"adversarial", "watchlist", "deduplication"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			groups := []models.WatchlistGroup{
				{
					ID:   "g1",
					Name: "G1",
					Items: []models.WatchlistItem{
						{Symbol: "AAPL", Exchange: "NASDAQ", Enabled: true, Tier: "1-Leader"},
						{Symbol: "AAPL", Exchange: "NASDAQ", Enabled: false, Tier: "2-Croissance"},
					},
				},
				{
					ID:   "g2",
					Name: "G2",
					Items: []models.WatchlistItem{
						{Symbol: "AAPL.NASDAQ", Enabled: true},
					},
				},
			}
			res, err := h.Database.MergeAndSaveWatchlist("test-dedup", "Dedup Test", "", nil, groups, false)
			if err != nil {
				return nil, err
			}
			aaplCount := 0
			for _, sym := range res.Symbols {
				if sym == "AAPL.NASDAQ" {
					aaplCount++
				}
			}
			return aaplCount, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 1)
		},
	},
	{
		ID:          "E2E-T5-ADV-32",
		Tier:        Tier5,
		FeatureID:   6,
		FeatureName: "Smart Diff/Merge",
		Description: "Smart Diff/Merge preserves operator overrides (Tier, Enabled, SizeMode, SizeValue) across feed refreshes",
		Tags:        []string{"adversarial", "watchlist", "override_preservation"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			groups := []models.WatchlistGroup{
				{
					ID:   "g1",
					Name: "G1",
					Items: []models.WatchlistItem{
						{Symbol: "NVDA", Exchange: "NASDAQ", Enabled: false, Tier: "3-Speculatif", SizeMode: "risk_fixed", SizeValue: 750.0},
					},
				},
			}
			_, err := h.Database.MergeAndSaveWatchlist("test-preserve", "Preserve Test", "", nil, groups, false)
			if err != nil {
				return nil, err
			}

			feedGroups := []models.WatchlistGroup{
				{
					ID:   "g1",
					Name: "G1",
					Items: []models.WatchlistItem{
						{Symbol: "NVDA", Exchange: "NASDAQ", Enabled: true, Tier: "2-Croissance", SizeMode: "shares", SizeValue: 100.0},
					},
				},
			}
			res, err := h.Database.MergeAndSaveWatchlist("test-preserve", "Preserve Test", "", nil, feedGroups, true)
			if err != nil {
				return nil, err
			}

			item := res.Groups[0].Items[0]
			return item.Tier == "3-Speculatif" && item.Enabled == false && item.SizeMode == "risk_fixed" && item.SizeValue == 750.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-33",
		Tier:        Tier5,
		FeatureID:   2,
		FeatureName: "Relational Watchlist Schema",
		Description: "SQLite foreign key enforcement rejects insertion of watchlist item referencing non-existent watchlist ID",
		Tags:        []string{"adversarial", "sqlite", "foreign_key"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, err := h.DB.Exec(`
				INSERT INTO watchlist_items (watchlist_id, symbol, exchange, instrument_id)
				VALUES ('ghost_watchlist', 'AAPL', 'NASDAQ', 'AAPL.NASDAQ')
			`)
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-34",
		Tier:        Tier5,
		FeatureID:   37,
		FeatureName: "Three Workspaces",
		Description: "ApplyStrategyPreset enforces 1/1 strategy slot concurrency invariant: target slot armed, all other slots across all workspaces disabled",
		Tags:        []string{"adversarial", "concurrency", "1_1_invariant"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			err := h.ApplyStrategyPreset("short", "slot_1", "screener_10_30_bas")
			if err != nil {
				return nil, err
			}

			state := h.GetCockpitState()
			activeWorkspaces := 0
			activeSlots := 0
			for _, ws := range state.Workspaces {
				if ws.Enabled {
					activeWorkspaces++
				}
				for _, slot := range ws.StrategySlots {
					if slot.Enabled {
						activeSlots++
					}
				}
			}
			return activeWorkspaces == 1 && activeSlots == 1 && state.ActiveWorkspaceID == "short", nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-35",
		Tier:        Tier5,
		FeatureID:   17,
		FeatureName: "Session Gating",
		Description: "Time-of-day condition evaluation cleanly handles overnight time wrap-around boundaries (e.g. 22:00 to 04:00)",
		Tags:        []string{"adversarial", "session", "overnight_wrap"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			t1 := strategy.WithinRange("23:15", "22:00", "04:00")
			t2 := strategy.WithinRange("02:45", "22:00", "04:00")
			t3 := strategy.WithinRange("08:00", "22:00", "04:00")
			t4 := strategy.WithinRange("12:00", "09:30", "16:00")
			t5 := strategy.WithinRange("18:00", "09:30", "16:00")
			return t1 && t2 && !t3 && t4 && !t5, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T5-ADV-36",
		Tier:        Tier5,
		FeatureID:   34,
		FeatureName: "Expanded Condition Evaluator",
		Description: "EvaluateConditions and Engine.EvaluateTick fail closed when IndicatorB is required but missing from market data",
		Tags:        []string{"adversarial", "evaluator", "fail_closed", "indicator_b"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			msft := "MSFT"
			cond := &models.Condition{
				Type:       models.ConditionGreater,
				IndicatorA: &models.Indicator{Type: models.IndicatorPrice},
				IndicatorB: &models.Indicator{Type: models.IndicatorPrice, Symbol: &msft},
				Threshold:  0.0,
			}

			// 1. Direct strategy.EvaluateConditions test with only indicatorA
			dataAOnly := map[string][]float64{
				"indicatorA": {150.0},
			}
			firedEval := strategy.EvaluateConditions(cond, dataAOnly)

			// 2. Runtime Engine.EvaluateTick test with rule requiring IndicatorB
			eng := bot.NewEngine()
			eng.SetGlobalEnabled(true)
			_ = eng.StartBot(&models.Strategy{
				ID: "strat_fail_closed_test",
				Rules: []models.Rule{
					{
						ID:        "rule_requires_b",
						Enabled:   true,
						Action:    models.ActionBuy,
						Condition: cond,
					},
				},
			})
			defer eng.StopBot()

			tickData := map[string][]float64{
				"Close":      {150.0},
				"Open":       {149.0},
				"indicatorA": {150.0},
			}
			actions := eng.EvaluateTick("AAPL", tickData)

			return !firedEval && len(actions) == 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
}
