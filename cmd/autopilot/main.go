package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"tws_traderbot/backend/bot"
	"github.com/scmhub/ibapi"
)

type LogSink struct{}

func (l *LogSink) OpenOrder(orderID int64, contract *ibapi.Contract, order *ibapi.Order, orderState *ibapi.OrderState) {
	log.Printf("Replay Sink: OpenOrder: %d, %s", orderID, contract.Symbol)
}
func (l *LogSink) OrderStatus(orderID int64, status string, filled ibapi.Decimal, remaining ibapi.Decimal, avgFillPrice float64, permID int64, parentID int64, lastFillPrice float64, clientID int64, whyHeld string, mktCapPrice float64) {
	log.Printf("Replay Sink: OrderStatus: %d, %s", orderID, status)
}
func (l *LogSink) ExecDetails(reqID int64, contract *ibapi.Contract, execution *ibapi.Execution) {
	log.Printf("Replay Sink: ExecDetails: %d, %s", reqID, contract.Symbol)
}
func (l *LogSink) CommissionAndFeesReport(commissionReport *ibapi.CommissionAndFeesReport) {
	log.Printf("Replay Sink: CommissionAndFeesReport")
}

func main() {
	mode := flag.String("mode", "dry-run", "Execution mode (dry-run, live, replay, paper-probe)")
	dbPath := flag.String("db", "cobalt.db", "Path to SQLite database")
	sourceDB := flag.String("source-db", "", "Path to source database (optional)")
	host := flag.String("host", "127.0.0.1", "IBKR TWS Host")
	port := flag.Int("port", 7496, "IBKR TWS Port")
	flag.Parse()

	log.Printf("Starting autopilot in %s mode...", *mode)
	log.Printf("Using DB: %s, Source DB: %s", *dbPath, *sourceDB)
	log.Printf("Connecting to IBKR at %s:%d", *host, *port)

	// Initialize Engine
	engine := bot.NewEngine()
	// Configure based on flags...
	switch *mode {
	case "dry-run":
		engine.SetDryRun(true)
	case "replay":
		simSink := &LogSink{}
		_, err := bot.NewReplayRuntime(engine, nil, simSink)
		if err != nil {
			log.Fatalf("Failed to initialize replay runtime: %v", err)
		}
		log.Println("Replay runtime initialized.")
	case "paper-probe":
		_ = bot.NewIBKRClient(engine)
		engine.AutopilotController.Start(bot.ModePaperLive)
		log.Println("Paper-probe mode started.")
	default:
		engine.SetDryRun(false)
	}


	// Setup signal handling
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan bool, 1)

	go func() {
		sig := <-sigs
		fmt.Println()
		log.Println("Received signal:", sig)
		done <- true
	}()

	log.Println("Autopilot running. Press Ctrl+C to stop.")

	<-done
	log.Println("Autopilot stopped.")
}
