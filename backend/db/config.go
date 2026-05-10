package db

import (
	"encoding/json"
	"fmt"
)

func (db *Database) LoadSystemConfig(key string) (map[string]interface{}, error) {
	var val string
	err := db.conn.QueryRow("SELECT value FROM system_config WHERE key = ?", key).Scan(&val)
	if err != nil {
		return nil, fmt.Errorf("%s not found in system_config: %w", key, err)
	}

	var state map[string]interface{}
	if err := json.Unmarshal([]byte(val), &state); err != nil {
		return nil, err
	}

	return state, nil
}

func (db *Database) SaveSystemConfig(key string, state map[string]interface{}) error {
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}

	_, err = db.conn.Exec(`
		INSERT INTO system_config (key, value) 
		VALUES (?, ?) 
		ON CONFLICT(key) DO UPDATE SET value=excluded.value
	`, key, string(b))
	return err
}
