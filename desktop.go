package main

import (
	"context"
	"fmt"
	"runtime"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type trayHandle interface{ Close() }
type desktopState struct {
	tray        trayHandle
	started     bool
	available   bool
	closeToTray bool
	quit        bool
	show        func()
	hide        func()
	exit        func()
}
type DesktopStatus struct {
	Supported   bool `json:"supported"`
	Available   bool `json:"available"`
	CloseToTray bool `json:"closeToTray"`
}

func (a *App) desktopReady(ctx context.Context) {
	a.mu.Lock()
	if a.desktop.started || a.closing {
		a.mu.Unlock()
		return
	}
	a.desktop.started = true
	// Wails' Windows WindowShow restores minimized windows and brings them
	// forward without unnecessarily restoring an already-maximized window.
	a.desktop.show = func() { wruntime.WindowShow(ctx) }
	a.desktop.hide = func() { wruntime.WindowHide(ctx) }
	a.desktop.exit = func() { wruntime.Quit(ctx) }
	a.mu.Unlock()
	go func() {
		tray, err := startNativeTray(a.ShowHostlight, a.QuitHostlight, func(available bool) {
			a.mu.Lock()
			a.desktop.available = available
			closing := a.closing
			a.mu.Unlock()
			if !available && !closing {
				go a.ShowHostlight()
			}
		})
		a.mu.Lock()
		if err != nil {
			a.desktop.available = false
			a.mu.Unlock()
			return
		}
		if a.closing {
			a.mu.Unlock()
			tray.Close()
			return
		}
		a.desktop.tray = tray
		a.mu.Unlock()
	}()
}

func (a *App) GetDesktopStatus() DesktopStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return DesktopStatus{Supported: runtime.GOOS == "windows", Available: a.desktop.available, CloseToTray: a.desktop.closeToTray}
}
func (a *App) SetCloseToTray(enabled bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closing {
		return fmt.Errorf("Hostlight is shutting down")
	}
	if enabled && !a.desktop.available {
		return fmt.Errorf("The Windows tray is unavailable; close-to-tray was not enabled")
	}
	previous := a.desktop.closeToTray
	a.desktop.closeToTray = enabled
	if err := a.persistLocked(); err != nil {
		a.desktop.closeToTray = previous
		return err
	}
	return nil
}
func (a *App) ShowHostlight() {
	a.mu.Lock()
	show := a.desktop.show
	closing := a.closing
	a.mu.Unlock()
	if !closing && show != nil {
		show()
	}
}
func (a *App) QuitHostlight() {
	a.mu.Lock()
	if a.desktop.quit || a.closing {
		a.mu.Unlock()
		return
	}
	exit := a.desktop.exit
	if exit != nil {
		a.desktop.quit = true
	}
	a.mu.Unlock()
	if exit != nil {
		exit()
	}
}
func (a *App) HideHostlight() error {
	a.mu.Lock()
	hide := a.desktop.hide
	available := a.desktop.available
	closing := a.closing
	a.mu.Unlock()
	if !available || hide == nil || closing {
		return fmt.Errorf("The tray is unavailable. Hostlight will stay visible")
	}
	hide()
	// If Explorer disappeared between our availability check and hiding, its
	// loss callback may already have restored the window. Restore again rather
	// than leaving the user without an accessible icon.
	a.mu.Lock()
	stillAvailable := a.desktop.available
	a.mu.Unlock()
	if !stillAvailable {
		a.ShowHostlight()
		return fmt.Errorf("The tray became unavailable. Hostlight was restored")
	}
	return nil
}
func (a *App) beforeClose(context.Context) bool {
	a.mu.Lock()
	hide := a.desktop.hide
	prevent := !a.closing && !a.desktop.quit && a.desktop.closeToTray && a.desktop.available && hide != nil
	a.mu.Unlock()
	if prevent {
		return a.HideHostlight() == nil
	}
	return prevent
}
