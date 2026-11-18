package strategy_test

import (
	"strings"
	"testing"

	"tws_traderbot/backend/db"
	"tws_traderbot/backend/strategy"
)

func TestCatalog_20Presets(t *testing.T) {
	// Feature 35: 20 Cobalt & Screener Presets
	presets := strategy.GetDefaultStrategySeeds()
	if len(presets) != 21 {
		t.Fatalf("expected exactly 21 presets, got %d", len(presets))
	}

	chartModelCount := 0
	screenerCount := 0
	coursCount := 0

	for _, p := range presets {
		if p.ID == "" {
			t.Errorf("preset missing ID: %+v", p)
		}
		if p.Name == "" {
			t.Errorf("preset %s missing Name", p.ID)
		}
		if len(p.Rules) == 0 {
			t.Errorf("preset %s has no rules", p.ID)
		}
		for _, r := range p.Rules {
			if r.Condition == nil {
				t.Errorf("preset %s rule %s has nil condition", p.ID, r.Name)
			}
		}

		if strings.HasPrefix(p.ID, "style_") {
			chartModelCount++
		} else if strings.HasPrefix(p.ID, "screener_") {
			screenerCount++
		} else if strings.HasPrefix(p.ID, "cours_") {
			coursCount++
		}
	}

	if chartModelCount != 13 {
		t.Errorf("expected 13 chart models, got %d", chartModelCount)
	}
	if screenerCount != 4 {
		t.Errorf("expected 4 screener presets, got %d", screenerCount)
	}
	if coursCount != 3 {
		t.Errorf("expected 3 cours setups, got %d", coursCount)
	}
}

func TestCatalog_DatabaseSeeding(t *testing.T) {
	// In-memory SQLite database test
	database, err := db.Connect(":memory:")
	if err != nil {
		t.Fatalf("failed to connect in-memory sqlite: %v", err)
	}
	defer database.Close()

	// Seed catalog
	err = strategy.SeedStrategyCatalog(database)
	if err != nil {
		t.Fatalf("failed to seed strategy catalog: %v", err)
	}

	lib, err := database.LoadStrategyLibrary()
	if err != nil {
		t.Fatalf("failed to load strategy library: %v", err)
	}

	if len(lib) != 21 {
		t.Fatalf("expected 21 strategies in database, got %d", len(lib))
	}

	// Idempotency: re-seeding should be a no-op
	err = strategy.SeedStrategyCatalog(database)
	if err != nil {
		t.Fatalf("re-seeding failed: %v", err)
	}

	libAfter, _ := database.LoadStrategyLibrary()
	if len(libAfter) != 21 {
		t.Fatalf("expected 21 strategies after idempotent re-seeding, got %d", len(libAfter))
	}

	// Verify loading individual preset
	strat, err := database.LoadStrategyPreset("screener_golden_cross")
	if err != nil || strat == nil || strat.ID != "screener_golden_cross" {
		t.Fatalf("failed to load specific preset 'screener_golden_cross': %v", err)
	}
}
