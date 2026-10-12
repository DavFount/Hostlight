package main

import (
	"math"
	"testing"
	"time"
)

func saveProbe(t *testing.T, s *incidentStore, id string, at time.Time, failed bool, latency float64) {
	t.Helper()
	target := fixtureTarget()
	target.ID = id
	target.Type = "tcp"
	target.State = "up"
	err := s.apply(historyEvent{target: target, kind: "result", at: at, sample: &storedProbeSample{Session: "session-1", Failed: failed, Latency: latency}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestProbeHistoryAggregationAndRestart(t *testing.T) {
	s := historyFixture(t)
	at := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	saveProbe(t, s, "1", at, false, 10)
	saveProbe(t, s, "1", at.Add(time.Second), false, 30)
	saveProbe(t, s, "1", at.Add(2*time.Second), true, 9999)
	saveProbe(t, s, "1", at.Add(time.Minute), false, 100)
	saveProbe(t, s, "2", at, true, 9999)
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM probe_minutes WHERE target_id='1'").Scan(&count); err != nil || count != 2 {
		t.Fatalf("not minute summaries: %d %v", count, err)
	}
	page, err := s.probeHistory("1", 24, at.Add(2*time.Minute))
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("aggregation %+v %v", page, err)
	}
	b := page.Items[0]
	if b.Checks != 4 || b.Failed != 1 || b.MinMS != 10 || b.MaxMS != 100 || math.Abs(b.MeanMS-140.0/3) > 0.0001 {
		t.Fatalf("weighted stats: %+v", b)
	}
	failed, err := s.probeHistory("2", 1, at.Add(2*time.Minute))
	if err != nil || len(failed.Items) != 1 || failed.Items[0].Checks != failed.Items[0].Failed || failed.Items[0].MeanMS != 0 {
		t.Fatalf("all-failed: %+v %v", failed, err)
	}
	if _, err := s.probeHistory("1", 999, at); err == nil {
		t.Fatal("invalid range accepted")
	}
	path := s.path
	s.db.Close()
	reopened, err := openIncidentStore(path, at.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.db.Close()
	a := &App{history: reopened}
	monitors, err := a.GetProbeHistoryMonitors()
	if err != nil || len(monitors) != 2 {
		t.Fatalf("restart lost monitor choices: %+v %v", monitors, err)
	}
	got, err := reopened.probeHistory("1", 24, at.Add(2*time.Minute))
	if err != nil || len(got.Items) != 1 || got.Items[0].Checks != 4 {
		t.Fatalf("restart lost samples: %+v %v", got, err)
	}
}

func TestProbeHistoryHardBoundAndRetention(t *testing.T) {
	s := historyFixture(t)
	// Seed many distinct summaries efficiently; normal upserts enforce the bound.
	_, err := s.db.Exec(`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i<50000)
 INSERT INTO probe_minutes(target_id,session,name,probe_type,minute,checks,failed,latency_sum,latency_min,latency_max)
 SELECT 'seed','session','Seed','tcp',i,1,0,10,10,10 FROM n`)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	saveProbe(t, s, "new", at, false, 15)
	var count int
	var oldest int64
	if err := s.db.QueryRow("SELECT count(*),MIN(id) FROM probe_minutes").Scan(&count, &oldest); err != nil || count != probeMinuteLimit || oldest != 2 {
		t.Fatalf("cap: %d %d %v", count, oldest, err)
	}
	// Repeated checks in the same minute must not evict another summary.
	saveProbe(t, s, "new", at, false, 25)
	if err := s.db.QueryRow("SELECT MIN(id) FROM probe_minutes").Scan(&oldest); err != nil || oldest != 2 {
		t.Fatalf("upsert evicted data: %d %v", oldest, err)
	}
	if err := s.prune(at); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow("SELECT count(*) FROM probe_minutes").Scan(&count); err != nil || count != 1 {
		t.Fatalf("retention: %d %v", count, err)
	}
}

func TestProbeHistoryMigrationPreservesIncidents(t *testing.T) {
	s := historyFixture(t)
	target := fixtureTarget()
	target.ID = "1"
	target.State = "down"
	at := time.Now().UTC()
	applyHistory(t, s, target, "result", at)
	if _, err := s.db.Exec("DROP TABLE probe_minutes; PRAGMA user_version=5;"); err != nil {
		t.Fatal(err)
	}
	s.db.Close()
	upgraded, err := openIncidentStore(s.path, at)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.db.Close()
	items := historyItems(t, upgraded)
	if len(items) != 1 || items[0].Status != "interrupted" {
		t.Fatalf("migration damaged incidents: %+v", items)
	}
	saveProbe(t, upgraded, "1", at, false, 10)
	page, err := upgraded.probeHistory("1", 1, at)
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("upgraded storage unusable: %+v %v", page, err)
	}
}
