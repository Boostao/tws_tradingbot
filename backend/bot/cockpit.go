package bot

import (
	"fmt"
	"sync"
	"tws_traderbot/backend/db"
	"tws_traderbot/backend/models"
)

type CockpitManager struct {
	mu     sync.Mutex
	db     *db.Database
	engine *Engine
}

func NewCockpitManager(db *db.Database, engine *Engine) *CockpitManager {
	return &CockpitManager{
		db:     db,
		engine: engine,
	}
}

func (cm *CockpitManager) GetState() *models.CockpitStateResponse {
	state, err := cm.db.LoadCockpitState()
	if err == nil && state != nil && len(state.Workspaces) >= 3 {
		lib, errLib := cm.db.LoadStrategyLibrary()
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
	lib, errLib := cm.db.LoadStrategyLibrary()
	if errLib == nil {
		defaultState.StrategyLibrary = lib
	}
	_ = cm.db.SaveCockpitState(defaultState)
	return defaultState
}

func (cm *CockpitManager) UpdateState(update *models.CockpitStateResponse) (*models.CockpitStateResponse, error) {
	if update == nil {
		return nil, fmt.Errorf("cockpit state update payload cannot be nil")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	state, err := cm.db.LoadCockpitState()
	if err != nil || state == nil || len(state.Workspaces) < 3 {
		state = cm.GetState()
	}

	state.GlobalEnabled = update.GlobalEnabled
	if cm.engine != nil && cm.engine.RiskBreakerTripped.Load() {
		state.GlobalEnabled = false
	}
	if update.ActiveWorkspaceID != "" {
		state.ActiveWorkspaceID = update.ActiveWorkspaceID
	}
	if update.Workspaces != nil && len(update.Workspaces) > 0 {
		state.Workspaces = update.Workspaces
	}

	// Enforce 1/1 active slot concurrency invariant across all workspaces:
	for i := range state.Workspaces {
		ws := &state.Workspaces[i]
		if ws.ID == state.ActiveWorkspaceID {
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

	lib, errLib := cm.db.LoadStrategyLibrary()
	if errLib == nil {
		state.StrategyLibrary = lib
	}

	err = cm.db.SaveCockpitState(state)
	if err != nil {
		return nil, err
	}

	_, activeStrategyID, strat := cm.ResolveSelectedStrategy(state)

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

	if cm.engine != nil {
		cm.engine.SetCockpitState(state)
		cm.engine.SetActiveStrategy(strat)
		globalEnabled := cm.engine.IsGlobalEnabled()
		if globalEnabled && !cm.engine.RiskBreakerTripped.Load() && activeSlotEnabled && activeStrategyID != "" && strat != nil {
			cm.engine.StopBot()
			_ = cm.engine.StartBot(strat)
		} else {
			cm.engine.StopBot()
		}
		cm.engine.RecordRuntimeReload("cockpit_update")
	}

	return state, nil
}

func (cm *CockpitManager) ResolveSelectedStrategy(state *models.CockpitStateResponse) (wsID, stratID string, strat *models.Strategy) {
	if cm.engine != nil {
		return cm.engine.ResolveSelectedStrategy(state)
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
	if stratID != "" && cm.db != nil {
		strat, _ = cm.db.LoadStrategyPreset(stratID)
	}
	return wsID, stratID, strat
}
