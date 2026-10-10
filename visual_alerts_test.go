package main

import (
	"testing"
	"time"
)

func TestVisualAlertsTransitions(t *testing.T) {
	s := historyFixture(t)
	a := &App{history: s}
	target := Target{ID: "visual", Name: "Router", State: "down", Interval: 5, Timeout: 3}
	at := time.Now()
	apply := func(state, previous string) {
		t.Helper()
		target.State = state
		at = at.Add(time.Second)
		if err := s.apply(historyEvent{target: target, kind: "result", previous: previous, at: at}); err != nil {
			t.Fatal(err)
		}
	}
	apply("down", "unknown")
	apply("up", "down")
	if len(a.GetVisualAlerts(0, false).Items) != 0 {
		t.Fatal("initial discovery must not alert")
	}
	apply("down", "up")
	apply("down", "down")
	apply("up", "down")
	page := a.GetVisualAlerts(0, false)
	if len(page.Items) != 2 || page.Items[0].Event != "down" || page.Items[1].Event != "up" {
		t.Fatalf("transitions: %+v", page)
	}
	if len(a.GetVisualAlerts(page.Cursor, false).Items) != 0 {
		t.Fatal("cursor replay")
	}
	if len(a.GetVisualAlerts(0, true).Items) != 0 {
		t.Fatal("baseline replay")
	}
}

func TestVisualAlertsPauseRetainsRecovery(t *testing.T) {
	s := historyFixture(t)
	a := &App{history: s}
	target := Target{ID: "visual", Name: "Router", State: "down", Interval: 5, Timeout: 3}
	at := time.Now()
	for _, e := range []historyEvent{
		{target: target, kind: "result", previous: "up", at: at},
		{target: target, kind: "Monitoring paused", at: at.Add(time.Second)},
		{target: target, kind: "Monitoring resumed", at: at.Add(time.Hour)},
	} {
		if err := s.apply(e); err != nil {
			t.Fatal(err)
		}
	}
	target.State = "up"
	if err := s.apply(historyEvent{target: target, kind: "result", previous: "unknown", at: at.Add(time.Hour + time.Second)}); err != nil {
		t.Fatal(err)
	}
	page := a.GetVisualAlerts(0, false)
	if len(page.Items) != 2 || page.Items[1].Event != "up" {
		t.Fatalf("pause recovery: %+v", page)
	}
}

func TestVisualAlertsSilencedAndBounded(t *testing.T) {
	s := historyFixture(t)
	a := &App{history: s}
	at := time.Now()
	_, err := s.db.Exec("INSERT INTO alert_silences(id,target_id,kind,starts,ends,reason) VALUES('silence','visual','snooze',?,?,'')", at.Add(-time.Minute).UnixMilli(), at.Add(time.Minute).UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	target := Target{ID: "visual", Name: "Router", State: "down", Interval: 5, Timeout: 3}
	if err = s.apply(historyEvent{target: target, kind: "result", previous: "up", at: at}); err != nil {
		t.Fatal(err)
	}
	target.State = "up"
	if err = s.apply(historyEvent{target: target, kind: "result", previous: "down", at: at.Add(2 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if len(a.GetVisualAlerts(0, false).Items) != 0 {
		t.Fatal("silenced outage must not produce orphan recovery")
	}
	for i := 0; i < 110; i++ {
		target.State = "down"
		at = at.Add(3 * time.Minute)
		if err = s.apply(historyEvent{target: target, kind: "result", previous: "up", at: at}); err != nil {
			t.Fatal(err)
		}
		target.State = "up"
		if err = s.apply(historyEvent{target: target, kind: "result", previous: "down", at: at.Add(time.Second)}); err != nil {
			t.Fatal(err)
		}
	}
	page := a.GetVisualAlerts(1, false)
	if len(page.Items) != 100 || !page.Dropped {
		t.Fatalf("bounded feed: %+v", page)
	}
}
