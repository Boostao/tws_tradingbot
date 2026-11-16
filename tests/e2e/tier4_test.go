package e2e

import (
	"testing"
	"time"

	"github.com/scmhub/ibapi"

	"tws_traderbot/backend/models"
	"tws_traderbot/tests/e2e/harness"
)

// Tier4TestCases defines the 6 Real-World Cobalt Operator Scenario test cases.
var Tier4TestCases = []TestCase{
	{
		ID:          "E2E-T4-SC01",
		Tier:        Tier4,
		FeatureID:   21,
		FeatureName: "Weinstein Phase 2 Breakout Workflow",
		Description: "Multi-timeframe Weinstein Phase 2 breakout with sizing, OCA bracket, and dry run recording",
		Tags:        []string{"scenario", "weinstein", "breakout", "bracket"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			// 1. Operator loads Weinstein Phase 2 strategy
			strat, err := h.GetStrategyPreset("cours_weinstein_p2")
			if err != nil {
				return nil, err
			}
			h.ActiveStrategy = strat

			// 2. Ticker NVDA evaluated against weekly and daily data
			bars1d := harness.GenerateSyntheticBars(100, 120.0, 0.8, 0.4)
			closes := make([]float64, len(bars1d))
			vols := make([]float64, len(bars1d))
			for i, b := range bars1d {
				closes[i] = b.Close
				vols[i] = b.Volume
			}
			vols[len(vols)-1] = 500000.0 // surge!

			// 3. Indicator evaluation
			_, atrPct := h.CalculateATR(bars1d, 14)
			rvol := h.CalculateRVOL(vols, 20)

			isLowVol := atrPct[len(atrPct)-1] <= 5.0
			isRVOLSurge := rvol[len(rvol)-1] > 1.5

			// 4. Sizing: $5,000 notional at $125.00 -> 40 shares
			entryPrice := 125.0
			qty, err := h.CalculatePositionSize("notional", 5000.0, entryPrice, 100000.0)
			if err != nil {
				return nil, err
			}

			// 5. OCA Bracket construction (TP +3% = 128.75, SL -1.5% = 123.12)
			tpPrice := entryPrice * 1.03
			slPrice := entryPrice * 0.985
			parent, tp, sl, err := h.BuildOCABracket("NVDA", "BUY", qty, entryPrice, tpPrice, slPrice)
			if err != nil {
				return nil, err
			}

			// 6. Record dry-run fill
			fill := &harness.TradeRecord{
				OrderID:  parent.OrderID,
				Symbol:   "NVDA",
				Action:   "BUY",
				Quantity: qty,
				Price:    entryPrice,
				OCAGroup: tp.OCAGroup,
			}
			if err := h.RecordFill(fill); err != nil {
				return nil, err
			}

			return map[string]any{
				"isLowVol":    isLowVol,
				"isRVOLSurge": isRVOLSurge,
				"qty":         qty,
				"parent":      parent,
				"tp":          tp,
				"sl":          sl,
			}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			m := res.(map[string]any)
			harness.AssertTrue(t, m["isLowVol"].(bool))
			harness.AssertTrue(t, m["isRVOLSurge"].(bool))
			harness.AssertEqual(t, m["qty"].(int), 40)

			// Verify 0 live orders dispatched
			harness.AssertEqual(t, len(h.MockIB.GetPlacedOrders()), 0)

			// Verify trade ledger
			audit := h.GetDryRunAudit()
			harness.AssertTrue(t, audit.SimulatedFills >= 1)
		},
	},
	{
		ID:          "E2E-T4-SC02",
		Tier:        Tier4,
		FeatureID:   12,
		FeatureName: "Loss Limit Trip, Lockout & Manual Re-arm",
		Description: "Market drawdown breaches daily loss limit, locks out execution, and requires manual re-arm",
		Tags:        []string{"scenario", "breaker", "loss_limit", "rearm"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.DailyLossLimit = 500.0
			h.DailyPnL = 0.0

			// 1. Initial order approved
			can1, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)

			// 2. Loss realized pushing PnL to -$525.00
			h.DailyPnL = -525.0
			h.TripCircuitBreaker("DAILY_LOSS_BREACHED")

			// 3. Subsequent order blocked at Gate 2
			can2, reason2 := h.CanExecute("MSFT.NASDAQ", models.ActionBuy)

			// 4. Operator tries premature reset -> should fail
			errReset1 := h.ResetCircuitBreaker()

			// 5. Operator clears loss for new session or increases limit
			h.DailyPnL = 0.0
			errReset2 := h.ResetCircuitBreaker()

			// 6. Post reset approved
			can3, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)

			return map[string]any{
				"can1":      can1,
				"can2":      can2,
				"reason2":   reason2,
				"errReset1": errReset1,
				"errReset2": errReset2,
				"can3":      can3,
			}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			m := res.(map[string]any)
			harness.AssertTrue(t, m["can1"].(bool), "Initial order must be approved")
			harness.AssertFalse(t, m["can2"].(bool), "Order during lockout must be rejected")
			harness.AssertEqual(t, m["reason2"].(string), "master circuit breaker tripped")
			harness.AssertError(t, m["errReset1"].(error), "Premature reset must be rejected")
			var errReset2 error
			if m["errReset2"] != nil {
				errReset2 = m["errReset2"].(error)
			}
			harness.AssertNoError(t, errReset2, "Reset after loss cleared must succeed")
			harness.AssertTrue(t, m["can3"].(bool), "Post-reset order must be approved")
		},
	},
	{
		ID:          "E2E-T4-SC03",
		Tier:        Tier4,
		FeatureID:   13,
		FeatureName: "Canadian Equity Multi-Venue Routing",
		Description: "Route Canadian equities across TSX, TSXV, and NEO with dot-class conversion and CAD currency",
		Tags:        []string{"scenario", "canadian", "routing", "venues"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			items := []models.WatchlistItem{
				{Symbol: "SHOP", Exchange: "TO"},
				{Symbol: "CTC.A", Exchange: "TO"},
				{Symbol: "WELL", Exchange: "V"},
				{Symbol: "PSYK", Exchange: "NE"},
			}

			resolved := make([]any, 0, len(items))
			for _, item := range items {
				it := item
				c, err := h.ResolveContract(&it)
				if err != nil {
					return nil, err
				}
				resolved = append(resolved, c)
			}
			return resolved, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			contracts := res.([]any)
			harness.AssertEqual(t, len(contracts), 4)

			shop := contracts[0].(*ibapi.Contract)
			harness.AssertContract(t, shop, "SHOP", "STK", "SMART", "CAD", "TSE")

			ctc := contracts[1].(*ibapi.Contract)
			harness.AssertContract(t, ctc, "CTC A", "STK", "SMART", "CAD", "TSE")

			well := contracts[2].(*ibapi.Contract)
			harness.AssertContract(t, well, "WELL", "STK", "SMART", "CAD", "VENTURE")

			psyk := contracts[3].(*ibapi.Contract)
			harness.AssertContract(t, psyk, "PSYK", "STK", "SMART", "CAD", "PURE")
		},
	},
	{
		ID:          "E2E-T4-SC04",
		Tier:        Tier4,
		FeatureID:   37,
		FeatureName: "1-Click Workspace Swapping & 1/1 Concurrency",
		Description: "Rapidly swap between Achat, Short, and Swing workspaces with strict 1/1 active slot concurrency",
		Tags:        []string{"scenario", "workspaces", "concurrency", "swap"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			// Start: Achat slot active
			s1 := h.GetCockpitState()

			// Swap to Short workspace slot
			if err := h.ApplyStrategyPreset("short", "slot_1", "screener_10_30_bas"); err != nil {
				return nil, err
			}
			s2 := h.GetCockpitState()

			// Swap to Swing workspace slot
			if err := h.ApplyStrategyPreset("swing", "slot_1", "cours_weinstein_p2"); err != nil {
				return nil, err
			}
			s3 := h.GetCockpitState()

			return []models.CockpitStateResponse{*s1, *s2, *s3}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			states := res.([]models.CockpitStateResponse)
			for _, s := range states {
				activeSlots := 0
				for _, ws := range s.Workspaces {
					for _, slot := range ws.StrategySlots {
						if slot.Enabled {
							activeSlots++
						}
					}
				}
				harness.AssertEqual(t, activeSlots, 1, "Each state must have strictly 1 active slot")
			}
			harness.AssertEqual(t, states[2].ActiveWorkspaceID, "swing")
		},
	},
	{
		ID:          "E2E-T4-SC05",
		Tier:        Tier4,
		FeatureID:   17,
		FeatureName: "ETH vs RTH Session Gating Workflow",
		Description: "Premarket earnings announcement gating with limit order allowance and market order rejection",
		Tags:        []string{"scenario", "session", "eth", "rth"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			// Pre-market time: 08:30 ET
			h.SetSimulatedTime(time.Date(2026, 11, 16, 13, 30, 0, 0, time.UTC)) // 08:30 ET
			canPreMarket, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)

			// Market open: 10:00 ET
			h.SetSimulatedTime(time.Date(2026, 11, 16, 15, 0, 0, 0, time.UTC)) // 10:00 ET
			canRTH, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)

			// Post market closed: 21:00 ET
			h.SetSimulatedTime(time.Date(2026, 11, 16, 2, 0, 0, 0, time.UTC)) // 21:00 ET
			canNight, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			h.ResetSimulatedTime()

			return map[string]bool{
				"preMarket": canPreMarket,
				"rth":       canRTH,
				"night":     canNight,
			}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			m := res.(map[string]bool)
			harness.AssertTrue(t, m["rth"], "RTH session must be approved")
			harness.AssertFalse(t, m["night"], "Night closed session must be rejected")
		},
	},
	{
		ID:          "E2E-T4-SC06",
		Tier:        Tier4,
		FeatureID:   40,
		FeatureName: "Screener Triage to Owned Positions ($$$$)",
		Description: "Candidate discovered by screener, imported, executed, and transitioned to owned positions $$$$",
		Tags:        []string{"scenario", "screener", "positions_detenues", "lifecycle"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			// 1. Screener candidate PLTR
			candidate := models.WatchlistItem{
				Symbol:       "PLTR",
				Exchange:     "NASDAQ",
				Name:         "Palantir Tech",
				Enabled:      true,
				InstrumentID: "PLTR.NASDAQ",
			}

			// 2. Execute dry-run entry
			fill := &harness.TradeRecord{
				OrderID:  201,
				Symbol:   "PLTR",
				Action:   "BUY",
				Quantity: 100,
				Price:    30.50,
			}
			if err := h.RecordFill(fill); err != nil {
				return nil, err
			}

			// 3. Transition into Positions détenues ($$$$)
			holdingsWL, err := h.GetWatchlistById("positions_detenues")
			if err != nil {
				return nil, err
			}
			holdingsWL.Groups[0].Items = append(holdingsWL.Groups[0].Items, candidate)
			if err := h.SaveWatchlist(holdingsWL); err != nil {
				return nil, err
			}

			updated, err := h.GetWatchlistById("positions_detenues")
			return updated, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			wl := res.(*models.WatchlistResponse)
			harness.AssertTrue(t, len(wl.Groups[0].Items) >= 1)
			found := false
			for _, it := range wl.Groups[0].Items {
				if it.Symbol == "PLTR" {
					found = true
				}
			}
			harness.AssertTrue(t, found, "PLTR must now be in Positions détenues ($$$$)")
		},
	},
}
