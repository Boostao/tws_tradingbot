package db

import (
	"sync"
	"testing"
	"tws_traderbot/backend/models"
)

func TestWatchlistState_Persistence(t *testing.T) {
	db := setupTestDB(t)

	// Save a state
	state := &models.WatchlistResponse{
		ID:   "custom-list",
		Name: "Custom List",
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
	loaded, err := db.GetWatchlistByID("custom-list")
	if err != nil {
		t.Fatalf("Failed to load watchlist state: %v", err)
	}

	if len(loaded.Symbols) != 1 || loaded.Symbols[0] != "AAPL.SMART" {
		t.Errorf("Symbols mismatch, expected [AAPL.SMART], got: %v", loaded.Symbols)
	}
	if len(loaded.Groups) != 1 || loaded.Groups[0].Name != "Tech" {
		t.Errorf("Groups mismatch: %v", loaded.Groups)
	}
}

func TestWatchlist_RelationalSchemaAndCascadingDelete(t *testing.T) {
	db := setupTestDB(t)

	wlID := "test-cascade-wl"
	_, err := db.MergeAndSaveWatchlist(wlID, "Cascade Test", "Desc", nil, []models.WatchlistGroup{
		{
			ID:   "grp1",
			Name: "Group 1",
			Items: []models.WatchlistItem{
				{Symbol: "MSFT", Exchange: "NASDAQ", Enabled: true},
				{Symbol: "GOOGL", Exchange: "NASDAQ", Enabled: true},
			},
		},
	}, false)
	if err != nil {
		t.Fatalf("failed to insert watchlist: %v", err)
	}

	// Check items exist in DB
	var itemCount int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM watchlist_items WHERE watchlist_id = ?", wlID).Scan(&itemCount)
	if err != nil || itemCount != 2 {
		t.Fatalf("expected 2 items, got %d (err: %v)", itemCount, err)
	}

	// Delete parent watchlist
	if err := db.DeleteWatchlist(wlID); err != nil {
		t.Fatalf("failed to delete watchlist: %v", err)
	}

	// Verify CASCADE deleted child items
	err = db.conn.QueryRow("SELECT COUNT(*) FROM watchlist_items WHERE watchlist_id = ?", wlID).Scan(&itemCount)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if itemCount != 0 {
		t.Errorf("expected 0 items after cascade delete, got %d", itemCount)
	}
}

func TestWatchlist_PreSeededCobaltLists(t *testing.T) {
	db := setupTestDB(t)

	summaries, err := db.GetWatchlists()
	if err != nil {
		t.Fatalf("failed to get watchlists: %v", err)
	}

	expectedIDs := map[string]bool{
		"cobalt-telegram":    false,
		"cobalt-home-run":    false,
		"cblt-marches":       false,
		"premarkets":         false,
		"positions-detenues": false,
	}

	for _, s := range summaries {
		if _, ok := expectedIDs[s.ID]; ok {
			expectedIDs[s.ID] = true
			if s.ItemCount == 0 {
				t.Errorf("seed watchlist %s has 0 items", s.ID)
			}
		}
	}

	for id, found := range expectedIDs {
		if !found {
			t.Errorf("expected pre-seeded watchlist %s was not found", id)
		}
	}
}

func TestWatchlist_SymbolNormalizationAndCollisions(t *testing.T) {
	db := setupTestDB(t)

	// Check collision symbols in cobalt-telegram
	wl, err := db.GetWatchlistByID("cobalt-telegram")
	if err != nil {
		t.Fatalf("failed to load cobalt-telegram: %v", err)
	}

	instruments := make(map[string]bool)
	for _, grp := range wl.Groups {
		for _, item := range grp.Items {
			instruments[item.InstrumentID] = true
		}
	}

	// Check 8 documented collision resolutions
	collisions := []string{
		"BTCUSD.BINANCE", "BTCUSD.COINBASE", "BTCUSD.INDEX",
		"ETHUSD.COINBASE", "ETHUSD.INDEX",
		"FETH.TSX", "FETH.CBOE",
		"AQN.TSX", "AQN.NYSE",
		"CNR.TSX", "CNR.NYSE",
		"NFLX.TSX", "NFLX.NASDAQ",
		"MDA.TSX", "MDA.NYSE",
	}

	for _, c := range collisions {
		if !instruments[c] {
			t.Errorf("expected collision-disambiguated instrument %s in cobalt-telegram", c)
		}
	}

	// Check SPX collision in cblt-marches
	marches, err := db.GetWatchlistByID("cblt-marches")
	if err != nil {
		t.Fatalf("failed to load cblt-marches: %v", err)
	}
	marchesInstruments := make(map[string]bool)
	for _, grp := range marches.Groups {
		for _, item := range grp.Items {
			marchesInstruments[item.InstrumentID] = true
			if item.InstrumentID == "VIX.TVC" || item.InstrumentID == "SPX.TVC" || item.InstrumentID == "TSX.TSX" {
				if item.IsOrderable {
					t.Errorf("expected %s to have IsOrderable=false", item.InstrumentID)
				}
				if !item.IsRegimeReference {
					t.Errorf("expected %s to have IsRegimeReference=true", item.InstrumentID)
				}
			}
		}
	}
	if !marchesInstruments["SPX.TVC"] {
		t.Errorf("expected SPX.TVC in cblt-marches")
	}
}

func TestWatchlist_SmartDiffMerge(t *testing.T) {
	db := setupTestDB(t)
	wlID := "test-diff-merge"

	// 1. Initial import
	initialGroups := []models.WatchlistGroup{
		{
			ID:   "g1",
			Name: "Group 1",
			Items: []models.WatchlistItem{
				{Symbol: "AAPL", Exchange: "NASDAQ", Enabled: true, Tier: "1-Blue Chip", SizeMode: "shares", SizeValue: 100},
				{Symbol: "TSLA", Exchange: "NASDAQ", Enabled: true, Tier: "2-Croissance", SizeMode: "shares", SizeValue: 50},
			},
		},
	}
	_, err := db.MergeAndSaveWatchlist(wlID, "Test Diff Merge", "Desc", nil, initialGroups, false)
	if err != nil {
		t.Fatalf("initial merge failed: %v", err)
	}

	// 2. Operator toggles AAPL OFF and changes sizing to $5000, and flags TSLA as holding ($$$$)
	_, err = db.conn.Exec(`
		UPDATE watchlist_items
		SET enabled = FALSE, size_mode = 'dollar', size_value = 5000.0, tier = '4-Rejeté'
		WHERE watchlist_id = ? AND instrument_id = 'AAPL.NASDAQ'
	`, wlID)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	_, err = db.conn.Exec(`
		UPDATE watchlist_items
		SET is_holding = TRUE
		WHERE watchlist_id = ? AND instrument_id = 'TSLA.NASDAQ'
	`, wlID)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	// 3. Incoming feed update re-imports AAPL, TSLA and a new ticker NVDA with default enabled=true
	incomingFeed := []models.WatchlistGroup{
		{
			ID:   "g1",
			Name: "Group 1",
			Items: []models.WatchlistItem{
				{Symbol: "AAPL", Exchange: "NASDAQ", Enabled: true, Tier: "1-Blue Chip", SizeMode: "shares", SizeValue: 100},
				{Symbol: "TSLA", Exchange: "NASDAQ", Enabled: true, Tier: "2-Croissance", SizeMode: "shares", SizeValue: 50},
				{Symbol: "NVDA", Exchange: "NASDAQ", Enabled: true, Tier: "1-Blue Chip", SizeMode: "shares", SizeValue: 100},
			},
		},
	}
	merged, err := db.MergeAndSaveWatchlist(wlID, "Test Diff Merge", "Desc", nil, incomingFeed, true)
	if err != nil {
		t.Fatalf("second merge failed: %v", err)
	}

	// Verify AAPL overrides are PRESERVED: enabled=false, size_mode='dollar', size_value=5000, tier='4-Rejeté'
	var aaplItem, tslaItem, nvdaItem *models.WatchlistItem
	for i := range merged.Groups[0].Items {
		item := &merged.Groups[0].Items[i]
		if item.InstrumentID == "AAPL.NASDAQ" {
			aaplItem = item
		} else if item.InstrumentID == "TSLA.NASDAQ" {
			tslaItem = item
		} else if item.InstrumentID == "NVDA.NASDAQ" {
			nvdaItem = item
		}
	}

	if aaplItem == nil {
		t.Fatalf("AAPL.NASDAQ not found in merged list")
	}
	if aaplItem.Enabled {
		t.Errorf("AAPL.NASDAQ should have remained DISABLED (enabled=false)")
	}
	if aaplItem.SizeMode != "dollar" || aaplItem.SizeValue != 5000.0 {
		t.Errorf("AAPL.NASDAQ sizing override lost: mode=%s, val=%f", aaplItem.SizeMode, aaplItem.SizeValue)
	}
	if aaplItem.Tier != "4-Rejeté" {
		t.Errorf("AAPL.NASDAQ tier override lost, expected 4-Rejeté, got %s", aaplItem.Tier)
	}

	if tslaItem == nil {
		t.Fatalf("TSLA.NASDAQ not found in merged list")
	}
	if !tslaItem.IsHolding {
		t.Errorf("TSLA.NASDAQ is_holding flag should have been preserved as true")
	}

	if nvdaItem == nil {
		t.Fatalf("NVDA.NASDAQ not found in merged list")
	}
	if !nvdaItem.Enabled {
		t.Errorf("new ticker NVDA.NASDAQ should be enabled by default")
	}
}

func TestWatchlist_SaveWatchlistState_RetainsDisabled(t *testing.T) {
	db := setupTestDB(t)
	wlID := "test-operator-save"

	// 1. Initial watchlist with an enabled item
	initialState := &models.WatchlistResponse{
		ID:   wlID,
		Name: "Operator Save Test",
		Groups: []models.WatchlistGroup{
			{
				ID:   "tech",
				Name: "Tech Stocks",
				Items: []models.WatchlistItem{
					{
						Symbol:    "AAPL",
						Exchange:  "NASDAQ",
						Name:      "Apple Inc",
						Enabled:   true,
						Tier:      "2-Croissance",
						SizeMode:  "shares",
						SizeValue: 100.0,
						IsHolding: false,
					},
				},
			},
		},
	}
	if err := db.SaveWatchlistState(initialState); err != nil {
		t.Fatalf("initial SaveWatchlistState failed: %v", err)
	}

	// Verify it was stored as enabled
	loaded, err := db.GetWatchlistByID(wlID)
	if err != nil {
		t.Fatalf("failed to load watchlist: %v", err)
	}
	if len(loaded.Groups) != 1 || len(loaded.Groups[0].Items) != 1 {
		t.Fatalf("unexpected structure in loaded watchlist: %+v", loaded)
	}
	if !loaded.Groups[0].Items[0].Enabled {
		t.Fatalf("expected item to start as enabled")
	}

	// 2. Operator modifies state: toggles Enabled to FALSE, changes tier, sizing, and holding
	modifiedState := &models.WatchlistResponse{
		ID:   wlID,
		Name: "Operator Save Test",
		Groups: []models.WatchlistGroup{
			{
				ID:   "tech",
				Name: "Tech Stocks",
				Items: []models.WatchlistItem{
					{
						Symbol:    "AAPL",
						Exchange:  "NASDAQ",
						Name:      "Apple Inc",
						Enabled:   false, // Operator disabled this item
						Tier:      "1-Blue Chip",
						SizeMode:  "dollar",
						SizeValue: 5000.0,
						IsHolding: true,
					},
				},
			},
		},
	}
	if err := db.SaveWatchlistState(modifiedState); err != nil {
		t.Fatalf("second SaveWatchlistState failed: %v", err)
	}

	// 3. Reload from DB and verify that Enabled = FALSE was retained (not reverted to previous true!)
	reloaded, err := db.GetWatchlistByID(wlID)
	if err != nil {
		t.Fatalf("failed to reload watchlist: %v", err)
	}

	item := reloaded.Groups[0].Items[0]
	if item.Enabled {
		t.Errorf("FAIL: SaveWatchlistState reverted Enabled=false back to Enabled=true!")
	}
	if item.Tier != "1-Blue Chip" {
		t.Errorf("FAIL: SaveWatchlistState reverted Tier to %s, expected 1-Blue Chip", item.Tier)
	}
	if item.SizeMode != "dollar" || item.SizeValue != 5000.0 {
		t.Errorf("FAIL: SaveWatchlistState reverted sizing: mode=%s, val=%f", item.SizeMode, item.SizeValue)
	}
	if !item.IsHolding {
		t.Errorf("FAIL: SaveWatchlistState reverted IsHolding to false, expected true")
	}
}

func TestSQLite_ConnectionPoolPRAGMAInheritance(t *testing.T) {
	db, err := Connect(":memory:")
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	var wg sync.WaitGroup
	var offCount int
	var mu sync.Mutex

	// Test 20 concurrent goroutines querying PRAGMA foreign_keys
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var fk int
			if err := db.conn.QueryRow("PRAGMA foreign_keys;").Scan(&fk); err != nil {
				t.Errorf("query failed: %v", err)
				return
			}
			if fk == 0 {
				mu.Lock()
				offCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if offCount > 0 {
		t.Errorf("CRITICAL DEFECT: %d/20 pooled connections ran with foreign_keys = 0", offCount)
	}
}
