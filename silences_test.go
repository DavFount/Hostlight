package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func silenceFixture(t *testing.T, endpoint string) (*App, *incidentStore, Target) {
	t.Helper()
	a, s := webhookFixture(t, endpoint)
	target := fixtureTarget()
	target.ID = "1"
	target.Group = "Office"
	target.State = "down"
	a.targets = map[string]*monitor{"1": {target: target}}
	return a, s, target
}
func addWindow(t *testing.T, a *App, start, end time.Time, reason string) {
	t.Helper()
	if err := a.addAlertSilence(AlertSilence{TargetID: "1", Kind: "maintenance", Starts: start.Format(time.RFC3339Nano), Ends: end.Format(time.RFC3339Nano), Reason: reason}, time.Now()); err != nil {
		t.Fatal(err)
	}
}
func TestMaintenanceSuppressesNotificationsButKeepsIncidentHistory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer server.Close()
	a, s, target := silenceFixture(t, server.URL)
	now := time.Now().UTC().Truncate(time.Millisecond)
	addWindow(t, a, now, now.Add(time.Hour), "Firmware upgrade")
	if err := s.apply(historyEvent{target: target, kind: "result", at: now, previous: "up"}); err != nil {
		t.Fatal(err)
	}
	entries, err := a.GetWebhookDeliveries()
	if err != nil || len(entries) != 1 || entries[0].Status != "suppressed" || entries[0].Attempts != 0 {
		t.Fatalf("alert not visibly suppressed: %+v %v", entries, err)
	}
	if entries[0].Result != "Scheduled maintenance · Firmware upgrade" {
		t.Fatal("missing maintenance reason")
	}
	if sent, err := s.deliverOne(context.Background(), webhookClient(), now.Add(time.Second)); err != nil || sent {
		t.Fatal("suppressed delivery sent")
	}
	target.State = "up"
	applyHistory(t, s, target, "result", now.Add(2*time.Second))
	if deliveryCount(t, s) != 1 || historyItems(t, s)[0].Status != "resolved" {
		t.Fatal("orphan recovery sent or incident history paused")
	}
	// There is no replay, but a new confirmed transition after the window sends normally.
	target.State = "down"
	if err = s.apply(historyEvent{target: target, kind: "result", at: now.Add(2 * time.Hour), previous: "up"}); err != nil {
		t.Fatal(err)
	}
	if sent, err := s.deliverOne(context.Background(), webhookClient(), now.Add(2*time.Hour)); err != nil || !sent {
		t.Fatalf("alerts failed to resume: %v %v", sent, err)
	}
}
func TestMaintenanceSuppressesQueuedRetriesAndCrossedWindowsAfterRestart(t *testing.T) {
	a, s, target := silenceFixture(t, "http://127.0.0.1/should-not-send")
	now := time.Now()
	if err := s.apply(historyEvent{target: target, kind: "result", at: now, previous: "up"}); err != nil {
		t.Fatal(err)
	}
	addWindow(t, a, now.Add(time.Minute), now.Add(time.Hour), "Planned work")
	if _, err := s.db.Exec("UPDATE deliveries SET attempts=2,next_attempt=?", now.Add(3*time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	path := s.path
	s.db.Close()
	reopened, err := openIncidentStore(path, now.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.db.Close()
	// The whole maintenance interval passed while closed. Even a not-yet-due retry must be suppressed.
	if sent, err := reopened.deliverOne(context.Background(), webhookClient(), now.Add(2*time.Hour)); err != nil || sent {
		t.Fatal("queued retry escaped maintenance")
	}
	entries, err := (&App{history: reopened}).GetWebhookDeliveries()
	if err != nil || entries[0].Status != "suppressed" || entries[0].Attempts != 2 {
		t.Fatalf("queued suppression not durable: %+v %v", entries, err)
	}
}
func TestSilenceBoundariesOverlapsAndCancellation(t *testing.T) {
	a, s, _ := silenceFixture(t, "http://127.0.0.1/webhook")
	now := time.Now().UTC().Truncate(time.Millisecond).Add(-time.Second)
	addWindow(t, a, now, now.Add(time.Hour), "First")
	ctx := context.Background()
	for _, c := range []struct {
		at   time.Time
		want bool
	}{{now.Add(-time.Millisecond), false}, {now, true}, {now.Add(time.Hour - time.Millisecond), true}, {now.Add(time.Hour), false}} {
		reason, err := activeSilence(ctx, s.db, "1", c.at)
		if err != nil || (reason != "") != c.want {
			t.Fatalf("bad boundary at %v: %q %v", c.at, reason, err)
		}
	}
	addWindow(t, a, now, now.Add(2*time.Hour), "Second")
	windows, err := a.GetAlertSilences()
	if err != nil || len(windows) != 2 {
		t.Fatal("overlapping windows not saved")
	}
	if err = a.CancelAlertSilence(windows[0].ID); err != nil {
		t.Fatal(err)
	}
	if reason, err := activeSilence(ctx, s.db, "1", time.Now()); err != nil || reason == "" {
		t.Fatal("ending one window cancelled another")
	}
	if err = a.CancelAlertSilence(windows[1].ID); err != nil {
		t.Fatal(err)
	}
	if reason, err := activeSilence(ctx, s.db, "1", time.Now()); err != nil || reason != "" {
		t.Fatal("alerts remained silenced")
	}
	if reason, err := activeSilence(ctx, s.db, "1", now); err != nil || reason == "" {
		t.Fatal("ending windows erased the historical interval")
	}
}
func TestSnoozeCancelsPendingAlertsButTestDeliveryBypassesIt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer server.Close()
	a, s, target := silenceFixture(t, server.URL)
	if err := s.apply(historyEvent{target: target, kind: "result", at: time.Now(), previous: "up"}); err != nil {
		t.Fatal(err)
	}
	if err := a.SnoozeTargetAlerts("1", 15, "Investigating"); err != nil {
		t.Fatal(err)
	}
	entries, err := a.GetWebhookDeliveries()
	if err != nil || entries[0].Status != "suppressed" {
		t.Fatal("existing pending alert not suppressed immediately")
	}
	if err = a.TestWebhook(); err != nil {
		t.Fatal(err)
	}
	if sent, err := s.deliverOne(context.Background(), webhookClient(), time.Now().Add(time.Second)); err != nil || !sent {
		t.Fatal("explicit test did not bypass silence")
	}
	windows, err := a.GetAlertSilences()
	if err != nil || len(windows) != 1 || windows[0].Kind != "snooze" {
		t.Fatal("snooze missing")
	}
	if err = a.CancelAlertSilence(windows[0].ID); err != nil {
		t.Fatal(err)
	}
	if sent, err := s.deliverOne(context.Background(), webhookClient(), time.Now().Add(time.Minute)); err != nil || sent {
		t.Fatal("ending snooze replayed old alerts")
	}
}
func TestRecoveryDuringMaintenanceIsSuppressedForPreviouslyAlertedOutage(t *testing.T) {
	a, s, target := silenceFixture(t, "http://127.0.0.1/webhook")
	now := time.Now()
	if err := s.apply(historyEvent{target: target, kind: "result", at: now, previous: "up"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("UPDATE deliveries SET status='delivered',attempts=1"); err != nil {
		t.Fatal(err)
	}
	addWindow(t, a, now, now.Add(time.Hour), "Recovery muted")
	target.State = "up"
	applyHistory(t, s, target, "result", now.Add(time.Second))
	entries, err := a.GetWebhookDeliveries()
	if err != nil || len(entries) != 2 || entries[0].Event != "up" || entries[0].Status != "suppressed" {
		t.Fatalf("recovery not suppressed: %+v %v", entries, err)
	}
}
func TestSilenceValidationAndUpcomingCancellation(t *testing.T) {
	a, _, _ := silenceFixture(t, "http://127.0.0.1/webhook")
	now := time.Now()
	for _, c := range []struct{ start, end string }{{"bad", "bad"}, {now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)}, {now.Add(-time.Hour).Format(time.RFC3339Nano), now.Add(time.Hour).Format(time.RFC3339Nano)}, {now.Format(time.RFC3339Nano), now.Add(31 * 24 * time.Hour).Format(time.RFC3339Nano)}} {
		if a.ScheduleTargetMaintenance("1", c.start, c.end, "") == nil {
			t.Fatal("invalid schedule accepted")
		}
	}
	if a.SnoozeTargetAlerts("missing", 15, "") == nil || a.SnoozeTargetAlerts("1", 0, "") == nil || a.SnoozeTargetAlerts("1", 10081, "") == nil {
		t.Fatal("invalid snooze accepted")
	}
	addWindow(t, a, now.Add(time.Hour), now.Add(2*time.Hour), "Upcoming")
	windows, err := a.GetAlertSilences()
	if err != nil || len(windows) != 1 {
		t.Fatal("upcoming window missing")
	}
	if err = a.CancelAlertSilence(windows[0].ID); err != nil {
		t.Fatal(err)
	}
	windows, err = a.GetAlertSilences()
	if err != nil || len(windows) != 0 {
		t.Fatal("upcoming window not cancelled")
	}
}

func TestSnoozePersistsAndVersionThreeMigrationPreservesQueuedTargets(t *testing.T) {
	a, s, target := silenceFixture(t, "http://127.0.0.1/webhook")
	if err := s.apply(historyEvent{target: target, kind: "result", at: time.Now(), previous: "up"}); err != nil {
		t.Fatal(err)
	}
	if err := a.SnoozeTargetAlerts("1", 60, "Persistent snooze"); err != nil {
		t.Fatal(err)
	}
	path := s.path
	s.db.Close()
	reopened, err := openIncidentStore(path, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	windows, err := (&App{history: reopened}).GetAlertSilences()
	if err != nil || len(windows) != 1 || windows[0].Reason != "Persistent snooze" {
		t.Fatalf("snooze did not survive restart: %+v %v", windows, err)
	}
	// Reconstruct v3's pre-silencing database shape and verify its upgrade.
	if _, err = reopened.db.Exec("DROP TABLE alert_silences; ALTER TABLE deliveries DROP COLUMN target_id; UPDATE deliveries SET status='pending',result='Waiting to send'; ALTER TABLE incidents DROP COLUMN excluded_ms; ALTER TABLE incidents DROP COLUMN paused; PRAGMA user_version=3"); err != nil {
		t.Fatal(err)
	}
	reopened.db.Close()
	migrated, err := openIncidentStore(path, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.db.Close()
	var id string
	if err = migrated.db.QueryRow("SELECT target_id FROM deliveries LIMIT 1").Scan(&id); err != nil || id != "1" {
		t.Fatalf("migration missing delivery target: %q %v", id, err)
	}
}
