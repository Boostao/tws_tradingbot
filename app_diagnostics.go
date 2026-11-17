package main

import (
	"time"
	"tws_traderbot/backend/models"
)

// GetDiagnostics returns system, runtime, session, and broker alert telemetry.
func (a *App) GetDiagnostics() (*models.DiagnosticsResponse, error) {
	if a.engine != nil {
		return a.engine.BuildDiagnostics()
	}

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

	tradingMode := "paper"
	if port == 7496 || port == 4001 {
		tradingMode = "live"
	}

	return &models.DiagnosticsResponse{
		Startup: models.DiagnosticsStartup{
			TradingMode: tradingMode,
			IBHost:      host,
			IBPort:      port,
			ClientID:    clientID,
		},
		Runtime: models.DiagnosticsRuntime{
			RunnerActive: false,
		},
		Symbols: models.DiagnosticsSymbols{},
	}, nil
}

// DryRunBot performs simulated execution against the active workspace, strategy, and watchlist.
func (a *App) DryRunBot() (*models.BotDryRun, error) {
	twsConnected := false
	if a.ibkrClient != nil {
		twsConnected = a.ibkrClient.IsConnected()
	}
	if a.engine != nil {
		return a.engine.RunDryRun(twsConnected)
	}

	return &models.BotDryRun{
		Strategy:      "",
		WorkspaceKind: "long",
		Subscriptions: []models.BotDryRunSubscription{},
		Positions:     map[string]interface{}{},
		OpenOrders:    []map[string]interface{}{},
		PlannedOrders: []map[string]interface{}{},
		State:         a.GetRuntimeState(),
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// ResetCircuitBreaker manually re-arms the execution engine
func (a *App) ResetCircuitBreaker() error {
	a.cockpitMu.Lock()
	defer a.cockpitMu.Unlock()

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
