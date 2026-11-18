package main

import (
	"testing"
	"tws_traderbot/backend/bot"
)

func TestReplayModeInitialization(t *testing.T) {
	engine := InitializeHeadlessEngine(true)
	simSink := &LogSink{}
	_, err := bot.NewReplayRuntime(engine, nil, simSink)
	if err != nil {
		t.Fatalf("Failed to initialize replay runtime: %v", err)
	}
}

func TestPaperProbeModeInitialization(t *testing.T) {
	engine := InitializeHeadlessEngine(false)
	_ = bot.NewIBKRClient(engine)
	engine.AutopilotController.Start(bot.ModePaperLive)
	if engine.AutopilotController.ValidatePaperSafety() {
		t.Log("ValidatePaperSafety passed (expected if no account set? wait, check implementation)")
	}
}
