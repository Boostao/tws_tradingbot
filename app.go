package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"tws_traderbot/backend/bot"
	"tws_traderbot/backend/db"
	"tws_traderbot/backend/models"
	"tws_traderbot/backend/strategy"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// App struct
type App struct {
	app        *application.App
	db         *db.Database
	engine     *bot.Engine
	ibkrClient *bot.IBKRClient
}

// NewApp creates a new App application struct
func NewApp(app *application.App) *App {
	database, err := db.Connect("tws_traderbot.sqlite")
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	_ = strategy.SeedStrategyCatalog(database)

	engine := bot.NewEngine()
	engine.AttachDB(database)
	// TWS Paper Trading Client
	ibkrClient := bot.NewIBKRClient(engine)

	return &App{
		app:        app,
		db:         database,
		engine:     engine,
		ibkrClient: ibkrClient,
	}
}

// ServiceStartup is called when the service starts. The app reference is already stored.
func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.engine.AttachApp(a.app)
	a.engine.SetCockpitState(a.GetCockpitState())

	// Fetch connection settings from DB or use defaults
	host := "127.0.0.1"
	port := 7497
	clientID := 1

	config, err := a.db.LoadSystemConfig("tws_connection")
	if err == nil && config != nil {
		if h, ok := config["host"].(string); ok {
			host = h
		}
		if p, ok := config["port"].(float64); ok {
			port = int(p)
		}
		if c, ok := config["client_id"].(float64); ok {
			clientID = int(c)
		}
	} else {
		// Save default if not found
		_ = a.db.SaveSystemConfig("tws_connection", map[string]interface{}{
			"host":      host,
			"port":      port,
			"client_id": clientID,
		})
	}

	// Automatically connect to TWS on startup
	go func() {
		err := a.ibkrClient.Connect(host, port, clientID)
		if err != nil {
			log.Printf("TWS Connect Error: %v\n", err)
		}
	}()

	// Continuous background telemetry ticker
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if a.app != nil {
					state := a.GetRuntimeState()
					a.app.Event.Emit("state_update", state)
					a.app.Event.Emit("pulse_tick", map[string]interface{}{
						"timestamp": state.LastHeartbeat,
						"equity":    state.Equity,
						"daily_pnl": state.DailyPnL,
					})
				}
			}
		}
	}()

	return nil
}

// GetTWSConnection returns the current TWS connection settings from the DB
func (a *App) GetTWSConnection() map[string]interface{} {
	config, err := a.db.LoadSystemConfig("tws_connection")
	if err != nil || config == nil {
		return map[string]interface{}{
			"host":      "127.0.0.1",
			"port":      7497,
			"client_id": 1,
		}
	}
	return config
}

// UpdateTWSConnection saves new settings, disconnects the current TWS session, and reconnects
func (a *App) UpdateTWSConnection(host string, port int, clientID int) error {
	err := a.db.SaveSystemConfig("tws_connection", map[string]interface{}{
		"host":      host,
		"port":      port,
		"client_id": clientID,
	})
	if err != nil {
		return err
	}

	a.ibkrClient.Disconnect()
	return a.ibkrClient.Connect(host, port, clientID)
}

// GetWatchlist returns the current default watchlist state
func (a *App) GetWatchlist() *models.WatchlistResponse {
	state, err := a.db.LoadWatchlistState()
	if err == nil && state != nil {
		return state
	}

	// Fallback/Default stub if missing in DB
	return &models.WatchlistResponse{
		Symbols: []string{},
		Groups: []models.WatchlistGroup{
			{
				ID:     "manual",
				Name:   "Manual",
				Source: "manual",
				Items:  []models.WatchlistItem{},
			},
		},
	}
}

// GetCockpitState returns the current cockpit state from SQLite
func (a *App) GetCockpitState() *models.CockpitStateResponse {
	state, err := a.db.LoadCockpitState()
	if err == nil && state != nil && len(state.Workspaces) >= 3 {
		lib, errLib := a.db.LoadStrategyLibrary()
		if errLib == nil {
			state.StrategyLibrary = lib
		}
		if state.ActiveWorkspaceID == "" {
			state.ActiveWorkspaceID = "achat"
		}
		return state
	}

	// Default initialization with 3 canonical Cobalt workspaces
	defaultState := &models.CockpitStateResponse{
		GlobalEnabled:     false,
		ActiveWorkspaceID: "achat",
		Workspaces: []models.CockpitWorkspace{
			{
				ID:      "achat",
				Name:    "Achat",
				Kind:    "long",
				Enabled: true,
				StrategySlots: []models.CockpitStrategySlot{
					{
						ID:         "slot_1",
						Label:      "Croisement 10-30",
						StrategyID: "screener_10_30_haut",
						Enabled:    true,
					},
				},
			},
			{
				ID:      "short",
				Name:    "Short",
				Kind:    "short",
				Enabled: false,
				StrategySlots: []models.CockpitStrategySlot{
					{
						ID:         "slot_1",
						Label:      "Croisement Bas",
						StrategyID: "screener_10_30_bas",
						Enabled:    false,
					},
				},
			},
			{
				ID:      "swing",
				Name:    "Swing",
				Kind:    "swing",
				Enabled: false,
				StrategySlots: []models.CockpitStrategySlot{
					{
						ID:         "slot_1",
						Label:      "Weinstein Phase 2",
						StrategyID: "cours_weinstein_p2",
						Enabled:    false,
					},
				},
			},
		},
	}
	lib, errLib := a.db.LoadStrategyLibrary()
	if errLib == nil {
		defaultState.StrategyLibrary = lib
	}
	_ = a.db.SaveCockpitState(defaultState)
	return defaultState
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// UpdateCockpitState updates the global cockpit state in SQLite
func (a *App) UpdateCockpitState(update *models.CockpitStateResponse) (*models.CockpitStateResponse, error) {
	if update == nil {
		return nil, fmt.Errorf("cockpit state update payload cannot be nil")
	}

	state, err := a.db.LoadCockpitState()
	if err != nil || state == nil || len(state.Workspaces) < 3 {
		state = a.GetCockpitState()
	}

	state.GlobalEnabled = update.GlobalEnabled
	if update.ActiveWorkspaceID != "" {
		state.ActiveWorkspaceID = update.ActiveWorkspaceID
	}
	if update.Workspaces != nil && len(update.Workspaces) > 0 {
		state.Workspaces = update.Workspaces
	}

	// Enforce 1/1 active slot concurrency invariant across all workspaces:
	// Only the slot matching the active workspace should remain enabled if any slot is enabled.
	for i := range state.Workspaces {
		ws := &state.Workspaces[i]
		if ws.ID == state.ActiveWorkspaceID {
			// In the active workspace, at most one slot may be enabled
			armedFound := false
			for j := range ws.StrategySlots {
				slot := &ws.StrategySlots[j]
				if slot.Enabled && !armedFound {
					armedFound = true
				} else {
					slot.Enabled = false
				}
			}
		} else {
			ws.Enabled = false
			for j := range ws.StrategySlots {
				ws.StrategySlots[j].Enabled = false
			}
		}
	}

	lib, errLib := a.db.LoadStrategyLibrary()
	if errLib == nil {
		state.StrategyLibrary = lib
	}

	err = a.db.SaveCockpitState(state)
	if err != nil {
		return nil, err
	}

	var activeStrategyID string
	for _, ws := range state.Workspaces {
		if ws.Enabled {
			for _, slot := range ws.StrategySlots {
				if slot.Enabled && slot.StrategyID != "" {
					activeStrategyID = slot.StrategyID
					break
				}
			}
		}
	}

	if a.engine != nil {
		a.engine.SetCockpitState(state)
		globalEnabled := a.engine.IsGlobalEnabled()
		if activeStrategyID != "" {
			strat, errStrat := a.db.LoadStrategyPreset(activeStrategyID)
			if errStrat == nil && strat != nil {
				a.engine.StopBot()
				_ = a.engine.StartBot(strat)
			} else {
				a.engine.StopBot()
			}
		} else {
			a.engine.StopBot()
		}
		if !globalEnabled {
			a.engine.StopBot()
		}
	}

	return state, nil
}

// GetRuntimeState returns the actual bot engine state
func (a *App) GetRuntimeState() *models.BotState {
	running := false
	equity := 100000.0
	lossLimit := 500.0
	realized, unrealized, totalPnL := 0.0, 0.0, 0.0
	breakerTripped := false
	globalEnabled := false
	activeStratName := ""

	if a.engine != nil {
		running = a.engine.IsRunning()
		equity = a.engine.GetEquity()
		lossLimit = a.engine.GetDailyLossLimit()
		realized, unrealized, totalPnL = a.engine.GetPnL()
		_ = realized
		_ = unrealized
		breakerTripped = a.engine.RiskBreakerTripped.Load()
		globalEnabled = a.engine.IsGlobalEnabled()
		strat := a.engine.GetActiveStrategy()
		if strat != nil {
			activeStratName = strat.Name
			if activeStratName == "" {
				activeStratName = strat.ID
			}
		}
	}

	twsConnected := false
	if a.ibkrClient != nil {
		twsConnected = a.ibkrClient.IsConnected()
	}

	dailyPnLPct := 0.0
	if equity > 0 {
		dailyPnLPct = (totalPnL / equity) * 100.0
	}

	status := "STOPPED"
	if breakerTripped {
		status = "CIRCUIT_BREAKER"
	} else if running {
		status = "RUNNING"
	} else if globalEnabled {
		status = "ARMED"
	}

	now := time.Now().UTC().Format(time.RFC3339)

	return &models.BotState{
		Status:                status,
		Running:               running,
		RunnerActive:          running,
		TwsConnected:          twsConnected,
		Equity:                equity,
		DailyPnL:              totalPnL,
		DailyPnLPercent:       dailyPnLPct,
		TotalPnL:              totalPnL,
		DailyLossLimit:        lossLimit,
		CircuitBreakerTripped: breakerTripped,
		ActiveStrategy:        activeStratName,
		LastHeartbeat:         now,
		LastUpdate:            now,
	}
}

// FailsafeStop issues a global disable
func (a *App) FailsafeStop() {
	log.Println("FAILSAFE INVOKED - Disconnecting TWS and stopping Engine")
	a.engine.StopBot()
	a.ibkrClient.Disconnect()
}

func (a *App) ConnectTws(host string, port int, clientID int) map[string]interface{} {
	err := a.UpdateTWSConnection(host, port, clientID)
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	return map[string]interface{}{"status": "ok"}
}

func (a *App) DisconnectTws() map[string]interface{} {
	a.ibkrClient.Disconnect()
	return map[string]interface{}{"status": "ok"}
}

func (a *App) StartBot() map[string]interface{} {
	// Just empty strategy for now, since we only need the boolean state
	err := a.engine.StartBot(&models.Strategy{})
	if err != nil {
		return map[string]interface{}{"status": "error"}
	}
	return map[string]interface{}{"status": "ok"}
}

func (a *App) StopBot() map[string]interface{} {
	a.engine.StopBot()
	return map[string]interface{}{"status": "ok"}
}
