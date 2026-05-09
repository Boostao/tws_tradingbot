package db

import (
	"testing"
	"tws_traderbot_go/backend/models"
)

func setupTestDB(t *testing.T) *Database {
	db, err := Connect(":memory:")
	if err != nil {
		t.Fatalf("Failed to connect to in-memory db: %v", err)
	}
	return db
}

func TestCockpitState_Persistence(t *testing.T) {
	db := setupTestDB(t)

	// 1. Loading empty should fail gracefully or error because it doesn't exist
	_, err := db.LoadCockpitState()
	if err == nil {
		t.Fatal("Expected error when loading missing cockpit state, got nil")
	}

	// 2. Save a state
	state := &models.CockpitStateResponse{
		GlobalEnabled: true,
		Workspaces: []models.CockpitWorkspace{
			{ID: "ws1", Name: "Test Workspace", Kind: "long", Enabled: true},
		},
	}
	if err := db.SaveCockpitState(state); err != nil {
		t.Fatalf("Failed to save cockpit state: %v", err)
	}

	// 3. Load the state and verify
	loaded, err := db.LoadCockpitState()
	if err != nil {
		t.Fatalf("Failed to load cockpit state: %v", err)
	}

	if !loaded.GlobalEnabled {
		t.Errorf("Expected GlobalEnabled=true")
	}
	if len(loaded.Workspaces) != 1 || loaded.Workspaces[0].ID != "ws1" {
		t.Errorf("Loaded workspaces mismatched: %+v", loaded.Workspaces)
	}

	// 4. Update existing (UPSERT test)
	state.GlobalEnabled = false
	if err := db.SaveCockpitState(state); err != nil {
		t.Fatalf("Failed to update cockpit state: %v", err)
	}

	loaded2, _ := db.LoadCockpitState()
	if loaded2.GlobalEnabled {
		t.Errorf("Expected GlobalEnabled=false after upsert")
	}
}
