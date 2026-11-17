package e2e

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"tws_traderbot/backend/bot"
	"tws_traderbot/backend/db"
	"tws_traderbot/backend/models"
	"tws_traderbot/backend/strategy"

	"github.com/scmhub/ibapi"
)

func newV2TestEnv(t *testing.T) (*db.Database, *bot.Engine, *bot.IBKRClient, *bot.IBWrapper) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "v2_e2e.sqlite")
	database, err := db.Connect(dbPath)
	if err != nil {
		t.Fatalf("db.Connect(%q) failed: %v", dbPath, err)
	}
	database.Conn().SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = database.Close()
	})

	if err := strategy.SeedStrategyCatalog(database); err != nil {
		t.Fatalf("strategy.SeedStrategyCatalog failed: %v", err)
	}

	engine := bot.NewEngine()
	engine.AttachDB(database)

	client := bot.NewIBKRClient(engine)
	wrapper := client.Wrapper()
	if wrapper == nil {
		t.Fatalf("client.Wrapper() returned nil")
	}

	return database, engine, client, wrapper
}

func assertOrderInvariant(t *testing.T, lib []models.CockpitStrategySummary) {
	t.Helper()
	for i := 1; i < len(lib); i++ {
		prev := lib[i-1]
		curr := lib[i]
		if !prev.Pinned && curr.Pinned {
			t.Fatalf("strategy stack order invariant violated at %d: unpinned %q before pinned %q", i, prev.ID, curr.ID)
		}
		if prev.Pinned == curr.Pinned {
			if prev.SortOrder > curr.SortOrder {
				t.Fatalf("strategy stack sort_order violated at %d: %q (%d) before %q (%d)",
					i, prev.ID, prev.SortOrder, curr.ID, curr.SortOrder)
			}
			if prev.SortOrder == curr.SortOrder && prev.Name > curr.Name {
				t.Fatalf("strategy stack tie-break name order violated at %d: %q before %q", i, prev.Name, curr.Name)
			}
		}
	}
}

// =============================================================================
// Tier 1: Isolated Feature Coverage (R2, R4-be, R5-be, R7-validate)
// =============================================================================

func connectMockTWSSocket(t *testing.T, client *bot.IBKRClient) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen failed: %v", err)
	}

	// Keep the accepted mock TWS socket open and drained until process exit rather than closing it
	// asynchronously in t.Cleanup: closing the TCP fd causes ibapi's EReader EOF -> watchOnce ->
	// EClient.Disconnect() -> EClient.reset() (unsynchronized write to EClient.conn), which TSan
	// flags as racing with prior EClient.IsConnected() reads because TCP EOF has no Go happens-before edge.
	go func() {
		conn, err := ln.Accept()
		_ = ln.Close()
		if err != nil {
			return
		}
		hdr := make([]byte, 8)
		if _, err := io.ReadFull(conn, hdr); err != nil {
			_ = conn.Close()
			return
		}
		vLen := binary.BigEndian.Uint32(hdr[4:8])
		if vLen > 0 && vLen < 4096 {
			vBuf := make([]byte, vLen)
			_, _ = io.ReadFull(conn, vBuf)
		}
		payload := []byte("176\x0020261117 10:00:00 EST\x00")
		frame := make([]byte, 4+len(payload))
		binary.BigEndian.PutUint32(frame[:4], uint32(len(payload)))
		copy(frame[4:], payload)
		if _, err := conn.Write(frame); err != nil {
			_ = conn.Close()
			return
		}
		_, _ = io.Copy(io.Discard, conn)
	}()

	addr := ln.Addr().(*net.TCPAddr)
	if err := client.Connect("127.0.0.1", addr.Port, 1); err != nil {
		_ = ln.Close()
		t.Fatalf("client.Connect to mock TWS socket failed: %v", err)
	}
}

func TestV2_Tier1_R2_LiveTelemetryCapture(t *testing.T) {
	_, engine, client, w := newV2TestEnv(t)

	// V2-T1-F02-01: ConnectAck + ManagedAccounts dynamically captures account ID
	connectMockTWSSocket(t, client)
	w.ManagedAccounts([]string{"DU998877"})

	// V2-T1-F02-02: AccountSummary updates NetLiquidation, CashUSD, CashCAD, BuyingPower
	w.AccountSummary(1, "DU998877", "NetLiquidation", "254321.50", "USD")
	w.AccountSummary(1, "DU998877", "TotalCashValue", "150000.00", "USD")
	w.AccountSummary(1, "DU998877", "TotalCashValue", "42000.00", "CAD")
	w.AccountSummary(1, "DU998877", "BuyingPower", "500000.00", "USD")
	w.AccountSummaryEnd(1)

	// V2-T1-F02-05: Pnl updates Daily/Realized/Unrealized PnL
	w.Pnl(1, 320.50, 120.00, 200.50)

	// V2-T1-F02-03: Position + PositionEnd + UpdatePortfolio populate OpenPositions
	googl := &ibapi.Contract{Symbol: "GOOGL", SecType: "STK", Exchange: "SMART", PrimaryExchange: "NASDAQ", Currency: "USD"}
	tsla := &ibapi.Contract{Symbol: "TSLA", SecType: "STK", Exchange: "SMART", PrimaryExchange: "NASDAQ", Currency: "USD"}
	w.Position("DU998877", googl, ibapi.StringToDecimal("15"), 175.25)
	w.Position("DU998877", tsla, ibapi.StringToDecimal("5"), 240.00)
	w.PositionEnd()
	w.UpdatePortfolio(googl, ibapi.StringToDecimal("15"), 182.00, 2730.00, 175.25, 101.25, 0.0, "DU998877")

	// V2-T1-F02-04: OpenOrder + OrderStatus populate OpenOrders
	ord := ibapi.NewOrder()
	ord.OrderID = 42
	ord.Action = "BUY"
	ord.OrderType = "LMT"
	ord.TotalQuantity = ibapi.StringToDecimal("10")
	ord.LmtPrice = 174.50
	ord.OutsideRTH = false
	w.OpenOrder(42, googl, ord, &ibapi.OrderState{Status: "Submitted"})
	w.OpenOrderEnd()
	w.OrderStatus(42, "Submitted", ibapi.ZERO, ibapi.StringToDecimal("10"), 0, 1001, 0, 0, 1, "", 0)

	if !client.IsConnected() {
		t.Fatal("expected client.IsConnected() == true after ConnectAck")
	}
	state := engine.BuildRuntimeState(client.IsConnected())
	if state == nil {
		t.Fatal("BuildRuntimeState returned nil")
	}
	if state.AccountID != "DU998877" {
		t.Errorf("expected AccountID 'DU998877', got %q", state.AccountID)
	}
	if math.Abs(state.Equity-254321.50) > 0.01 {
		t.Errorf("expected Equity 254321.50, got %.2f", state.Equity)
	}
	if math.Abs(state.CashUSD-150000.00) > 0.01 {
		t.Errorf("expected CashUSD 150000.00, got %.2f", state.CashUSD)
	}
	if math.Abs(state.CashCAD-42000.00) > 0.01 {
		t.Errorf("expected CashCAD 42000.00, got %.2f", state.CashCAD)
	}
	if math.Abs(state.BuyingPower-500000.00) > 0.01 {
		t.Errorf("expected BuyingPower 500000.00, got %.2f", state.BuyingPower)
	}
	if math.Abs(state.DailyPnL-320.50) > 0.01 {
		t.Errorf("expected DailyPnL 320.50, got %.2f", state.DailyPnL)
	}
	if math.Abs(state.RealizedPnL-200.50) > 0.01 || math.Abs(state.UnrealizedPnL-120.00) > 0.01 {
		t.Errorf("expected Realized=200.50, Unrealized=120.00, got realized=%.2f unrealized=%.2f",
			state.RealizedPnL, state.UnrealizedPnL)
	}
	if state.OpenPositionsCount != 2 || len(state.OpenPositions) != 2 {
		t.Errorf("expected 2 open positions, got count=%d slice=%d", state.OpenPositionsCount, len(state.OpenPositions))
	} else {
		var foundGOOGL bool
		for _, pos := range state.OpenPositions {
			if pos.Symbol == "GOOGL" {
				foundGOOGL = true
				if math.Abs(pos.Position-15) > 0.01 ||
					math.Abs(pos.AvgCost-175.25) > 0.01 ||
					math.Abs(pos.MarketValue-2730.00) > 0.01 ||
					math.Abs(pos.UnrealizedPnL-101.25) > 0.01 {
					t.Errorf("unexpected GOOGL OpenPosition fields: %+v", pos)
				}
			}
		}
		if !foundGOOGL {
			t.Errorf("GOOGL missing from OpenPositions: %+v", state.OpenPositions)
		}
	}
	if state.PendingOrdersCount != 1 || len(state.OpenOrders) != 1 {
		t.Errorf("expected 1 open order, got count=%d slice=%d", state.PendingOrdersCount, len(state.OpenOrders))
	} else {
		o := state.OpenOrders[0]
		if o.OrderID != 42 || o.Status != "Submitted" || math.Abs(o.RemainingQty-10) > 0.01 {
			t.Errorf("unexpected OpenOrder: %+v", o)
		}
	}

	// UpdateAccountValue("CashBalance") precedence over AccountSummary("TotalCashValue") + CashBase
	w.UpdateAccountValue("CashBalance", "152500.75", "USD", "DU998877")
	w.UpdateAccountValue("CashBalance", "198000.25", "BASE", "DU998877")
	w.AccountSummary(1, "DU998877", "TotalCashValue", "140000.00", "USD")
	cashState := engine.BuildRuntimeState(client.IsConnected())
	if math.Abs(cashState.CashUSD-152500.75) > 0.01 {
		t.Errorf("expected native CashBalance USD=152500.75 to take precedence over TotalCashValue fallback, got %.2f", cashState.CashUSD)
	}
	if math.Abs(cashState.CashBase-198000.25) > 0.01 {
		t.Errorf("expected CashBase=198000.25, got %.2f", cashState.CashBase)
	}

	// Partial fill -> OpenOrder re-broadcast preserving fill progress -> terminal removal
	w.OrderStatus(42, "Submitted", ibapi.StringToDecimal("4"), ibapi.StringToDecimal("6"), 174.50, 1001, 0, 174.50, 1, "", 0)
	w.OpenOrder(42, googl, ord, &ibapi.OrderState{Status: "Submitted"})
	partialState := engine.BuildRuntimeState(client.IsConnected())
	if len(partialState.OpenOrders) != 1 {
		t.Fatalf("expected 1 open order after partial fill + re-broadcast, got %d", len(partialState.OpenOrders))
	}
	po := partialState.OpenOrders[0]
	if math.Abs(po.FilledQty-4) > 0.01 || math.Abs(po.RemainingQty-6) > 0.01 || math.Abs(po.AvgFillPrice-174.50) > 0.01 {
		t.Errorf("OpenOrder re-broadcast failed to preserve partial fill progress: %+v", po)
	}

	// Terminal Filled removes order 42; add order 43 and verify terminal Cancelled removes it too
	w.OrderStatus(42, "Filled", ibapi.StringToDecimal("10"), ibapi.ZERO, 174.55, 1001, 0, 174.60, 1, "", 0)
	ord2 := ibapi.NewOrder()
	ord2.OrderID = 43
	ord2.Action = "SELL"
	ord2.OrderType = "LMT"
	ord2.TotalQuantity = ibapi.StringToDecimal("5")
	ord2.LmtPrice = 250.0
	w.OpenOrder(43, tsla, ord2, &ibapi.OrderState{Status: "Submitted"})
	w.OrderStatus(43, "Cancelled", ibapi.ZERO, ibapi.StringToDecimal("5"), 0, 1002, 0, 0, 1, "", 0)
	termState := engine.BuildRuntimeState(client.IsConnected())
	if termState.PendingOrdersCount != 0 || len(termState.OpenOrders) != 0 {
		t.Errorf("expected 0 open orders after Filled and Cancelled terminal statuses, got %+v", termState.OpenOrders)
	}
}

func TestV2_Tier1_R2_EmptyHoldingsSeedAndSync(t *testing.T) {
	database, engine, _, w := newV2TestEnv(t)

	// V2-T1-F03-01: Fresh DB has positions-detenues empty (no AAPL/SHOP hardcodes)
	held, err := database.GetWatchlistByID("positions-detenues")
	if err != nil {
		t.Fatalf("GetWatchlistByID('positions-detenues') failed: %v", err)
	}
	if len(held.Symbols) != 0 {
		t.Fatalf("expected fresh positions-detenues to have 0 seeded symbols, got %v", held.Symbols)
	}

	// V2-T1-F03-02: IBWrapper.Position (including short pos=-10) + PositionEnd automatically syncs SQLite
	googl := &ibapi.Contract{Symbol: "GOOGL", SecType: "STK", Exchange: "SMART", PrimaryExchange: "NASDAQ", Currency: "USD"}
	tslaShort := &ibapi.Contract{Symbol: "TSLA", SecType: "STK", Exchange: "SMART", PrimaryExchange: "NASDAQ", Currency: "USD"}
	w.ConnectAck()
	w.ManagedAccounts([]string{"DU998877"})
	w.Position("DU998877", googl, ibapi.StringToDecimal("15"), 175.25)
	w.Position("DU998877", tslaShort, ibapi.StringToDecimal("-10"), 240.00)
	w.PositionEnd()

	posState := engine.BuildRuntimeState(true)
	if len(posState.OpenPositions) != 2 {
		t.Fatalf("expected 2 live positions after PositionEnd, got %+v", posState.OpenPositions)
	}
	foundShortTSLA := false
	for _, p := range posState.OpenPositions {
		if p.Symbol == "TSLA" && math.Abs(p.Position-(-10)) < 0.01 {
			foundShortTSLA = true
		}
	}
	if !foundShortTSLA {
		t.Errorf("expected short TSLA position (pos=-10) in OpenPositions, got %+v", posState.OpenPositions)
	}

	heldAfter, err := database.GetWatchlistByID("positions-detenues")
	if err != nil {
		t.Fatalf("GetWatchlistByID('positions-detenues') after PositionEnd failed: %v", err)
	}
	if len(heldAfter.Symbols) != 2 {
		t.Fatalf("expected 2 symbols automatically synced to positions-detenues after PositionEnd, got %v", heldAfter.Symbols)
	}
	for _, sym := range heldAfter.Symbols {
		if strings.Contains(sym, "AAPL") || strings.Contains(sym, "SHOP") {
			t.Fatalf("unexpected stale seeded symbol %q in positions-detenues", sym)
		}
	}

	// Verify $$$$ is_holding propagates to cobalt-telegram automatically via PositionEnd
	telegram, err := database.GetWatchlistByID("cobalt-telegram")
	if err != nil {
		t.Fatalf("GetWatchlistByID('cobalt-telegram') failed: %v", err)
	}
	foundGOOGLHolding := false
	foundTSLAHolding := false
	for _, g := range telegram.Groups {
		for _, it := range g.Items {
			if it.Symbol == "GOOGL" && it.IsHolding {
				foundGOOGLHolding = true
			}
			if it.Symbol == "TSLA" && it.IsHolding {
				foundTSLAHolding = true
			}
			if it.Symbol == "AAPL" && it.IsHolding {
				t.Errorf("AAPL should NOT have is_holding=true when not held in live positions")
			}
		}
	}
	if !foundGOOGLHolding || !foundTSLAHolding {
		t.Errorf("expected GOOGL and short TSLA in cobalt-telegram to have is_holding=true after PositionEnd (googl=%v tsla=%v)",
			foundGOOGLHolding, foundTSLAHolding)
	}

	// V2-T1-F03-03: IBWrapper.Position(pos=0) + PositionEnd automatically clears positions-detenues & is_holding
	w.Position("DU998877", googl, ibapi.ZERO, 0)
	w.PositionEnd()
	heldReduced, err := database.GetWatchlistByID("positions-detenues")
	if err != nil {
		t.Fatalf("GetWatchlistByID failed: %v", err)
	}
	if len(heldReduced.Symbols) != 1 || !strings.Contains(heldReduced.Symbols[0], "TSLA") {
		t.Errorf("expected only TSLA remaining in positions-detenues after GOOGL pos=0 + PositionEnd, got %v", heldReduced.Symbols)
	}
	telegramAfter, _ := database.GetWatchlistByID("cobalt-telegram")
	for _, g := range telegramAfter.Groups {
		for _, it := range g.Items {
			if it.Symbol == "GOOGL" && it.IsHolding {
				t.Errorf("expected GOOGL is_holding cleared to false after Position(pos=0) + PositionEnd")
			}
		}
	}

	// V2-T1-F03-04: RunDryRun evaluates bound non-default watchlist, strategy rules & ActionFilter with zero SPY.SMART stub
	vixSym := "VIX"
	dryStrat := &models.Strategy{
		ID:   "v2_dry_run_bound_wl",
		Name: "V2 DryRun Bound WL Strategy",
		Rules: []models.Rule{
			{
				ID:      "dr_filter",
				Name:    "VIX < 25",
				Scope:   models.ScopeGlobal,
				Action:  models.ActionFilter,
				Enabled: true,
				Condition: &models.Condition{
					Type:       models.ConditionLess,
					IndicatorA: &models.Indicator{Type: models.IndicatorVIX, Symbol: &vixSym, Timeframe: models.TimeframeD1},
					Threshold:  25.0,
				},
			},
			{
				ID:      "dr_buy",
				Name:    "Price > 50",
				Scope:   models.ScopeGlobal,
				Action:  models.ActionBuy,
				Enabled: true,
				Condition: &models.Condition{
					Type:       models.ConditionGreater,
					IndicatorA: &models.Indicator{Type: models.IndicatorPrice, Timeframe: models.TimeframeD1, Source: "close"},
					Threshold:  50.0,
				},
			},
		},
	}
	if err := database.SaveStrategyPreset(dryStrat, &models.CockpitStrategySummary{
		ID:               dryStrat.ID,
		Name:             dryStrat.Name,
		RuleCount:        2,
		EnabledRuleCount: 2,
		Source:           "custom",
	}); err != nil {
		t.Fatalf("SaveStrategyPreset failed: %v", err)
	}

	// Populate BarFrameBundle with cobalt-home-run symbols (RBRK, SHOP) + VIX=18 (<25)
	engine.SetEquity(100000.0)
	bundle := engine.GetBarBundle()
	baseTs := int64(1730000000)
	for i := 0; i < 5; i++ {
		ts := baseTs + int64(i*86400)
		bundle.AddBar("VIX", models.TimeframeD1, ts, 18.0, 18.5, 17.5, 18.0, 1000)
		bundle.AddBar("RBRK", models.TimeframeD1, ts, 60.0, 66.0, 59.0, 65.0, 100000)
		bundle.AddBar("SHOP", models.TimeframeD1, ts, 95.0, 102.0, 94.0, 100.0, 250000)
	}

	cs := engine.GetCockpitState()
	cs.ActiveWorkspaceID = "swing"
	for i := range cs.Workspaces {
		ws := &cs.Workspaces[i]
		ws.Enabled = ws.ID == "swing"
		if ws.Enabled {
			ws.WatchlistID = "cobalt-home-run"
		}
		if len(ws.StrategySlots) > 0 {
			ws.StrategySlots[0].Enabled = ws.Enabled
			if ws.Enabled {
				ws.StrategySlots[0].StrategyID = dryStrat.ID
			}
		}
	}
	engine.SetCockpitState(cs)

	dryRun, err := engine.RunDryRun(true)
	if err != nil {
		t.Fatalf("bot.RunDryRun failed: %v", err)
	}
	if dryRun == nil {
		t.Fatal("bot.RunDryRun returned nil")
	}
	if dryRun.WorkspaceKind != "swing" {
		t.Errorf("expected WorkspaceKind 'swing', got %q", dryRun.WorkspaceKind)
	}
	if len(dryRun.Subscriptions) == 0 {
		t.Fatalf("expected non-empty RunDryRun Subscriptions for bound watchlist cobalt-home-run")
	}
	foundRBRKSub := false
	for _, sub := range dryRun.Subscriptions {
		if sub.Symbol == "SPY.SMART" {
			t.Errorf("RunDryRun returned legacy hardcoded SPY.SMART stub subscription")
		}
		if strings.HasPrefix(sub.Symbol, "NVDA.") {
			t.Errorf("RunDryRun leaked default cobalt-telegram symbol %q instead of bound cobalt-home-run", sub.Symbol)
		}
		if sub.Symbol == "RBRK.NYSE" && sub.Timeframe == "1d" {
			foundRBRKSub = true
		}
	}
	if !foundRBRKSub {
		t.Errorf("expected bound watchlist subscription RBRK.NYSE (1d) in RunDryRun, got %+v", dryRun.Subscriptions)
	}

	if len(dryRun.PlannedOrders) == 0 {
		t.Fatalf("expected non-empty PlannedOrders when VIX=18 < 25 and RBRK/SHOP bars > 50")
	}
	foundRBRKOrder := false
	for _, po := range dryRun.PlannedOrders {
		if po["symbol"] == "RBRK" && po["action"] == "BUY" {
			qty, _ := po["quantity"].(float64)
			if qty <= 0 {
				t.Errorf("expected positive calculated quantity for RBRK BUY planned order, got %+v", po)
			}
			foundRBRKOrder = true
		}
	}
	if !foundRBRKOrder {
		t.Errorf("expected planned BUY order for bound-watchlist symbol RBRK, got %+v", dryRun.PlannedOrders)
	}

	// Raise VIX to 29.0 (>= 25) -> ActionFilter must block PlannedOrders
	bundle.AddBar("VIX", models.TimeframeD1, baseTs+6*86400, 28.0, 30.0, 27.5, 29.0, 1000)
	dryRunBlocked, err := engine.RunDryRun(true)
	if err != nil {
		t.Fatalf("bot.RunDryRun (ActionFilter blocked) failed: %v", err)
	}
	if len(dryRunBlocked.PlannedOrders) != 0 {
		t.Errorf("expected ActionFilter (VIX >= 25) to block all PlannedOrders, got %+v", dryRunBlocked.PlannedOrders)
	}

	// Confirm 0 live orders placed
	trades, err := database.GetRecentTrades(50)
	if err != nil {
		t.Fatalf("GetRecentTrades failed: %v", err)
	}
	for _, tr := range trades {
		if tr.Mode == "live" {
			t.Errorf("RunDryRun must never place live orders: %+v", tr)
		}
	}
}

func TestV2_Tier1_R4_StrategyStackPinAndReorder(t *testing.T) {
	database, _, _, _ := newV2TestEnv(t)

	// V2-T1-F05-01: strategies table has pinned & sort_order columns
	rows, err := database.Conn().Query("PRAGMA table_info(strategies)")
	if err != nil {
		t.Fatalf("PRAGMA table_info(strategies) failed: %v", err)
	}
	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			rows.Close()
			t.Fatalf("scan table_info failed: %v", err)
		}
		cols[name] = true
	}
	rows.Close()
	if !cols["pinned"] || !cols["sort_order"] {
		t.Fatalf("strategies table missing required V2 columns: pinned=%v sort_order=%v", cols["pinned"], cols["sort_order"])
	}

	// V2-T1-F05-02: ToggleStrategyPin moves pinned items to top sorted by (pinned DESC, sort_order ASC, name ASC)
	initialLib, err := database.LoadStrategyLibrary()
	if err != nil || len(initialLib) < 3 {
		t.Fatalf("LoadStrategyLibrary failed or <3 presets: err=%v len=%d", err, len(initialLib))
	}
	assertOrderInvariant(t, initialLib)

	targetA := initialLib[len(initialLib)-1].ID
	targetB := initialLib[len(initialLib)-2].ID

	if err := database.ToggleStrategyPin(targetA); err != nil {
		t.Fatalf("ToggleStrategyPin(%q) failed: %v", targetA, err)
	}
	libPinned, err := database.LoadStrategyLibrary()
	if err != nil {
		t.Fatalf("LoadStrategyLibrary failed: %v", err)
	}
	assertOrderInvariant(t, libPinned)
	if !libPinned[0].Pinned || libPinned[0].ID != targetA {
		t.Fatalf("expected pinned strategy %q at index 0, got %+v", targetA, libPinned[0])
	}

	if err := database.ToggleStrategyPin(targetB); err != nil {
		t.Fatalf("ToggleStrategyPin(%q) failed: %v", targetB, err)
	}
	libPinned2, err := database.LoadStrategyLibrary()
	if err != nil {
		t.Fatalf("LoadStrategyLibrary failed: %v", err)
	}
	assertOrderInvariant(t, libPinned2)

	// V2-T1-F05-03: MoveStrategyOrder reorders deterministically within pinned partition
	pinnedIDs := []string{}
	for _, s := range libPinned2 {
		if s.Pinned {
			pinnedIDs = append(pinnedIDs, s.ID)
		}
	}
	if len(pinnedIDs) != 2 {
		t.Fatalf("expected 2 pinned strategies, got %d", len(pinnedIDs))
	}
	secondPinned := pinnedIDs[1]
	if err := database.MoveStrategyOrder(secondPinned, "up"); err != nil {
		t.Fatalf("MoveStrategyOrder(%q, 'up') failed: %v", secondPinned, err)
	}
	libMoved, err := database.LoadStrategyLibrary()
	if err != nil {
		t.Fatalf("LoadStrategyLibrary failed: %v", err)
	}
	assertOrderInvariant(t, libMoved)
	if libMoved[0].ID != secondPinned {
		t.Errorf("expected %q moved to index 0, got %q", secondPinned, libMoved[0].ID)
	}

	// Move down back to index 1
	if err := database.MoveStrategyOrder(secondPinned, "down"); err != nil {
		t.Fatalf("MoveStrategyOrder(%q, 'down') failed: %v", secondPinned, err)
	}
	libMovedDown, err := database.LoadStrategyLibrary()
	if err != nil {
		t.Fatalf("LoadStrategyLibrary failed: %v", err)
	}
	assertOrderInvariant(t, libMovedDown)
	if libMovedDown[1].ID != secondPinned {
		t.Errorf("expected %q moved back to index 1, got %q", secondPinned, libMovedDown[1].ID)
	}

	// Equal (pinned, sort_order) tie-breaking via SQLite binary name ASC + close/reopen disk persistence
	persistPath := filepath.Join(t.TempDir(), "persist_stack.sqlite")
	db1, err := db.Connect(persistPath)
	if err != nil {
		t.Fatalf("db.Connect(%q) failed: %v", persistPath, err)
	}
	if err := strategy.SeedStrategyCatalog(db1); err != nil {
		db1.Close()
		t.Fatalf("SeedStrategyCatalog failed: %v", err)
	}
	tieStrategies := []struct {
		id   string
		name string
	}{
		{id: "tie_lower_a", name: "apple Tie Strat"},
		{id: "tie_upper_b", name: "Banana Tie Strat"},
	}
	for _, ts := range tieStrategies {
		if err := db1.SaveStrategyPreset(&models.Strategy{
			ID:   ts.id,
			Name: ts.name,
			Rules: []models.Rule{{
				ID: "r1", Name: "Rule", Scope: models.ScopeGlobal, Action: models.ActionBuy, Enabled: true,
				Condition: &models.Condition{Type: models.ConditionGreater, IndicatorA: &models.Indicator{Type: models.IndicatorPrice, Timeframe: models.TimeframeD1}, Threshold: 1},
			}},
		}, &models.CockpitStrategySummary{
			ID: ts.id, Name: ts.name, RuleCount: 1, EnabledRuleCount: 1, Source: "custom", Pinned: true, SortOrder: 500,
		}); err != nil {
			db1.Close()
			t.Fatalf("SaveStrategyPreset(%q) failed: %v", ts.id, err)
		}
	}
	if err := db1.ToggleStrategyPin(targetA); err != nil {
		db1.Close()
		t.Fatalf("ToggleStrategyPin on disk DB failed: %v", err)
	}
	if err := db1.Close(); err != nil {
		t.Fatalf("db1.Close failed: %v", err)
	}

	db2, err := db.Connect(persistPath)
	if err != nil {
		t.Fatalf("reopen db.Connect(%q) failed: %v", persistPath, err)
	}
	defer db2.Close()

	reloadedLib, err := db2.LoadStrategyLibrary()
	if err != nil {
		t.Fatalf("reopened LoadStrategyLibrary failed: %v", err)
	}
	assertOrderInvariant(t, reloadedLib)
	var pinnedOrder []string
	for _, s := range reloadedLib {
		if s.Pinned {
			pinnedOrder = append(pinnedOrder, s.ID)
		}
	}
	if len(pinnedOrder) != 3 {
		t.Fatalf("expected 3 pinned strategies persisted across DB reopen, got %v", pinnedOrder)
	}
	if pinnedOrder[0] != targetA || pinnedOrder[1] != "tie_upper_b" || pinnedOrder[2] != "tie_lower_a" {
		t.Errorf("expected persisted pinned order [%s, tie_upper_b ('Banana'), tie_lower_a ('apple')], got %v", targetA, pinnedOrder)
	}
}

func TestV2_Tier1_R5_DiagnosticsSessionAndAlerts(t *testing.T) {
	_, engine, _, w := newV2TestEnv(t)

	// Pre-ManagedAccounts diagnostics must have empty Account fields (zero DUO185814 stub)
	preDiag, err := engine.BuildDiagnostics()
	if err != nil {
		t.Fatalf("pre-ManagedAccounts BuildDiagnostics failed: %v", err)
	}
	if preDiag.Account != "" || preDiag.Startup.Account != "" {
		t.Fatalf("expected empty Account & Startup.Account before ManagedAccounts, got account=%q startup=%q",
			preDiag.Account, preDiag.Startup.Account)
	}

	w.ConnectAck()
	w.ManagedAccounts([]string{"DU7654321"})
	w.Error(10, 0, 10089, "Requested market data requires additional subscription for API", "")
	w.Error(11, 0, 504, "Not connected", "")

	// V2-T1-F06-01: BuildDiagnostics returns live account, session_status, broker_alerts without dev noise
	diag, err := engine.BuildDiagnostics()
	if err != nil {
		t.Fatalf("BuildDiagnostics failed: %v", err)
	}
	if diag == nil {
		t.Fatal("BuildDiagnostics returned nil")
	}
	if diag.Account != "DU7654321" || diag.Startup.Account != "DU7654321" {
		t.Errorf("expected live account 'DU7654321' in Account & Startup.Account, got account=%q startup=%q",
			diag.Account, diag.Startup.Account)
	}
	diagJSON, err := json.Marshal(diag)
	if err != nil {
		t.Fatalf("json.Marshal(diag) failed: %v", err)
	}
	diagStr := string(diagJSON)
	forbidden := []string{"DUO185814", "desktop_local", "tws_traderbot.sqlite", "tws_traderbot.log", "local_sqlite"}
	for _, bad := range forbidden {
		if strings.Contains(diagStr, bad) {
			t.Errorf("Diagnostics JSON contains forbidden legacy dev stub %q in %s", bad, diagStr)
		}
	}
	if diag.SessionStatus == "" {
		t.Errorf("expected non-empty SessionStatus in DiagnosticsResponse")
	}
	if len(diag.BrokerAlerts) < 2 {
		t.Errorf("expected >= 2 broker alerts (10089, 504), got %d", len(diag.BrokerAlerts))
	} else {
		found10089 := false
		found504 := false
		for _, al := range diag.BrokerAlerts {
			if al.Code == 10089 && strings.TrimSpace(al.Hint) != "" {
				found10089 = true
			}
			if al.Code == 504 && strings.TrimSpace(al.Hint) != "" {
				found504 = true
			}
		}
		if !found10089 || !found504 {
			t.Errorf("expected actionable hints for 10089 and 504, got %+v", diag.BrokerAlerts)
		}
	}
}

func TestV2_Tier1_R7_ValidateStrategyAllowsActionFilter(t *testing.T) {
	// V2-T1-F08-01: ValidateStrategy allows ActionFilter alongside ActionBuy/ActionSell
	strat := &models.Strategy{
		ID:   "v2_filter_strat",
		Name: "Cobalt Macro Filter + Buy",
		Rules: []models.Rule{
			{
				ID:      "r_macro",
				Name:    "VIX Regime Filter",
				Scope:   models.ScopeGlobal,
				Action:  models.ActionFilter,
				Enabled: true,
				Condition: &models.Condition{
					Type:       models.ConditionLess,
					IndicatorA: &models.Indicator{Type: models.IndicatorVIX, Timeframe: models.TimeframeD1},
					Threshold:  25.0,
				},
			},
			{
				ID:      "r_buy",
				Name:    "EMA Cross Buy",
				Scope:   models.ScopePerTicker,
				Action:  models.ActionBuy,
				Enabled: true,
				Condition: &models.Condition{
					Type:       models.ConditionCrossAbove,
					IndicatorA: &models.Indicator{Type: models.IndicatorEMA, Period: 10, Timeframe: models.TimeframeD1},
					IndicatorB: &models.Indicator{Type: models.IndicatorEMA, Period: 30, Timeframe: models.TimeframeD1},
				},
			},
		},
	}

	res := models.ValidateStrategy(strat)
	if res == nil || !res.Valid || len(res.Errors) > 0 {
		t.Fatalf("expected ActionFilter strategy to validate cleanly, got valid=%v errors=%v",
			res != nil && res.Valid, func() []string {
				if res != nil {
					return res.Errors
				}
				return nil
			}())
	}
}

// =============================================================================
// Tier 2: Boundary & Corner Cases
// =============================================================================

func TestV2_Tier2_R2_DisconnectedZeroEquityAndMalformedCallbacks(t *testing.T) {
	_, engine, _, w := newV2TestEnv(t)

	// V2-T2-F02-01: Fresh disconnected state must report Equity == 0 (never 100000.0 stub)
	state := engine.BuildRuntimeState(false)
	if state.Equity != 0 {
		t.Errorf("expected disconnected initial Equity == 0 (no 100000.0 stub), got %.2f", state.Equity)
	}
	if state.AccountID != "" {
		t.Errorf("expected empty AccountID before ManagedAccounts, got %q", state.AccountID)
	}

	// V2-T2-F02-04: ManagedAccounts with trailing comma / empty entries
	w.ManagedAccounts([]string{"", "DU111222", "DU333444,"})
	state = engine.BuildRuntimeState(true)
	if state.AccountID != "DU111222" {
		t.Errorf("expected primary non-empty AccountID 'DU111222', got %q", state.AccountID)
	}

	// V2-T2-F02-02: Malformed AccountSummary strings must not propagate NaN or Inf
	w.AccountSummary(2, "DU111222", "NetLiquidation", "NaN", "USD")
	w.AccountSummary(2, "DU111222", "TotalCashValue", "N/A", "USD")
	w.AccountSummary(2, "DU111222", "BuyingPower", "", "USD")
	w.AccountSummaryEnd(2)

	// V2-T2-F02-03: Unset MaxFloat64 in Pnl & zero-div DailyPnLPercent guard when Equity == 0
	w.Pnl(1, math.MaxFloat64, -125.0, -50.0)
	state = engine.BuildRuntimeState(true)
	if math.IsNaN(state.Equity) || math.IsInf(state.Equity, 0) || state.Equity != 0 {
		t.Errorf("expected safe Equity=0 after malformed inputs, got %v", state.Equity)
	}
	if math.IsNaN(state.DailyPnLPercent) || math.IsInf(state.DailyPnLPercent, 0) || state.DailyPnLPercent != 0 {
		t.Errorf("expected DailyPnLPercent==0 when Equity==0, got %v", state.DailyPnLPercent)
	}

	// V2-T2-F02-05: Benign 2104/2106/2158 informational codes must NOT pollute BrokerAlerts
	w.Error(-1, 0, 2104, "Market data farm connection is OK:usfarm", "")
	w.Error(-1, 0, 2106, "HMDS data farm connection is OK", "")
	w.Error(-1, 0, 2158, "Sec-def data farm connection is OK", "")
	state = engine.BuildRuntimeState(true)
	for _, al := range state.BrokerAlerts {
		if al.Code == 2104 || al.Code == 2106 || al.Code == 2158 {
			t.Errorf("benign IBKR info code %d should be suppressed from BrokerAlerts", al.Code)
		}
	}
}

func TestV2_Tier2_R2_HoldingsSyncAndDryRunCorners(t *testing.T) {
	database, engine, _, _ := newV2TestEnv(t)

	// V2-T2-F03-01 & V2-T2-F03-02: Prune stale entries & empty slice clears all holdings
	_, err := database.MergeAndSaveWatchlist(
		"positions-detenues",
		"Positions détenues ($$$$)",
		"test",
		nil,
		[]models.WatchlistGroup{
			{
				ID:   "holding",
				Name: "Positions Détenues",
				Items: []models.WatchlistItem{
					{Symbol: "AAPL", Exchange: "NASDAQ", Enabled: true, IsHolding: true},
					{Symbol: "SHOP", Exchange: "TSX", Enabled: true, IsHolding: true},
				},
			},
		},
		false,
	)
	if err != nil {
		t.Fatalf("seed stale holdings failed: %v", err)
	}

	// Sync with only GOOGL held -> AAPL/SHOP must be pruned
	if err := database.SyncLivePositions([]models.BrokerPosition{
		{Symbol: "GOOGL", Venue: "NASDAQ", InstrumentID: "GOOGL.NASDAQ", Position: 10},
		{Symbol: "MSFT", Venue: "NASDAQ", InstrumentID: "MSFT.NASDAQ", Position: 0}, // V2-T2-F03-03: pos==0 excluded
	}); err != nil {
		t.Fatalf("SyncLivePositions failed: %v", err)
	}
	held, _ := database.GetWatchlistByID("positions-detenues")
	if len(held.Symbols) != 1 || !strings.Contains(held.Symbols[0], "GOOGL") {
		t.Errorf("expected strictly [GOOGL.*] after pruning stale AAPL/SHOP and pos==0 MSFT, got %v", held.Symbols)
	}

	// Empty sync clears positions-detenues and all is_holding flags
	if err := database.SyncLivePositions(nil); err != nil {
		t.Fatalf("SyncLivePositions(nil) failed: %v", err)
	}
	heldEmpty, _ := database.GetWatchlistByID("positions-detenues")
	if len(heldEmpty.Symbols) != 0 {
		t.Errorf("expected 0 symbols after empty SyncLivePositions, got %v", heldEmpty.Symbols)
	}

	// V2-T2-F03-04: DryRunBot with unassigned strategy slot returns safely with 0 planned orders and no SPY.SMART
	cs := engine.GetCockpitState()
	cs.ActiveWorkspaceID = "swing"
	for i := range cs.Workspaces {
		ws := &cs.Workspaces[i]
		ws.Enabled = ws.ID == "swing"
		for j := range ws.StrategySlots {
			ws.StrategySlots[j].StrategyID = ""
			ws.StrategySlots[j].Enabled = false
		}
	}
	engine.SetCockpitState(cs)

	dryRun, err := engine.RunDryRun(true)
	if err != nil {
		t.Fatalf("RunDryRun with unassigned slot failed: %v", err)
	}
	if len(dryRun.PlannedOrders) != 0 {
		t.Errorf("expected 0 planned orders when no strategy armed, got %d", len(dryRun.PlannedOrders))
	}
	for _, sub := range dryRun.Subscriptions {
		if sub.Symbol == "SPY.SMART" {
			t.Errorf("unexpected SPY.SMART fallback in RunDryRun")
		}
	}
}

func TestV2_Tier2_R4_StrategyStackBoundaries(t *testing.T) {
	database, _, _, _ := newV2TestEnv(t)

	// V2-T2-F05-03..05: Idempotent migration, boundary moves, unknown ID, invalid direction
	database.InitSchema() // Idempotent second call

	lib, err := database.LoadStrategyLibrary()
	if err != nil || len(lib) == 0 {
		t.Fatalf("LoadStrategyLibrary failed: %v", err)
	}

	// Boundary move up at index 0 & down at len-1 must be safe no-op
	firstID := lib[0].ID
	lastID := lib[len(lib)-1].ID

	if err := database.MoveStrategyOrder(firstID, "up"); err != nil {
		t.Fatalf("MoveStrategyOrder at top index 'up' returned error: %v", err)
	}
	afterUp, _ := database.LoadStrategyLibrary()
	if afterUp[0].ID != firstID {
		t.Errorf("expected top item %q unchanged after 'up' at boundary, got %q", firstID, afterUp[0].ID)
	}

	if err := database.MoveStrategyOrder(lastID, "down"); err != nil {
		t.Fatalf("MoveStrategyOrder at bottom index 'down' returned error: %v", err)
	}
	afterDown, _ := database.LoadStrategyLibrary()
	if afterDown[len(afterDown)-1].ID != lastID {
		t.Errorf("expected bottom item %q unchanged after 'down' at boundary, got %q", lastID, afterDown[len(afterDown)-1].ID)
	}

	// Unknown/empty ID
	if err := database.ToggleStrategyPin("does_not_exist"); err == nil {
		t.Errorf("expected error for ToggleStrategyPin on unknown ID")
	}
	if err := database.ToggleStrategyPin(""); err == nil {
		t.Errorf("expected error for ToggleStrategyPin on empty ID")
	}
	if err := database.MoveStrategyOrder("does_not_exist", "up"); err == nil {
		t.Errorf("expected error for MoveStrategyOrder on unknown ID")
	}

	// Invalid direction
	if err := database.MoveStrategyOrder(firstID, "left"); err == nil {
		t.Errorf("expected error for MoveStrategyOrder invalid direction 'left'")
	}
	if err := database.MoveStrategyOrder(firstID, ""); err == nil {
		t.Errorf("expected error for MoveStrategyOrder empty direction")
	}
}

func TestV2_Tier2_R5_PausedActiveStrategyAndOutOfHoursSession(t *testing.T) {
	_, engine, _, _ := newV2TestEnv(t)

	// V2-T2-F06-03: Out-of-hours timestamp reports ETH/CLOSED for NY & Toronto
	nyLoc, err := time.LoadLocation("America/New_York")
	if err != nil {
		nyLoc = time.FixedZone("EST", -5*3600)
	}
	// Saturday 11:00 ET -> CLOSED
	engine.SetTimeProvider(func() time.Time {
		return time.Date(2026, 11, 21, 11, 0, 0, 0, nyLoc)
	})

	// V2-T2-F06-04: Armed strategy visible in RuntimeState even when engine is paused (running: false)
	cs := engine.GetCockpitState()
	cs.ActiveWorkspaceID = "short"
	for i := range cs.Workspaces {
		ws := &cs.Workspaces[i]
		if ws.ID == "short" {
			ws.Enabled = true
			ws.StrategySlots = []models.CockpitStrategySlot{
				{ID: "slot_1", Label: "Croisement Bas", StrategyID: "screener_10_30_bas", Enabled: true},
			}
		} else {
			ws.Enabled = false
			for j := range ws.StrategySlots {
				ws.StrategySlots[j].Enabled = false
			}
		}
	}
	engine.SetCockpitState(cs)
	engine.StopBot()

	state := engine.BuildRuntimeState(false)
	if state.Running {
		t.Fatalf("expected Running=false while bot paused")
	}
	if state.ActiveStrategyID != "screener_10_30_bas" {
		t.Errorf("expected ActiveStrategyID 'screener_10_30_bas' while paused, got %q", state.ActiveStrategyID)
	}
	if state.ActiveStrategy == "" {
		t.Errorf("expected non-empty ActiveStrategy name while paused with armed Cockpit slot")
	}
	if state.SessionStatus == "" || (!strings.Contains(strings.ToUpper(state.SessionStatus), "CLOSED") &&
		!strings.Contains(strings.ToUpper(state.SessionStatus), "ETH") &&
		!strings.Contains(strings.ToUpper(state.SessionStatus), "NY")) {
		t.Errorf("expected SessionStatus reflecting weekend/CLOSED NY+Toronto status, got %q", state.SessionStatus)
	}
}

func TestV2_Tier2_R7_ValidateStrategyRejectsInvalidInputs(t *testing.T) {
	// V2-T2-F08-01: nil, empty name, 0 rules, invalid action
	cases := []struct {
		name  string
		strat *models.Strategy
	}{
		{name: "nil strategy", strat: nil},
		{name: "empty name", strat: &models.Strategy{Name: "", Rules: []models.Rule{{Name: "r", Action: models.ActionBuy, Condition: &models.Condition{Type: models.ConditionGreater}}}}},
		{name: "zero rules", strat: &models.Strategy{Name: "Empty", Rules: nil}},
		{name: "invalid action hold", strat: &models.Strategy{Name: "BadAction", Rules: []models.Rule{{Name: "r", Action: models.ActionType("hold"), Condition: &models.Condition{Type: models.ConditionGreater}}}}},
		{name: "unsupported action alert", strat: &models.Strategy{Name: "AlertAction", Rules: []models.Rule{{Name: "r", Action: models.ActionAlert, Condition: &models.Condition{Type: models.ConditionGreater}}}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := models.ValidateStrategy(tc.strat)
			if res == nil || res.Valid || len(res.Errors) == 0 {
				t.Errorf("expected Valid=false with non-empty Errors for %s, got %+v", tc.name, res)
			}
		})
	}
}

// =============================================================================
// Tier 3: Cross-Feature Interactions & Concurrent Race Hardening
// =============================================================================

func TestV2_Tier3_FullPipelineAndConcurrentCallbacks(t *testing.T) {
	t.Run("T3-V2-01_TelemetryHoldingsDryRunPipeline", func(t *testing.T) {
		database, engine, _, w := newV2TestEnv(t)

		w.ConnectAck()
		w.ManagedAccounts([]string{"DU555111"})
		w.AccountSummary(1, "DU555111", "NetLiquidation", "180000.00", "USD")
		w.AccountSummary(1, "DU555111", "TotalCashValue", "95000.00", "USD")
		w.AccountSummary(1, "DU555111", "TotalCashValue", "35000.00", "CAD")
		w.AccountSummary(1, "DU555111", "BuyingPower", "360000.00", "USD")
		w.AccountSummaryEnd(1)

		shop := &ibapi.Contract{Symbol: "SHOP", SecType: "STK", Exchange: "SMART", PrimaryExchange: "TSE", Currency: "CAD"}
		w.Position("DU555111", shop, ibapi.StringToDecimal("25"), 110.50)
		w.PositionEnd()

		state := engine.BuildRuntimeState(true)
		if err := database.SyncLivePositions(state.OpenPositions); err != nil {
			t.Fatalf("SyncLivePositions failed: %v", err)
		}

		// Arm Weinstein strategy in Swing workspace
		cs := engine.GetCockpitState()
		cs.ActiveWorkspaceID = "swing"
		for i := range cs.Workspaces {
			ws := &cs.Workspaces[i]
			ws.Enabled = ws.ID == "swing"
			if len(ws.StrategySlots) > 0 {
				ws.StrategySlots[0].Enabled = ws.Enabled
				if ws.Enabled {
					ws.StrategySlots[0].StrategyID = "cours_weinstein_p2"
				}
			}
		}
		engine.SetCockpitState(cs)

		dryRun, err := engine.RunDryRun(true)
		if err != nil {
			t.Fatalf("RunDryRun failed: %v", err)
		}
		if dryRun.WorkspaceKind != "swing" {
			t.Errorf("expected WorkspaceKind 'swing', got %q", dryRun.WorkspaceKind)
		}
		if !strings.Contains(strings.ToLower(dryRun.Strategy), "weinstein") {
			t.Errorf("expected Weinstein strategy in dryRun, got %q", dryRun.Strategy)
		}
		trades, err := database.GetRecentTrades(20)
		if err != nil {
			t.Fatalf("GetRecentTrades failed: %v", err)
		}
		for _, tr := range trades {
			if tr.Mode == "live" {
				t.Errorf("DryRunBot must never place or record live orders: %+v", tr)
			}
		}
	})

	t.Run("T3-V2-02_DisconnectClearsOrMarksStaleWithoutStubs", func(t *testing.T) {
		_, engine, client, w := newV2TestEnv(t)

		googl := &ibapi.Contract{Symbol: "GOOGL", SecType: "STK", Exchange: "SMART", PrimaryExchange: "NASDAQ", Currency: "USD"}
		ord := ibapi.NewOrder()
		ord.OrderID = 77
		ord.Action = "BUY"
		ord.OrderType = "LMT"
		ord.TotalQuantity = ibapi.StringToDecimal("10")
		ord.LmtPrice = 175.0

		w.ConnectAck()
		w.ManagedAccounts([]string{"DU888999"})
		w.AccountSummary(1, "DU888999", "NetLiquidation", "125000.00", "USD")
		w.AccountSummary(1, "DU888999", "TotalCashValue", "75000.00", "USD")
		w.AccountSummary(1, "DU888999", "TotalCashValue", "25000.00", "CAD")
		w.AccountSummary(1, "DU888999", "BuyingPower", "250000.00", "USD")
		w.AccountSummaryEnd(1)
		w.Position("DU888999", googl, ibapi.StringToDecimal("10"), 175.0)
		w.PositionEnd()
		w.OpenOrder(77, googl, ord, &ibapi.OrderState{Status: "Submitted"})
		w.OrderStatus(77, "Submitted", ibapi.ZERO, ibapi.StringToDecimal("10"), 0, 7701, 0, 0, 1, "", 0)

		preDisc := engine.BuildRuntimeState(client.IsConnected())
		if preDisc.Equity <= 0 || len(preDisc.OpenPositions) != 1 || len(preDisc.OpenOrders) != 1 {
			t.Fatalf("expected populated live telemetry before disconnect, got %+v", preDisc)
		}

		w.ConnectionClosed()
		if client.IsConnected() {
			t.Errorf("expected client.IsConnected() == false after ConnectionClosed")
		}

		postDisc := engine.BuildRuntimeState(client.IsConnected())
		if postDisc.Equity != 0 || postDisc.CashUSD != 0 || postDisc.CashCAD != 0 || postDisc.BuyingPower != 0 {
			t.Errorf("expected balances zeroed on BuildRuntimeState(false) after ConnectionClosed, got equity=%.2f usd=%.2f cad=%.2f bp=%.2f",
				postDisc.Equity, postDisc.CashUSD, postDisc.CashCAD, postDisc.BuyingPower)
		}
		if len(postDisc.OpenPositions) != 0 || postDisc.OpenPositionsCount != 0 {
			t.Errorf("expected OpenPositions cleared after ConnectionClosed, got %+v", postDisc.OpenPositions)
		}
		if len(postDisc.OpenOrders) != 0 || postDisc.PendingOrdersCount != 0 {
			t.Errorf("expected OpenOrders cleared after ConnectionClosed, got %+v", postDisc.OpenOrders)
		}

		diag, err := engine.BuildDiagnostics()
		if err != nil {
			t.Fatalf("BuildDiagnostics failed: %v", err)
		}
		if diag.Runtime.LastDisconnectAt == "" {
			t.Errorf("expected LastDisconnectAt recorded after ConnectionClosed")
		}
		if diag.Startup.Account == "DUO185814" {
			t.Errorf("unexpected legacy DUO185814 stub after ConnectionClosed")
		}
	})

	t.Run("T3-V2-03_ConcurrentCallbackStorm_Race", func(t *testing.T) {
		_, engine, _, w := newV2TestEnv(t)

		const goroutines = 16
		var wg sync.WaitGroup
		wg.Add(goroutines)

		for g := 0; g < goroutines; g++ {
			go func(idx int) {
				defer wg.Done()
				sym := fmt.Sprintf("SYM%d", idx%4)
				c := &ibapi.Contract{Symbol: sym, SecType: "STK", Exchange: "SMART", PrimaryExchange: "NASDAQ", Currency: "USD"}
				for iter := 0; iter < 15; iter++ {
					w.ManagedAccounts([]string{"DU700001"})
					w.AccountSummary(int64(idx+1), "DU700001", "NetLiquidation", fmt.Sprintf("%.2f", 100000.0+float64(iter)), "USD")
					w.AccountSummary(int64(idx+1), "DU700001", "TotalCashValue", "50000.00", "USD")
					w.AccountSummary(int64(idx+1), "DU700001", "TotalCashValue", "20000.00", "CAD")
					w.Position("DU700001", c, ibapi.StringToDecimal("10"), 100.0+float64(iter))
					w.PositionEnd()
					ord := ibapi.NewOrder()
					ord.OrderID = int64(idx*100 + iter)
					ord.Action = "BUY"
					ord.OrderType = "LMT"
					ord.TotalQuantity = ibapi.StringToDecimal("5")
					w.OpenOrder(ord.OrderID, c, ord, &ibapi.OrderState{Status: "Submitted"})
					w.OrderStatus(ord.OrderID, "Submitted", ibapi.ZERO, ibapi.StringToDecimal("5"), 0, ord.OrderID, 0, 0, 1, "", 0)
					w.Pnl(1, float64(iter), float64(iter)/2, float64(iter)/2)
					w.Error(int64(iter), 0, 10089, "Delayed data", "")
					_ = engine.BuildRuntimeState(true)
					_, _ = engine.BuildDiagnostics()
				}
			}(g)
		}
		wg.Wait()
	})
}

// =============================================================================
// Tier 4: Real-World Operator Scenario & Optional Live TWS Paper Gate
// =============================================================================

func TestV2_Tier4_PaperSessionFXCONVAndHoldingsWorkflow(t *testing.T) {
	database, engine, _, w := newV2TestEnv(t)

	// Simulate NY/Toronto RTH weekday morning
	nyLoc, _ := time.LoadLocation("America/New_York")
	if nyLoc == nil {
		nyLoc = time.FixedZone("EST", -5*3600)
	}
	engine.SetTimeProvider(func() time.Time {
		return time.Date(2026, 11, 18, 10, 15, 0, 0, nyLoc)
	})

	// 1. Connect & capture live USD + CAD balances and multi-currency holdings
	w.ConnectAck()
	w.ManagedAccounts([]string{"DU919191"})
	w.AccountSummary(1, "DU919191", "NetLiquidation", "312500.75", "USD")
	w.AccountSummary(1, "DU919191", "TotalCashValue", "84200.50", "USD")
	w.AccountSummary(1, "DU919191", "TotalCashValue", "61400.25", "CAD")
	w.AccountSummary(1, "DU919191", "BuyingPower", "625000.00", "USD")
	w.AccountSummaryEnd(1)

	googl := &ibapi.Contract{Symbol: "GOOGL", SecType: "STK", Exchange: "SMART", PrimaryExchange: "NASDAQ", Currency: "USD"}
	shop := &ibapi.Contract{Symbol: "SHOP", SecType: "STK", Exchange: "SMART", PrimaryExchange: "TSE", Currency: "CAD"}
	w.Position("DU919191", googl, ibapi.StringToDecimal("20"), 175.0)
	w.Position("DU919191", shop, ibapi.StringToDecimal("30"), 115.0)
	w.PositionEnd()

	w.Error(40, 0, 10089, "Requested market data requires additional subscription", "")

	state := engine.BuildRuntimeState(true)
	if !strings.Contains(strings.ToUpper(state.SessionStatus), "RTH") {
		t.Errorf("expected SessionStatus to report RTH at weekday 10:15 ET, got %q", state.SessionStatus)
	}
	if err := database.SyncLivePositions(state.OpenPositions); err != nil {
		t.Fatalf("SyncLivePositions failed: %v", err)
	}

	// 2. Pin Weinstein Phase 2 strategy and verify stack ordering
	if err := database.ToggleStrategyPin("cours_weinstein_p2"); err != nil {
		t.Fatalf("ToggleStrategyPin failed: %v", err)
	}
	lib, err := database.LoadStrategyLibrary()
	if err != nil || len(lib) == 0 || lib[0].ID != "cours_weinstein_p2" {
		t.Fatalf("expected cours_weinstein_p2 pinned first, got %+v (err=%v)", lib, err)
	}

	// 3. Validate ActionFilter strategy & run DryRunBot against active workspace
	strat, err := database.LoadStrategyPreset("cours_weinstein_p2")
	if err != nil {
		t.Fatalf("LoadStrategyPreset failed: %v", err)
	}
	strat.Rules = append(strat.Rules, models.Rule{
		ID:      "macro_vix_ok",
		Name:    "VIX < 25",
		Scope:   models.ScopeGlobal,
		Action:  models.ActionFilter,
		Enabled: true,
		Condition: &models.Condition{
			Type:       models.ConditionLess,
			IndicatorA: &models.Indicator{Type: models.IndicatorVIX, Timeframe: models.TimeframeD1},
			Threshold:  25,
		},
	})
	val := models.ValidateStrategy(strat)
	if !val.Valid {
		t.Fatalf("ValidateStrategy with ActionFilter failed: %+v", val)
	}

	dryRun, err := engine.RunDryRun(true)
	if err != nil {
		t.Fatalf("RunDryRun failed: %v", err)
	}
	if dryRun.State == nil || dryRun.State.CashUSD <= 0 || dryRun.State.CashCAD <= 0 {
		t.Errorf("expected positive USD & CAD cash balances in DryRun state, got %+v", dryRun.State)
	}

	held, err := database.GetWatchlistByID("positions-detenues")
	if err != nil || len(held.Symbols) != 2 {
		t.Errorf("expected 2 live holdings in positions-detenues, got %v (err=%v)", held, err)
	}
}

func TestV2_Tier2_R5_DailyLossBreakerHeadroomTripAndReset(t *testing.T) {
	_, engine, _, w := newV2TestEnv(t)

	engine.SetDailyLossLimit(500.0)
	engine.SetGlobalEnabled(true)

	// 92% of daily loss limit (-460 vs -500 limit): headroom remains, breaker NOT tripped
	w.Pnl(1, -460.0, -460.0, 0.0)
	warnState := engine.BuildRuntimeState(true)
	if warnState.CircuitBreakerTripped || engine.RiskBreakerTripped.Load() {
		t.Fatalf("breaker should NOT trip at 92%% (-460 / -500), got %+v", warnState)
	}
	if !engine.IsGlobalEnabled() {
		t.Errorf("expected globalEnabled to stay true at 92%% headroom")
	}

	// 105% of daily loss limit (-525 <= -500): breaker trips automatically & disarms globalEnabled
	w.Pnl(1, -525.0, -525.0, 0.0)
	trippedState := engine.BuildRuntimeState(true)
	if !trippedState.CircuitBreakerTripped || !engine.RiskBreakerTripped.Load() {
		t.Fatalf("expected breaker tripped at 105%% (-525 <= -500)")
	}
	if engine.IsGlobalEnabled() {
		t.Errorf("expected globalEnabled=false after breaker trip")
	}
	if trippedState.Status != "CIRCUIT_BREAKER" {
		t.Errorf("expected status CIRCUIT_BREAKER, got %q", trippedState.Status)
	}
	if ok, reason := engine.CanExecuteWithPrice("GOOGL", models.ActionBuy, 180.0); ok {
		t.Errorf("expected CanExecuteWithPrice blocked while breaker tripped, got ok=true (%s)", reason)
	}

	// ResetCircuitBreaker must refuse while daily loss is still actively breached
	if err := engine.ResetCircuitBreaker(); err == nil {
		t.Errorf("expected ResetCircuitBreaker to refuse while daily loss (-525 <= -500) is actively breached")
	}

	// Recover PnL above limit (-100 > -500) -> ResetCircuitBreaker succeeds and re-arms engine
	w.Pnl(1, -100.0, -100.0, 0.0)
	if err := engine.ResetCircuitBreaker(); err != nil {
		t.Fatalf("ResetCircuitBreaker failed after PnL recovery: %v", err)
	}
	if engine.RiskBreakerTripped.Load() {
		t.Errorf("expected RiskBreakerTripped=false after ResetCircuitBreaker")
	}
	if !engine.IsGlobalEnabled() {
		t.Errorf("expected globalEnabled=true after ResetCircuitBreaker")
	}
	recoveredState := engine.BuildRuntimeState(true)
	if recoveredState.CircuitBreakerTripped || recoveredState.Status != "ARMED" {
		t.Errorf("expected ARMED recovered state after ResetCircuitBreaker, got status=%q tripped=%v",
			recoveredState.Status, recoveredState.CircuitBreakerTripped)
	}
}

func TestV2_LiveTWS(t *testing.T) {
	if os.Getenv("TWS_LIVE_TEST") != "1" {
		t.Skip("Skipping live TWS Paper test; set TWS_LIVE_TEST=1 to run against 127.0.0.1:7497")
	}

	database, engine, client, _ := newV2TestEnv(t)
	if err := client.Connect("127.0.0.1", 7497, 77); err != nil {
		t.Fatalf("failed to connect to live TWS Paper on 127.0.0.1:7497: %v", err)
	}
	defer client.Disconnect()

	deadline := time.Now().Add(10 * time.Second)
	var state *models.BotState
	for time.Now().Before(deadline) {
		state = engine.BuildRuntimeState(client.IsConnected())
		if state != nil && state.AccountID != "" && state.Equity > 0 {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}

	if state == nil || state.AccountID == "" {
		t.Fatalf("live TWS Paper did not report AccountID within timeout; state=%+v", state)
	}
	if state.Equity <= 0 {
		t.Fatalf("live TWS Paper did not report positive Equity within timeout; equity=%.2f", state.Equity)
	}

	if err := database.SyncLivePositions(state.OpenPositions); err != nil {
		t.Fatalf("SyncLivePositions against live TWS failed: %v", err)
	}

	dryRun, err := engine.RunDryRun(true)
	if err != nil {
		t.Fatalf("live TWS RunDryRun failed: %v", err)
	}
	if dryRun == nil {
		t.Fatal("live TWS RunDryRun returned nil")
	}
}
