package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopClosePolicy(t *testing.T) {
	for _, tc := range []struct {
		name                                       string
		enabled, available, quit, closing, prevent bool
	}{
		{name: "normal close exits"},
		{name: "opt in hides", enabled: true, available: true, prevent: true},
		{name: "no reachable tray exits", enabled: true},
		{name: "explicit quit bypasses hide", enabled: true, available: true, quit: true},
		{name: "shutdown bypasses hide", enabled: true, available: true, closing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hidden := 0
			a := NewApp()
			a.desktop = desktopState{closeToTray: tc.enabled, available: tc.available, quit: tc.quit, hide: func() { hidden++ }}
			a.closing = tc.closing
			if got := a.beforeClose(context.Background()); got != tc.prevent {
				t.Fatalf("prevent=%v", got)
			}
			if (hidden == 1) != tc.prevent {
				t.Fatalf("hidden=%d", hidden)
			}
		})
	}
}
func TestDesktopSettingsPersistenceAndRollback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "targets.json")
	a := storedApp(t, path)
	a.desktop.available = true
	if err := a.SetCloseToTray(true); err != nil {
		t.Fatal(err)
	}
	restored := storedApp(t, path)
	if !restored.GetDesktopStatus().CloseToTray {
		t.Fatal("close preference lost on restart")
	}
	a.configError = "read only"
	if err := a.SetCloseToTray(false); err == nil {
		t.Fatal("write failure not reported")
	}
	if !a.GetDesktopStatus().CloseToTray {
		t.Fatal("write failure changed preference")
	}
	a.desktop.available = false
	if err := a.SetCloseToTray(true); err == nil {
		t.Fatal("enabled without tray")
	}
	a.configError = ""
	if err := a.SetCloseToTray(false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		t.Fatal("settings file lost")
	}
}
func TestDesktopHideKeepsMonitoringAndExplicitQuit(t *testing.T) {
	a := NewApp()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hidden, shown, quits := 0, 0, 0
	a.desktop = desktopState{available: true, closeToTray: true, hide: func() { hidden++ }, show: func() { shown++ }, exit: func() {
		quits++
		if a.beforeClose(ctx) {
			t.Fatal("explicit quit hidden")
		}
	}}
	a.targets["1"] = &monitor{target: Target{ID: "1", State: "down"}, cancel: cancel}
	if err := a.HideHostlight(); err != nil {
		t.Fatal(err)
	}
	if hidden != 1 || ctx.Err() != nil || a.targets["1"].target.State != "down" || a.closing {
		t.Fatal("hiding changed monitoring")
	}
	a.ShowHostlight()
	if shown != 1 {
		t.Fatal("show not called")
	}
	a.QuitHostlight()
	a.QuitHostlight()
	if quits != 1 {
		t.Fatal("quit must be explicit and idempotent")
	}
}
func TestDesktopHideAvailabilityRace(t *testing.T) {
	a := NewApp()
	shown := 0
	a.desktop.available = true
	a.desktop.show = func() { shown++ }
	a.desktop.hide = func() { a.mu.Lock(); a.desktop.available = false; a.mu.Unlock() }
	if a.HideHostlight() == nil || shown != 1 {
		t.Fatal("window not restored after tray loss")
	}
}
