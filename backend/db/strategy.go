package db

import (
	"encoding/json"
	"fmt"
	"tws_traderbot/backend/models"
)

// StrategyCatalogSeeder is an optional hook registered by strategy package to seed default presets
var StrategyCatalogSeeder func(db *Database) error

// LoadStrategyLibrary returns all strategy summaries from SQLite
func (db *Database) LoadStrategyLibrary() ([]models.CockpitStrategySummary, error) {
	rows, err := db.conn.Query("SELECT summary FROM strategies ORDER BY name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []models.CockpitStrategySummary
	for rows.Next() {
		var sumJSON string
		if err := rows.Scan(&sumJSON); err != nil {
			return nil, err
		}
		var s models.CockpitStrategySummary
		if err := json.Unmarshal([]byte(sumJSON), &s); err == nil {
			summaries = append(summaries, s)
		}
	}
	return summaries, nil
}

// SaveStrategyPreset saves or updates a strategy and its summary in SQLite
func (db *Database) SaveStrategyPreset(strat *models.Strategy, summary *models.CockpitStrategySummary) error {
	if strat == nil || summary == nil {
		return fmt.Errorf("strategy and summary cannot be nil")
	}

	stratJSON, err := json.Marshal(strat)
	if err != nil {
		return fmt.Errorf("failed to marshal strategy: %w", err)
	}

	sumJSON, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("failed to marshal summary: %w", err)
	}

	_, err = db.conn.Exec(`
		INSERT INTO strategies (id, name, summary, data, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			summary = excluded.summary,
			data = excluded.data,
			updated_at = CURRENT_TIMESTAMP
	`, strat.ID, strat.Name, string(sumJSON), string(stratJSON))

	return err
}

// LoadStrategyPreset retrieves a single strategy preset by ID
func (db *Database) LoadStrategyPreset(id string) (*models.Strategy, error) {
	var stratJSON string
	err := db.conn.QueryRow("SELECT data FROM strategies WHERE id = ?", id).Scan(&stratJSON)
	if err != nil {
		return nil, fmt.Errorf("strategy preset %s not found: %w", id, err)
	}

	var strat models.Strategy
	if err := json.Unmarshal([]byte(stratJSON), &strat); err != nil {
		return nil, fmt.Errorf("failed to unmarshal strategy preset: %w", err)
	}
	return &strat, nil
}

// DeleteStrategyPreset removes a preset from the database
func (db *Database) DeleteStrategyPreset(id string) error {
	_, err := db.conn.Exec("DELETE FROM strategies WHERE id = ?", id)
	return err
}
