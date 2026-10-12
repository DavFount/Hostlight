package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func webhookFixture(t *testing.T, endpoint string) (*App, *incidentStore) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("Current-user DPAPI requires Windows")
	}
	s := historyFixture(t)
	a := &App{history: s}
	if err := a.SaveWebhookSettings(WebhookSettings{Enabled: true, URL: endpoint, AllowHTTP: true, Bearer: "token-secret", SigningKey: "signing-secret"}); err != nil {
		t.Fatal(err)
	}
	return a, s
}
func deliveryCount(t *testing.T, s *incidentStore) int {
	t.Helper()
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM deliveries").Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
func TestWebhookTransitionsAreDurableAndDeduplicated(t *testing.T) {
	_, s := webhookFixture(t, "http://127.0.0.1/webhook")
	target := fixtureTarget()
	target.ID = "1"
	target.State = "down"
	at := time.Now()
	// Initial discovery is silent, including its recovery.
	applyHistory(t, s, target, "result", at)
	target.State = "up"
	applyHistory(t, s, target, "result", at.Add(time.Second))
	if deliveryCount(t, s) != 0 {
		t.Fatal("initial discovery alerted")
	}
	target.State = "down"
	if err := s.apply(historyEvent{target: target, kind: "result", at: at.Add(2 * time.Second), previous: "up"}); err != nil {
		t.Fatal(err)
	}
	applyHistory(t, s, target, "result", at.Add(3*time.Second))
	target.State = "up"
	applyHistory(t, s, target, "result", at.Add(4*time.Second))
	applyHistory(t, s, target, "result", at.Add(5*time.Second))
	if deliveryCount(t, s) != 2 {
		t.Fatal("expected one outage and one recovery")
	}
	var payload []byte
	if err := s.db.QueryRow("SELECT payload FROM deliveries WHERE event='up'").Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var event WebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatal(err)
	}
	if event.ID == "" || event.SchemaVersion != 1 || event.DurationSeconds != 2 || event.Target.ID != "1" {
		t.Fatalf("bad event: %+v", event)
	}
}
func TestWebhookSecretsAndSettingsReplacement(t *testing.T) {
	a, s := webhookFixture(t, "http://127.0.0.1/endpoint-secret")
	settings, err := a.GetWebhookSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.URL != "" || settings.Bearer != "" || settings.SigningKey != "" || !settings.Configured || !settings.HasBearer {
		t.Fatal("secret exposed to UI")
	}
	var blob []byte
	if err = s.db.QueryRow("SELECT secret FROM webhook_destinations").Scan(&blob); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, []byte("endpoint-secret")) || bytes.Contains(blob, []byte("token-secret")) {
		t.Fatal("plaintext secret persisted")
	}
	if err = a.TestWebhook(); err != nil {
		t.Fatal(err)
	}
	if err = a.SaveWebhookSettings(WebhookSettings{Enabled: false, AllowHTTP: true, ClearBearer: true, ClearSigningKey: true}); err != nil {
		t.Fatal(err)
	}
	settings, err = a.GetWebhookSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.Enabled || settings.HasBearer || settings.HasSigningKey {
		t.Fatal("settings not updated")
	}
	entries, err := a.GetWebhookDeliveries()
	if err != nil || len(entries) != 1 || entries[0].Status != "cancelled" {
		t.Fatalf("unsent event not cancelled: %+v %v", entries, err)
	}
	if a.TestWebhook() == nil {
		t.Fatal("test allowed with disabled endpoint")
	}
}
func TestWebhookRetrySurvivesRestartWithSameID(t *testing.T) {
	var ids []string
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		ids = append(ids, r.Header.Get("X-Hostlight-Event-ID"))
		if attempts == 1 {
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(429)
		} else {
			w.WriteHeader(204)
		}
	}))
	defer server.Close()
	a, s := webhookFixture(t, server.URL)
	if err := a.TestWebhook(); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(time.Second)
	if sent, err := s.deliverOne(context.Background(), webhookClient(), now); err != nil || !sent {
		t.Fatalf("first delivery: %v %v", sent, err)
	}
	if sent, err := s.deliverOne(context.Background(), webhookClient(), now.Add(5*time.Second)); err != nil || sent {
		t.Fatal("ignored retry-after")
	}
	path := s.path
	s.db.Close()
	reopened, err := openIncidentStore(path, now)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.db.Close()
	if sent, err := reopened.deliverOne(context.Background(), webhookClient(), now.Add(31*time.Second)); err != nil || !sent {
		t.Fatalf("restart delivery: %v %v", sent, err)
	}
	if len(ids) != 2 || ids[0] == "" || ids[0] != ids[1] {
		t.Fatalf("event ID changed: %v", ids)
	}
	entries, err := (&App{history: reopened}).GetWebhookDeliveries()
	if err != nil || entries[0].Status != "delivered" || entries[0].Attempts != 2 {
		t.Fatalf("bad result: %+v %v", entries, err)
	}
}
func TestWebhookAuthenticationAndTransportPolicy(t *testing.T) {
	body := []byte(`{"event":"test"}`)
	var auth, signature string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		signature = r.Header.Get("X-Hostlight-Signature")
		w.WriteHeader(204)
	}))
	defer server.Close()
	secret := webhookSecret{URL: server.URL, AllowHTTP: true, Bearer: "token", SigningKey: "key"}
	result, retry, _ := sendWebhook(context.Background(), webhookClient(), secret, "id", body)
	mac := hmac.New(sha256.New, []byte("key"))
	mac.Write(body)
	if result != "HTTP 204" || retry || auth != "Bearer token" || signature != "sha256="+hex.EncodeToString(mac.Sum(nil)) {
		t.Fatal("authentication or signing failed")
	}
	for _, bad := range []webhookSecret{{URL: server.URL}, {URL: "https://user:pass@example.com"}, {URL: "file:///tmp/webhook"}, {URL: "https://example.com/#secret"}, {URL: "https://example.com", Bearer: "token\nheader"}} {
		if validateWebhook(bad) == nil {
			t.Fatalf("accepted unsafe settings: %+v", bad)
		}
	}
}
func TestWebhookDoesNotFollowRedirectsOrRetryPermanentFailures(t *testing.T) {
	called := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 302) }))
	defer server.Close()
	result, retry, _ := sendWebhook(context.Background(), webhookClient(), webhookSecret{URL: server.URL, AllowHTTP: true}, "id", []byte("{}"))
	if called || retry || result != "HTTP 302" {
		t.Fatal("redirect followed or retried")
	}
}
func TestWebhookMigrationFromVersionOne(t *testing.T) {
	s := historyFixture(t)
	path := s.path
	if _, err := s.db.Exec("DROP TABLE webhook_config; DROP TABLE webhook_destinations; DROP TABLE alert_silences; DROP TABLE deliveries; DROP TABLE probe_minutes; PRAGMA user_version=1"); err != nil {
		t.Fatal(err)
	}
	target := fixtureTarget()
	target.ID = "1"
	target.State = "down"
	applyHistory(t, s, target, "result", time.Now())
	if _, err := s.db.Exec("ALTER TABLE incidents DROP COLUMN excluded_ms; ALTER TABLE incidents DROP COLUMN paused"); err != nil {
		t.Fatal(err)
	}
	s.db.Close()
	migrated, err := openIncidentStore(path, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.db.Close()
	if len(historyItems(t, migrated)) != 1 {
		t.Fatal("migration lost incidents")
	}
	if filepath.Base(migrated.path) != "history.db" || deliveryCount(t, migrated) != 0 {
		t.Fatal("migration failed")
	}
}

func TestWebhookSettingsChangeSuppressesOldRecovery(t *testing.T) {
	a, s := webhookFixture(t, "http://127.0.0.1/old")
	target := fixtureTarget()
	target.ID = "1"
	target.State = "down"
	now := time.Now()
	if err := s.apply(historyEvent{target: target, kind: "result", at: now, previous: "up"}); err != nil {
		t.Fatal(err)
	}
	// Even a delivered outage must not create a recovery at the new endpoint.
	if _, err := s.db.Exec("UPDATE deliveries SET status='delivered'"); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveWebhookSettings(WebhookSettings{Enabled: true, URL: "http://127.0.0.1/new", AllowHTTP: true}); err != nil {
		t.Fatal(err)
	}
	target.State = "up"
	applyHistory(t, s, target, "result", now.Add(time.Second))
	if deliveryCount(t, s) != 1 {
		t.Fatal("recovery routed to a different destination")
	}
}

func TestWebhookTimeoutAndRetryLimit(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(100 * time.Millisecond):
		}
	}))
	defer slow.Close()
	client := webhookClient()
	client.Timeout = 10 * time.Millisecond
	result, retry, _ := sendWebhook(context.Background(), client, webhookSecret{URL: slow.URL, AllowHTTP: true}, "id", []byte("{}"))
	if !retry || result != "Network or TLS error (endpoint details hidden)" {
		t.Fatal("timeout not retriable or leaked endpoint")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer server.Close()
	a, s := webhookFixture(t, server.URL)
	if err := a.TestWebhook(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("UPDATE deliveries SET attempts=7"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.deliverOne(context.Background(), webhookClient(), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	entries, err := a.GetWebhookDeliveries()
	if err != nil || entries[0].Attempts != 8 || entries[0].Status != "failed" {
		t.Fatalf("unbounded retries: %+v %v", entries, err)
	}
}

func TestWebhookPermanentFailureAndExpiry(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(401)
		w.Write([]byte("secret-response-body"))
	}))
	defer server.Close()
	a, s := webhookFixture(t, server.URL)
	if err := a.TestWebhook(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.deliverOne(context.Background(), webhookClient(), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	entries, err := a.GetWebhookDeliveries()
	if err != nil || entries[0].Status != "failed" || entries[0].Result != "HTTP 401" {
		t.Fatalf("bad terminal result: %+v %v", entries, err)
	}
	if err = a.TestWebhook(); err != nil {
		t.Fatal(err)
	}
	if _, err = s.deliverOne(context.Background(), webhookClient(), time.Now().Add(25*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatal("expired delivery was sent")
	}
}

func TestWebhookOutboxAndIncidentCommitAtomically(t *testing.T) {
	_, s := webhookFixture(t, "http://127.0.0.1/webhook")
	if _, err := s.db.Exec("CREATE TRIGGER reject_delivery BEFORE INSERT ON deliveries BEGIN SELECT RAISE(ABORT,'simulated disk failure'); END"); err != nil {
		t.Fatal(err)
	}
	target := fixtureTarget()
	target.ID = "1"
	target.State = "down"
	event := historyEvent{target: target, kind: "result", at: time.Now(), previous: "up"}
	if s.apply(event) == nil {
		t.Fatal("delivery failure was hidden")
	}
	if len(historyItems(t, s)) != 0 || deliveryCount(t, s) != 0 {
		t.Fatal("partially committed incident/outbox")
	}
	if _, err := s.db.Exec("DROP TRIGGER reject_delivery"); err != nil {
		t.Fatal(err)
	}
	if err := s.apply(event); err != nil {
		t.Fatal(err)
	}
	if len(historyItems(t, s)) != 1 || deliveryCount(t, s) != 1 {
		t.Fatal("transaction did not recover")
	}
}

func TestWebhookInterruptionsDoNotAlertRecovery(t *testing.T) {
	_, s := webhookFixture(t, "http://127.0.0.1/webhook")
	target := fixtureTarget()
	target.ID = "1"
	target.State = "down"
	now := time.Now()
	if err := s.apply(historyEvent{target: target, kind: "result", at: now, previous: "up"}); err != nil {
		t.Fatal(err)
	}
	applyHistory(t, s, target, "Monitor removed", now.Add(time.Second))
	target.State = "up"
	applyHistory(t, s, target, "result", now.Add(2*time.Second))
	if deliveryCount(t, s) != 1 {
		t.Fatal("deletion produced recovery alert")
	}
}
