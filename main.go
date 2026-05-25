package main

import (
	"embed"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := application.New(application.Options{
		Name: "tws_traderbot",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	svc := NewApp(app)
	app.RegisterService(application.NewService(svc))

	application.NewWindow(application.WebviewWindowOptions{
		Title:            "tws_traderbot",
		Width:            1024,
		Height:           768,
		BackgroundColour: application.NewRGB(27, 38, 54),
	})

	if err := app.Run(); err != nil {
		println("Error:", err.Error())
	}
}
