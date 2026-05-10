package main

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
	return update, nil
}
