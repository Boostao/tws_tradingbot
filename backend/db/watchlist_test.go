package db

import (
	"testing"
	"tws_traderbot/backend/models"
)

func TestWatchlistState_Persistence(t *testing.T) {
	db := setupTestDB(t)

	// Save a state
	state := &models.WatchlistResponse{
		Symbols: []string{"AAPL"},
		Groups: []models.WatchlistGroup{
			{ID: "g1", Name: "Tech", Source: "manual", Items: []models.WatchlistItem{
				{Symbol: "AAPL", Exchange: "SMART", Name: "Apple", Enabled: true},
			}},
		},
	}
	if err := db.SaveWatchlistState(state); err != nil {
		t.Fatalf("Failed to save watchlist state: %v", err)
	}

	// Load and verify
	loaded, err := db.LoadWatchlistState()
	if err != nil {
		t.Fatalf("Failed to load watchlist state: %v", err)
	}

	if len(loaded.Symbols) != 1 || loaded.Symbols[0] != "AAPL" {
		t.Errorf("Symbols mismatch: %v", loaded.Symbols)
	}
	if len(loaded.Groups) != 1 || loaded.Groups[0].Name != "Tech" {
		t.Errorf("Groups mismatch: %v", loaded.Groups)
	}
}
