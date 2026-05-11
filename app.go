package main

import (
	"context"
	"fmt"
	"log"

	"tws_traderbot/backend/bot"
	"tws_traderbot/backend/db"
	"tws_traderbot/backend/models"
)

// App struct
type App struct {
	ctx        context.Context
	db         *db.Database
	engine     *bot.Engine
	ibkrClient *bot.IBKRClient
}

// NewApp creates a new App application struct
func NewApp() *App {
	database, err := db.Connect("tws_traderbot.sqlite")
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}

	engine := bot.NewEngine()
	// TWS Paper Trading Client
	ibkrClient := bot.NewIBKRClient(engine)

	return &App{
		db:         database,
		engine:     engine,
		ibkrClient: ibkrClient,
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// engine is accessible globally or stored on app, we would inject ctx here
	a.engine.AttachContext(ctx)

	// Fetch connection settings from DB or use defaults
	host := "127.0.0.1"
	port := 7497
	clientID := 1

	config, err := a.db.LoadSystemConfig("tws_connection")
	if err == nil && config != nil {
		if h, ok := config["host"].(string); ok { host = h }
		if p, ok := config["port"].(float64); ok { port = int(p) }
		if c, ok := config["client_id"].(float64); ok { clientID = int(c) }
	} else {
		// Save default if not found
		_ = a.db.SaveSystemConfig("tws_connection", map[string]interface{}{
			"host": host,
			"port": port,
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
}

// GetTWSConnection returns the current TWS connection settings from the DB
func (a *App) GetTWSConnection() map[string]interface{} {
	config, err := a.db.LoadSystemConfig("tws_connection")
	if err != nil || config == nil {
		return map[string]interface{}{
			"host": "127.0.0.1",
			"port": 7497,
			"client_id": 1,
		}
	}
	return config
}

// UpdateTWSConnection saves new settings, disconnects the current TWS session, and reconnects
func (a *App) UpdateTWSConnection(host string, port int, clientID int) error {
	err := a.db.SaveSystemConfig("tws_connection", map[string]interface{}{
		"host": host,
		"port": port,
		"client_id": clientID,
	})
	if err != nil {
		return err
	}

	a.ibkrClient.Disconnect()
	return a.ibkrClient.Connect(host, port, clientID)
}

// GetWatchlist replaces the FastAPI GET /api/watchlist endpoint
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

// GetCockpitState replaces the FastAPI GET /api/cockpit/state endpoint
func (a *App) GetCockpitState() *models.CockpitStateResponse {
	state, err := a.db.LoadCockpitState()
	if err == nil && state != nil {
		return state
	}

	// Default initialization if none exists
	defaultState := &models.CockpitStateResponse{
		GlobalEnabled: false,
		Workspaces: []models.CockpitWorkspace{
			{
				ID:      "default",
				Name:    "Main Trading",
				Kind:    "long",
				Enabled: false,
			},
		},
	}
	_ = a.db.SaveCockpitState(defaultState)
	return defaultState
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}


// UpdateCockpitState replaces the FastAPI PUT /api/cockpit endpoint
// Instead of a partial 'UpdateRequest' struct, we can just accept the full state payload
// from the frontend, or selectively update the existing state.
func (a *App) UpdateCockpitState(update *models.CockpitStateResponse) (*models.CockpitStateResponse, error) {
	state, err := a.db.LoadCockpitState()
	if err != nil {
		state = &models.CockpitStateResponse{}
	}

	state.GlobalEnabled = update.GlobalEnabled
	state.ActiveWorkspaceID = update.ActiveWorkspaceID
	
	if update.Workspaces != nil {
		state.Workspaces = update.Workspaces
	}

	err = a.db.SaveCockpitState(state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

// GetRuntimeState returns the actual bot engine state
func (a *App) GetRuntimeState() *models.BotState {
    return &models.BotState{
        Running: a.engine.IsRunning,
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
