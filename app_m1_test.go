package main

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"tws_traderbot/backend/bot"
	"tws_traderbot/backend/db"
	"tws_traderbot/backend/models"
	"tws_traderbot/backend/strategy"
)

func setupTestApp(t *testing.T) *App {
	t.Helper()
	tmpDir := t.TempDir()
	database, err := db.Connect(filepath.Join(tmpDir, "app_test.sqlite"))
	if err != nil {
		t.Fatalf("db.Connect failed: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := strategy.SeedStrategyCatalog(database); err != nil {
		t.Fatalf("SeedStrategyCatalog failed: %v", err)
	}

	eng := bot.NewEngine()
	eng.AttachDB(database)
	ibkr := bot.NewIBKRClient(eng)

	a := &App{
		db:         database,
		engine:     eng,
		ibkrClient: ibkr,
	}

	cfg := a.GetConfig()
	a.syncConfigToEngine(cfg)
	cockpit := a.GetCockpitState()
	eng.SetCockpitState(cockpit)
	_, _, strat := a.resolveSelectedStrategy(cockpit)
	eng.SetActiveStrategy(strat)
	eng.RecordRuntimeReload("test_init")

	return a
}

func TestApp_UpdateConfigDeepMergeAndEngineSync(t *testing.T) {
	a := setupTestApp(t)

	_, err := a.UpdateConfig(map[string]interface{}{
		"ib": map[string]interface{}{
			"host":         "127.0.0.1",
			"port":         7497,
			"client_id":    2,
			"trading_mode": "paper",
		},
		"runtime": map[string]interface{}{
			"dry_run":          false,
			"daily_loss_limit": 750.0,
			"bracket_enabled":  true,
			"take_profit_pct":  4.5,
			"stop_loss_pct":    1.8,
		},
	})
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Partial update of ib.account must NOT clobber host/port/client_id/trading_mode
	cfg, err := a.UpdateConfig(map[string]interface{}{
		"ib": map[string]interface{}{
			"account": "DU777888",
		},
	})
	if err != nil {
		t.Fatalf("partial UpdateConfig failed: %v", err)
	}

	ibMap := cfg["ib"].(map[string]interface{})
	if ibMap["host"] != "127.0.0.1" || ibMap["account"] != "DU777888" || ibMap["trading_mode"] != "paper" {
		t.Errorf("deep merge failed on ib config: %+v", ibMap)
	}
	if a.engine.IsDryRun() {
		t.Errorf("expected engine DryRun=false after UpdateConfig")
	}
	if a.engine.GetDailyLossLimit() != 750.0 {
		t.Errorf("expected engine dailyLossLimit=750, got %v", a.engine.GetDailyLossLimit())
	}
	tp, sl := a.engine.GetBracketPercentages()
	if !a.engine.IsBracketEnabled() || tp != 4.5 || sl != 1.8 {
		t.Errorf("unexpected bracket config: enabled=%v tp=%v sl=%v", a.engine.IsBracketEnabled(), tp, sl)
	}
}

func TestApp_StrategyLifecycleAndValidation(t *testing.T) {
	a := setupTestApp(t)

	// 1. ValidateStrategy accepts ActionBuy, ActionSell, and ActionFilter
	res, err := a.ValidateStrategy(&models.Strategy{
		Name: "Filter + Buy Strategy",
		Rules: []models.Rule{
			{
				Name:   "Macro VIX Filter",
				Action: models.ActionFilter,
				Condition: &models.Condition{
					Type:       models.ConditionLess,
					IndicatorA: &models.Indicator{Type: models.IndicatorVIX, Timeframe: models.TimeframeD1},
					Threshold:  25.0,
				},
			},
			{
				Name:   "Entry",
				Action: models.ActionBuy,
				Condition: &models.Condition{
					Type:       models.ConditionCrossAbove,
					IndicatorA: &models.Indicator{Type: models.IndicatorEMA, Period: 10, Timeframe: models.TimeframeD1},
					IndicatorB: &models.Indicator{Type: models.IndicatorEMA, Period: 30, Timeframe: models.TimeframeD1},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("ValidateStrategy returned err: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected ActionFilter strategy to be valid, got errors: %v", res.Errors)
	}

	// 2. ToggleStrategyPin & MoveStrategyOrder
	lib, err := a.GetStrategyLibrary()
	if err != nil || len(lib) < 3 {
		t.Fatalf("expected seeded strategy library, got len=%d err=%v", len(lib), err)
	}
	targetID := lib[2].ID
	pinnedLib, err := a.ToggleStrategyPin(targetID)
	if err != nil {
		t.Fatalf("ToggleStrategyPin failed: %v", err)
	}
	if pinnedLib[0].ID != targetID || !pinnedLib[0].Pinned {
		t.Errorf("expected %s pinned at top, got %+v", targetID, pinnedLib[0])
	}

	secondTarget := pinnedLib[2].ID
	reordered, err := a.MoveStrategyOrder(secondTarget, "up")
	if err != nil {
		t.Fatalf("MoveStrategyOrder failed: %v", err)
	}
	if reordered[1].ID != secondTarget {
		t.Errorf("expected %s moved up to index 1, got %s", secondTarget, reordered[1].ID)
	}

	// 3. ApplyStrategyPreset with GlobalEnabled=false keeps engine paused while hydrating ActiveStrategy
	if err := a.ApplyStrategyPreset("swing", "slot_1", targetID); err != nil {
		t.Fatalf("ApplyStrategyPreset failed: %v", err)
	}
	if a.engine.IsRunning() {
		t.Errorf("expected engine to stay stopped when GlobalEnabled=false")
	}
	if a.engine.GetActiveStrategy() == nil || a.engine.GetActiveStrategy().ID != targetID {
		t.Errorf("expected ActiveStrategy=%s when paused", targetID)
	}

	rt := a.GetRuntimeState()
	if rt.Equity != 0 {
		t.Errorf("expected Equity=0 before TWS report, got %v", rt.Equity)
	}
	if rt.ActiveStrategyID != targetID || rt.ActiveStrategy == "" {
		t.Errorf("expected RuntimeState ActiveStrategyID=%s, got id=%q name=%q", targetID, rt.ActiveStrategyID, rt.ActiveStrategy)
	}

	// 4. StartBot uses resolved strategy (NOT empty &models.Strategy{})
	startRes := a.StartBot()
	if startRes["status"] != "ok" {
		t.Fatalf("StartBot failed: %+v", startRes)
	}
	if a.engine.GetActiveStrategy() == nil || a.engine.GetActiveStrategy().ID != targetID {
		t.Errorf("StartBot overwrote ActiveStrategy: %+v", a.engine.GetActiveStrategy())
	}

	// 5. FailsafeStop disables GlobalEnabled and persists to SQLite
	a.FailsafeStop()
	if a.engine.IsRunning() || a.engine.IsGlobalEnabled() {
		t.Errorf("expected FailsafeStop to stop engine and disarm GlobalEnabled")
	}
	persisted, _ := a.db.LoadCockpitState()
	if persisted.GlobalEnabled {
		t.Errorf("expected FailsafeStop to persist GlobalEnabled=false")
	}
}

func TestApp_DiagnosticsDenoisedAndReplaceWatchlistPreservesMetadata(t *testing.T) {
	a := setupTestApp(t)

	diag1, err := a.GetDiagnostics()
	if err != nil {
		t.Fatalf("GetDiagnostics failed: %v", err)
	}
	if diag1.Startup.Account == "DUO185814" || diag1.Account == "DUO185814" {
		t.Errorf("hardcoded DUO185814 leaked in diagnostics: %+v", diag1)
	}
	if diag1.Startup.Environment != "" || diag1.Startup.LogFile != "" || diag1.Startup.WatchlistPath != "" || diag1.Symbols.Source != "" {
		t.Errorf("legacy developer noise leaked in diagnostics: %+v", diag1)
	}
	if diag1.SessionStatus == "" || diag1.ActiveStrategyName == "" {
		t.Errorf("expected SessionStatus and ActiveStrategyName populated, got %+v", diag1)
	}

	time.Sleep(15 * time.Millisecond)
	diag2, _ := a.GetDiagnostics()
	if diag1.Runtime.LastRuntimeReloadAt != diag2.Runtime.LastRuntimeReloadAt {
		t.Errorf("LastRuntimeReloadAt churned between GetDiagnostics calls: %q vs %q", diag1.Runtime.LastRuntimeReloadAt, diag2.Runtime.LastRuntimeReloadAt)
	}
	if diag1.Symbols.LastCheckedAt != diag2.Symbols.LastCheckedAt {
		t.Errorf("LastCheckedAt churned between GetDiagnostics calls: %q vs %q", diag1.Symbols.LastCheckedAt, diag2.Symbols.LastCheckedAt)
	}

	// ReplaceWatchlist without Name/Feed preserves cobalt-telegram's title and TradingView feed URL
	updated, err := a.ReplaceWatchlist(&models.ReplaceWatchlistRequest{
		WatchlistID: "cobalt-telegram",
		Groups: []models.WatchlistGroup{
			{
				ID:   "titres",
				Name: "Titres",
				Items: []models.WatchlistItem{
					{Symbol: "GOOGL", Exchange: "NASDAQ", Enabled: true},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("ReplaceWatchlist failed: %v", err)
	}
	if updated.Name != "✝️@ Cobalt- TELEGRAM" {
		t.Errorf("expected preserved name '✝️@ Cobalt- TELEGRAM', got %q", updated.Name)
	}
	if updated.Feed == nil || updated.Feed.URL == "" {
		t.Errorf("expected TradingView feed URL preserved, got %+v", updated.Feed)
	}
}

func TestApp_CockpitMutexBreakerClampAndSaveRefresh(t *testing.T) {
	a := setupTestApp(t)

	// 1. SaveStrategyPreset refreshes engine.ActiveStrategy when ID matches active strategy
	strat, err := a.GetStrategyPreset("screener_10_30_haut")
	if err != nil || strat == nil {
		t.Fatalf("GetStrategyPreset failed: %v", err)
	}
	strat.Name = "Screener 10-30 Refreshed"
	if _, err := a.SaveStrategyPreset(strat); err != nil {
		t.Fatalf("SaveStrategyPreset failed: %v", err)
	}
	if active := a.engine.GetActiveStrategy(); active == nil || active.Name != "Screener 10-30 Refreshed" {
		t.Errorf("expected SaveStrategyPreset to refresh engine.ActiveStrategy, got %+v", active)
	}

	// 2. UpdateCockpitState, Engine.SetCockpitState, and StartBot must enforce RiskBreakerTripped=true
	a.engine.SetDailyLossLimit(500.0)
	a.engine.UpdateLivePnL(-600.0, -600.0, 0.0)
	if !a.engine.RiskBreakerTripped.Load() {
		t.Fatal("expected RiskBreakerTripped=true")
	}

	if startErrRes := a.StartBot(); startErrRes["status"] != "error" {
		t.Errorf("expected StartBot to refuse while RiskBreakerTripped=true, got %+v", startErrRes)
	}

	a.engine.SetCockpitState(&models.CockpitStateResponse{GlobalEnabled: true})
	if a.engine.IsGlobalEnabled() || a.engine.GetCockpitState().GlobalEnabled {
		t.Errorf("expected Engine.SetCockpitState to clamp GlobalEnabled=false while RiskBreakerTripped=true")
	}

	cs := a.GetCockpitState()
	cs.GlobalEnabled = true
	res, err := a.UpdateCockpitState(cs)
	if err != nil {
		t.Fatalf("UpdateCockpitState failed: %v", err)
	}
	if res.GlobalEnabled || a.engine.IsGlobalEnabled() || a.engine.IsRunning() {
		t.Errorf("expected UpdateCockpitState to clamp GlobalEnabled=false when breaker tripped, got global=%v running=%v",
			res.GlobalEnabled, a.engine.IsRunning())
	}

	// 3. Concurrent UpdateCockpitState + ApplyStrategyPreset + FailsafeStop + ResetCircuitBreaker under -race
	a.engine.UpdateLivePnL(100.0, 100.0, 0.0)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				switch id {
				case 0:
					st := a.GetCockpitState()
					st.GlobalEnabled = j%2 == 0
					_, _ = a.UpdateCockpitState(st)
				case 1:
					_ = a.ApplyStrategyPreset("swing", "slot_1", "cours_weinstein_p2")
				case 2:
					a.FailsafeStop()
				case 3:
					_ = a.ResetCircuitBreaker()
				}
			}
		}(i)
	}
	wg.Wait()
}
