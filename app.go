package main

import (
	"context"
	"fmt"
	"log"

	"tws_traderbot/backend/db"
	"tws_traderbot/backend/models"
)

// App struct
type App struct {
	ctx context.Context
	db  *db.Database
}

// NewApp creates a new App application struct
func NewApp() *App {
	database, err := db.Connect("tws_traderbot.sqlite")
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}

	return &App{
		db: database,
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
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
        Running: false,
    }
}

// FailsafeStop issues a global disable
func (a *App) FailsafeStop() {
    log.Println("FAILSAFE INVOKED")
    // implementation
}
