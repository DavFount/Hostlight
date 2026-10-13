package main

import (
	"testing"
	"time"
)

func TestStoragePrunePreservesPendingAndActiveRecoveryMarkers(t *testing.T) {
	s := historyFixture(t)
	now := time.Now().UTC()
	old := now.AddDate(0, 0, -100).UnixMilli()
	target := fixtureTarget()
	target.ID, target.State = "active-retention", "down"
	applyHistory(t, s, target, "result", time.UnixMilli(old))
	incident := historyItems(t, s)[0].ID
	for _, item := range []struct {
		id, status string
		incident   int64
	}{
		{"pending-old", "pending", 0},
		{"active-marker", "delivered", incident},
		{"expired-delivery", "delivered", 0},
	} {
		_, err := s.db.Exec(`INSERT INTO deliveries(id,incident_id,event,payload,secret,created,next_attempt,status) VALUES(?,?,'down',X'7B7D',X'00',?,?,?)`, item.id, item.incident, old, old, item.status)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []struct {
		id   string
		ends time.Time
	}{
		{"expired", now.AddDate(0, 0, -31)},
		{"recent", now.Add(-time.Hour)},
		{"future", now.Add(time.Hour)},
	} {
		_, err := s.db.Exec(`INSERT INTO alert_silences(id,target_id,kind,starts,ends,reason) VALUES(?,'active-retention','maintenance',?,?,'')`, item.id, old, item.ends.UnixMilli())
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := s.prune(now); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"pending-old", "active-marker"} {
		var count int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM deliveries WHERE id=?", id).Scan(&count); err != nil || count != 1 {
			t.Fatalf("required delivery removed: %s count=%d error=%v", id, count, err)
		}
	}
	var expired, silences int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM deliveries WHERE id='expired-delivery'").Scan(&expired); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM alert_silences").Scan(&silences); err != nil {
		t.Fatal(err)
	}
	items := historyItems(t, s)
	if expired != 0 || silences != 2 || len(items) != 1 || items[0].Status != "open" {
		t.Fatalf("prune damaged retained state: expired=%d silences=%d incidents=%+v", expired, silences, items)
	}
}

func TestProbeRetentionExactMinuteBoundary(t *testing.T) {
	s := historyFixture(t)
	now := time.Now().UTC().Truncate(time.Minute)
	cutoff := now.Add(-7 * 24 * time.Hour)
	saveProbe(t, s, "older", cutoff.Add(-time.Minute), false, 1)
	saveProbe(t, s, "boundary", cutoff, false, 2)
	saveProbe(t, s, "recent", now, true, 0)
	if err := s.prune(now); err != nil {
		t.Fatal(err)
	}
	var checks, failed, summaries int
	if err := s.db.QueryRow("SELECT SUM(checks),SUM(failed),COUNT(*) FROM probe_minutes").Scan(&checks, &failed, &summaries); err != nil {
		t.Fatal(err)
	}
	if checks != 2 || failed != 1 || summaries != 2 {
		t.Fatalf("wrong boundary retention: checks=%d failed=%d summaries=%d", checks, failed, summaries)
	}
	for id, expected := range map[string]int{"older": 0, "boundary": 1, "recent": 1} {
		var count int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM probe_minutes WHERE target_id=?", id).Scan(&count); err != nil || count != expected {
			t.Fatalf("retention boundary for %s: count=%d expected=%d error=%v", id, count, expected, err)
		}
	}
}
