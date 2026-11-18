package bot

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"tws_traderbot/backend/db"
	"tws_traderbot/backend/models"

	"github.com/scmhub/ibapi"
)

func TestNewEngine_ZeroDefaultEquityAndSessionFormat(t *testing.T) {
	eng := NewEngine()
	if eq := eng.GetEquity(); eq != 0 {
		t.Fatalf("expected default equity 0.0, got %v", eq)
	}
	eng.SetEquity(50000.0)
	if eq := eng.GetEquity(); eq != 50000.0 {
		t.Fatalf("expected equity 50000.0, got %v", eq)
	}
	eng.SetEquity(0)
	if eq := eng.GetEquity(); eq != 0 {
		t.Fatalf("expected SetEquity(0) to reset equity to 0.0, got %v", eq)
	}

	eng.SetTimeProvider(func() time.Time {
		return time.Date(2026, 11, 18, 10, 15, 0, 0, eng.sessionGate.Location())
	})
	status := eng.GetSessionStatus()
	if !strings.Contains(status, "NY/Toronto RTH") {
		t.Errorf("expected NY/Toronto RTH session status, got %q", status)
	}
}

func TestIBWrapper_FullTelemetryIngestionAndHoldingsSync(t *testing.T) {
	tmpDir := t.TempDir()
	database, err := db.Connect(filepath.Join(tmpDir, "telemetry.sqlite"))
	if err != nil {
		t.Fatalf("db.Connect failed: %v", err)
	}
	defer database.Close()

	eng := NewEngine()
	eng.AttachDB(database)
	wrapper := NewIBWrapper(eng)

	// 1. ManagedAccounts -> sets Engine AccountID and persists config.ib.account without clobbering host/port
	_ = database.SaveSystemConfig("config", map[string]interface{}{
		"ib": map[string]interface{}{
			"host":         "127.0.0.1",
			"port":         7497,
			"client_id":    1,
			"trading_mode": "paper",
		},
	})
	wrapper.ManagedAccounts([]string{"DU9876543"})
	if got := eng.GetAccountID(); got != "DU9876543" {
		t.Fatalf("expected AccountID DU9876543, got %q", got)
	}
	cfg, _ := database.LoadSystemConfig("config")
	ibCfg := cfg["ib"].(map[string]interface{})
	if ibCfg["account"] != "DU9876543" || ibCfg["host"] != "127.0.0.1" {
		t.Fatalf("expected persisted account preserving host, got %+v", ibCfg)
	}

	// 2. AccountSummary & UpdateAccountValue (native CashBalance vs TotalCashValue fallback)
	wrapper.AccountSummary(1, "DU9876543", "NetLiquidation", "125432.50", "USD")
	wrapper.AccountSummary(1, "DU9876543", "BuyingPower", "250000.00", "USD")
	wrapper.AccountSummary(1, "DU9876543", "TotalCashValue", "45000.00", "BASE")
	wrapper.UpdateAccountValue("CashBalance", "30000.25", "USD", "DU9876543")
	wrapper.UpdateAccountValue("CashBalance", "20500.75", "CAD", "DU9876543")
	wrapper.AccountSummaryEnd(1)

	eq, bp, usd, cad, base := eng.GetAccountTelemetry()
	if eq != 125432.50 || bp != 250000.00 || usd != 30000.25 || cad != 20500.75 || base != 45000.00 {
		t.Fatalf("unexpected account telemetry: eq=%v bp=%v usd=%v cad=%v base=%v", eq, bp, usd, cad, base)
	}

	// 3. Position + UpdatePortfolio ConID merge + PositionEnd -> DB sync
	googlCon := &ibapi.Contract{
		ConID:    208813720,
		Symbol:   "GOOGL",
		SecType:  "STK",
		Exchange: "SMART",
		Currency: "USD",
	}
	wrapper.Position("DU9876543", googlCon, ibapi.StringToDecimal("10"), 175.50)

	googlPortfolioCon := &ibapi.Contract{
		ConID:           208813720,
		Symbol:          "GOOGL",
		SecType:         "STK",
		PrimaryExchange: "NASDAQ",
		Currency:        "USD",
	}
	wrapper.UpdatePortfolio(googlPortfolioCon, ibapi.StringToDecimal("10"), 182.0, 1820.0, 175.50, 65.0, 0, "DU9876543")
	wrapper.PositionEnd()

	positions := eng.GetOpenPositions()
	if len(positions) != 1 {
		t.Fatalf("expected 1 open position, got %d", len(positions))
	}
	if positions[0].InstrumentID != "GOOGL.NASDAQ" || positions[0].MarketPrice != 182.0 || positions[0].UnrealizedPnL != 65.0 {
		t.Errorf("expected merged GOOGL.NASDAQ valuation, got %+v", positions[0])
	}

	pd, err := database.GetWatchlistByID("positions-detenues")
	if err != nil {
		t.Fatalf("GetWatchlistByID positions-detenues failed: %v", err)
	}
	if len(pd.Symbols) != 1 || pd.Symbols[0] != "GOOGL.NASDAQ" {
		t.Errorf("expected positions-detenues [GOOGL.NASDAQ], got %v", pd.Symbols)
	}

	// 4. OpenOrder + OrderStatus (UNSET_FLOAT limit price sanitization + terminal removal)
	ord := ibapi.NewOrder()
	ord.Action = "BUY"
	ord.OrderType = "MKT"
	ord.TotalQuantity = ibapi.StringToDecimal("5")
	wrapper.OpenOrder(42, googlPortfolioCon, ord, &ibapi.OrderState{Status: "Submitted"})
	wrapper.OpenOrderEnd()
	wrapper.OrderStatus(42, "PreSubmitted", ibapi.StringToDecimal("0"), ibapi.StringToDecimal("5"), 0, 1, 0, 0, 1, "", 0)

	openOrds := eng.GetOpenOrders()
	if len(openOrds) != 1 || openOrds[0].LimitPrice != 0 || openOrds[0].Status != "PreSubmitted" {
		t.Fatalf("unexpected open order state: %+v", openOrds)
	}
	wrapper.OrderStatus(42, "Filled", ibapi.StringToDecimal("5"), ibapi.StringToDecimal("0"), 181.25, 1, 0, 181.25, 1, "", 0)
	if len(eng.GetOpenOrders()) != 0 {
		t.Errorf("expected Filled order to be removed from openOrders")
	}

	// 5. Pnl (lowercase nl) + UNSET_FLOAT sanitization
	wrapper.Pnl(99, 120.50, 65.0, ibapi.UNSET_FLOAT)
	realized, unrealized, daily := eng.GetPnL()
	if daily != 120.50 || unrealized != 65.0 || realized != 0 {
		t.Errorf("unexpected PnL: real=%v unreal=%v daily=%v", realized, unrealized, daily)
	}

	// 6. Error alert mapping & suppression
	wrapper.Error(0, 0, 2104, "Market data farm connection is OK:usfarm", "")
	wrapper.Error(1, 0, 10089, "Requested market data requires additional subscription for API", "")
	wrapper.Error(0, 0, 504, "Not connected", "")
	alerts := eng.GetBrokerAlerts()
	if len(alerts) != 2 {
		t.Fatalf("expected 2 alerts (2104 suppressed), got %d: %+v", len(alerts), alerts)
	}
	if alerts[0].Code != 10089 || alerts[1].Code != 504 {
		t.Errorf("unexpected alert codes: %+v", alerts)
	}

	// 7. BuildRuntimeState & BuildDiagnostics reflect live state with zero fake stubs
	rt := eng.BuildRuntimeState(true)
	if rt.AccountID != "DU9876543" || rt.Equity != 125432.50 || rt.CashCAD != 20500.75 || rt.OpenPositionsCount != 1 {
		t.Errorf("unexpected BuildRuntimeState: %+v", rt)
	}
	diag, err := eng.BuildDiagnostics()
	if err != nil {
		t.Fatalf("BuildDiagnostics failed: %v", err)
	}
	if diag.Account != "DU9876543" || diag.Startup.Environment != "" || diag.Startup.LogFile != "" || diag.Symbols.Source != "" {
		t.Errorf("unexpected de-noised DiagnosticsResponse: %+v", diag)
	}

	// 8. ConnectionClosed resets telemetry while keeping alerts & disconnect reason
	wrapper.ConnectionClosed()
	if eng.GetEquity() != 0 || len(eng.GetOpenPositions()) != 0 {
		t.Errorf("expected equity & positions cleared on ConnectionClosed")
	}
	if len(eng.GetBrokerAlerts()) != 2 {
		t.Errorf("expected broker alerts preserved after ConnectionClosed")
	}
	discAt, discReason := eng.GetLastDisconnect()
	if discAt == "" || discReason == "" {
		t.Errorf("expected disconnect metadata recorded")
	}
}

func TestEngine_RunDryRun_DynamicEvaluation(t *testing.T) {
	tmpDir := t.TempDir()
	database, err := db.Connect(filepath.Join(tmpDir, "dryrun_eval.sqlite"))
	if err != nil {
		t.Fatalf("db.Connect failed: %v", err)
	}
	defer database.Close()

	eng := NewEngine()
	eng.AttachDB(database)
	eng.SetEquity(100000.0)
	eng.SetTimeProvider(func() time.Time {
		return time.Date(2026, 11, 18, 10, 30, 0, 0, eng.sessionGate.Location())
	})

	// Configure a watchlist bound to the swing workspace
	_, err = database.MergeAndSaveWatchlist("swing-focus", "Swing Focus", "", nil, []models.WatchlistGroup{
		{
			ID:   "core",
			Name: "Core",
			Items: []models.WatchlistItem{
				{Symbol: "NVDA", Exchange: "NASDAQ", Enabled: true, SizeMode: "shares", SizeValue: 10},
			},
		},
	}, false)
	if err != nil {
		t.Fatalf("MergeAndSaveWatchlist failed: %v", err)
	}

	strat := &models.Strategy{
		ID:   "strat_dryrun_test",
		Name: "DryRun Cross Test",
		Rules: []models.Rule{
			{
				ID:      "r_buy",
				Name:    "Price Above Threshold",
				Scope:   models.ScopeGlobal,
				Action:  models.ActionBuy,
				Enabled: true,
				Condition: &models.Condition{
					Type:      models.ConditionGreater,
					Threshold: 100.0,
					IndicatorA: &models.Indicator{
						Type:      models.IndicatorPrice,
						Timeframe: models.TimeframeM15,
					},
				},
			},
		},
	}
	_ = database.SaveStrategyPreset(strat, &models.CockpitStrategySummary{
		ID:               strat.ID,
		Name:             strat.Name,
		RuleCount:        1,
		EnabledRuleCount: 1,
	})

	cockpit := &models.CockpitStateResponse{
		GlobalEnabled:     false,
		ActiveWorkspaceID: "swing",
		Workspaces: []models.CockpitWorkspace{
			{
				ID:          "swing",
				Name:        "Swing",
				Kind:        "swing",
				Enabled:     false,
				WatchlistID: "swing-focus",
				StrategySlots: []models.CockpitStrategySlot{
					{ID: "slot_1", Label: "Slot 1", StrategyID: strat.ID, Enabled: false},
				},
			},
		},
	}
	eng.SetCockpitState(cockpit)
	eng.SetActiveStrategy(strat)

	// Feed 15m bar into Engine's BarFrameBundle
	eng.GetBarBundle().AddBar("NVDA", models.TimeframeM15, 1700000000, 120, 125, 119, 124.5, 10000)

	dr, err := eng.RunDryRun(true)
	if err != nil {
		t.Fatalf("RunDryRun failed: %v", err)
	}
	if dr.Strategy != "DryRun Cross Test" || dr.WorkspaceKind != "swing" {
		t.Errorf("unexpected DryRun metadata: strategy=%q kind=%q", dr.Strategy, dr.WorkspaceKind)
	}
	if len(dr.Subscriptions) != 1 || dr.Subscriptions[0].Symbol != "NVDA.NASDAQ" || dr.Subscriptions[0].Timeframe != "15m" {
		t.Errorf("unexpected subscriptions: %+v", dr.Subscriptions)
	}
	if len(dr.PlannedOrders) != 1 || dr.PlannedOrders[0]["symbol"] != "NVDA" || dr.PlannedOrders[0]["action"] != "BUY" {
		t.Errorf("expected 1 planned BUY order for NVDA, got %+v", dr.PlannedOrders)
	}
}

func TestEngine_ResetCircuitBreaker_ConcurrentWithUpdateLivePnL(t *testing.T) {
	eng := NewEngine()
	eng.SetDailyLossLimit(500.0)
	wrapper := NewIBWrapper(eng)

	const workers = 4
	const iters = 150
	var wg sync.WaitGroup

	// Writers mutating PnL & App concurrently with ResetCircuitBreaker & IBWrapper.Error
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				if i%10 == 0 {
					eng.UpdateLivePnL(-600.0, -400.0, -200.0)
				} else {
					eng.UpdateLivePnL(float64(id*10+i%50), 25.0, 10.0)
				}
				eng.AttachEmitter(nil)
			}
		}(w)

		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				_ = eng.ResetCircuitBreaker()
				_, _, _ = eng.GetPnL()
				wrapper.Error(1, 0, 10089, "Delayed market data warning", "")
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock detected running ResetCircuitBreaker concurrently with UpdateLivePnL")
	}

	// Deterministic post-concurrency breach check
	eng.UpdateLivePnL(-600.0, -600.0, 0)
	if err := eng.ResetCircuitBreaker(); err == nil {
		t.Errorf("expected ResetCircuitBreaker to reject while daily loss limit is breached")
	}
	eng.UpdateLivePnL(100.0, 100.0, 0)
	if err := eng.ResetCircuitBreaker(); err != nil {
		t.Errorf("expected ResetCircuitBreaker to succeed after PnL recovers, got: %v", err)
	}
}

func TestEngine_RunDryRun_ScopedBoundWatchlistItem(t *testing.T) {
	tmpDir := t.TempDir()
	database, err := db.Connect(filepath.Join(tmpDir, "dryrun_scoped.sqlite"))
	if err != nil {
		t.Fatalf("db.Connect failed: %v", err)
	}
	defer database.Close()

	// Suspend NVDA in cobalt-telegram (lower SQLite row ID)
	if _, err := database.Conn().Exec(`
		UPDATE watchlist_items
		SET enabled = FALSE
		WHERE watchlist_id = 'cobalt-telegram' AND symbol = 'NVDA'
	`); err != nil {
		t.Fatalf("failed to suspend NVDA in cobalt-telegram: %v", err)
	}

	// Create wl-swing-2 with NVDA enabled
	_, err = database.MergeAndSaveWatchlist("wl-swing-2", "Swing 2", "", nil, []models.WatchlistGroup{
		{
			ID:   "core",
			Name: "Core",
			Items: []models.WatchlistItem{
				{Symbol: "NVDA", Exchange: "NASDAQ", Enabled: true, SizeMode: "shares", SizeValue: 10},
			},
		},
	}, false)
	if err != nil {
		t.Fatalf("MergeAndSaveWatchlist failed: %v", err)
	}

	eng := NewEngine()
	eng.AttachDB(database)
	eng.SetEquity(100000.0)
	eng.SetTimeProvider(func() time.Time {
		return time.Date(2026, 11, 18, 10, 30, 0, 0, eng.sessionGate.Location())
	})

	strat := &models.Strategy{
		ID:   "strat_scoped_nvda",
		Name: "Scoped NVDA Swing",
		Rules: []models.Rule{
			{
				ID:      "r_buy",
				Name:    "Price > 100",
				Scope:   models.ScopeGlobal,
				Action:  models.ActionBuy,
				Enabled: true,
				Condition: &models.Condition{
					Type:      models.ConditionGreater,
					Threshold: 100.0,
					IndicatorA: &models.Indicator{
						Type:      models.IndicatorPrice,
						Timeframe: models.TimeframeD1,
					},
				},
			},
		},
	}
	_ = database.SaveStrategyPreset(strat, &models.CockpitStrategySummary{
		ID:               strat.ID,
		Name:             strat.Name,
		RuleCount:        1,
		EnabledRuleCount: 1,
	})

	cockpit := &models.CockpitStateResponse{
		GlobalEnabled:     true,
		ActiveWorkspaceID: "swing",
		Workspaces: []models.CockpitWorkspace{
			{
				ID:          "swing",
				Name:        "Swing",
				Kind:        "swing",
				Enabled:     true,
				WatchlistID: "wl-swing-2",
				StrategySlots: []models.CockpitStrategySlot{
					{ID: "slot_1", Label: "Slot 1", StrategyID: strat.ID, Enabled: true},
				},
			},
		},
	}
	eng.SetCockpitState(cockpit)
	if err := eng.StartBot(strat); err != nil {
		t.Fatalf("StartBot failed: %v", err)
	}
	eng.GetBarBundle().AddBar("NVDA", models.TimeframeD1, 1700000000, 140, 145, 139, 144.0, 10000)

	dr, err := eng.RunDryRun(true)
	if err != nil {
		t.Fatalf("RunDryRun failed: %v", err)
	}
	if len(dr.PlannedOrders) != 1 {
		t.Fatalf("expected 1 planned order, got %+v", dr.PlannedOrders)
	}
	if canExec, _ := dr.PlannedOrders[0]["can_execute"].(bool); !canExec {
		t.Fatalf("expected can_execute=true for enabled NVDA in bound wl-swing-2, got %+v", dr.PlannedOrders[0])
	}
	if reason, _ := dr.PlannedOrders[0]["gate_reason"].(string); reason != "OK" {
		t.Errorf("expected gate_reason=OK, got %q", reason)
	}

	// Verify CanExecuteWithItem still rejects disabled items directly
	disabledItem := &models.WatchlistItem{Symbol: "NVDA", Enabled: false, IsOrderable: true, SizeMode: "shares", SizeValue: 10}
	if ok, reason := eng.CanExecuteWithItem(disabledItem, models.ActionBuy, 144.0); ok || !strings.Contains(reason, "SUSPENDED") {
		t.Errorf("expected disabled item rejection, got ok=%v reason=%q", ok, reason)
	}
}

func TestIBWrapper_MultiCurrencyAccountSummaryAndUpdateAccountValuePriority(t *testing.T) {
	eng := NewEngine()
	wrapper := NewIBWrapper(eng)

	// 1. Primary AccountSummary arrives in CAD (base account currency), followed by USD secondary row
	wrapper.AccountSummary(1, "DUO185814", "NetLiquidation", "998844.93", "CAD")
	wrapper.AccountSummary(1, "DUO185814", "BuyingPower", "3223887.99", "CAD")
	wrapper.AccountSummary(1, "DUO185814", "NetLiquidation", "715000.00", "USD")
	wrapper.AccountSummary(1, "DUO185814", "BuyingPower", "2300000.00", "USD")

	eq, bp, _, _, _ := eng.GetAccountTelemetry()
	if eq != 998844.93 || bp != 3223887.99 {
		t.Fatalf("secondary USD row must not overwrite primary CAD Equity/BuyingPower: got eq=%v bp=%v", eq, bp)
	}

	// 2. UpdateAccountValue arrives with BASE -> overrides single-currency primary and resists subsequent CAD/USD rows
	wrapper.UpdateAccountValue("NetLiquidation", "1001234.50", "BASE", "DUO185814")
	wrapper.UpdateAccountValue("BuyingPower", "3300111.00", "BASE", "DUO185814")
	wrapper.UpdateAccountValue("NetLiquidation", "720000.00", "USD", "DUO185814")
	wrapper.UpdateAccountValue("BuyingPower", "2350000.00", "USD", "DUO185814")
	wrapper.UpdateAccountValue("NetLiquidation", "999999.00", "CAD", "DUO185814")
	wrapper.UpdateAccountValue("BuyingPower", "3100000.00", "CAD", "DUO185814")

	eq, bp, _, _, _ = eng.GetAccountTelemetry()
	if eq != 1001234.50 || bp != 3300111.00 {
		t.Fatalf("expected BASE Equity/BuyingPower preserved against CAD/USD updates: got eq=%v bp=%v", eq, bp)
	}

	// 3. Subsequent BASE update streams cleanly
	wrapper.UpdateAccountValue("NetLiquidation", "1002000.00", "BASE", "DUO185814")
	wrapper.UpdateAccountValue("BuyingPower", "3305000.00", "BASE", "DUO185814")
	eq, bp, _, _, _ = eng.GetAccountTelemetry()
	if eq != 1002000.00 || bp != 3305000.00 {
		t.Fatalf("expected subsequent BASE updates to apply: got eq=%v bp=%v", eq, bp)
	}

	// 4. Native UpdateAccountValue CashBalance BASE overrides TotalCashValue BASE and resists subsequent AccountSummary BASE fallback
	wrapper.AccountSummary(1, "DUO185814", "TotalCashValue", "30000.00", "BASE")
	_, _, _, _, cashBase := eng.GetAccountTelemetry()
	if cashBase != 30000.00 {
		t.Fatalf("expected initial TotalCashValue BASE fallback 30000.00, got %v", cashBase)
	}
	wrapper.UpdateAccountValue("CashBalance", "66666.00", "BASE", "DUO185814")
	wrapper.AccountSummary(1, "DUO185814", "TotalCashValue", "31000.00", "BASE")
	_, _, _, _, cashBase = eng.GetAccountTelemetry()
	if cashBase != 66666.00 {
		t.Fatalf("expected native CashBalance BASE (66666.00) preserved against AccountSummary TotalCashValue fallback, got %v", cashBase)
	}

	// 5. ConnectionClosed resets currency tracking and hasCashBase so next session starts clean
	wrapper.ConnectionClosed()
	wrapper.AccountSummary(2, "DUO185814", "NetLiquidation", "500000.00", "USD")
	wrapper.AccountSummary(2, "DUO185814", "BuyingPower", "1500000.00", "USD")
	wrapper.AccountSummary(2, "DUO185814", "TotalCashValue", "25000.00", "BASE")
	eq, bp, _, _, cashBase = eng.GetAccountTelemetry()
	if eq != 500000.00 || bp != 1500000.00 || cashBase != 25000.00 {
		t.Fatalf("expected reset currency tracking after ConnectionClosed: got eq=%v bp=%v cashBase=%v", eq, bp, cashBase)
	}
}

func TestEngine_ConcurrentStartBotVsBuildRuntimeStateAndDryRun(t *testing.T) {
	tmpDir := t.TempDir()
	database, err := db.Connect(filepath.Join(tmpDir, "concurrent_cockpit.sqlite"))
	if err != nil {
		t.Fatalf("db.Connect failed: %v", err)
	}
	defer database.Close()

	_, err = database.MergeAndSaveWatchlist("wl-concurrent", "Concurrent WL", "", nil, []models.WatchlistGroup{
		{
			ID:   "core",
			Name: "Core",
			Items: []models.WatchlistItem{
				{Symbol: "AAPL", Exchange: "NASDAQ", Enabled: true, SizeMode: "shares", SizeValue: 5},
			},
		},
	}, false)
	if err != nil {
		t.Fatalf("MergeAndSaveWatchlist failed: %v", err)
	}

	stratA := &models.Strategy{
		ID:   "strat_concurrent_a",
		Name: "Concurrent Strategy A",
		Rules: []models.Rule{
			{
				ID:      "r_buy",
				Name:    "Price > 100",
				Scope:   models.ScopeGlobal,
				Action:  models.ActionBuy,
				Enabled: true,
				Condition: &models.Condition{
					Type:      models.ConditionGreater,
					Threshold: 100.0,
					IndicatorA: &models.Indicator{
						Type:      models.IndicatorPrice,
						Timeframe: models.TimeframeD1,
					},
				},
			},
		},
	}
	stratB := &models.Strategy{
		ID:    "strat_concurrent_b",
		Name:  "Concurrent Strategy B",
		Rules: stratA.Rules,
	}
	_ = database.SaveStrategyPreset(stratA, &models.CockpitStrategySummary{
		ID: stratA.ID, Name: stratA.Name, RuleCount: 1, EnabledRuleCount: 1,
	})
	_ = database.SaveStrategyPreset(stratB, &models.CockpitStrategySummary{
		ID: stratB.ID, Name: stratB.Name, RuleCount: 1, EnabledRuleCount: 1,
	})

	eng := NewEngine()
	eng.AttachDB(database)
	eng.SetEquity(100000.0)
	eng.SetTimeProvider(func() time.Time {
		return time.Date(2026, 11, 18, 10, 30, 0, 0, eng.sessionGate.Location())
	})
	eng.GetBarBundle().AddBar("AAPL", models.TimeframeD1, 1700000000, 150, 155, 149, 154, 10000)

	makeCockpit := func(stratID string, enabled bool) *models.CockpitStateResponse {
		return &models.CockpitStateResponse{
			GlobalEnabled:     enabled,
			ActiveWorkspaceID: "achat",
			Workspaces: []models.CockpitWorkspace{
				{
					ID:          "achat",
					Name:        "Achat",
					Kind:        "long",
					Enabled:     true,
					WatchlistID: "wl-concurrent",
					StrategySlots: []models.CockpitStrategySlot{
						{ID: "slot_1", Label: "Slot 1", StrategyID: stratID, Enabled: true},
					},
				},
			},
			StrategyLibrary: []models.CockpitStrategySummary{
				{ID: stratA.ID, Name: stratA.Name, RuleCount: 1, EnabledRuleCount: 1},
				{ID: stratB.ID, Name: stratB.Name, RuleCount: 1, EnabledRuleCount: 1},
			},
			Feed: &models.WatchlistFeed{
				Provider: "manual",
				URL:      "local",
			},
		}
	}
	eng.SetCockpitState(makeCockpit(stratA.ID, true))

	const writers = 2
	const readers = 4
	const iters = 300
	var wg sync.WaitGroup

	// Writers hammering StartBot / StopBot / SetGlobalEnabled / SetCockpitState
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				if i%2 == 0 {
					_ = eng.StartBot(stratA)
				} else {
					_ = eng.StartBot(stratB)
				}
				eng.SetGlobalEnabled(i%3 != 0)
				cs := makeCockpit(stratA.ID, i%2 == 0)
				eng.SetCockpitState(cs)
				// Mutate caller's struct after SetCockpitState to verify clone isolation
				cs.Workspaces[0].StrategySlots[0].StrategyID = "mutated_after_set"
				eng.StopBot()
			}
		}(w)
	}

	// Readers hammering BuildRuntimeState / BuildDiagnostics / RunDryRun / GetCockpitState
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				rt := eng.BuildRuntimeState(true)
				if rt == nil {
					t.Errorf("BuildRuntimeState returned nil")
					return
				}
				diag, err := eng.BuildDiagnostics()
				if err != nil || diag == nil {
					t.Errorf("BuildDiagnostics failed: %v", err)
					return
				}
				if i%5 == 0 {
					dr, err := eng.RunDryRun(true)
					if err != nil || dr == nil {
						t.Errorf("RunDryRun failed: %v", err)
						return
					}
				}
				got := eng.GetCockpitState()
				if got != nil && len(got.Workspaces) > 0 && len(got.Workspaces[0].StrategySlots) > 0 {
					// Mutate returned clone to verify caller mutations do not race Engine state
					got.Workspaces[0].StrategySlots[0].StrategyID = "caller_mutated"
				}
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("timeout waiting for concurrent StartBot/BuildRuntimeState/RunDryRun race test")
	}
}

func TestEngine_EvaluateTick_ScopeSymbolFilter(t *testing.T) {
	eng := NewEngine()
	eng.SetGlobalEnabled(true)

	strat := &models.Strategy{
		ID:   "strat_scope_symbol",
		Name: "Symbol Scoped Tick Test",
		Rules: []models.Rule{
			{
				ID:      "r_aapl_only",
				Name:    "AAPL",
				Scope:   models.ScopeSymbol,
				Action:  models.ActionBuy,
				Enabled: true,
				Condition: &models.Condition{
					Type:            models.ConditionSlopeBelow,
					Threshold:       0.0,
					LookbackPeriods: 2,
				},
			},
		},
	}
	if err := eng.StartBot(strat); err != nil {
		t.Fatalf("StartBot failed: %v", err)
	}

	marketData := map[string][]float64{
		"Close":      {100, 99, 98},
		"indicatorA": {25, 20, 18},
	}

	if actions := eng.EvaluateTick("MSFT", marketData); len(actions) != 0 {
		t.Fatalf("expected ScopeSymbol rule for AAPL to skip MSFT, got %v", actions)
	}
	actions := eng.EvaluateTick("AAPL", marketData)
	if len(actions) != 1 || actions[0] != models.ActionBuy {
		t.Fatalf("expected ScopeSymbol rule for AAPL to fire BUY on AAPL, got %v", actions)
	}
}

func TestIBKRConnection_LiveTelemetry(t *testing.T) {
	if os.Getenv("TWS_LIVE_TEST") != "1" {
		t.Skip("Skipping live TWS telemetry test; set TWS_LIVE_TEST=1 to run against live Paper TWS")
	}

	tmpDir := t.TempDir()
	database, err := db.Connect(filepath.Join(tmpDir, "live_tws.sqlite"))
	if err != nil {
		t.Fatalf("db.Connect failed: %v", err)
	}
	defer database.Close()

	eng := NewEngine()
	eng.AttachDB(database)
	client := NewIBKRClient(eng)

	if err := client.Connect("127.0.0.1", 7497, 98); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Disconnect()

	deadline := time.Now().Add(5 * time.Second)
	var state *models.BotState
	for time.Now().Before(deadline) {
		state = eng.BuildRuntimeState(client.IsConnected())
		if state.AccountID != "" && state.Equity > 0 && client.Wrapper().positionsHydrated.Load() {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}

	state = eng.BuildRuntimeState(client.IsConnected())
	diag, err := eng.BuildDiagnostics()
	if err != nil {
		t.Fatalf("BuildDiagnostics failed: %v", err)
	}
	dr, err := eng.RunDryRun(client.IsConnected())
	if err != nil {
		t.Fatalf("RunDryRun failed: %v", err)
	}
	pd, err := database.GetWatchlistByID("positions-detenues")
	if err != nil {
		t.Fatalf("GetWatchlistByID(positions-detenues) failed: %v", err)
	}

	t.Logf("Live AccountID=%q Equity=%.2f BuyingPower=%.2f CashUSD=%.2f CashCAD=%.2f CashBase=%.2f DailyPnL=%.2f Realized=%.2f Unrealized=%.2f",
		state.AccountID, state.Equity, state.BuyingPower, state.CashUSD, state.CashCAD, state.CashBase, state.DailyPnL, state.RealizedPnL, state.UnrealizedPnL)
	t.Logf("Live OpenPositions=%d (%+v) OpenOrders=%d Alerts=%d Session=%q",
		len(state.OpenPositions), state.OpenPositions, len(state.OpenOrders), len(state.BrokerAlerts), diag.SessionStatus)
	t.Logf("Live positions-detenues=%v DryRun Strategy=%q Subscriptions=%d",
		pd.Symbols, dr.Strategy, len(dr.Subscriptions))

	if state.AccountID == "" {
		t.Errorf("expected live AccountID captured")
	}
	if state.Equity <= 0 {
		t.Errorf("expected live NetLiquidation > 0, got %v", state.Equity)
	}
}
