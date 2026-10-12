package main

import (
	"embed"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()
	var windowsOptions *windows.Options
	if app.developmentWorkspace != "" {
		windowsOptions = &windows.Options{WebviewUserDataPath: filepath.Join(app.developmentWorkspace, "webview")}
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:     app.windowTitle(),
		Windows:   windowsOptions,
		Width:     1280,
		Height:    820,
		MinWidth:  760,
		MinHeight: 580,
		Menu:      app.applicationMenu(),
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour:   &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:          app.startup,
		OnDomReady:         app.desktopReady,
		OnBeforeClose:      app.beforeClose,
		OnShutdown:         app.shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{UniqueId: app.desktopInstanceID(), OnSecondInstanceLaunch: func(options.SecondInstanceData) { app.ShowHostlight() }},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
