package main

import (
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) applicationMenu() *menu.Menu {
	root := menu.NewMenu()
	navigate := func(destination string) menu.Callback {
		return func(*menu.CallbackData) {
			a.mu.Lock()
			ctx := a.ctx
			closing := a.closing
			a.mu.Unlock()
			if ctx != nil && !closing {
				a.ShowHostlight()
				wruntime.EventsEmit(ctx, "hostlight:navigate", destination)
			}
		}
	}
	file := root.AddSubmenu("File")
	file.AddText("Settings…", keys.CmdOrCtrl(","), navigate("settings"))
	file.AddText("Monitor backups…", nil, navigate("backups"))
	file.AddSeparator()
	file.AddText("Quit Hostlight", keys.CmdOrCtrl("q"), func(*menu.CallbackData) { a.QuitHostlight() })
	view := root.AddSubmenu("View")
	view.AddText("Monitors", keys.CmdOrCtrl("1"), navigate("monitors"))
	view.AddText("Incident history", keys.CmdOrCtrl("2"), navigate("history"))
	help := root.AddSubmenu("Help")
	help.AddText("About Hostlight", nil, navigate("about"))
	return root
}
