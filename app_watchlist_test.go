package main
package main

import (
	"encoding/json"
	"testing"

	"tws_traderbot/backend/models"
)

// watchlistSample returns a representative WatchlistResponse for use in tests.
func watchlistSample() *models.WatchlistResponse {
	return &models.WatchlistResponse{
		Symbols: []string{"AAPL", "MSFT"},
		Groups: []models.WatchlistGroup{
			{
				ID:     "manual",
				Name:   "Manual",
				Source: "manual",
				Items: []models.WatchlistItem{
					{Symbol: "AAPL", Exchange: "SMART", Name: "Apple Inc.", Enabled: true},
					{Symbol: "MSFT", Exchange: "SMART", Name: "Microsoft Corp.", Enabled: false},
				},
			},
		},
		Feed: &models.WatchlistFeed{
			Provider:        "tradingview",
			URL:             "https://example.com/watchlist",
			Title:           "My List",
			ExternalID:      "12345",
			LastRefreshedAt: "2024-01-01T00:00:00Z",
		},
	}
}

// TestExportWatchlistFile_Serialization verifies that a WatchlistResponse serialises
// to valid JSON and that the exported bytes faithfully round-trip back to the original structure.
func TestExportWatchlistFile_Serialization(t *testing.T) {
	original := watchlistSample()

	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("serialised JSON is empty")
	}

	// Round-trip
	var decoded models.WatchlistResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal of exported JSON failed: %v", err)
	}

	if len(decoded.Symbols) != len(original.Symbols) {
		t.Errorf("symbols length mismatch: got %d want %d", len(decoded.Symbols), len(original.Symbols))
	}
	for i, s := range original.Symbols {
		if decoded.Symbols[i] != s {
			t.Errorf("symbol[%d] mismatch: got %q want %q", i, decoded.Symbols[i], s)
		}
	}

	if len(decoded.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(decoded.Groups))
	}
	g := decoded.Groups[0]
	if g.ID != "manual" {
		t.Errorf("group ID: got %q want \"manual\"", g.ID)
	}
	if len(g.Items) != 2 {
		t.Fatalf("expected 2 items in group, got %d", len(g.Items))
	}
	if g.Items[0].Symbol != "AAPL" || !g.Items[0].Enabled {
		t.Errorf("first item mismatch: %+v", g.Items[0])
	}
	if g.Items[1].Symbol != "MSFT" || g.Items[1].Enabled {
		t.Errorf("second item mismatch: %+v", g.Items[1])
	}

	if decoded.Feed == nil {
		t.Fatal("feed is nil after round-trip")
	}
	if decoded.Feed.Provider != "tradingview" {
		t.Errorf("feed provider: got %q want \"tradingview\"", decoded.Feed.Provider)
	}
}

// TestImportWatchlistFile_ParseValidJSON confirms that a valid JSON payload is parsed
// into a WatchlistResponse without errors (models the import path before the DB write).
func TestImportWatchlistFile_ParseValidJSON(t *testing.T) {
	original := watchlistSample()
	data, _ := json.Marshal(original)

	var parsed models.WatchlistResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed on valid JSON: %v", err)
	}

	if len(parsed.Groups) == 0 {
		t.Error("expected at least one group after parsing")
	}
}

// TestImportWatchlistFile_ParseInvalidJSON confirms that malformed JSON produces an error.
func TestImportWatchlistFile_ParseInvalidJSON(t *testing.T) {
	bad := []byte(`{not valid json}`)

	var parsed models.WatchlistResponse
	if err := json.Unmarshal(bad, &parsed); err == nil {
		t.Error("expected an error for invalid JSON, got nil")
	}
}

// TestImportWatchlistFile_EmptyFile confirms that an empty byte slice is rejected.
func TestImportWatchlistFile_EmptyFile(t *testing.T) {
	var parsed models.WatchlistResponse
	if err := json.Unmarshal([]byte{}, &parsed); err == nil {
		t.Error("expected an error for empty input, got nil")
	}
}
