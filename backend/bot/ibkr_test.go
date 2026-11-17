package bot

import (
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"tws_traderbot/backend/db"
	"tws_traderbot/backend/models"

	"github.com/scmhub/ibapi"
)

// TestIBWrapper_ThreadSafety verifies that concurrent bar streaming and reading
// does not trigger data races under `go test -race`.
func TestIBWrapper_ThreadSafety(t *testing.T) {
	eng := NewEngine()
	eng.SetGlobalEnabled(true)
	_ = eng.StartBot(&models.Strategy{ID: "test_strat"})
	eng.SetWatchlistItem(&models.WatchlistItem{
		Symbol:      "AAPL",
		Exchange:    "SMART",
		Venue:       "SMART",
		Enabled:     true,
		IsOrderable: true,
		SizeMode:    "shares",
		SizeValue:   100.0,
	})
	eng.SetLatestPrice("AAPL", 150.0)
	eng.SetTimeProvider(func() time.Time {
		// Wednesday 10:00 AM ET (RTH)
		return time.Date(2026, 11, 18, 10, 0, 0, 0, eng.sessionGate.Location())
	})

	wrapper := &IBWrapper{
		engine:     eng,
		marketData: make(map[string][]float64),
	}
	wrapper.nextOrderID.Store(100)

	var wg sync.WaitGroup
	const workers = 16
	const barsPerWorker = 50

	// Concurrently push historical bars
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(reqID int64) {
			defer wg.Done()
			for b := 0; b < barsPerWorker; b++ {
				wrapper.HistoricalData(reqID, &ibapi.Bar{
					Open:  float64(100 + b),
					High:  float64(105 + b),
					Low:   float64(95 + b),
					Close: float64(102 + b),
				})
			}
			wrapper.HistoricalDataEnd(reqID, "", "")
		}(int64(w + 1))
	}

	// Concurrently allocate order IDs via ExecuteAction
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wrapper.ExecuteAction("AAPL", "BUY")
		}()
	}

	wg.Wait()

	finalOrderID := wrapper.nextOrderID.Load()
	if finalOrderID != int64(100+workers) {
		t.Errorf("expected nextOrderID %d, got %d", 100+workers, finalOrderID)
	}
}

func TestNextValidID_Monotonicity(t *testing.T) {
	wrapper := &IBWrapper{}
	wrapper.nextOrderID.Store(1000)

	// Backwards regression ignored
	wrapper.NextValidID(500)
	if id := wrapper.nextOrderID.Load(); id != 1000 {
		t.Fatalf("expected orderID 1000 after regressive NextValidID(500), got %d", id)
	}

	// Non-positive ignored
	wrapper.NextValidID(-10)
	if id := wrapper.nextOrderID.Load(); id != 1000 {
		t.Fatalf("expected orderID 1000 after negative NextValidID(-10), got %d", id)
	}
	wrapper.NextValidID(0)
	if id := wrapper.nextOrderID.Load(); id != 1000 {
		t.Fatalf("expected orderID 1000 after zero NextValidID(0), got %d", id)
	}

	// Forward advance accepted
	wrapper.NextValidID(2000)
	if id := wrapper.nextOrderID.Load(); id != 2000 {
		t.Fatalf("expected orderID 2000 after forward NextValidID(2000), got %d", id)
	}
}

func TestExecuteAction_LiveBracketFailureFailClosed(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_bracket_leak.db")
	database, err := db.Connect(dbPath)
	if err != nil {
		t.Fatalf("failed to connect sqlite: %v", err)
	}
	defer database.Close()

	eng, _ := setupFullyArmedEngine()
	eng.AttachDB(database)
	eng.SetDryRun(false)                   // live mode!
	eng.SetBracketConfig(true, 5.0, 150.0) // 150% SL causes stop price <= 0 -> CreateBracketOrders fails

	wrapper := &IBWrapper{
		engine:     eng,
		marketData: make(map[string][]float64),
	}
	wrapper.nextOrderID.Store(5000)

	// ExecuteAction must fail closed, NOT falling through to unbracketed live market order
	wrapper.ExecuteAction("AAPL", "BUY")

	// Verify no trades were recorded in database
	trades, err := database.GetRecentTrades(10)
	if err != nil {
		t.Fatalf("failed to query trades: %v", err)
	}
	if len(trades) > 0 {
		t.Fatalf("expected 0 trades recorded on bracket failure, got %d", len(trades))
	}
}

func TestIBKRConnection(t *testing.T) {
	if os.Getenv("TWS_LIVE_TEST") != "1" {
		t.Skip("Skipping live TWS connection test; set TWS_LIVE_TEST=1 to run against live Gateway")
	}

	eng := NewEngine()
	client := NewIBKRClient(eng)
	err := client.Connect("127.0.0.1", 7497, 1)
	if err != nil {
		t.Fatalf("Failed to connect to live TWS: %v", err)
	}
	t.Log("Successfully connected to live TWS on 127.0.0.1:7497")
	client.Disconnect()
}

func readClientHandshakePrefix(conn net.Conn) error {
	hdr := make([]byte, 8)
	if _, err := io.ReadFull(conn, hdr); err != nil {
		return err
	}
	vLen := binary.BigEndian.Uint32(hdr[4:8])
	if vLen > 0 && vLen < 4096 {
		vBuf := make([]byte, vLen)
		if _, err := io.ReadFull(conn, vBuf); err != nil {
			return err
		}
	}
	return nil
}

func writeServerHandshakeFrame(conn net.Conn) error {
	payload := []byte("176\x0020261117 10:00:00 EST\x00")
	frame := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(frame[:4], uint32(len(payload)))
	copy(frame[4:], payload)
	_, err := conn.Write(frame)
	return err
}

func TestIBKRClient_HandshakeCleanEOF_ReturnsErrorAndDisconnected(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen failed: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				_ = readClientHandshakePrefix(c)
				_ = c.Close() // Clean TCP FIN (io.EOF) before serverVersion
			}(conn)
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)

	eng := NewEngine()
	client := NewIBKRClient(eng)
	if err := client.Connect("127.0.0.1", addr.Port, 1); err == nil {
		t.Fatalf("expected non-nil error on clean handshake EOF, got nil (IsConnected=%v)", client.IsConnected())
	}
	if client.IsConnected() {
		t.Fatalf("expected IBKRClient.IsConnected() == false after handshake EOF")
	}

	raw := ibapi.NewEClient(NewIBWrapper(eng))
	if err := raw.Connect("127.0.0.1", addr.Port, 2); err == nil {
		t.Fatalf("expected raw EClient.Connect to return non-nil error on clean handshake EOF, got nil")
	}
	if raw.IsConnected() {
		t.Fatalf("expected raw EClient.IsConnected() == false after handshake EOF")
	}
}

func TestIBKRClient_PostHandshakeEOFAndConcurrentReqDisconnectRace(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen failed: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				if err := readClientHandshakePrefix(c); err != nil {
					return
				}
				if err := writeServerHandshakeFrame(c); err != nil {
					return
				}
				// Drain briefly or drop immediately after handshake to race EReader EOF watcher vs Connect/Req*/Disconnect
				buf := make([]byte, 64)
				_ = c.SetReadDeadline(time.Now().Add(2 * time.Millisecond))
				_, _ = c.Read(buf)
			}(conn)
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)
	eng := NewEngine()
	client := NewIBKRClient(eng)
	w := client.Wrapper()

	for cycle := 0; cycle < 15; cycle++ {
		_ = client.Connect("127.0.0.1", addr.Port, 100+cycle)

		var wg sync.WaitGroup
		wg.Add(3)
		go func() {
			defer wg.Done()
			w.ManagedAccounts([]string{"DU999001"})
		}()
		go func() {
			defer wg.Done()
			w.Error(-1, 0, 1102, "Connectivity restored", "")
		}()
		go func() {
			defer wg.Done()
			client.Disconnect()
		}()
		wg.Wait()
	}

	raw := ibapi.NewEClient(NewIBWrapper(eng))
	for cycle := 0; cycle < 15; cycle++ {
		_ = raw.Connect("127.0.0.1", addr.Port, int64(200+cycle))
		var wg sync.WaitGroup
		wg.Add(3)
		go func() {
			defer wg.Done()
			raw.ReqPositions()
		}()
		go func() {
			defer wg.Done()
			raw.ReqPnL(int64(cycle+1), "DU999001", "")
		}()
		go func() {
			defer wg.Done()
			_ = raw.Disconnect()
		}()
		wg.Wait()
	}
}
