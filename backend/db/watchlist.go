package db

import (
	"encoding/json"
	"fmt"

	"tws_traderbot/backend/models"
)

// In TWS Traderbot, watchlist state was loaded entirely via a JSON block on disk/db
// so we'll store the whole WatchlistResponse array in `system_config` table under `key="watchlist_state"`
// to match the document-store access pattern used in FastAPI for Groups and Feed data.

func (db *Database) LoadWatchlistState() (*models.WatchlistResponse, error) {
	var val string
	err := db.conn.QueryRow("SELECT value FROM system_config WHERE key = 'watchlist_state'").Scan(&val)
	if err != nil {
		return nil, fmt.Errorf("watchlist state not found in system_config: %w", err)
	}

	var state models.WatchlistResponse
	if err := json.Unmarshal([]byte(val), &state); err != nil {
		return nil, err
	}

	return &state, nil
}

func (db *Database) SaveWatchlistState(state *models.WatchlistResponse) error {
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}

	_, err = db.conn.Exec(`
		INSERT INTO system_config (key, value) 
		VALUES ('watchlist_state', ?) 
		ON CONFLICT(key) DO UPDATE SET value=excluded.value
	`, string(b))
	return err
}
