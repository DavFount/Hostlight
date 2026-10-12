package main

import (
	"testing"
	"time"
)

func TestActiveOutageObservedDuration(t *testing.T) {
	s := historyFixture(t)
	a := NewApp()
	a.history = s
	target := fixtureTarget()
	target.ID = "1"
	target.State = "down"
	at := time.Now().UTC()
	applyHistory(t, s, target, "result", at)
	applyHistory(t, s, target, "result", at.Add(10*time.Second))
	check := func(want int64) {
		t.Helper()
		items, err := a.GetActiveOutages()
		if err != nil || len(items) != 1 || items[0].ID != "1" || items[0].DurationSeconds != want {
			t.Fatalf("got %+v, %v; want %d", items, err, want)
		}
	}
	check(10)
	applyHistory(t, s, target, "Monitoring paused", at.Add(15*time.Second))
	check(10)
	applyHistory(t, s, target, "Monitoring resumed", at.Add(time.Hour))
	check(10)
	applyHistory(t, s, target, "result", at.Add(time.Hour+5*time.Second))
	check(15)
	target.State = "up"
	applyHistory(t, s, target, "result", at.Add(time.Hour+10*time.Second))
	items, err := a.GetActiveOutages()
	if err != nil || len(items) != 0 {
		t.Fatalf("resolved outage retained: %+v %v", items, err)
	}
}

func TestActiveOutageUnavailable(t *testing.T) {
	if _, err := NewApp().GetActiveOutages(); err == nil {
		t.Fatal("missing history should report error")
	}
}

func TestActiveOutageGapAndEdit(t *testing.T) {
	s := historyFixture(t)
	a := NewApp()
	a.history = s
	target := fixtureTarget()
	target.ID = "1"
	target.State = "down"
	at := time.Now().UTC()
	applyHistory(t, s, target, "result", at)
	applyHistory(t, s, target, "result", at.Add(24*time.Hour))
	items, err := a.GetActiveOutages()
	if err != nil || len(items) != 1 || items[0].DurationSeconds != 0 {
		t.Fatalf("gap counted as outage: %+v %v", items, err)
	}
	applyHistory(t, s, target, "Monitor edited", at.Add(24*time.Hour+time.Second))
	items, err = a.GetActiveOutages()
	if err != nil || len(items) != 0 {
		t.Fatalf("edited outage retained: %+v %v", items, err)
	}
}
