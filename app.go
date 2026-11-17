package main

import (
	"context"
	"fmt"
	"log"
	"sync"
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
	cockpitMu  sync.Mutex
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

	cfg := a.GetConfig()
	a.syncConfigToEngine(cfg)

	cockpit := a.GetCockpitState()
	a.engine.SetCockpitState(cockpit)
	_, _, strat := a.resolveSelectedStrategy(cockpit)
	a.engine.SetActiveStrategy(strat)
	a.engine.RecordRuntimeReload("startup")

	// Fetch connection settings from DB, falling back to config.ib, then defaults
	host := "127.0.0.1"
	port := 7497
	clientID := 1

	if ibCfg, ok := cfg["ib"].(map[string]interface{}); ok {
		if h, ok := ibCfg["host"].(string); ok && h != "" {
			host = h
		}
		if p, ok := ibCfg["port"].(float64); ok && p > 0 {
			port = int(p)
		}
		if c, ok := ibCfg["client_id"].(float64); ok && c >= 0 {
			clientID = int(c)
		}
	}

	twsCfg, err := a.db.LoadSystemConfig("tws_connection")
	if err == nil && twsCfg != nil {
		if h, ok := twsCfg["host"].(string); ok && h != "" {
			host = h
		}
		if p, ok := twsCfg["port"].(float64); ok && p > 0 {
			port = int(p)
		}
		if c, ok := twsCfg["client_id"].(float64); ok && c >= 0 {
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

// UpdateTWSConnection saves new settings, keeps config.ib aligned, disconnects the current TWS session, and reconnects
func (a *App) UpdateTWSConnection(host string, port int, clientID int) error {
	err := a.db.SaveSystemConfig("tws_connection", map[string]interface{}{
		"host":      host,
		"port":      port,
		"client_id": clientID,
	})
	if err != nil {
		return err
	}

	cfg := a.GetConfig()
	cfg = bot.DeepMergeMap(cfg, map[string]interface{}{
		"ib": map[string]interface{}{
			"host":      host,
			"port":      port,
			"client_id": clientID,
		},
	})
	_ = a.db.SaveSystemConfig("config", cfg)

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

	a.cockpitMu.Lock()
	defer a.cockpitMu.Unlock()

	state, err := a.db.LoadCockpitState()
	if err != nil || state == nil || len(state.Workspaces) < 3 {
		state = a.GetCockpitState()
	}

	state.GlobalEnabled = update.GlobalEnabled
	if a.engine != nil && a.engine.RiskBreakerTripped.Load() {
		state.GlobalEnabled = false
	}
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

	_, activeStrategyID, strat := a.resolveSelectedStrategy(state)

	activeSlotEnabled := false
	for _, ws := range state.Workspaces {
		if ws.Enabled {
			for _, slot := range ws.StrategySlots {
				if slot.Enabled && slot.StrategyID != "" {
					activeSlotEnabled = true
					break
				}
			}
		}
	}

	if a.engine != nil {
		a.engine.SetCockpitState(state)
		a.engine.SetActiveStrategy(strat)
		globalEnabled := a.engine.IsGlobalEnabled()
		if globalEnabled && !a.engine.RiskBreakerTripped.Load() && activeSlotEnabled && activeStrategyID != "" && strat != nil {
			a.engine.StopBot()
			_ = a.engine.StartBot(strat)
		} else {
			a.engine.StopBot()
		}
		a.engine.RecordRuntimeReload("cockpit_update")
	}

	return state, nil
}

func (a *App) resolveSelectedStrategy(state *models.CockpitStateResponse) (wsID, stratID string, strat *models.Strategy) {
	if a.engine != nil {
		return a.engine.ResolveSelectedStrategy(state)
	}
	if state == nil || len(state.Workspaces) == 0 {
		return "", "", nil
	}
	var targetWS *models.CockpitWorkspace
	for i := range state.Workspaces {
		if state.Workspaces[i].ID == state.ActiveWorkspaceID {
			targetWS = &state.Workspaces[i]
			break
		}
	}
	if targetWS == nil {
		targetWS = &state.Workspaces[0]
	}
	wsID = targetWS.ID
	for _, slot := range targetWS.StrategySlots {
		if slot.Enabled && slot.StrategyID != "" {
			stratID = slot.StrategyID
			break
		}
	}
	if stratID == "" {
		for _, slot := range targetWS.StrategySlots {
			if slot.StrategyID != "" {
				stratID = slot.StrategyID
				break
			}
		}
	}
	if stratID != "" && a.db != nil {
		strat, _ = a.db.LoadStrategyPreset(stratID)
	}
	return wsID, stratID, strat
}

// GetRuntimeState returns the actual bot engine state
func (a *App) GetRuntimeState() *models.BotState {
	twsConnected := false
	if a.ibkrClient != nil {
		twsConnected = a.ibkrClient.IsConnected()
	}

	if a.engine != nil {
		return a.engine.BuildRuntimeState(twsConnected)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	return &models.BotState{
		Status:        "STOPPED",
		TwsConnected:  twsConnected,
		Equity:        0.0,
		LastHeartbeat: now,
		LastUpdate:    now,
	}
}

// FailsafeStop issues a global disable, persists GlobalEnabled=false, stops the engine, and disconnects TWS.
func (a *App) FailsafeStop() {
	a.cockpitMu.Lock()
	defer a.cockpitMu.Unlock()

	log.Println("FAILSAFE INVOKED - Disconnecting TWS and stopping Engine")
	if a.engine != nil {
		a.engine.SetGlobalEnabled(false)
		a.engine.StopBot()
	}
	if a.db != nil {
		if state, err := a.db.LoadCockpitState(); err == nil && state != nil {
			state.GlobalEnabled = false
			_ = a.db.SaveCockpitState(state)
		}
	}
	if a.ibkrClient != nil {
		a.ibkrClient.Disconnect()
	}
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
	a.cockpitMu.Lock()
	defer a.cockpitMu.Unlock()

	if a.engine == nil {
		return map[string]interface{}{"status": "error", "message": "engine unavailable"}
	}
	if a.engine.RiskBreakerTripped.Load() {
		return map[string]interface{}{"status": "error", "message": "daily loss circuit breaker tripped"}
	}
	strat := a.engine.GetActiveStrategy()
	if strat == nil {
		_, _, strat = a.resolveSelectedStrategy(a.GetCockpitState())
	}
	if strat == nil {
		return map[string]interface{}{"status": "error", "message": "no strategy configured"}
	}
	err := a.engine.StartBot(strat)
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	return map[string]interface{}{"status": "ok"}
}

func (a *App) StopBot() map[string]interface{} {
	a.engine.StopBot()
	return map[string]interface{}{"status": "ok"}
}
