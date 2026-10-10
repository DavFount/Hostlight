package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func historyFixture(t *testing.T) *incidentStore {
	t.Helper()
	s, err := openIncidentStore(filepath.Join(t.TempDir(), "history.db"), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.db.Close() })
	return s
}
func applyHistory(t *testing.T, s *incidentStore, target Target, kind string, at time.Time) {
	t.Helper()
	if err := s.apply(historyEvent{target: target, kind: kind, at: at}); err != nil {
		t.Fatal(err)
	}
}
func historyItems(t *testing.T, s *incidentStore) []Incident {
	t.Helper()
	page, err := s.query("", "all", 0, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	return page.Items
}

func TestIncidentLifecycle(t *testing.T) {
	s := historyFixture(t)
	target := fixtureTarget()
	target.ID = "1"
	target.Type = "tcp"
	target.State = "up"
	at := time.Now().UTC().Truncate(time.Second)
	applyHistory(t, s, target, "result", at)
	if len(historyItems(t, s)) != 0 {
		t.Fatal("initial success created outage")
	}
	target.State = "down"
	target.Message = "Connection refused"
	applyHistory(t, s, target, "result", at)
	applyHistory(t, s, target, "result", at.Add(5*time.Second))
	items := historyItems(t, s)
	if len(items) != 1 || items[0].Status != "open" || items[0].DurationSeconds != 5 {
		t.Fatalf("unexpected open incident: %+v", items)
	}
	target.State = "up"
	applyHistory(t, s, target, "result", at.Add(10*time.Second))
	applyHistory(t, s, target, "result", at.Add(15*time.Second))
	items = historyItems(t, s)
	if len(items) != 1 || items[0].Status != "resolved" || items[0].DurationSeconds != 10 || !strings.Contains(items[0].Reason, "Connection refused") {
		t.Fatalf("unexpected resolved incident: %+v", items)
	}
	target.State = "down"
	target.Name = "Renamed fixture"
	applyHistory(t, s, target, "result", at.Add(20*time.Second))
	items = historyItems(t, s)
	if len(items) != 2 || items[0].Name != "Renamed fixture" || items[1].Name != "Fixture" {
		t.Fatal("incident snapshots were not preserved")
	}
}

func TestInterruptedDurationAndGaps(t *testing.T) {
	for _, kind := range []string{"Monitoring paused", "Monitor removed", "Monitor settings changed", "Monitoring stopped when Hostlight closed", "result"} {
		t.Run(kind, func(t *testing.T) {
			s := historyFixture(t)
			target := fixtureTarget()
			target.ID = "1"
			target.State = "down"
			at := time.Now().UTC().Truncate(time.Second)
			applyHistory(t, s, target, "result", at)
			applyHistory(t, s, target, "result", at.Add(5*time.Second))
			if kind == "result" {
				target.State = "error"
			}
			applyHistory(t, s, target, kind, at.Add(10*time.Second))
			item := historyItems(t, s)[0]
			if item.Status != "interrupted" || item.DurationSeconds != 5 {
				t.Fatalf("interruption included unobserved time: %+v", item)
			}
		})
	}
	s := historyFixture(t)
	target := fixtureTarget()
	target.ID = "1"
	target.State = "down"
	at := time.Now().UTC().Truncate(time.Second)
	applyHistory(t, s, target, "result", at)
	applyHistory(t, s, target, "result", at.Add(5*time.Second))
	applyHistory(t, s, target, "result", at.Add(time.Hour))
	items := historyItems(t, s)
	if len(items) != 2 || items[0].Status != "open" || items[1].Status != "interrupted" || items[1].DurationSeconds != 5 {
		t.Fatalf("gap counted as continuous outage: %+v", items)
	}
}

func TestHistoryRestartRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	at := time.Now().UTC().Truncate(time.Second)
	s, err := openIncidentStore(path, at)
	if err != nil {
		t.Fatal(err)
	}
	target := fixtureTarget()
	target.ID = "1"
	target.State = "down"
	applyHistory(t, s, target, "result", at)
	applyHistory(t, s, target, "result", at.Add(5*time.Second))
	s.db.Close()
	reopened, err := openIncidentStore(path, at.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.db.Close()
	item := historyItems(t, reopened)[0]
	if item.Status != "interrupted" || item.DurationSeconds != 5 || item.Ended != at.Add(5*time.Second).Format(time.RFC3339Nano) {
		t.Fatalf("restart gap counted: %+v", item)
	}
	target.State = "up"
	applyHistory(t, reopened, target, "result", at.Add(24*time.Hour))
	if historyItems(t, reopened)[0].Status != "interrupted" {
		t.Fatal("fresh success fabricated recovery")
	}
}

func TestHistoryFiltersRetentionAndMigration(t *testing.T) {
	s := historyFixture(t)
	at := time.Now()
	target := fixtureTarget()
	target.ID = "1"
	target.Name = "Office SQL"
	target.Group = "Production"
	target.State = "down"
	applyHistory(t, s, target, "result", at.AddDate(0, 0, -100))
	target.State = "up"
	applyHistory(t, s, target, "result", at.AddDate(0, 0, -100).Add(time.Second))
	target.ID = "2"
	target.State = "down"
	applyHistory(t, s, target, "result", at.AddDate(0, 0, -100))
	if err := s.prune(at); err != nil {
		t.Fatal(err)
	}
	page, err := s.query("production", "open", 0, 0, 100)
	if err != nil || page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("filters/retention: %+v %v", page, err)
	}
	page, err = s.query("%", "all", 0, 0, 100)
	if err != nil || page.Total != 0 {
		t.Fatal("search treated SQL wildcard as a match")
	}
	if _, err = s.query("", "invalid", 0, 0, 100); err == nil {
		t.Fatal("invalid status accepted")
	}
	if _, err = s.query("", "all", -1, 0, 100); err == nil {
		t.Fatal("invalid date range accepted")
	}
	var version int
	if err = s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 4 {
		t.Fatal("schema version missing")
	}
	path := filepath.Join(t.TempDir(), "future.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("PRAGMA user_version=99"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if other, err := openIncidentStore(path, at); err == nil {
		other.db.Close()
		t.Fatal("opened unsupported schema")
	}
}

func TestIncidentCSV(t *testing.T) {
	var output bytes.Buffer
	items := []Incident{{Name: " =HYPERLINK(\"bad\")", Host: "server,example", Group: "Production\nEast", DurationSeconds: 12}}
	if err := writeIncidentCSV(&output, items); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&output).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[1][0] != "' =HYPERLINK(\"bad\")" || rows[1][1] != "server,example" || rows[1][2] != "Production\nEast" {
		t.Fatalf("bad CSV: %+v", rows)
	}
}

func TestRetentionCapPreservesActiveIncidents(t *testing.T) {
	s := historyFixture(t)
	now := time.Now()
	target := fixtureTarget()
	target.ID = "active"
	target.State = "down"
	applyHistory(t, s, target, "result", now.AddDate(0, 0, -100))
	_, err := s.db.Exec(`WITH RECURSIVE numbers(n) AS (VALUES(1) UNION ALL SELECT n+1 FROM numbers WHERE n<50002)
 INSERT INTO incidents(target_id,name,host,group_name,probe_type,port,opened,last_seen,ended,status,reason)
 SELECT 'closed-' || n,'Fixture','localhost','','tcp',443,?,?,?,'resolved','Recovered' FROM numbers`, now.UnixMilli(), now.UnixMilli(), now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.prune(now); err != nil {
		t.Fatal(err)
	}
	var completed, active int
	if err = s.db.QueryRow("SELECT count(*) FROM incidents WHERE status='resolved'").Scan(&completed); err != nil {
		t.Fatal(err)
	}
	if err = s.db.QueryRow("SELECT count(*) FROM incidents WHERE status='open'").Scan(&active); err != nil {
		t.Fatal(err)
	}
	if completed != 50000 || active != 1 {
		t.Fatalf("retention counts: completed=%d active=%d", completed, active)
	}
}

func TestHistoryQueueFailureVisible(t *testing.T) {
	s := historyFixture(t)
	a := NewApp()
	a.history = s
	for i := 0; i <= cap(s.events); i++ {
		a.recordHistoryLocked(fixtureTarget(), "result")
	}
	if s.errorMessage() == "" {
		t.Fatal("overflow silently lost history")
	}
	a.closing = true
	close(s.events)
	a.recordHistoryLocked(fixtureTarget(), "result") // Must not send on the closed queue.
}

func TestAppPauseClosesIncident(t *testing.T) {
	a := storedApp(t, filepath.Join(t.TempDir(), "targets.json"))
	target := fixtureTarget()
	if err := a.SaveTarget(target); err != nil {
		t.Fatal(err)
	}
	target = a.GetTargets()[0]
	target.State = "down"
	a.mu.Lock()
	a.recordHistoryLocked(target, "result")
	a.mu.Unlock()
	if err := a.PauseTarget(target.ID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		page, err := a.GetIncidents("", "all", 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) == 1 && page.Items[0].Status == "interrupted" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pause did not interrupt incident: %+v", page)
		}
		time.Sleep(10 * time.Millisecond)
	}
	a.shutdown(context.Background()) // Also verifies draining shutdown is idempotent via fixture cleanup.
}
