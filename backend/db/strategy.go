package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"tws_traderbot/backend/models"
)

// StrategyCatalogSeeder is an optional hook registered by strategy package to seed default presets
var StrategyCatalogSeeder func(db *Database) error

// LoadStrategyLibrary returns all strategy summaries from SQLite sorted by (pinned DESC, sort_order ASC, name ASC, id ASC).
func (db *Database) LoadStrategyLibrary() ([]models.CockpitStrategySummary, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, summary, pinned, sort_order, updated_at
		FROM strategies
		ORDER BY pinned DESC, sort_order ASC, name ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summaries := make([]models.CockpitStrategySummary, 0)
	for rows.Next() {
		var id, name, sumJSON string
		var pinned bool
		var sortOrder int
		var updatedAt sql.NullString
		if err := rows.Scan(&id, &name, &sumJSON, &pinned, &sortOrder, &updatedAt); err != nil {
			return nil, err
		}
		var s models.CockpitStrategySummary
		if err := json.Unmarshal([]byte(sumJSON), &s); err == nil {
			if s.ID == "" {
				s.ID = id
			}
			if s.Name == "" {
				s.Name = name
			}
			if s.UpdatedAt == "" {
				s.UpdatedAt = updatedAt.String
			}
			s.Pinned = pinned
			s.SortOrder = sortOrder
			summaries = append(summaries, s)
		}
	}
	return summaries, rows.Err()
}

// SaveStrategyPreset saves or updates a strategy and its summary in SQLite while preserving pinned & sort_order on updates.
func (db *Database) SaveStrategyPreset(strat *models.Strategy, summary *models.CockpitStrategySummary) error {
	if strat == nil || summary == nil {
		return fmt.Errorf("strategy and summary cannot be nil")
	}

	sortOrder := summary.SortOrder
	if sortOrder <= 0 {
		var maxOrder int
		if err := db.conn.QueryRow("SELECT COALESCE(MAX(sort_order), 0) FROM strategies").Scan(&maxOrder); err == nil {
			sortOrder = maxOrder + 1
		} else {
			sortOrder = 1
		}
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
		INSERT INTO strategies (id, name, summary, data, pinned, sort_order, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			summary = excluded.summary,
			data = excluded.data,
			updated_at = CURRENT_TIMESTAMP
	`, strat.ID, strat.Name, string(sumJSON), string(stratJSON), summary.Pinned, sortOrder)
	if err != nil {
		return err
	}

	var dbPinned bool
	var dbOrder int
	if err := db.conn.QueryRow("SELECT pinned, sort_order FROM strategies WHERE id = ?", strat.ID).Scan(&dbPinned, &dbOrder); err == nil {
		summary.Pinned = dbPinned
		summary.SortOrder = dbOrder
	}

	return nil
}

// ToggleStrategyPin flips the pinned state of a strategy preset.
func (db *Database) ToggleStrategyPin(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("strategy id cannot be empty")
	}
	res, err := db.conn.Exec("UPDATE strategies SET pinned = NOT pinned, updated_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("strategy %q not found", id)
	}
	return nil
}

// MoveStrategyOrder moves a strategy up or down within its pinned group and normalizes sort_order 1..K.
func (db *Database) MoveStrategyOrder(id string, direction string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("strategy id cannot be empty")
	}
	dir := strings.ToLower(strings.TrimSpace(direction))
	if dir != "up" && dir != "down" {
		return fmt.Errorf("invalid move direction %q: expected \"up\" or \"down\"", direction)
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var pinned bool
	if err := tx.QueryRow("SELECT pinned FROM strategies WHERE id = ?", id).Scan(&pinned); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("strategy %q not found", id)
		}
		return err
	}

	rows, err := tx.Query(`
		SELECT id
		FROM strategies
		WHERE pinned = ?
		ORDER BY sort_order ASC, name ASC, id ASC
	`, pinned)
	if err != nil {
		return err
	}

	var ids []string
	targetIdx := -1
	for rows.Next() {
		var sid string
		if err := rows.Scan(&sid); err != nil {
			rows.Close()
			return err
		}
		if sid == id {
			targetIdx = len(ids)
		}
		ids = append(ids, sid)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	if targetIdx < 0 {
		return fmt.Errorf("strategy %q not found in pin group", id)
	}

	if dir == "up" && targetIdx > 0 {
		ids[targetIdx], ids[targetIdx-1] = ids[targetIdx-1], ids[targetIdx]
	} else if dir == "down" && targetIdx < len(ids)-1 {
		ids[targetIdx], ids[targetIdx+1] = ids[targetIdx+1], ids[targetIdx]
	}

	for i, sid := range ids {
		if sid == id {
			if _, err := tx.Exec("UPDATE strategies SET sort_order = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", i+1, sid); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec("UPDATE strategies SET sort_order = ? WHERE id = ?", i+1, sid); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
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
