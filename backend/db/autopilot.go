package db

import (
	"database/sql"
	"fmt"
)

// InitAutopilotSchema sets up the required tables for autopilot persistence.
func InitAutopilotSchema(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS autopilot_runs (
			run_id TEXT PRIMARY KEY,
			strategy_id TEXT NOT NULL,
			started_at INTEGER NOT NULL,
			is_active BOOLEAN NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS autopilot_decisions (
			decision_id TEXT PRIMARY KEY NOT NULL,
			run_id TEXT NOT NULL,
			strategy_id TEXT NOT NULL,
			rule_id TEXT NOT NULL,
			instrument_id TEXT NOT NULL,
			timeframe TEXT NOT NULL,
			bar_ts INTEGER NOT NULL,
			action TEXT NOT NULL
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_autopilot_decisions_dedup ON autopilot_decisions (strategy_id, rule_id, instrument_id, timeframe, bar_ts, action);`,
		`CREATE TABLE IF NOT EXISTS autopilot_events (
			seq INTEGER PRIMARY KEY AUTOINCREMENT,
			decision_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			order_id INTEGER,
			signal_price REAL,
			fill_price REAL,
			payload_json TEXT NOT NULL
		);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("failed to create autopilot schema: %w", err)
		}
	}
	return nil
}
