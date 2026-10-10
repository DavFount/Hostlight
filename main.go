package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "Hostlight",
		Width:     1280,
		Height:    820,
		MinWidth:  760,
		MinHeight: 580,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour:   &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:          app.startup,
		OnDomReady:         app.desktopReady,
		OnBeforeClose:      app.beforeClose,
		OnShutdown:         app.shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{UniqueId: "hostlight-desktop-91be6b13-35d0-4f18-aad5-402d19b3aa49", OnSecondInstanceLaunch: func(options.SecondInstanceData) { app.ShowHostlight() }},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
