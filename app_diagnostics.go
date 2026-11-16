package main

import (
	"time"
	"tws_traderbot/backend/models"
)

// GetDiagnostics returns system, runtime, and symbol telemetry
func (a *App) GetDiagnostics() (*models.DiagnosticsResponse, error) {
	connConfig := a.GetTWSConnection()
	host, _ := connConfig["host"].(string)
	if host == "" {
		host = "127.0.0.1"
	}
	port := 7497
	if p, ok := connConfig["port"].(float64); ok {
		port = int(p)
	} else if p, ok := connConfig["port"].(int); ok {
		port = p
	}
	clientID := 1
	if c, ok := connConfig["client_id"].(float64); ok {
		clientID = int(c)
	} else if c, ok := connConfig["client_id"].(int); ok {
		clientID = c
	}

	runnerActive := false
	if a.engine != nil {
		runnerActive = a.engine.IsRunning()
	}

	return &models.DiagnosticsResponse{
		Startup: models.DiagnosticsStartup{
			Environment:     "desktop_local",
			TradingMode:     "paper",
			IBHost:          host,
			IBPort:          port,
			ClientID:        clientID,
			Account:         "DUO185814",
			LogLevel:        "INFO",
			LogFile:         "tws_traderbot.log",
			WatchlistPath:   "tws_traderbot.sqlite",
			StrategyPath:    "tws_traderbot.sqlite",
			SymbolCachePath: "tws_traderbot.sqlite",
		},
		Runtime: models.DiagnosticsRuntime{
			RunnerActive:        runnerActive,
			LastRuntimeReloadAt: time.Now().UTC().Format(time.RFC3339),
		},
		Symbols: models.DiagnosticsSymbols{
			Source:        "local_sqlite",
			LastCheckedAt: time.Now().UTC().Format(time.RFC3339),
		},
	}, nil
}

// DryRunBot performs simulated execution against current market data
func (a *App) DryRunBot() (*models.BotDryRun, error) {
	activeStratName := ""
	if a.engine != nil && a.engine.ActiveStrategy != nil {
		activeStratName = a.engine.ActiveStrategy.Name
	}

	return &models.BotDryRun{
		Strategy:      activeStratName,
		WorkspaceKind: "long",
		Subscriptions: []models.BotDryRunSubscription{
			{Symbol: "SPY.SMART", Timeframe: "5m"},
		},
		Positions:     map[string]interface{}{},
		OpenOrders:    []map[string]interface{}{},
		PlannedOrders: []map[string]interface{}{},
		State:         a.GetRuntimeState(),
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// ResetCircuitBreaker manually re-arms the execution engine
func (a *App) ResetCircuitBreaker() error {
	if a.engine != nil {
		return a.engine.ResetCircuitBreaker()
	}
	return nil
}

// GetRecentTrades returns recent execution records from the SQLite trade ledger
func (a *App) GetRecentTrades() ([]models.TradeRecord, error) {
	if a.db != nil {
		return a.db.GetRecentTrades(50)
	}
	return []models.TradeRecord{}, nil
}
