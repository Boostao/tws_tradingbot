package main

import (
	"embed"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {
	app := application.New(application.Options{
		Name: "tws_traderbot",
		Icon: appIcon,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Linux: application.LinuxOptions{
			ApplicationID: "ca.boostao.tws_traderbot",
			ProgramName:   "ca.boostao.tws_traderbot",
		},
	})

	svc := NewApp(app)
	app.RegisterService(application.NewService(svc))

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "Cobalt — Poste de pilotage TWS",
		Width:            1440,
		Height:           900,
		MinWidth:         1024,
		MinHeight:        700,
		BackgroundColour: application.NewRGB(10, 12, 16),
		URL:              "/",
		Linux: application.LinuxWindow{
			Icon: appIcon,
		},
	})

	if err := app.Run(); err != nil {
		println("Error:", err.Error())
	}
}
