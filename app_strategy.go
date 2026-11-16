package main

import (
	"fmt"
	"time"
	"tws_traderbot/backend/models"
	"tws_traderbot/backend/strategy"
)

// GetStrategyLibrary returns all strategy presets stored in SQLite
func (a *App) GetStrategyLibrary() ([]models.CockpitStrategySummary, error) {
	lib, err := a.db.LoadStrategyLibrary()
	if err == nil && len(lib) < 20 {
		_ = strategy.SeedStrategyCatalog(a.db)
		return a.db.LoadStrategyLibrary()
	}
	return lib, err
}

// SaveStrategyPreset saves or updates a strategy preset in the library
func (a *App) SaveStrategyPreset(strat *models.Strategy) (*models.CockpitStrategySummary, error) {
	if strat == nil {
		return nil, fmt.Errorf("strategy cannot be nil")
	}
	if strat.ID == "" {
		strat.ID = fmt.Sprintf("strat_%d", time.Now().UnixNano())
	}
	strat.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	ruleCount := len(strat.Rules)
	enabledRuleCount := 0
	for _, r := range strat.Rules {
		if r.Enabled {
			enabledRuleCount++
		}
	}

	summary := &models.CockpitStrategySummary{
		ID:               strat.ID,
		Name:             strat.Name,
		RuleCount:        ruleCount,
		EnabledRuleCount: enabledRuleCount,
		Source:           "preset",
		UpdatedAt:        strat.UpdatedAt,
	}

	err := a.db.SaveStrategyPreset(strat, summary)
	if err != nil {
		return nil, fmt.Errorf("failed to save preset: %w", err)
	}

	return summary, nil
}

// GetStrategyPreset retrieves a single strategy preset by ID
func (a *App) GetStrategyPreset(id string) (*models.Strategy, error) {
	return a.db.LoadStrategyPreset(id)
}

// ApplyStrategyPreset binds a strategy preset to a workspace slot and updates active engine
func (a *App) ApplyStrategyPreset(workspaceId string, slotId string, strategyId string) error {
	strat, err := a.db.LoadStrategyPreset(strategyId)
	if err != nil {
		return fmt.Errorf("preset %s not found: %w", strategyId, err)
	}

	state := a.GetCockpitState()
	if workspaceId == "" {
		workspaceId = state.ActiveWorkspaceID
		if workspaceId == "" {
			workspaceId = "achat"
		}
	}
	if slotId == "" {
		slotId = "slot_1"
	}

	state.ActiveWorkspaceID = workspaceId

	// Enforce 1/1 concurrency:
	// Enable target workspace and slot, disable all others
	for wIdx := range state.Workspaces {
		ws := &state.Workspaces[wIdx]
		if ws.ID == workspaceId {
			ws.Enabled = true
			slotMatched := false
			for sIdx := range ws.StrategySlots {
				slot := &ws.StrategySlots[sIdx]
				if slot.ID == slotId {
					slot.StrategyID = strategyId
					slot.Enabled = true
					slotMatched = true
				} else {
					slot.Enabled = false
				}
			}
			if !slotMatched {
				if len(ws.StrategySlots) > 0 {
					ws.StrategySlots[0].StrategyID = strategyId
					ws.StrategySlots[0].Enabled = true
				} else {
					ws.StrategySlots = append(ws.StrategySlots, models.CockpitStrategySlot{
						ID:         slotId,
						Label:      "Slot 1",
						StrategyID: strategyId,
						Enabled:    true,
					})
				}
			}
		} else {
			ws.Enabled = false
			for sIdx := range ws.StrategySlots {
				ws.StrategySlots[sIdx].Enabled = false
			}
		}
	}

	_ = a.db.SaveCockpitState(state)

	// Hot reload in Engine
	if a.engine != nil {
		a.engine.SetCockpitState(state)
		a.engine.StopBot()
		_ = a.engine.StartBot(strat)
	}

	return nil
}

// DeleteStrategyPreset removes a preset from the database
func (a *App) DeleteStrategyPreset(id string) error {
	return a.db.DeleteStrategyPreset(id)
}

// ValidateStrategy performs structural validation on strategy rules and indicators
func (a *App) ValidateStrategy(strat *models.Strategy) (*models.ValidationResult, error) {
	var errors []string

	if strat == nil {
		return &models.ValidationResult{Valid: false, Errors: []string{"strategy is nil"}}, nil
	}
	if strat.Name == "" {
		errors = append(errors, "strategy name is required")
	}
	if len(strat.Rules) == 0 {
		errors = append(errors, "strategy must have at least one rule")
	}

	for i, rule := range strat.Rules {
		if rule.Name == "" {
			errors = append(errors, fmt.Sprintf("rule %d: name is required", i))
		}
		if rule.Condition == nil {
			errors = append(errors, fmt.Sprintf("rule %s: condition is missing", rule.Name))
			continue
		}
		if rule.Condition.Type == "" {
			errors = append(errors, fmt.Sprintf("rule %s: condition type is missing", rule.Name))
		}
		if rule.Action != models.ActionBuy && rule.Action != models.ActionSell {
			errors = append(errors, fmt.Sprintf("rule %s: invalid action %q", rule.Name, rule.Action))
		}
	}

	return &models.ValidationResult{
		Valid:  len(errors) == 0,
		Errors: errors,
	}, nil
}
