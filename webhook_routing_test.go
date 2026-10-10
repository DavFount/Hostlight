package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func addDestination(t *testing.T, a *App, name, scope string, groups, ids []string, enabled bool, endpoint string) string {
	t.Helper()
	if err := a.SaveWebhookDestination(WebhookSettings{Name: name, Scope: scope, Groups: groups, MonitorIDs: ids, Enabled: enabled, URL: endpoint, AllowHTTP: true}); err != nil {
		t.Fatal(err)
	}
	all, err := a.GetWebhookDestinations()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range all {
		if d.Name == name {
			return d.ID
		}
	}
	t.Fatal("destination not saved")
	return ""
}

func TestWebhookScopeMatching(t *testing.T) {
	target := Target{ID: "1", Group: "Office"}
	cases := []struct {
		name, scope string
		groups, ids []string
		want        bool
	}{
		{"global", "global", nil, nil, true},
		{"group", "selected", []string{"Office"}, nil, true},
		{"monitor", "selected", nil, []string{"1"}, true},
		{"overlap", "selected", []string{"Office", "Office"}, []string{"1", "1"}, true},
		{"other group", "selected", []string{"Servers"}, nil, false},
		{"other monitor", "selected", nil, []string{"2"}, false},
		{"empty", "selected", nil, nil, false},
		{"invalid scope", "invalid", nil, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := webhookMatches(c.scope, c.groups, c.ids, target); got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
	if !webhookMatches("selected", []string{""}, nil, Target{Group: ""}) {
		t.Fatal("ungrouped not matched")
	}
}

func TestWebhookFanoutAndOverlapDeduplication(t *testing.T) {
	endpoint := "http://127.0.0.1/webhook"
	a, s := webhookFixture(t, endpoint)
	addDestination(t, a, "Office team", "selected", []string{"Office"}, nil, true, endpoint)
	addDestination(t, a, "Gateway owner", "selected", nil, []string{"1"}, true, endpoint)
	addDestination(t, a, "Both selections", "selected", []string{"Office", "Office"}, []string{"1", "1"}, true, endpoint)
	addDestination(t, a, "Other group", "selected", []string{"Servers"}, nil, true, endpoint)
	addDestination(t, a, "Disabled", "global", nil, nil, false, endpoint)
	target := fixtureTarget()
	target.ID = "1"
	target.Group = "Office"
	target.State = "down"
	now := time.Now()
	if err := s.apply(historyEvent{target: target, kind: "result", at: now, previous: "up"}); err != nil {
		t.Fatal(err)
	}
	applyHistory(t, s, target, "result", now.Add(time.Second))
	if deliveryCount(t, s) != 4 {
		t.Fatal("expected four matching destinations, once each")
	}
	target.State = "up"
	applyHistory(t, s, target, "result", now.Add(2*time.Second))
	if deliveryCount(t, s) != 8 {
		t.Fatal("recovery did not fan out to original destinations")
	}
	target.ID = "2"
	target.State = "down"
	if err := s.apply(historyEvent{target: target, kind: "result", at: now.Add(3 * time.Second), previous: "up"}); err != nil {
		t.Fatal(err)
	}
	if deliveryCount(t, s) != 11 {
		t.Fatal("new group member did not match group rules dynamically")
	}
	entries, err := a.GetWebhookDeliveries()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range entries {
		if d.DestinationID == "" || d.DestinationName == "" {
			t.Fatal("delivery missing destination snapshot")
		}
	}
}

func TestWebhookDestinationChangesAreIsolated(t *testing.T) {
	a, s := webhookFixture(t, "http://127.0.0.1/global")
	id := addDestination(t, a, "Team", "selected", []string{"Office"}, nil, true, "http://127.0.0.1/team")
	target := fixtureTarget()
	target.ID = "1"
	target.Group = "Office"
	target.State = "down"
	now := time.Now()
	if err := s.apply(historyEvent{target: target, kind: "result", at: now, previous: "up"}); err != nil {
		t.Fatal(err)
	}
	d, err := a.getWebhookDestination(id)
	if err != nil {
		t.Fatal(err)
	}
	d.Name = "Team renamed"
	if err = a.SaveWebhookDestination(d); err != nil {
		t.Fatal(err)
	}
	target.State = "up"
	applyHistory(t, s, target, "result", now.Add(time.Second))
	var pending, cancelled int
	if err = s.db.QueryRow("SELECT count(*) FROM deliveries WHERE status='pending' AND destination_id='legacy'").Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if err = s.db.QueryRow("SELECT count(*) FROM deliveries WHERE status='cancelled' AND destination_id=?", id).Scan(&cancelled); err != nil {
		t.Fatal(err)
	}
	if pending != 2 || cancelled != 1 || deliveryCount(t, s) != 3 {
		t.Fatal("editing one destination affected another or sent mismatched recovery")
	}
	// A test targets exactly the selected saved destination, independent of its routing filter.
	if err = a.TestWebhookDestination(id); err != nil {
		t.Fatal(err)
	}
	if err = a.DeleteWebhookDestination(id); err != nil {
		t.Fatal(err)
	}
	var teamPending int
	if err = s.db.QueryRow("SELECT count(*) FROM deliveries WHERE status='pending' AND destination_id=?", id).Scan(&teamPending); err != nil {
		t.Fatal(err)
	}
	if teamPending != 0 {
		t.Fatal("deleted destination left pending deliveries")
	}
	if a.TestWebhookDestination(id) == nil || a.SaveWebhookDestination(d) == nil {
		t.Fatal("deleted destination could be tested or silently recreated")
	}
}

func TestWebhookRetryDoesNotBlockAnotherDestination(t *testing.T) {
	retrying := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Retry-After", "60"); w.WriteHeader(503) }))
	defer retrying.Close()
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer healthy.Close()
	a, s := webhookFixture(t, retrying.URL)
	id := addDestination(t, a, "Healthy", "global", nil, nil, true, healthy.URL)
	if err := a.TestWebhook(); err != nil {
		t.Fatal(err)
	}
	if err := a.TestWebhookDestination(id); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(time.Second)
	for i := 0; i < 2; i++ {
		if sent, err := s.deliverOne(context.Background(), webhookClient(), now); err != nil || !sent {
			t.Fatalf("destination blocked: %v %v", sent, err)
		}
	}
	entries, err := a.GetWebhookDeliveries()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range entries {
		if d.DestinationID == id && d.Status != "delivered" {
			t.Fatal("healthy destination blocked by retrying destination")
		}
	}
}

func TestWebhookMigrationPreservesVersionTwoConfigurationAndQueue(t *testing.T) {
	a, s := webhookFixture(t, "http://127.0.0.1/migrated")
	if err := a.TestWebhook(); err != nil {
		t.Fatal(err)
	}
	var secret []byte
	var eventID string
	if err := s.db.QueryRow("SELECT secret FROM webhook_destinations WHERE id='legacy'").Scan(&secret); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow("SELECT id FROM deliveries").Scan(&eventID); err != nil {
		t.Fatal(err)
	}
	// Reconstruct the exact v2 shape that an existing installation has on disk.
	if _, err := s.db.Exec(`DROP INDEX delivery_destination; ALTER TABLE deliveries DROP COLUMN destination_id; ALTER TABLE deliveries DROP COLUMN destination_name; DROP TABLE webhook_destinations; PRAGMA user_version=2;`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("INSERT INTO webhook_config VALUES(1,1,?)", secret); err != nil {
		t.Fatal(err)
	}
	path := s.path
	s.db.Close()
	migrated, err := openIncidentStore(path, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.db.Close()
	app := &App{history: migrated}
	destinations, err := app.GetWebhookDestinations()
	if err != nil {
		t.Fatal(err)
	}
	if len(destinations) != 1 || destinations[0].Scope != "global" || !destinations[0].Enabled || !destinations[0].HasBearer || destinations[0].Name != "Default webhook" {
		t.Fatalf("migration changed destination: %+v", destinations)
	}
	entries, err := app.GetWebhookDeliveries()
	if err != nil || len(entries) != 1 || entries[0].ID != eventID || entries[0].DestinationID != "legacy" || entries[0].Status != "pending" {
		t.Fatalf("migration changed queued delivery: %+v %v", entries, err)
	}
}

func TestWebhookRoutingValidation(t *testing.T) {
	for _, d := range []WebhookSettings{{Scope: "global"}, {Name: "Test", Scope: "invalid"}, {Name: "Test", Scope: "selected"}, {Name: "Test", Scope: "selected", MonitorIDs: []string{" "}}} {
		if validateRouting(&d) == nil {
			t.Fatalf("accepted invalid rule: %+v", d)
		}
	}
	d := WebhookSettings{Name: " Team ", Scope: "selected", Groups: []string{"Office", "Office"}, MonitorIDs: []string{"1", "1"}}
	if err := validateRouting(&d); err != nil || d.Name != "Team" || len(d.Groups) != 1 || len(d.MonitorIDs) != 1 {
		t.Fatalf("normalization failed: %+v %v", d, err)
	}
}

func TestMonitorIDsAreNotReusedAfterDeletionAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "targets.json")
	a := storedApp(t, path)
	if err := a.SaveTarget(fixtureTarget()); err != nil {
		t.Fatal(err)
	}
	id := a.GetTargets()[0].ID
	if err := a.DeleteTarget(id); err != nil {
		t.Fatal(err)
	}
	a.shutdown(context.Background())
	b := storedApp(t, path)
	if err := b.SaveTarget(fixtureTarget()); err != nil {
		t.Fatal(err)
	}
	if b.GetTargets()[0].ID == id {
		t.Fatal("a replacement monitor inherited a deleted monitor's identity and webhook scope")
	}
}

func TestWebhookFanoutCommitsAllOrNothing(t *testing.T) {
	a, s := webhookFixture(t, "http://127.0.0.1/global")
	addDestination(t, a, "Second", "global", nil, nil, true, "http://127.0.0.1/second")
	if _, err := s.db.Exec("CREATE TRIGGER reject_second_delivery BEFORE INSERT ON deliveries WHEN (SELECT count(*) FROM deliveries)=1 BEGIN SELECT RAISE(ABORT,'simulated failure'); END"); err != nil {
		t.Fatal(err)
	}
	target := fixtureTarget()
	target.ID = "1"
	target.State = "down"
	if err := s.apply(historyEvent{target: target, kind: "result", at: time.Now(), previous: "up"}); err == nil {
		t.Fatal("fanout failure hidden")
	}
	if deliveryCount(t, s) != 0 || len(historyItems(t, s)) != 0 {
		t.Fatal("partial multi-destination fanout committed")
	}
}
