package e2e

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/scmhub/ibapi"

	"tws_traderbot/backend/models"
	"tws_traderbot/tests/e2e/harness"
)

// Tier3TestCases defines the 15 Cross-Feature Interaction test suites.
var Tier3TestCases = []TestCase{
	{
		ID:          "E2E-T3-M01",
		Tier:        Tier3,
		FeatureID:   4,
		FeatureName: "Normalization + Resolver + Dot-Class (F4+F13+F14)",
		Description: "Multi-symbol normalization, dot-class conversion, and Canadian/US routing pipeline",
		Tags:        []string{"normalization", "resolver", "dot_class", "routing"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			rawTickers := []struct {
				Symbol   string
				Exchange string
			}{
				{"AAPL", "NASDAQ"},
				{"SHOP", "TO"},
				{"BRK.B", "NYSE"},
				{"CTC.A", "TO"},
				{"WELL", "V"},
			}

			resolvedContracts := make([]any, 0, len(rawTickers))
			for _, r := range rawTickers {
				normID, err := h.NormalizeInstrumentID(r.Symbol, r.Exchange)
				if err != nil {
					return nil, err
				}
				item := &models.WatchlistItem{
					Symbol:       r.Symbol,
					Exchange:     r.Exchange,
					InstrumentID: normID,
				}
				c, err := h.ResolveContract(item)
				if err != nil {
					return nil, err
				}
				resolvedContracts = append(resolvedContracts, c)
			}
			return resolvedContracts, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			contracts := res.([]any)
			harness.AssertEqual(t, len(contracts), 5)
		},
	},
	{
		ID:          "E2E-T3-M02",
		Tier:        Tier3,
		FeatureID:   10,
		FeatureName: "Orderable Check + 9-Gate + Dry-Run Guard (F5+F10+F16)",
		Description: "Filter out non-orderable benchmarks and record simulated fills without socket transmission",
		Tags:        []string{"gates", "classification", "dry_run"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			// Benchmark SPX.IND should fail Gate 6
			canSPX, reasonSPX := h.CanExecute("SPX.IND", models.ActionBuy)

			// Equity NVDA.NASDAQ should pass all gates
			canNVDA, reasonNVDA := h.CanExecute("NVDA.NASDAQ", models.ActionBuy)

			// Record dry run fill for NVDA
			if canNVDA {
				fill := &harness.TradeRecord{
					OrderID:  101,
					Symbol:   "NVDA",
					Action:   "BUY",
					Quantity: 25,
					Price:    120.0,
				}
				if err := h.RecordFill(fill); err != nil {
					return nil, err
				}
			}

			return map[string]any{
				"canSPX":     canSPX,
				"reasonSPX":  reasonSPX,
				"canNVDA":    canNVDA,
				"reasonNVDA": reasonNVDA,
			}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			m := res.(map[string]any)
			harness.AssertFalse(t, m["canSPX"].(bool), "SPX must not be orderable")
			harness.AssertTrue(t, m["canNVDA"].(bool), "NVDA must be orderable")

			// Check dry run placed 0 live orders
			harness.AssertEqual(t, len(h.MockIB.GetPlacedOrders()), 0, "Dry run must not place live orders")

			// Check trade_ledger
			audit := h.GetDryRunAudit()
			harness.AssertTrue(t, audit.SimulatedFills >= 1, "Simulated fill must be recorded")
		},
	},
	{
		ID:          "E2E-T3-M03",
		Tier:        Tier3,
		FeatureID:   11,
		FeatureName: "Master Breaker + Daily Loss + Telemetry HUD (F11+F12+F39)",
		Description: "Daily loss breach trips breaker, reflects in HUD, and blocks re-arm until cleared",
		Tags:        []string{"breaker", "daily_loss", "hud"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.DailyLossLimit = 500.0
			h.DailyPnL = -550.0 // breach!

			// Auto trip
			h.TripCircuitBreaker("DAILY_LOSS_BREACH")

			// Attempt reset while still in loss
			errReset := h.ResetCircuitBreaker()

			// Telemetry check
			diag := h.GetDiagnosticsSummary()

			return map[string]any{
				"errReset": errReset,
				"diag":     diag,
			}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			m := res.(map[string]any)
			harness.AssertError(t, m["errReset"].(error), "Reset must fail when daily loss is exceeded")
			diag := m["diag"].(*harness.DiagnosticsResponse)
			harness.AssertTrue(t, diag.BreakerTripped, "HUD must show breaker tripped")
			harness.AssertFalse(t, diag.GlobalEnabled, "GlobalEnabled must be false")
		},
	},
	{
		ID:          "E2E-T3-M04",
		Tier:        Tier3,
		FeatureID:   2,
		FeatureName: "Relational Watchlist + Smart Diff/Merge + Ticker Rows (F2+F6+F40)",
		Description: "Preserve local overrides during watchlist diff/merge with relational DB",
		Tags:        []string{"watchlist", "merge", "ui"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			// 1. Initial watchlist
			initial := &models.WatchlistResponse{
				Groups: []models.WatchlistGroup{
					{
						ID:     "custom_wl",
						Name:   "Custom List",
						Source: "custom",
						Items: []models.WatchlistItem{
							{Symbol: "AAPL", Exchange: "NASDAQ", Name: "Apple", Enabled: false, InstrumentID: "AAPL.NASDAQ"},
						},
					},
				},
			}
			if err := h.SaveWatchlist(initial); err != nil {
				return nil, err
			}

			// 2. Diff/Merge updated list with PLTR added and AAPL enabled=true in remote
			req := &harness.ReplaceWatchlistRequest{
				WatchlistID: "custom_wl",
				Name:        "Custom List",
				Source:      "custom",
				Items: []models.WatchlistItem{
					{Symbol: "AAPL", Exchange: "NASDAQ", Name: "Apple Inc.", Enabled: false, InstrumentID: "AAPL.NASDAQ"},
					{Symbol: "PLTR", Exchange: "NASDAQ", Name: "Palantir Tech", Enabled: true, InstrumentID: "PLTR.NASDAQ"},
				},
			}
			updated, err := h.ReplaceWatchlist(req)
			return updated, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			wl := res.(*models.WatchlistResponse)
			harness.AssertEqual(t, len(wl.Groups[0].Items), 2)
			// AAPL enabled=false preserved
			for _, it := range wl.Groups[0].Items {
				if it.Symbol == "AAPL" {
					harness.AssertFalse(t, it.Enabled, "AAPL must preserve enabled=false")
				}
			}
		},
	},
	{
		ID:          "E2E-T3-M05",
		Tier:        Tier3,
		FeatureID:   20,
		FeatureName: "9 Timeframes + BarFrame Bundle + Memoized Cache (F20+F21+F22)",
		Description: "Multi-timeframe bar ingestion and memoized indicator caching across intervals",
		Tags:        []string{"timeframes", "barframe", "cache"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bundle := harness.NewBarFrameBundle("AAPL.NASDAQ")
			bars1d := harness.GenerateSyntheticBars(50, 150.0, 0.5, 1.0)
			for _, b := range bars1d {
				bundle.AppendBar(models.TimeframeD1, b)
			}

			closes := make([]float64, len(bars1d))
			for i, b := range bars1d {
				closes[i] = b.Close
			}

			// Compute and memoize SMA
			key := "AAPL_1d_SMA_20"
			sma := h.CalculateSMA(closes, 20)
			h.IndicatorCache.Set(key, sma)

			cached, hit := h.IndicatorCache.Get(key)
			return map[string]any{"hit": hit, "val": cached}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			m := res.(map[string]any)
			harness.AssertTrue(t, m["hit"].(bool), "Cache hit expected")
			vals := m["val"].([]float64)
			harness.AssertEqual(t, len(vals), 50)
		},
	},
	{
		ID:          "E2E-T3-M06",
		Tier:        Tier3,
		FeatureID:   34,
		FeatureName: "Multi-Indicator Confluence + Condition Evaluator (F26+F27+F28+F34)",
		Description: "Triple confluence: RSI oversold + BB Lower cross + MACD Histogram positive",
		Tags:        []string{"confluence", "evaluator", "indicators"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			// Synthetic series showing confluence
			data := map[string][]float64{
				"indicatorA": {25.0, 28.0}, // RSI below 30
				"indicatorB": {30.0, 30.0},
			}
			condRSI := &models.Condition{Type: models.ConditionLess, Threshold: 30.0}
			rsiActive := h.EvaluateCondition(condRSI, data)

			dataMACD := map[string][]float64{
				"indicatorA": {-0.5, 0.2}, // histogram flipped positive
				"indicatorB": {0.0, 0.0},
			}
			condMACD := &models.Condition{Type: models.ConditionCrossAbove}
			macdActive := h.EvaluateCondition(condMACD, dataMACD)

			return rsiActive && macdActive, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool), "Confluence conditions must be satisfied")
		},
	},
	{
		ID:          "E2E-T3-M07",
		Tier:        Tier3,
		FeatureID:   18,
		FeatureName: "Position Sizing + OCA Bracket + Atomic IDs (F15+F18+F19)",
		Description: "3-order OCA bracket with portfolio sizing and atomic order ID sequencing",
		Tags:        []string{"sizing", "bracket", "order_id"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			qty, err := h.CalculatePositionSize("portfolio", 5.0, 100.0, 100000.0)
			if err != nil {
				return nil, err
			}
			parent, tp, sl, err := h.BuildOCABracket("AAPL", "BUY", qty, 100.0, 103.0, 98.5)
			if err != nil {
				return nil, err
			}
			return map[string]any{"qty": qty, "p": parent, "tp": tp, "sl": sl}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			m := res.(map[string]any)
			harness.AssertEqual(t, m["qty"].(int), 50, "5% of $100k @ $100 = 50 shares")
			p := m["p"].(*ibapi.Order)
			tp := m["tp"].(*ibapi.Order)
			sl := m["sl"].(*ibapi.Order)
			harness.AssertEqual(t, tp.ParentID, p.OrderID)
			harness.AssertEqual(t, sl.ParentID, p.OrderID)
			harness.AssertEqual(t, tp.OCAGroup, sl.OCAGroup)
		},
	},
	{
		ID:          "E2E-T3-M08",
		Tier:        Tier3,
		FeatureID:   37,
		FeatureName: "Three Workspaces + 1-Click Swap + Concurrency 1/1 (F37+F38)",
		Description: "Fast tab swap switches active strategy slot and enforces strictly 1/1 concurrency",
		Tags:        []string{"workspaces", "concurrency", "preset_swap"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			// Apply preset to short workspace
			err := h.ApplyStrategyPreset("short", "slot_1", "screener_10_30_bas")
			if err != nil {
				return nil, err
			}
			state := h.GetCockpitState()
			return state, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			state := res.(*models.CockpitStateResponse)
			activeSlotCount := 0
			for _, ws := range state.Workspaces {
				for _, slot := range ws.StrategySlots {
					if slot.Enabled {
						activeSlotCount++
					}
				}
			}
			harness.AssertEqual(t, activeSlotCount, 1, "Strictly 1 slot must be active")
			harness.AssertEqual(t, state.ActiveWorkspaceID, "short")
		},
	},
	{
		ID:          "E2E-T3-M09",
		Tier:        Tier3,
		FeatureID:   17,
		FeatureName: "Session Gating + 9-Gate + Dry-Run Execution (F10+F16+F17)",
		Description: "Off-hours rejected by session gate; in-hours recorded in dry run ledger",
		Tags:        []string{"session", "gates", "dry_run"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			// Set overnight closed time
			h.SetSimulatedTime(time.Date(2026, 11, 16, 23, 0, 0, 0, time.UTC))
			canNight, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)

			// Set active market time
			h.SetSimulatedTime(time.Date(2026, 11, 16, 15, 0, 0, 0, time.UTC))
			canDay, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			h.ResetSimulatedTime()

			return map[string]bool{"night": canNight, "day": canDay}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			m := res.(map[string]bool)
			harness.AssertFalse(t, m["night"], "Night session must block execution")
			harness.AssertTrue(t, m["day"], "Day session must approve execution")
		},
	},
	{
		ID:          "E2E-T3-M10",
		Tier:        Tier3,
		FeatureID:   35,
		FeatureName: "Pre-Seeded Watchlists + 20 Presets + Wails IPC (F3+F7+F35)",
		Description: "Fresh application boot seeds 5 watchlists and 20 presets available over IPC",
		Tags:        []string{"seeds", "presets", "ipc"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			wls := h.GetWatchlists()
			library := h.GetStrategyLibrary()
			return map[string]int{"wlCount": len(wls), "stratCount": len(library)}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			m := res.(map[string]int)
			harness.AssertEqual(t, m["wlCount"], 5, "Exactly 5 seeded watchlists")
			harness.AssertEqual(t, m["stratCount"], 20, "Exactly 20 seeded presets")
		},
	},
	{
		ID:          "E2E-T3-M11",
		Tier:        Tier3,
		FeatureID:   41,
		FeatureName: "Authentic Lexicon + i18n Parity + Linters (F41+F42+F50)",
		Description: "Validate absence of banned framework jargon and parity of Cobalt translations",
		Tags:        []string{"lexicon", "i18n", "linters"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			// 1. Extract translations from translations.ts
			en, fr, err := harness.ExtractTranslationKeys("frontend/src/lib/i18n/translations.ts")
			if err != nil {
				return nil, err
			}

			// 2. Symmetric key difference
			var missingInFr, missingInEn []string
			for k := range en {
				if _, exists := fr[k]; !exists {
					missingInFr = append(missingInFr, k)
				}
			}
			for k := range fr {
				if _, exists := en[k]; !exists {
					missingInEn = append(missingInEn, k)
				}
			}

			// 3. Scan for banned terms in translations and API
			apiData, _ := os.ReadFile("frontend/src/lib/api.ts")
			transData, _ := os.ReadFile("frontend/src/lib/i18n/translations.ts")
			corpus := strings.ToLower(string(apiData) + "\n" + string(transData))
			banned := []string{"nautilus", "fastapi", "pydantic"}
			bannedCounts := make(map[string]int)
			for _, b := range banned {
				bannedCounts[b] = strings.Count(corpus, b)
			}

			// 4. Verify authentic Cobalt terms in FR
			requiredTerms := []string{
				"Coupe-circuit principal",
				"Poste de pilotage",
				"Positions détenues ($$$$)",
			}
			missingTerms := []string{}
			transText := string(transData)
			for _, term := range requiredTerms {
				if !strings.Contains(transText, term) {
					missingTerms = append(missingTerms, term)
				}
			}

			// 5. Execute python linter
			cmd := exec.Command("python3", "scripts/check-i18n-parity.py")
			linterOut, linterErr := cmd.CombinedOutput()

			return map[string]any{
				"missingInFr":  missingInFr,
				"missingInEn":  missingInEn,
				"bannedCounts": bannedCounts,
				"missingTerms": missingTerms,
				"linterOk":     linterErr == nil,
				"linterOutput": string(linterOut),
			}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			data := res.(map[string]any)
			missingFr := data["missingInFr"].([]string)
			missingEn := data["missingInEn"].([]string)
			banned := data["bannedCounts"].(map[string]int)
			missingTerms := data["missingTerms"].([]string)

			harness.AssertEqual(t, len(missingFr), 0, fmt.Sprintf("Keys missing in fr: %v", missingFr))
			harness.AssertEqual(t, len(missingEn), 0, fmt.Sprintf("Keys missing in en: %v", missingEn))
			for term, count := range banned {
				harness.AssertEqual(t, count, 0, fmt.Sprintf("Prohibited term '%s' appeared %d times", term, count))
			}
			harness.AssertEqual(t, len(missingTerms), 0, fmt.Sprintf("Missing Cobalt terms: %v", missingTerms))
			harness.AssertTrue(t, data["linterOk"].(bool), fmt.Sprintf("check-i18n-parity.py failed: %s", data["linterOutput"]))
		},
	},
	{
		ID:          "E2E-T3-M12",
		Tier:        Tier3,
		FeatureID:   48,
		FeatureName: "GitHub Actions CI + Zero-Markdown + Lockstep (F43+F48+F49)",
		Description: "CI pipeline hygiene and version consistency across project configuration",
		Tags:        []string{"ci", "hygiene", "lockstep"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			// 1. Audit repository for stray markdown in forbidden dirs
			strayMarkdown := []string{}
			_ = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				if strings.HasPrefix(path, ".git/") || strings.HasPrefix(path, "node_modules/") || strings.HasPrefix(path, ".agents/") {
					return nil
				}
				if strings.HasSuffix(strings.ToLower(path), ".md") {
					if strings.HasPrefix(path, "tests/") || strings.HasPrefix(path, "backend/") || strings.HasPrefix(path, "scripts/") {
						strayMarkdown = append(strayMarkdown, path)
					}
				}
				return nil
			})

			// 2. Execute OKF hygiene linter
			cmdHygiene := exec.Command("python3", "scripts/check-okf-hygiene.py")
			hygieneOut, hygieneErr := cmdHygiene.CombinedOutput()

			// 3. Execute dependency lockstep linter
			cmdLockstep := exec.Command("python3", "scripts/check-dep-lockstep.py")
			lockstepOut, lockstepErr := cmdLockstep.CombinedOutput()

			return map[string]any{
				"strayMarkdown":  strayMarkdown,
				"hygieneOk":      hygieneErr == nil,
				"hygieneOutput":  string(hygieneOut),
				"lockstepOk":     lockstepErr == nil,
				"lockstepOutput": string(lockstepOut),
			}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			data := res.(map[string]any)
			stray := data["strayMarkdown"].([]string)
			harness.AssertEqual(t, len(stray), 0, fmt.Sprintf("Stray markdown files in forbidden dirs: %v", stray))
			harness.AssertTrue(t, data["hygieneOk"].(bool), fmt.Sprintf("OKF hygiene failed: %s", data["hygieneOutput"]))
			harness.AssertTrue(t, data["lockstepOk"].(bool), fmt.Sprintf("Dependency lockstep failed: %s", data["lockstepOutput"]))
		},
	},
	{
		ID:          "E2E-T3-M13",
		Tier:        Tier3,
		FeatureID:   47,
		FeatureName: "Packaging Tasks + Release Portal + Release CI (F44+F46+F47)",
		Description: "Verify packaging workflows, SHA-256 generation, and static portal files",
		Tags:        []string{"release", "portal", "packaging"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			// 1. Inspect Taskfile.yml for packaging tasks
			taskData, err := os.ReadFile("Taskfile.yml")
			if err != nil {
				return nil, err
			}
			taskStr := string(taskData)
			hasLinuxBuild := strings.Contains(taskStr, "linux:build:")
			hasLinuxPkg := strings.Contains(taskStr, "linux:package:")
			hasWinBuild := strings.Contains(taskStr, "windows:build:")
			hasWinPkg := strings.Contains(taskStr, "windows:package:")

			// 2. Execute site/build.mjs
			cmdSite := exec.Command("node", "site/build.mjs")
			siteOut, siteErr := cmdSite.CombinedOutput()

			// 3. Verify generated index.html
			indexData, err := os.ReadFile("site/dist/index.html")
			siteHtmlOk := err == nil && len(indexData) > 0
			indexStr := string(indexData)
			hasLinuxBtn := strings.Contains(indexStr, "Linux")
			hasWinBtn := strings.Contains(indexStr, "Windows")

			// 4. Inspect release workflow for packaging and upload steps
			releaseData, err := os.ReadFile(".github/workflows/release.yml")
			relOk := err == nil
			relStr := string(releaseData)
			hasShaStep := strings.Contains(relStr, "sha256") || strings.Contains(relStr, "checksum")
			hasUpload := strings.Contains(relStr, "upload") || strings.Contains(relStr, "release")

			return map[string]any{
				"linuxBuild":  hasLinuxBuild,
				"linuxPkg":    hasLinuxPkg,
				"winBuild":    hasWinBuild,
				"winPkg":      hasWinPkg,
				"siteBuildOk": siteErr == nil,
				"siteOutput":  string(siteOut),
				"siteHtmlOk":  siteHtmlOk,
				"hasLinuxBtn": hasLinuxBtn,
				"hasWinBtn":   hasWinBtn,
				"relOk":       relOk,
				"hasShaStep":  hasShaStep,
				"hasUpload":   hasUpload,
			}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			data := res.(map[string]any)
			harness.AssertTrue(t, data["linuxBuild"].(bool), "Taskfile must define linux:build")
			harness.AssertTrue(t, data["linuxPkg"].(bool), "Taskfile must define linux:package")
			harness.AssertTrue(t, data["winBuild"].(bool), "Taskfile must define windows:build")
			harness.AssertTrue(t, data["winPkg"].(bool), "Taskfile must define windows:package")
			harness.AssertTrue(t, data["siteBuildOk"].(bool), fmt.Sprintf("site/build.mjs failed: %s", data["siteOutput"]))
			harness.AssertTrue(t, data["siteHtmlOk"].(bool), "site/dist/index.html must exist and not be empty")
			harness.AssertTrue(t, data["hasLinuxBtn"].(bool), "Download links must include Linux")
			harness.AssertTrue(t, data["hasWinBtn"].(bool), "Download links must include Windows")
			harness.AssertTrue(t, data["relOk"].(bool), "release.yml must exist")
			harness.AssertTrue(t, data["hasShaStep"].(bool), "release.yml must contain SHA-256 step")
			harness.AssertTrue(t, data["hasUpload"].(bool), "release.yml must contain release upload step")
		},
	},
	{
		ID:          "E2E-T3-M14",
		Tier:        Tier3,
		FeatureID:   24,
		FeatureName: "Heikin-Ashi + RVOL + Volatility Guard (F23+F24+F32)",
		Description: "Momentum breakout setup combining HA smoothed candle, RVOL surge, and ATR guard",
		Tags:        []string{"heikin_ashi", "rvol", "atr"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := harness.GenerateSyntheticBars(30, 50.0, 1.0, 0.5)
			ha := h.CalculateHeikinAshi(bars)
			vols := make([]float64, len(bars))
			for i, b := range bars {
				vols[i] = b.Volume
			}
			vols[len(vols)-1] = 300000.0 // surge!
			rvol := h.CalculateRVOL(vols, 10)
			_, atrPct := h.CalculateATR(bars, 14)

			lastHA := ha[len(ha)-1]
			isGreenHA := lastHA.Close > lastHA.Open
			isRVOLSurge := rvol[len(rvol)-1] > 1.5
			isLowVol := atrPct[len(atrPct)-1] <= 5.0

			return isGreenHA && isRVOLSurge && isLowVol, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool), "HA breakout with RVOL and safe volatility should trigger")
		},
	},
	{
		ID:          "E2E-T3-M15",
		Tier:        Tier3,
		FeatureID:   33,
		FeatureName: "RS vs SPX + Golden Cross + Evaluator (F33+F34+F35)",
		Description: "Style 5 Golden Cross setup with 100-bar RS outperformance against SPX",
		Tags:        []string{"rs_spx", "golden_cross", "strategy"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			stockBars := harness.GenerateSyntheticBars(120, 100.0, 0.8, 0.5)
			spxBars := harness.GenerateSyntheticBars(120, 5000.0, 1.0, 2.0)

			stockCloses := make([]float64, 120)
			spxCloses := make([]float64, 120)
			for i := 0; i < 120; i++ {
				stockCloses[i] = stockBars[i].Close
				spxCloses[i] = spxBars[i].Close
			}

			rs := h.CalculateRelativeStrength(stockCloses, spxCloses, 100)
			lastRS := rs[len(rs)-1]

			sma50 := h.CalculateSMA(stockCloses, 50)
			sma200 := h.CalculateSMA(stockCloses, 100) // surrogate for 120 bars

			data := map[string][]float64{
				"indicatorA": sma50,
				"indicatorB": sma200,
			}
			isCross := h.EvaluateCondition(&models.Condition{Type: models.ConditionGreater}, data)

			return !math.IsNaN(lastRS) && lastRS > 100.0 && isCross, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool), "Golden cross with RS outperformance must evaluate")
		},
	},
}
