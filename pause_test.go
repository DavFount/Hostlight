package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestVersionFourMigrationPreservesHistory(t *testing.T) {
	s := historyFixture(t)
	target := fixtureTarget()
	target.ID = "1"
	target.State = "down"
	at := time.Now()
	applyHistory(t, s, target, "result", at)
	id := historyItems(t, s)[0].ID
	if _, err := s.db.Exec("ALTER TABLE incidents DROP COLUMN excluded_ms; ALTER TABLE incidents DROP COLUMN paused; PRAGMA user_version=4"); err != nil {
		t.Fatal(err)
	}
	path := s.path
	s.db.Close()
	migrated, err := openIncidentStore(path, at.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.db.Close()
	items := historyItems(t, migrated)
	if len(items) != 1 || items[0].ID != id || items[0].Status != "interrupted" {
		t.Fatalf("migration lost history: %+v", items)
	}
	var version int
	if err = migrated.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 5 {
		t.Fatal("migration version", version, err)
	}
}

func TestPausedOutageRestartStillInterrupts(t *testing.T) {
	s := historyFixture(t)
	target := fixtureTarget()
	target.ID = "1"
	target.State = "down"
	at := time.Now()
	applyHistory(t, s, target, "result", at)
	applyHistory(t, s, target, "Monitoring paused", at.Add(time.Second))
	path := s.path
	s.db.Close()
	reopened, err := openIncidentStore(path, at.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.db.Close()
	item := historyItems(t, reopened)[0]
	if item.Status != "interrupted" || item.DurationSeconds != 0 || !strings.Contains(item.Reason, "restart") {
		t.Fatalf("restart invented observed time: %+v", item)
	}
}

func TestPausedOutageResumesWithoutDuplicateAndRecovers(t *testing.T) {
	_, s := webhookFixture(t, "http://127.0.0.1/webhook")
	target := fixtureTarget()
	target.ID = "1"
	target.State = "down"
	at := time.Now().UTC().Truncate(time.Second)
	if err := s.apply(historyEvent{target: target, kind: "result", at: at, previous: "up"}); err != nil {
		t.Fatal(err)
	}
	applyHistory(t, s, target, "result", at.Add(5*time.Second))
	id := historyItems(t, s)[0].ID
	applyHistory(t, s, target, "Monitoring paused", at.Add(6*time.Second))
	applyHistory(t, s, target, "Monitoring paused", at.Add(time.Minute))
	item := historyItems(t, s)[0]
	if item.Status != "open" || item.DurationSeconds != 5 {
		t.Fatalf("paused outage changed: %+v", item)
	}
	applyHistory(t, s, target, "Monitoring resumed", at.Add(time.Hour))
	applyHistory(t, s, target, "Monitoring resumed", at.Add(time.Hour+time.Second))
	target.State = "unknown"
	applyHistory(t, s, target, "result", at.Add(time.Hour+2*time.Second))
	target.State = "down"
	applyHistory(t, s, target, "result", at.Add(time.Hour+5*time.Second))
	if deliveryCount(t, s) != 1 || historyItems(t, s)[0].ID != id {
		t.Fatal("resume duplicated outage")
	}
	applyHistory(t, s, target, "Monitoring paused", at.Add(time.Hour+6*time.Second))
	applyHistory(t, s, target, "Monitoring resumed", at.Add(2*time.Hour))
	target.State = "up"
	applyHistory(t, s, target, "result", at.Add(2*time.Hour+5*time.Second))
	applyHistory(t, s, target, "result", at.Add(2*time.Hour+10*time.Second))
	item = historyItems(t, s)[0]
	if item.Status != "resolved" || item.DurationSeconds != 15 || item.ID != id || deliveryCount(t, s) != 2 {
		t.Fatalf("bad recovery: %+v", item)
	}
	var payload []byte
	if err := s.db.QueryRow("SELECT payload FROM deliveries WHERE event='up'").Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var event WebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatal(err)
	}
	if event.IncidentID != id || event.DurationSeconds != 15 {
		t.Fatalf("bad recovery event: %+v", event)
	}
}

func TestDeletePausedOutageClosesWithoutRecovery(t *testing.T) {
	_, s := webhookFixture(t, "http://127.0.0.1/webhook")
	target := fixtureTarget()
	target.ID = "1"
	target.State = "down"
	at := time.Now()
	if err := s.apply(historyEvent{target: target, kind: "result", at: at, previous: "up"}); err != nil {
		t.Fatal(err)
	}
	applyHistory(t, s, target, "Monitoring paused", at.Add(time.Second))
	applyHistory(t, s, target, "Monitor removed", at.Add(time.Hour))
	item := historyItems(t, s)[0]
	if item.Status != "interrupted" || item.DurationSeconds != 0 || deliveryCount(t, s) != 1 {
		t.Fatalf("delete implied recovery: %+v", item)
	}
}
