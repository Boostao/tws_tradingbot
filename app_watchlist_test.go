package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"tws_traderbot/backend/db"
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

// TestRefreshWatchlistFeed_PreservesTargetWatchlistID verifies that RefreshWatchlistFeed
// retains the target watchlist ID (e.g. cobalt-telegram) instead of clobbering it with the
// TradingView numeric feed ID (e.g. 209453826), and preserves existing operator overrides.
func TestRefreshWatchlistFeed_PreservesTargetWatchlistID(t *testing.T) {
	// 1. Mock TradingView server responding with HTML containing embedded JSON payload
	mockHTML := `<html><body>
	<script type="application/prs.init-data+json">
	{
		"sharedWatchlist": {
			"list": {
				"id": 209453826,
				"name": "✝️@ Cobalt- TELEGRAM",
				"symbols": ["NASDAQ:AAPL", "TSX:SHOP"]
			}
		}
	}
	</script>
	</body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, mockHTML)
	}))
	defer ts.Close()

	// 2. Setup database and mock App
	database, err := db.Connect(":memory:")
	if err != nil {
		t.Fatalf("failed to connect to memory db: %v", err)
	}

	app := &App{db: database}

	// 3. Setup target watchlist "cobalt-telegram" with feed pointing to mock server
	// and AAPL disabled by operator (Enabled = false)
	targetID := "cobalt-telegram"
	initialFeed := &models.WatchlistFeed{
		Provider: "tradingview",
		URL:      ts.URL,
	}
	initialGroups := []models.WatchlistGroup{
		{
			ID:   "default",
			Name: "Default",
			Items: []models.WatchlistItem{
				{
					Symbol:    "AAPL",
					Exchange:  "NASDAQ",
					Enabled:   false, // Operator override
					Tier:      "1-Blue Chip",
					SizeMode:  "dollar",
					SizeValue: 2500.0,
				},
			},
		},
	}
	if _, err := database.MergeAndSaveWatchlist(targetID, "✝️@ Cobalt- TELEGRAM", "", initialFeed, initialGroups, false); err != nil {
		t.Fatalf("setup target watchlist failed: %v", err)
	}

	// 4. Call RefreshWatchlistFeed with target watchlist ID
	refreshed, err := app.RefreshWatchlistFeed(targetID)
	if err != nil {
		t.Fatalf("RefreshWatchlistFeed failed: %v", err)
	}

	// 5. Verify the target ID was preserved (NOT replaced with numeric TradingView ID "209453826")
	if refreshed.ID != targetID {
		t.Errorf("expected refreshed ID to be %q, got %q", targetID, refreshed.ID)
	}
	if refreshed.Feed == nil || refreshed.Feed.ExternalID != "209453826" {
		t.Errorf("expected ExternalID to be \"209453826\", got %+v", refreshed.Feed)
	}

	// 6. Verify that AAPL operator overrides were preserved across feed refresh
	var aaplItem *models.WatchlistItem
	for _, grp := range refreshed.Groups {
		for i := range grp.Items {
			if grp.Items[i].InstrumentID == "AAPL.NASDAQ" {
				aaplItem = &grp.Items[i]
				break
			}
		}
	}
	if aaplItem == nil {
		t.Fatalf("AAPL.NASDAQ not found in refreshed watchlist")
	}
	if aaplItem.Enabled {
		t.Errorf("AAPL.NASDAQ operator override lost: expected Enabled=false, got true")
	}
	if aaplItem.SizeMode != "dollar" || aaplItem.SizeValue != 2500.0 {
		t.Errorf("AAPL.NASDAQ sizing override lost: mode=%s, val=%f", aaplItem.SizeMode, aaplItem.SizeValue)
	}

	// 7. Verify no duplicate list was created with the numeric ID "209453826"
	if _, err := database.GetWatchlistByID("209453826"); err == nil {
		t.Errorf("expected numeric TradingView ID \"209453826\" to NOT exist as a separate watchlist")
	}
}
