package e2e

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tws_traderbot/backend/bot"
	"tws_traderbot/backend/db"
	"tws_traderbot/backend/models"
	"tws_traderbot/backend/strategy"

	"github.com/scmhub/ibapi"
)

// [BUG-GUARD: BUG-1] Concurrent SQLite writes using raw db.Connect (production pool settings) must not fail with SQLITE_BUSY.
func TestV2_Tier5_SQLiteConcurrentWritesNoBusy(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tier5_concurrent.sqlite")
	database, err := db.Connect(dbPath)
	if err != nil {
		t.Fatalf("db.Connect: %v", err)
	}
	defer database.Close()
	if err := strategy.SeedStrategyCatalog(database); err != nil {
		t.Fatalf("SeedStrategyCatalog: %v", err)
	}

	eng := bot.NewEngine()
	eng.AttachDB(database)

	var errCount atomic.Int64
	var firstErrMu sync.Mutex
	var firstErr error
	recordErr := func(err error) {
		if err != nil {
			errCount.Add(1)
			firstErrMu.Lock()
			if firstErr == nil {
				firstErr = err
			}
			firstErrMu.Unlock()
		}
	}

	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				switch id {
				case 0:
					recordErr(database.SyncLivePositions([]models.BrokerPosition{
						{Symbol: "GOOGL", Venue: "NASDAQ", InstrumentID: "GOOGL.NASDAQ", Currency: "USD", Position: float64(10 + i)},
					}))
				case 1:
					recordErr(database.MoveStrategyOrder("cours_weinstein_p2", "up"))
				case 2:
					recordErr(database.ToggleStrategyPin("style_01"))
				case 3:
					eng.PersistCapturedAccount(fmt.Sprintf("DU%d", i))
					_ = eng.BuildRuntimeState(true)
				}
			}
		}(g)
	}
	wg.Wait()

	if errCount.Load() > 0 {
		t.Fatalf("expected 0 SQLITE_BUSY/write errors under concurrent DB access, got %d (first: %v)", errCount.Load(), firstErr)
	}
}

// [BUG-GUARD: BUG-2] IBKRClient connect/disconnect/reconnect with concurrent IsConnected polling must pass -race cleanly.
func TestV2_Tier5_IBKRClient_ConnectDisconnectReconnectRace(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen failed: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				hdr := make([]byte, 8)
				if _, err := io.ReadFull(c, hdr); err != nil {
					return
				}
				vLen := binary.BigEndian.Uint32(hdr[4:8])
				if vLen > 0 && vLen < 4096 {
					vBuf := make([]byte, vLen)
					_, _ = io.ReadFull(c, vBuf)
				}
				payload := []byte("176\x0020261117 10:00:00 EST\x00")
				frame := make([]byte, 4+len(payload))
				binary.BigEndian.PutUint32(frame[:4], uint32(len(payload)))
				copy(frame[4:], payload)
				if _, err := c.Write(frame); err != nil {
					return
				}
				_, _ = io.Copy(io.Discard, c)
			}(conn)
		}
	}()

	_, engine, client, _ := newV2TestEnv(t)
	addr := ln.Addr().(*net.TCPAddr)

	stopPoll := make(chan struct{})
	var pollWg sync.WaitGroup
	for i := 0; i < 3; i++ {
		pollWg.Add(1)
		go func() {
			defer pollWg.Done()
			for {
				select {
				case <-stopPoll:
					return
				default:
					_ = client.IsConnected()
					_ = engine.BuildRuntimeState(client.IsConnected())
					time.Sleep(time.Millisecond)
				}
			}
		}()
	}

	for cycle := 0; cycle < 3; cycle++ {
		if err := client.Connect("127.0.0.1", addr.Port, 10+cycle); err != nil {
			close(stopPoll)
			pollWg.Wait()
			t.Fatalf("cycle %d Connect failed: %v", cycle, err)
		}
		if !client.IsConnected() {
			close(stopPoll)
			pollWg.Wait()
			t.Fatalf("cycle %d expected IsConnected=true after Connect", cycle)
		}
		client.Disconnect()
		if client.IsConnected() {
			close(stopPoll)
			pollWg.Wait()
			t.Fatalf("cycle %d expected IsConnected=false after Disconnect", cycle)
		}
	}

	close(stopPoll)
	pollWg.Wait()
}

// [BUG-GUARD: BUG-3] Switching workspace/slot via SetCockpitState while paused must update RuntimeState/Diagnostics/DryRun strategy.
func TestV2_Tier5_ActiveStrategyFollowsCockpitWhenPaused(t *testing.T) {
	database, engine, _, _ := newV2TestEnv(t)

	initial, err := database.LoadStrategyPreset("screener_10_30_haut")
	if err != nil {
		t.Fatalf("LoadStrategyPreset: %v", err)
	}
	engine.SetActiveStrategy(initial) // simulates ServiceStartup hydration

	cs := engine.GetCockpitState()
	cs.ActiveWorkspaceID = "swing"
	for i := range cs.Workspaces {
		ws := &cs.Workspaces[i]
		ws.Enabled = ws.ID == "swing"
		if len(ws.StrategySlots) > 0 {
			ws.StrategySlots[0].Enabled = ws.Enabled
			if ws.Enabled {
				ws.StrategySlots[0].StrategyID = "cours_weinstein_p2"
			}
		}
	}
	engine.SetCockpitState(cs)

	rt := engine.BuildRuntimeState(false)
	if rt.ActiveStrategyID != "cours_weinstein_p2" {
		t.Errorf("BuildRuntimeState stale ActiveStrategyID=%q, want cours_weinstein_p2", rt.ActiveStrategyID)
	}
	diag, err := engine.BuildDiagnostics()
	if err != nil {
		t.Fatalf("BuildDiagnostics: %v", err)
	}
	if diag.ActiveStrategyID != "cours_weinstein_p2" {
		t.Errorf("BuildDiagnostics stale ActiveStrategyID=%q, want cours_weinstein_p2", diag.ActiveStrategyID)
	}
	dr, err := engine.RunDryRun(false)
	if err != nil {
		t.Fatalf("RunDryRun: %v", err)
	}
	if !strings.Contains(strings.ToLower(dr.Strategy), "weinstein") {
		t.Errorf("RunDryRun stale Strategy=%q, want Weinstein", dr.Strategy)
	}
}

// [BUG-GUARD: BUG-4] RunDryRun evaluates BarFrameBundle keyed by InstrumentID (matching Subscriptions).
func TestV2_Tier5_RunDryRun_BundleKeyedByInstrumentID(t *testing.T) {
	database, engine, _, _ := newV2TestEnv(t)

	_, err := database.MergeAndSaveWatchlist("wl-inst-id", "Inst ID WL", "", nil, []models.WatchlistGroup{
		{
			ID:   "core",
			Name: "Core",
			Items: []models.WatchlistItem{
				{Symbol: "NVDA", Exchange: "NASDAQ", Enabled: true, SizeMode: "shares", SizeValue: 10},
			},
		},
	}, false)
	if err != nil {
		t.Fatalf("MergeAndSaveWatchlist: %v", err)
	}

	engine.SetEquity(100000.0)
	engine.SetTimeProvider(func() time.Time {
		return time.Date(2026, 11, 18, 10, 30, 0, 0, time.FixedZone("EST", -5*3600))
	})
	strat := &models.Strategy{
		ID:   "strat_inst_key",
		Name: "Inst Key Price > 100",
		Rules: []models.Rule{{
			ID: "r1", Name: "R1", Scope: models.ScopeGlobal, Action: models.ActionBuy, Enabled: true,
			Condition: &models.Condition{
				Type:       models.ConditionGreater,
				Threshold:  100.0,
				IndicatorA: &models.Indicator{Type: models.IndicatorPrice, Timeframe: models.TimeframeD1},
			},
		}},
	}
	_ = database.SaveStrategyPreset(strat, &models.CockpitStrategySummary{ID: strat.ID, Name: strat.Name, RuleCount: 1, EnabledRuleCount: 1})

	cs := engine.GetCockpitState()
	cs.GlobalEnabled = true
	cs.ActiveWorkspaceID = "achat"
	cs.Workspaces[0].Enabled = true
	cs.Workspaces[0].WatchlistID = "wl-inst-id"
	cs.Workspaces[0].StrategySlots[0].StrategyID = strat.ID
	cs.Workspaces[0].StrategySlots[0].Enabled = true
	engine.SetCockpitState(cs)

	// Populate bundle using InstrumentID (exact key emitted in Subscriptions)
	engine.GetBarBundle().AddBar("NVDA.NASDAQ", models.TimeframeD1, 1700000000, 120, 125, 119, 124.0, 10000)

	dr, err := engine.RunDryRun(true)
	if err != nil {
		t.Fatalf("RunDryRun: %v", err)
	}
	if len(dr.PlannedOrders) != 1 || dr.PlannedOrders[0]["instrument_id"] != "NVDA.NASDAQ" {
		t.Fatalf("expected 1 planned order for NVDA.NASDAQ when bundle keyed by InstrumentID, got %+v", dr.PlannedOrders)
	}
}

// [BUG-GUARD: BUG-5] SyncLivePositions maps ISLAND->NASDAQ and IBWrapper maps AEQS->NEO consistently.
func TestV2_Tier5_SyncLivePositions_ISLANDAndAEQSCanonicalization(t *testing.T) {
	database, engine, _, w := newV2TestEnv(t)

	w.Position("DU1", &ibapi.Contract{
		ConID: 272093, Symbol: "MSFT", SecType: "STK", Exchange: "SMART", PrimaryExchange: "ISLAND", Currency: "USD",
	}, ibapi.StringToDecimal("10"), 420.0)
	w.Position("DU1", &ibapi.Contract{
		ConID: 999001, Symbol: "ZSP", SecType: "STK", Exchange: "SMART", PrimaryExchange: "AEQS", Currency: "CAD",
	}, ibapi.StringToDecimal("20"), 75.0)
	w.PositionEnd()

	for _, p := range engine.GetOpenPositions() {
		if p.Symbol == "ZSP" && p.InstrumentID != "ZSP.NEO" {
			t.Errorf("expected in-memory AEQS position canonicalized to ZSP.NEO, got %q", p.InstrumentID)
		}
	}

	ct, err := database.GetWatchlistByID("cobalt-telegram")
	if err != nil {
		t.Fatalf("GetWatchlistByID: %v", err)
	}
	for _, g := range ct.Groups {
		for _, it := range g.Items {
			if it.InstrumentID == "MSFT.NASDAQ" && !it.IsHolding {
				t.Errorf("expected MSFT.NASDAQ flagged is_holding=true for ISLAND primary exchange holding")
			}
		}
	}
}

// [BUG-GUARD: BUG-6] ManagedAccounts handles comma-delimited entries and SetBaseCurrency prioritizes configured base currency.
func TestV2_Tier5_ManagedAccountsCSVAndBaseCurrencyPriority(t *testing.T) {
	_, engine, _, w := newV2TestEnv(t)

	w.ManagedAccounts([]string{"DU111222,DU333444"})
	if got := engine.GetAccountID(); got != "DU111222" {
		t.Errorf("expected first comma-split account 'DU111222', got %q", got)
	}

	engine.SetBaseCurrency("CAD")
	w.AccountSummary(1, "DU111222", "NetLiquidation", "715000.00", "USD")
	w.AccountSummary(1, "DU111222", "NetLiquidation", "998844.93", "CAD")
	w.AccountSummary(1, "DU111222", "NetLiquidation", "720000.00", "USD")
	eq, _, _, _, _ := engine.GetAccountTelemetry()
	if eq != 998844.93 {
		t.Errorf("expected configured base currency CAD (998844.93) to win over USD, got %.2f", eq)
	}
}

// [BUG-GUARD: BUG-7] UpdatePortfolio(pos=0) syncs SQLite holdings and OrderStatus-before-OpenOrder emits no empty-symbol ghost order.
func TestV2_Tier5_UpdatePortfolioZeroSyncsDBAndOrderStatusMerge(t *testing.T) {
	database, engine, _, w := newV2TestEnv(t)

	// 1. OrderStatus before OpenOrder must NOT surface an empty-symbol ghost order
	w.OrderStatus(101, "Submitted", ibapi.ZERO, ibapi.StringToDecimal("10"), 0, 1, 0, 0, 1, "", 0)
	if ords := engine.GetOpenOrders(); len(ords) != 0 {
		t.Fatalf("expected 0 unhydrated ghost orders before OpenOrder arrives, got %+v", ords)
	}
	ord := ibapi.NewOrder()
	ord.Action = "BUY"
	ord.OrderType = "LMT"
	ord.TotalQuantity = ibapi.StringToDecimal("10")
	ord.LmtPrice = 180.50
	w.OpenOrder(101, &ibapi.Contract{Symbol: "NVDA", SecType: "STK", Exchange: "SMART", PrimaryExchange: "NASDAQ", Currency: "USD"}, ord, &ibapi.OrderState{Status: ""})
	ords := engine.GetOpenOrders()
	if len(ords) != 1 || ords[0].Symbol != "NVDA" || ords[0].RemainingQty != 10 || ords[0].Status != "Submitted" {
		t.Fatalf("expected merged hydrated order after OpenOrder, got %+v", ords)
	}

	// 2. UpdatePortfolio(pos=0) after hydration must clear SQLite positions-detenues
	googl := &ibapi.Contract{ConID: 208813720, Symbol: "GOOGL", SecType: "STK", Exchange: "SMART", PrimaryExchange: "NASDAQ", Currency: "USD"}
	w.Position("DU1", googl, ibapi.StringToDecimal("5"), 175.0)
	w.PositionEnd()

	pdBefore, _ := database.GetWatchlistByID("positions-detenues")
	if len(pdBefore.Symbols) != 1 {
		t.Fatalf("expected 1 holding before flat UpdatePortfolio, got %v", pdBefore.Symbols)
	}

	w.UpdatePortfolio(googl, ibapi.ZERO, 175.0, 0, 175.0, 0, 0, "DU1")
	pdAfter, _ := database.GetWatchlistByID("positions-detenues")
	if len(pdAfter.Symbols) != 0 {
		t.Fatalf("expected positions-detenues cleared after UpdatePortfolio(pos=0), got %v", pdAfter.Symbols)
	}
}

// [BUG-GUARD: BUG-8] ConditionWithin converts now to America/New_York and ValidateStrategy rejects nil IndicatorA & unknown ConditionType.
func TestV2_Tier5_ConditionWithinETAndValidateStrategyMalformed(t *testing.T) {
	// 19:00 UTC = 14:00 EST -> must fall within 09:30..16:00 ET
	rule := &models.Rule{
		ID:      "r_time",
		Name:    "Market Hours",
		Enabled: true,
		Action:  models.ActionFilter,
		Condition: &models.Condition{
			Type:       models.ConditionWithin,
			RangeStart: "09:30",
			RangeEnd:   "16:00",
		},
	}
	utcTime := time.Date(2026, 11, 18, 19, 0, 0, 0, time.UTC)
	if !strategy.EvaluateRule(rule, strategy.NewBarFrameBundle(), "AAPL", utcTime) {
		t.Errorf("expected 19:00 UTC (14:00 EST) to pass ConditionWithin 09:30..16:00 ET")
	}

	// Malformed ValidateStrategy cases
	badCases := []struct {
		name  string
		strat *models.Strategy
	}{
		{
			name: "missing IndicatorA on ConditionGreater",
			strat: &models.Strategy{
				Name: "Missing IndicatorA",
				Rules: []models.Rule{{
					Name:      "R1",
					Action:    models.ActionBuy,
					Condition: &models.Condition{Type: models.ConditionGreater, Threshold: 10},
				}},
			},
		},
		{
			name: "unknown ConditionType",
			strat: &models.Strategy{
				Name: "Unknown CondType",
				Rules: []models.Rule{{
					Name:   "R1",
					Action: models.ActionBuy,
					Condition: &models.Condition{
						Type:       models.ConditionType("does_not_exist"),
						IndicatorA: &models.Indicator{Type: models.IndicatorPrice, Timeframe: models.TimeframeD1},
					},
				}},
			},
		},
	}
	for _, tc := range badCases {
		res := models.ValidateStrategy(tc.strat)
		if res == nil || res.Valid || len(res.Errors) == 0 {
			t.Errorf("%s: expected Valid=false with errors, got %+v", tc.name, res)
		}
	}
}

// [BUG-GUARD: BUG-9] Daily loss breaker trip must persist GlobalEnabled=false to SQLite cockpit_state.
func TestV2_Tier5_DailyLossBreakerPersistsGlobalDisabled(t *testing.T) {
	database, engine, _, w := newV2TestEnv(t)

	cs := engine.GetCockpitState()
	cs.GlobalEnabled = true
	if err := database.SaveCockpitState(cs); err != nil {
		t.Fatalf("SaveCockpitState: %v", err)
	}
	engine.SetCockpitState(cs)
	engine.SetDailyLossLimit(500.0)

	w.Pnl(1, -600.0, -600.0, 0.0)

	if engine.IsGlobalEnabled() || !engine.RiskBreakerTripped.Load() {
		t.Fatalf("expected breaker tripped and engine disarmed")
	}
	persisted, err := database.LoadCockpitState()
	if err != nil {
		t.Fatalf("LoadCockpitState: %v", err)
	}
	if persisted.GlobalEnabled {
		t.Errorf("expected persisted cockpit_state.global_enabled=false after breaker trip, got true")
	}
}

// Hardening coverage: Concurrent polling/telemetry storm (BuildRuntimeState + BuildDiagnostics + RunDryRun + BrokerAlerts 25-cap & dedup).
func TestV2_Tier5_ConcurrentTelemetryStormAndAlertCap(t *testing.T) {
	database, engine, _, w := newV2TestEnv(t)
	engine.SetEquity(100000.0)
	engine.GetBarBundle().AddBar("AAPL", models.TimeframeD1, 1700000000, 150, 155, 149, 154.0, 10000)

	var wg sync.WaitGroup
	// Mutators: alerts (40 distinct + duplicates), PnL, positions, SetCockpitState
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 40; i++ {
			w.Error(int64(i), 0, 10089, fmt.Sprintf("Alert %d", i), "")
			w.Error(int64(i), 0, 10089, fmt.Sprintf("Alert %d", i), "") // consecutive duplicate
			w.Pnl(1, float64(i), float64(i), 0)
			if i%10 == 0 {
				_ = database.ToggleStrategyPin("style_01")
			}
		}
	}()

	// Readers: BuildRuntimeState + BuildDiagnostics + RunDryRun
	for r := 0; r < 3; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				rt := engine.BuildRuntimeState(true)
				if rt == nil {
					t.Errorf("BuildRuntimeState nil")
					return
				}
				if _, err := engine.BuildDiagnostics(); err != nil {
					t.Errorf("BuildDiagnostics error: %v", err)
					return
				}
				if i%4 == 0 {
					dr, err := engine.RunDryRun(true)
					if err != nil || dr == nil {
						t.Errorf("RunDryRun error: %v", err)
						return
					}
				}
			}
		}()
	}
	wg.Wait()

	alerts := engine.GetBrokerAlerts()
	if len(alerts) != 25 {
		t.Fatalf("expected BrokerAlerts capped at 25 after 40 distinct alerts, got %d", len(alerts))
	}
	if alerts[0].Message != "Alert 15" || alerts[24].Message != "Alert 39" {
		t.Errorf("unexpected FIFO window in capped BrokerAlerts: first=%q last=%q", alerts[0].Message, alerts[24].Message)
	}

	// Verify LastDryRun copy isolation
	snap := engine.GetLastDryRun()
	if snap == nil {
		t.Fatal("expected non-nil LastDryRun snapshot")
	}
	snap["strategy"] = "MUTATED_BY_CALLER"
	snap2 := engine.GetLastDryRun()
	if snap2["strategy"] == "MUTATED_BY_CALLER" {
		t.Errorf("GetLastDryRun did not isolate internal map from caller mutation")
	}
}

// Hardening coverage: Strategy stack pin -> move -> unpin -> move normalization and SaveStrategyPreset preservation.
func TestV2_Tier5_StrategyStackLifecycleAndBoundaries(t *testing.T) {
	database, _, _, _ := newV2TestEnv(t)

	if err := database.ToggleStrategyPin("cours_weinstein_p2"); err != nil {
		t.Fatalf("ToggleStrategyPin(pin): %v", err)
	}
	if err := database.ToggleStrategyPin("style_01"); err != nil {
		t.Fatalf("ToggleStrategyPin(pin style_01): %v", err)
	}
	if err := database.MoveStrategyOrder("style_01", "up"); err != nil {
		t.Fatalf("MoveStrategyOrder(up): %v", err)
	}
	lib, err := database.LoadStrategyLibrary()
	if err != nil || len(lib) < 2 {
		t.Fatalf("LoadStrategyLibrary: %v", err)
	}
	if lib[0].ID != "style_01" || !lib[0].Pinned {
		t.Errorf("expected style_01 first in pinned group, got %+v", lib[0])
	}

	// SaveStrategyPreset must preserve pinned & sort_order
	strat, err := database.LoadStrategyPreset("style_01")
	if err != nil {
		t.Fatalf("LoadStrategyPreset: %v", err)
	}
	strat.Name = "Style 01 Updated"
	if err := database.SaveStrategyPreset(strat, &models.CockpitStrategySummary{
		ID: strat.ID, Name: strat.Name, RuleCount: len(strat.Rules), EnabledRuleCount: len(strat.Rules),
	}); err != nil {
		t.Fatalf("SaveStrategyPreset: %v", err)
	}
	libAfterSave, _ := database.LoadStrategyLibrary()
	if libAfterSave[0].ID != "style_01" || !libAfterSave[0].Pinned || libAfterSave[0].Name != "Style 01 Updated" {
		t.Errorf("SaveStrategyPreset clobbered pinned/sort_order: %+v", libAfterSave[0])
	}

	// Unpin & move boundary (moving bottom unpinned or top pinned should not error/corrupt)
	if err := database.ToggleStrategyPin("style_01"); err != nil {
		t.Fatalf("ToggleStrategyPin(unpin): %v", err)
	}
	_ = database.MoveStrategyOrder("cours_weinstein_p2", "up")
	libFinal, _ := database.LoadStrategyLibrary()
	if libFinal[0].ID != "cours_weinstein_p2" || !libFinal[0].Pinned || libFinal[1].Pinned {
		t.Errorf("expected only cours_weinstein_p2 pinned after unpin, got %+v", libFinal[:2])
	}
}

// Hardening coverage: RunDryRun safety matrix (DryRun=false, breaker tripped, equity=0 percent sizing, missing data).
func TestV2_Tier5_RunDryRun_SafetyMatrix(t *testing.T) {
	database, engine, _, _ := newV2TestEnv(t)

	_, err := database.MergeAndSaveWatchlist("wl-pct", "Pct WL", "", nil, []models.WatchlistGroup{
		{
			ID:   "core",
			Name: "Core",
			Items: []models.WatchlistItem{
				{Symbol: "AAPL", Exchange: "NASDAQ", Enabled: true, SizeMode: "percent", SizeValue: 5.0},
			},
		},
	}, false)
	if err != nil {
		t.Fatalf("MergeAndSaveWatchlist: %v", err)
	}

	strat := &models.Strategy{
		ID:   "strat_safety_matrix",
		Name: "Safety Matrix",
		Rules: []models.Rule{{
			ID: "r1", Name: "Price > 100", Scope: models.ScopeGlobal, Action: models.ActionBuy, Enabled: true,
			Condition: &models.Condition{
				Type:       models.ConditionGreater,
				Threshold:  100.0,
				IndicatorA: &models.Indicator{Type: models.IndicatorPrice, Timeframe: models.TimeframeD1},
			},
		}},
	}
	_ = database.SaveStrategyPreset(strat, &models.CockpitStrategySummary{ID: strat.ID, Name: strat.Name, RuleCount: 1, EnabledRuleCount: 1})

	cs := engine.GetCockpitState()
	cs.GlobalEnabled = true
	cs.ActiveWorkspaceID = "achat"
	cs.Workspaces[0].Enabled = true
	cs.Workspaces[0].WatchlistID = "wl-pct"
	cs.Workspaces[0].StrategySlots[0].StrategyID = strat.ID
	cs.Workspaces[0].StrategySlots[0].Enabled = true
	engine.SetCockpitState(cs)
	_ = engine.StartBot(strat)
	engine.SetTimeProvider(func() time.Time {
		return time.Date(2026, 11, 18, 10, 30, 0, 0, time.FixedZone("EST", -5*3600))
	})

	// 1. Missing market data -> 0 planned orders, no panic
	drMissing, err := engine.RunDryRun(true)
	if err != nil || len(drMissing.PlannedOrders) != 0 {
		t.Fatalf("expected 0 planned orders on missing market data, got err=%v orders=%+v", err, drMissing.PlannedOrders)
	}

	// 2. Add bar, Equity=0 with percent sizing + DryRun=false -> Gate 9 Rejected, 0 live orders
	engine.SetDryRun(false)
	engine.SetEquity(0.0)
	engine.GetBarBundle().AddBar("AAPL", models.TimeframeD1, 1700000000, 149, 152, 148, 150.0, 10000)

	drZeroEq, err := engine.RunDryRun(true)
	if err != nil {
		t.Fatalf("RunDryRun(DryRun=false, Equity=0): %v", err)
	}
	if len(drZeroEq.PlannedOrders) != 1 || drZeroEq.PlannedOrders[0]["can_execute"].(bool) {
		t.Fatalf("expected Gate 9 Rejected when Equity=0 on percent sizing, got %+v", drZeroEq.PlannedOrders)
	}
	if !strings.Contains(drZeroEq.PlannedOrders[0]["gate_reason"].(string), "Gate 9") {
		t.Errorf("expected Gate 9 reason, got %v", drZeroEq.PlannedOrders[0]["gate_reason"])
	}

	// 3. Trip circuit breaker -> Gate 2 Rejected, 0 live trades recorded
	engine.SetEquity(100000.0)
	engine.UpdateLivePnL(-600.0, -600.0, 0.0)
	drTripped, err := engine.RunDryRun(true)
	if err != nil {
		t.Fatalf("RunDryRun(BreakerTripped): %v", err)
	}
	if len(drTripped.PlannedOrders) != 1 || drTripped.PlannedOrders[0]["can_execute"].(bool) {
		t.Fatalf("expected breaker rejection in PlannedOrders, got %+v", drTripped.PlannedOrders)
	}
	trades, err := database.GetRecentTrades(20)
	if err != nil {
		t.Fatalf("GetRecentTrades: %v", err)
	}
	if len(trades) != 0 {
		t.Errorf("RunDryRun must never insert trade_ledger records even when DryRun=false, got %+v", trades)
	}
}

// Hardening coverage: SessionGate DST transitions and exact boundary seconds.
func TestV2_Tier5_SessionGate_DSTAndExactBoundaries(t *testing.T) {
	sg := bot.NewSessionGate()
	loc := sg.Location()

	if !sg.IsRTH(time.Date(2026, 3, 9, 13, 30, 0, 0, time.UTC)) {
		t.Errorf("expected 2026-03-09 13:30 UTC (09:30 EDT post-spring-forward) to be RTH")
	}
	if sg.IsRTH(time.Date(2026, 11, 2, 13, 30, 0, 0, time.UTC)) {
		t.Errorf("expected 2026-11-02 13:30 UTC (08:30 EST post-fall-back) to be Pre-ETH, not RTH")
	}
	if !sg.IsRTH(time.Date(2026, 11, 2, 14, 30, 0, 0, time.UTC)) {
		t.Errorf("expected 2026-11-02 14:30 UTC (09:30 EST post-fall-back) to be RTH")
	}

	cases := []struct {
		h, m, s int
		want    bot.SessionType
	}{
		{3, 59, 59, bot.SessionClosed},
		{4, 0, 0, bot.SessionPreETH},
		{9, 29, 59, bot.SessionPreETH},
		{9, 30, 0, bot.SessionRTH},
		{15, 59, 59, bot.SessionRTH},
		{16, 0, 0, bot.SessionPostETH},
		{19, 59, 59, bot.SessionPostETH},
		{20, 0, 0, bot.SessionClosed},
	}
	for _, tc := range cases {
		ts := time.Date(2026, 11, 18, tc.h, tc.m, tc.s, 0, loc)
		if got := sg.GetSession(ts); got != tc.want {
			t.Errorf("%02d:%02d:%02d ET: expected %s, got %s", tc.h, tc.m, tc.s, tc.want, got)
		}
	}
}

// Hardening coverage: Dual-listed simultaneous holdings (CAD + USD) and empty<->non-empty holdings transitions.
func TestV2_Tier5_SyncLivePositions_DualListedAndTransitions(t *testing.T) {
	database, _, _, _ := newV2TestEnv(t)

	// Simultaneous AQN.TSX (CAD) and AQN.NYSE (USD) + BRK B dot-class
	if err := database.SyncLivePositions([]models.BrokerPosition{
		{Symbol: "AQN", Venue: "SMART", InstrumentID: "AQN.SMART", Currency: "CAD", Position: 100, AvgCost: 7.5},
		{Symbol: "AQN", Venue: "SMART", InstrumentID: "AQN.SMART", Currency: "USD", Position: 50, AvgCost: 5.5},
		{Symbol: "BRK B", Venue: "SMART", InstrumentID: "BRK B.SMART", Currency: "USD", Position: 2, AvgCost: 410.0},
	}); err != nil {
		t.Fatalf("SyncLivePositions(dual): %v", err)
	}

	pd, _ := database.GetWatchlistByID("positions-detenues")
	if len(pd.Symbols) != 3 {
		t.Fatalf("expected 3 distinct holdings [AQN.TSX, AQN.NYSE, BRK.B.NYSE], got %v", pd.Symbols)
	}

	// Transition to empty -> clears positions-detenues and all is_holding flags
	if err := database.SyncLivePositions(nil); err != nil {
		t.Fatalf("SyncLivePositions(nil): %v", err)
	}
	pdEmpty, _ := database.GetWatchlistByID("positions-detenues")
	if len(pdEmpty.Symbols) != 0 {
		t.Fatalf("expected 0 symbols after empty transition, got %v", pdEmpty.Symbols)
	}
	ct, _ := database.GetWatchlistByID("cobalt-telegram")
	for _, g := range ct.Groups {
		for _, it := range g.Items {
			if it.IsHolding {
				t.Errorf("expected is_holding=false after empty transition for %s", it.InstrumentID)
			}
		}
	}
}

// [DEFECT-1 & DEFECT-2 REGRESSION] Clean TCP FIN during Connect() returns error + disconnected;
// immediate post-handshake EOF + concurrent ManagedAccounts/Error(1102) Req* vs Disconnect passes -race.
func TestV2_Tier5_IBKRClient_HandshakeEOFAndPostHandshakeDropRace(t *testing.T) {
	// 1. Handshake clean TCP FIN EOF
	eofLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen failed: %v", err)
	}
	defer eofLn.Close()

	go func() {
		for {
			conn, err := eofLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				hdr := make([]byte, 8)
				if _, err := io.ReadFull(c, hdr); err == nil {
					vLen := binary.BigEndian.Uint32(hdr[4:8])
					if vLen > 0 && vLen < 4096 {
						_, _ = io.ReadFull(c, make([]byte, vLen))
					}
				}
				_ = c.Close()
			}(conn)
		}
	}()

	eofAddr := eofLn.Addr().(*net.TCPAddr)
	_, _, eofClient, _ := newV2TestEnv(t)
	if err := eofClient.Connect("127.0.0.1", eofAddr.Port, 1); err == nil {
		t.Fatalf("expected non-nil error on clean handshake FIN EOF, got nil (IsConnected=%v)", eofClient.IsConnected())
	}
	if eofClient.IsConnected() {
		t.Fatalf("expected IsConnected()==false after clean handshake FIN EOF")
	}

	// 2. Post-handshake immediate EOF + concurrent ManagedAccounts / Error(1102) / Disconnect under -race
	dropLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen failed: %v", err)
	}
	defer dropLn.Close()

	go func() {
		for {
			conn, err := dropLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				hdr := make([]byte, 8)
				if _, err := io.ReadFull(c, hdr); err != nil {
					return
				}
				vLen := binary.BigEndian.Uint32(hdr[4:8])
				if vLen > 0 && vLen < 4096 {
					_, _ = io.ReadFull(c, make([]byte, vLen))
				}
				payload := []byte("176\x0020261117 10:00:00 EST\x00")
				frame := make([]byte, 4+len(payload))
				binary.BigEndian.PutUint32(frame[:4], uint32(len(payload)))
				copy(frame[4:], payload)
				_, _ = c.Write(frame)
			}(conn)
		}
	}()

	dropAddr := dropLn.Addr().(*net.TCPAddr)
	_, _, dropClient, w := newV2TestEnv(t)
	for cycle := 0; cycle < 10; cycle++ {
		_ = dropClient.Connect("127.0.0.1", dropAddr.Port, 50+cycle)
		var wg sync.WaitGroup
		wg.Add(3)
		go func() {
			defer wg.Done()
			w.ManagedAccounts([]string{"DU555111"})
		}()
		go func() {
			defer wg.Done()
			w.Error(-1, 0, 1102, "Connectivity restored", "")
		}()
		go func() {
			defer wg.Done()
			dropClient.Disconnect()
		}()
		wg.Wait()
	}
}

// [DEFECT-3 REGRESSION] RunDryRun honors Engine.Now() (not wall clock) and ScopeSymbol bare symbol rules on InstrumentID-keyed bundles.
func TestV2_Tier5_RunDryRun_TimeProviderAndScopeSymbolOnInstrumentID(t *testing.T) {
	database, engine, _, _ := newV2TestEnv(t)

	_, err := database.MergeAndSaveWatchlist("wl-def3", "DEFECT-3 WL", "", nil, []models.WatchlistGroup{
		{
			ID:   "core",
			Name: "Core",
			Items: []models.WatchlistItem{
				{Symbol: "NVDA", Exchange: "NASDAQ", Enabled: true, SizeMode: "shares", SizeValue: 10},
				{Symbol: "AAPL", Exchange: "NASDAQ", Enabled: true, SizeMode: "shares", SizeValue: 10},
			},
		},
	}, false)
	if err != nil {
		t.Fatalf("MergeAndSaveWatchlist: %v", err)
	}

	engine.SetEquity(100000.0)
	strat := &models.Strategy{
		ID:   "strat_def3_time_and_scope",
		Name: "Time Window + ScopeSymbol",
		Rules: []models.Rule{
			{
				ID:      "r_hours_buy",
				Name:    "Hours",
				Scope:   models.ScopeGlobal,
				Action:  models.ActionBuy,
				Enabled: true,
				Condition: &models.Condition{
					Type:       models.ConditionWithin,
					RangeStart: "09:30",
					RangeEnd:   "16:00",
				},
			},
			{
				ID:      "r_nvda_symbol",
				Name:    "NVDA", // Bare symbol name matching NVDA.NASDAQ bundle key
				Scope:   models.ScopeSymbol,
				Action:  models.ActionBuy,
				Enabled: true,
				Condition: &models.Condition{
					Type:       models.ConditionGreater,
					Threshold:  100.0,
					IndicatorA: &models.Indicator{Type: models.IndicatorPrice, Timeframe: models.TimeframeD1},
				},
			},
		},
	}
	_ = database.SaveStrategyPreset(strat, &models.CockpitStrategySummary{
		ID: strat.ID, Name: strat.Name, RuleCount: 2, EnabledRuleCount: 2,
	})

	cs := engine.GetCockpitState()
	cs.GlobalEnabled = true
	cs.ActiveWorkspaceID = "achat"
	cs.Workspaces[0].Enabled = true
	cs.Workspaces[0].WatchlistID = "wl-def3"
	cs.Workspaces[0].StrategySlots[0].StrategyID = strat.ID
	cs.Workspaces[0].StrategySlots[0].Enabled = true
	engine.SetCockpitState(cs)

	// Populate InstrumentID-keyed frames for both NVDA.NASDAQ and AAPL.NASDAQ
	engine.GetBarBundle().AddBar("NVDA.NASDAQ", models.TimeframeD1, 1700000000, 120, 125, 119, 124.0, 10000)
	engine.GetBarBundle().AddBar("AAPL.NASDAQ", models.TimeframeD1, 1700000000, 180, 185, 179, 184.0, 10000)

	// Case A: Engine.Now() is Saturday 22:00 EST (outside ConditionWithin 09:30-16:00) even if wall clock is in RTH.
	// Disable r_nvda_symbol temporarily or check only Hours rule fires 0 orders when outside RTH.
	est := time.FixedZone("EST", -5*3600)
	engine.SetTimeProvider(func() time.Time {
		return time.Date(2026, 11, 21, 22, 0, 0, 0, est) // Saturday 22:00 EST
	})
	drOffHours, err := engine.RunDryRun(true)
	if err != nil {
		t.Fatalf("RunDryRun(off-hours): %v", err)
	}
	for _, ord := range drOffHours.PlannedOrders {
		if ord["rule"] == "Hours" {
			t.Fatalf("RunDryRun leaked wall clock: 'Hours' ConditionWithin fired at Saturday 22:00 EST: %+v", ord)
		}
	}
	// ScopeSymbol bare rule "NVDA" must fire ONLY for NVDA.NASDAQ (not AAPL.NASDAQ)
	if len(drOffHours.PlannedOrders) != 1 || drOffHours.PlannedOrders[0]["instrument_id"] != "NVDA.NASDAQ" || drOffHours.PlannedOrders[0]["rule"] != "NVDA" {
		t.Fatalf("expected exactly 1 ScopeSymbol planned order for NVDA.NASDAQ, got %+v", drOffHours.PlannedOrders)
	}

	// Case B: Engine.Now() is Wednesday 10:30 EST (inside ConditionWithin 09:30-16:00) -> 'Hours' must fire for both + 'NVDA' for NVDA.NASDAQ = 3 planned orders.
	engine.SetTimeProvider(func() time.Time {
		return time.Date(2026, 11, 18, 10, 30, 0, 0, est) // Wednesday 10:30 EST
	})
	drRTH, err := engine.RunDryRun(true)
	if err != nil {
		t.Fatalf("RunDryRun(RTH): %v", err)
	}
	if len(drRTH.PlannedOrders) != 3 {
		t.Fatalf("expected 3 planned orders in RTH (Hours x2 + ScopeSymbol NVDA x1), got %d: %+v", len(drRTH.PlannedOrders), drRTH.PlannedOrders)
	}
}
