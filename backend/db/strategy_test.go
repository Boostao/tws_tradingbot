package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	"tws_traderbot/backend/models"
)

func TestStrategies_MigrationPinAndOrder(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "legacy_strat.sqlite")

	// 1. Create a legacy 5-column strategies table without pinned/sort_order
	rawConn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw sqlite: %v", err)
	}
	_, err = rawConn.Exec(`
		CREATE TABLE strategies (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			summary JSON,
			data JSON,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO strategies (id, name, summary, data) VALUES
			('s_alpha', 'Alpha', '{"id":"s_alpha","name":"Alpha","rule_count":1,"enabled_rule_count":1}', '{"id":"s_alpha","name":"Alpha","rules":[]}'),
			('s_beta', 'Beta', '{"id":"s_beta","name":"Beta","rule_count":1,"enabled_rule_count":1}', '{"id":"s_beta","name":"Beta","rules":[]}'),
			('s_gamma', 'Gamma', '{"id":"s_gamma","name":"Gamma","rule_count":1,"enabled_rule_count":1}', '{"id":"s_gamma","name":"Gamma","rules":[]}');
	`)
	rawConn.Close()
	if err != nil {
		t.Fatalf("seed legacy table: %v", err)
	}

	// 2. Connect via db.Connect -> runs InitSchema + migrateStrategiesTable
	database, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("db.Connect failed: %v", err)
	}
	defer database.Close()

	// Verify columns exist and sort_order was backfilled 1..3
	lib, err := database.LoadStrategyLibrary()
	if err != nil {
		t.Fatalf("LoadStrategyLibrary failed: %v", err)
	}
	if len(lib) != 3 {
		t.Fatalf("expected 3 strategies, got %d", len(lib))
	}
	if lib[0].ID != "s_alpha" || lib[0].SortOrder != 1 || lib[1].ID != "s_beta" || lib[1].SortOrder != 2 || lib[2].ID != "s_gamma" || lib[2].SortOrder != 3 {
		t.Fatalf("unexpected initial order/sort_order: %+v", lib)
	}

	// 3. Move Gamma up -> Alpha, Gamma, Beta
	if err := database.MoveStrategyOrder("s_gamma", "up"); err != nil {
		t.Fatalf("MoveStrategyOrder up failed: %v", err)
	}
	lib, err = database.LoadStrategyLibrary()
	if err != nil {
		t.Fatalf("LoadStrategyLibrary failed: %v", err)
	}
	if lib[0].ID != "s_alpha" || lib[1].ID != "s_gamma" || lib[2].ID != "s_beta" {
		t.Errorf("expected [s_alpha, s_gamma, s_beta], got [%s, %s, %s]", lib[0].ID, lib[1].ID, lib[2].ID)
	}

	// 4. Pin Beta -> moves to top of library (pinned DESC)
	if err := database.ToggleStrategyPin("s_beta"); err != nil {
		t.Fatalf("ToggleStrategyPin failed: %v", err)
	}
	lib, err = database.LoadStrategyLibrary()
	if err != nil {
		t.Fatalf("LoadStrategyLibrary failed: %v", err)
	}
	if lib[0].ID != "s_beta" || !lib[0].Pinned {
		t.Errorf("expected s_beta pinned at index 0, got %+v", lib[0])
	}

	// 5. SaveStrategyPreset on pinned strategy preserves pinned=true and sort_order
	sum := &models.CockpitStrategySummary{
		ID:               "s_beta",
		Name:             "Beta Updated",
		RuleCount:        1,
		EnabledRuleCount: 1,
		Source:           "preset",
	}
	if err := database.SaveStrategyPreset(&models.Strategy{ID: "s_beta", Name: "Beta Updated"}, sum); err != nil {
		t.Fatalf("SaveStrategyPreset failed: %v", err)
	}
	if !sum.Pinned {
		t.Errorf("expected SaveStrategyPreset to preserve Pinned=true on summary, got %+v", sum)
	}

	// 6. Invalid direction & missing ID errors
	if err := database.MoveStrategyOrder("s_beta", "sideways"); err == nil {
		t.Errorf("expected error for invalid move direction")
	}
	if err := database.ToggleStrategyPin("does_not_exist"); err == nil {
		t.Errorf("expected error for unknown strategy id")
	}
}
