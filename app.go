package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"tws_traderbot/backend/api"
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
	ibkrClient     *bot.IBKRClient
	cockpitManager *bot.CockpitManager
	cockpitMu      sync.Mutex
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
	cockpitManager := bot.NewCockpitManager(database, engine)

	return &App{
		app:            app,
		db:             database,
		engine:         engine,
		ibkrClient:     ibkrClient,
		cockpitManager: cockpitManager,
		cockpitMu:      sync.Mutex{},
	}
}

// Emit implements bot.Emitter.
func (a *App) Emit(name string, data any) {
	if a.app != nil {
		a.app.Event.Emit(name, data)
	}
}

// ServiceStartup is called when the service starts. The app reference is already stored.
func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.engine.AttachEmitter(a)

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
	return a.cockpitManager.GetState()
}

// GetAutopilotState returns the current autopilot system state
func (a *App) GetAutopilotState() *models.AutopilotState {
	return api.GetAutopilotState(a.engine)
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// UpdateCockpitState updates the global cockpit state in SQLite
func (a *App) UpdateCockpitState(update *models.CockpitStateResponse) (*models.CockpitStateResponse, error) {
	return a.cockpitManager.UpdateState(update)
}

func (a *App) resolveSelectedStrategy(state *models.CockpitStateResponse) (wsID, stratID string, strat *models.Strategy) {
	return a.cockpitManager.ResolveSelectedStrategy(state)
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
