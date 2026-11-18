package bot

import (
	"path/filepath"
	"sync"
	"testing"

	"tws_traderbot/backend/db"
)

func TestDryRunGuard_RecordsSimulatedFillInSQLite(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_ledger.sqlite")

	database, err := db.Connect(dbPath)
	if err != nil {
		t.Fatalf("failed to connect sqlite: %v", err)
	}
	defer database.Close()

	eng, _ := setupFullyArmedEngine()
	eng.AttachDB(database)
	eng.SetDryRun(true) // Explicit dry-run mode

	wrapper := NewIBWrapper(eng)
	wrapper.client = nil // Nil client: if PlaceOrder is ever called, it would panic or fail
	wrapper.NextValidID(1000)

	// Execute action under dry-run
	wrapper.ExecuteAction("AAPL", "BUY")

	// Verify trade was recorded in SQLite trade_ledger
	trades, err := database.GetTradesByMode("dry_run")
	if err != nil {
		t.Fatalf("failed to query dry_run trades: %v", err)
	}
	if len(trades) != 1 {
		t.Fatalf("expected 1 dry-run trade record, got %d", len(trades))
	}

	trade := trades[0]
	if trade.OrderID != 1000 {
		t.Errorf("expected OrderID 1000, got %d", trade.OrderID)
	}
	if trade.Symbol != "AAPL" {
		t.Errorf("expected Symbol AAPL, got %s", trade.Symbol)
	}
	if trade.Action != "BUY" {
		t.Errorf("expected Action BUY, got %s", trade.Action)
	}
	if trade.Status != "SIMULATED_FILL" {
		t.Errorf("expected Status SIMULATED_FILL, got %s", trade.Status)
	}
	if trade.Mode != "dry_run" {
		t.Errorf("expected Mode dry_run, got %s", trade.Mode)
	}
	if trade.Price != 150.0 {
		t.Errorf("expected Price 150.0, got %f", trade.Price)
	}
	if trade.Quantity != 100 {
		t.Errorf("expected Quantity 100, got %d", trade.Quantity)
	}

	// Verify recent trades query
	recent, err := database.GetRecentTrades(10)
	if err != nil {
		t.Fatalf("failed to query recent trades: %v", err)
	}
	if len(recent) != 1 {
		t.Errorf("expected 1 recent trade, got %d", len(recent))
	}
}

func TestDryRun_BracketOCAGroupRecorded(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_bracket_ledger.sqlite")

	database, err := db.Connect(dbPath)
	if err != nil {
		t.Fatalf("failed to connect sqlite: %v", err)
	}
	defer database.Close()

	eng, _ := setupFullyArmedEngine()
	eng.AttachDB(database)
	eng.SetDryRun(true)
	eng.SetBracketConfig(true, 3.0, 1.5) // Enable brackets

	wrapper := NewIBWrapper(eng)
	wrapper.client = nil
	wrapper.NextValidID(5000)

	wrapper.ExecuteAction("AAPL", "BUY")

	trades, err := database.GetTradesByMode("dry_run")
	if err != nil || len(trades) != 1 {
		t.Fatalf("expected 1 trade, err=%v, count=%d", err, len(trades))
	}

	trade := trades[0]
	if trade.OCAGroup != "OCA_5000" {
		t.Errorf("expected OCAGroup OCA_5000, got %q", trade.OCAGroup)
	}
}

func TestAtomicNextValidID_ConcurrentRaceSafety(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_race_ledger.sqlite")

	database, err := db.Connect(dbPath)
	if err != nil {
		t.Fatalf("failed to connect sqlite: %v", err)
	}
	defer database.Close()

	eng, _ := setupFullyArmedEngine()
	eng.AttachDB(database)
	eng.SetDryRun(true)

	wrapper := NewIBWrapper(eng)
	wrapper.client = nil
	const startID = int64(7000)
	wrapper.NextValidID(startID)

	const goroutines = 32
	const ordersPerRoutine = 5
	var wg sync.WaitGroup

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for o := 0; o < ordersPerRoutine; o++ {
				wrapper.ExecuteAction("AAPL", "BUY")
			}
		}()
	}

	wg.Wait()

	expectedTotalOrders := goroutines * ordersPerRoutine
	finalOrderID := wrapper.nextOrderID.Load()
	if finalOrderID != startID+int64(expectedTotalOrders) {
		t.Errorf("expected final order ID %d, got %d", startID+int64(expectedTotalOrders), finalOrderID)
	}

	trades, err := database.GetTradesByMode("dry_run")
	if err != nil {
		t.Fatalf("failed to query trades: %v", err)
	}
	if len(trades) != expectedTotalOrders {
		t.Errorf("expected %d trades recorded in database, got %d", expectedTotalOrders, len(trades))
	}

	// Verify all OrderIDs are unique
	seenIDs := make(map[int64]bool)
	for _, tr := range trades {
		if seenIDs[tr.OrderID] {
			t.Errorf("duplicate OrderID detected: %d", tr.OrderID)
		}
		seenIDs[tr.OrderID] = true
	}
}
