package api

import (
	"testing"
	"tws_traderbot/backend/bot"
)

func TestGetAutopilotState(t *testing.T) {
	engine := bot.NewEngine()
	state := GetAutopilotState(engine)
	if state == nil {
		t.Fatal("Expected state, got nil")
	}
	
	if state.ExecMode != int(bot.ModeOff) {
		t.Errorf("Expected ExecMode ModeOff, got %d", state.ExecMode)
	}
}
