package main

import (
	"tws_traderbot/backend/bot"
)

// InitializeHeadlessEngine creates a pre-configured engine for headless operation
func InitializeHeadlessEngine(dryRun bool) *bot.Engine {
	engine := bot.NewEngine()
	engine.SetDryRun(dryRun)
	return engine
}
