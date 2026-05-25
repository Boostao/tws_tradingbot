package main

import (
	"encoding/json"
	"os"
	"tws_traderbot/backend/models"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// GetConfig returns the system configuration from SQLite
func (a *App) GetConfig() map[string]interface{} {
	state, err := a.db.LoadSystemConfig("config")
	if err == nil && state != nil {
		return state
	}

	// Default initialization if none exists
	defaultState := map[string]interface{}{}
	_ = a.db.SaveSystemConfig("config", defaultState)
	return defaultState
}

// UpdateConfig updates the system configuration
func (a *App) UpdateConfig(update map[string]interface{}) (map[string]interface{}, error) {
	state := a.GetConfig()

	// Merge updates
	for k, v := range update {
		state[k] = v
	}

	err := a.db.SaveSystemConfig("config", state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

// GetStrategy returns the active strategy state from SQLite
func (a *App) GetStrategy() map[string]interface{} {
	state, err := a.db.LoadSystemConfig("strategy")
	if err == nil && state != nil {
		return state
	}

	defaultState := map[string]interface{}{}
	_ = a.db.SaveSystemConfig("strategy", defaultState)
	return defaultState
}

// UpdateStrategy updates the active strategy state
func (a *App) UpdateStrategy(update map[string]interface{}) (map[string]interface{}, error) {
	err := a.db.SaveSystemConfig("strategy", update)
	if err != nil {
		return nil, err
	}

	// Convert to JSON and parse as models.Strategy to hot-reload in memory
	b, err := json.Marshal(update)
	if err == nil {
		var s models.Strategy
		if err := json.Unmarshal(b, &s); err == nil {
			if a.engine != nil {
				a.engine.ActiveStrategy = &s
			}
		}
	}

	return update, nil
}

// ImportStrategyFile allows user to pick a Strategy JSON file and load it
func (a *App) ImportStrategyFile() (map[string]interface{}, error) {
	selectedPath, err := a.app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:   "Import Strategy JSON",
		Filters: []application.FileFilter{{DisplayName: "JSON Files", Pattern: "*.json"}},
	}).PromptForSingleSelection()
	if err != nil || selectedPath == "" {
		return nil, err
	}

	data, err := os.ReadFile(selectedPath)
	if err != nil {
		return nil, err
	}

	var strategy map[string]interface{}
	if err := json.Unmarshal(data, &strategy); err != nil {
		return nil, err
	}

	return a.UpdateStrategy(strategy)
}

// ExportStrategyFile allows user to save current strategy to disk
func (a *App) ExportStrategyFile(strategy map[string]interface{}) error {
	selectedPath, err := a.app.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:    "Export Strategy JSON",
		Filename: "strategy.json",
		Filters:  []application.FileFilter{{DisplayName: "JSON Files", Pattern: "*.json"}},
	}).PromptForSingleSelection()
	if err != nil || selectedPath == "" {
		return err
	}

	data, err := json.MarshalIndent(strategy, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(selectedPath, data, 0644)
}
