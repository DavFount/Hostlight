package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSchedulingPersistenceAndRestart(t *testing.T) {
	a := NewApp()
	a.configPath = filepath.Join(t.TempDir(), "targets.json")
	desired := SchedulingSettings{3, 1500}
	if err := a.SaveSchedulingSettings(desired); err != nil {
		t.Fatal(err)
	}
	s := a.GetSchedulingSettings()
	if s.Saved != desired || !s.RestartRequired || cap(a.slots) != 32 {
		t.Fatal(s)
	}
	b := NewApp()
	b.configPath = a.configPath
	b.loadConfiguration()
	if s = b.GetSchedulingSettings(); s.Active != desired || s.RestartRequired || cap(b.slots) != 3 {
		t.Fatal(s)
	}
}
func TestSchedulingValidationAndRollback(t *testing.T) {
	a := NewApp()
	a.configPath = ""
	before := a.GetSchedulingSettings()
	for _, s := range []SchedulingSettings{{0, 0}, {129, 0}, {32, -1}, {32, 5001}} {
		if a.SaveSchedulingSettings(s) == nil || a.GetSchedulingSettings() != before {
			t.Fatal(s)
		}
	}
	a.configError = "fixture"
	if a.SaveSchedulingSettings(SchedulingSettings{1, 0}) == nil || a.GetSchedulingSettings() != before {
		t.Fatal("failed save changed settings")
	}
}
func TestSchedulingInvalidFilePreserved(t *testing.T) {
	a := NewApp()
	a.configPath = filepath.Join(t.TempDir(), "targets.json")
	data := []byte(`{"version":4,"scheduling":{"concurrency":0,"spreadMilliseconds":0},"targets":[]}`)
	if err := os.WriteFile(a.configPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	a.loadConfiguration()
	got, _ := os.ReadFile(a.configPath)
	if a.configError == "" || string(got) != string(data) || cap(a.slots) != 32 {
		t.Fatal("invalid settings not preserved")
	}
}
func TestProbeSpreadAndCancellation(t *testing.T) {
	seen := map[time.Duration]bool{}
	for _, id := range []string{"1", "2", "3", "4", "5"} {
		d := probeStartDelay(id, 250)
		if d < 0 || d > 250*time.Millisecond || d != probeStartDelay(id, 250) {
			t.Fatal(d)
		}
		seen[d] = true
	}
	if len(seen) < 2 || probeStartDelay("1", 0) != 0 {
		t.Fatal("not spread")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if waitForProbe(ctx, time.Hour) {
		t.Fatal("ignored cancellation")
	}
}
