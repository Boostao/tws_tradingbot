package db

import (
	"encoding/json"
	"fmt"

	"tws_traderbot/backend/models"
)

func (db *Database) LoadCockpitState() (*models.CockpitStateResponse, error) {
	var val string
	err := db.conn.QueryRow("SELECT value FROM system_config WHERE key = 'cockpit_state'").Scan(&val)
	if err != nil {
		return nil, fmt.Errorf("cockpit state not found in system_config: %w", err)
	}

	var state models.CockpitStateResponse
	if err := json.Unmarshal([]byte(val), &state); err != nil {
		return nil, err
	}

	return &state, nil
}

func (db *Database) SaveCockpitState(state *models.CockpitStateResponse) error {
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}

	_, err = db.conn.Exec(`
		INSERT INTO system_config (key, value) 
		VALUES ('cockpit_state', ?) 
		ON CONFLICT(key) DO UPDATE SET value=excluded.value
	`, string(b))
	return err
}
