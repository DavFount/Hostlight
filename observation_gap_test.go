package main

import (
	"testing"
	"time"
)

func TestObservationGapFreshEvidence(t *testing.T) {
	at := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	for _, success := range []bool{false, true} {
		a := NewApp()
		m := &monitor{target: Target{State: "down", Interval: 5, Timeout: 3, Checked: at.Format(time.RFC3339), CertificateExpires: "old"}, checks: newCheckWindow()}
		m.checks.record(false)
		rules := statusRules{good: 1, bad: 2, failures: 3, recovery: 2}
		finished := at.Add(time.Hour)
		if a.resetObservationGap(m, &rules, at, finished.Add(-time.Second), finished) {
			t.Fatal("fresh post-gap check was discarded")
		}
		if m.target.State != "unknown" || rules.good != 0 || rules.bad != 0 || m.target.Checked != "" || m.target.CertificateExpires != "" {
			t.Fatalf("stale evidence retained: %+v %+v", m.target, rules)
		}
		if got := rules.apply(m.target.State, success); got != "unknown" {
			t.Fatalf("one fresh result established state: %s", got)
		}
		if m.checks.sessionChecks != 1 {
			t.Fatal("gap changed completed-check totals")
		}
	}
}

func TestObservationGapSpanningCheckAndBoundary(t *testing.T) {
	a := NewApp()
	at := time.Now()
	m := &monitor{target: Target{State: "up", Interval: 20, Timeout: 10}}
	rules := statusRules{good: 4, failures: 3, recovery: 2}
	if a.resetObservationGap(m, &rules, at, at, at.Add(time.Minute)) || m.target.State != "up" {
		t.Fatal("exact boundary reset status")
	}
	if !a.resetObservationGap(m, &rules, at, at, at.Add(time.Minute+time.Millisecond)) || m.target.State != "unknown" {
		t.Fatal("check spanning gap was accepted")
	}
}

func TestObservationGapInterruptsIncident(t *testing.T) {
	s := historyFixture(t)
	a := NewApp()
	a.history = s
	target := fixtureTarget()
	target.ID = "gap"
	target.State = "down"
	at := time.Now().UTC()
	if err := s.apply(historyEvent{target: target, kind: "result", previous: "up", at: at}); err != nil {
		t.Fatal(err)
	}
	applyHistory(t, s, target, "result", at.Add(5*time.Second))
	m := &monitor{target: target}
	rules := statusRules{bad: 2, failures: 3, recovery: 2}
	finished := at.Add(time.Hour)
	a.resetObservationGap(m, &rules, at, finished.Add(-time.Second), finished)
	select {
	case event := <-s.events:
		if err := s.apply(event); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("gap interruption not queued")
	}
	items := historyItems(t, s)
	if len(items) != 1 || items[0].Status != "interrupted" || items[0].DurationSeconds != 5 {
		t.Fatalf("gap counted as observed downtime: %+v", items)
	}
	// Fresh successes establish availability without resolving the old outage.
	m.target.State = rules.apply(m.target.State, true)
	applyHistory(t, s, m.target, "result", finished)
	m.target.State = rules.apply(m.target.State, true)
	applyHistory(t, s, m.target, "result", finished.Add(5*time.Second))
	if historyItems(t, s)[0].Status != "interrupted" {
		t.Fatal("gap fabricated recovery")
	}
	alerts := a.GetVisualAlerts(0, false).Items
	if len(alerts) != 1 || alerts[0].Event != "down" {
		t.Fatalf("gap fabricated popup recovery: %+v", alerts)
	}
}

func TestObservationGapDoesNotSendRecoveryWebhook(t *testing.T) {
	a, s := webhookFixture(t, "http://127.0.0.1/webhook")
	target := fixtureTarget()
	target.ID, target.State = "gap-webhook", "down"
	at := time.Now()
	if err := s.apply(historyEvent{target: target, kind: "result", previous: "up", at: at}); err != nil {
		t.Fatal(err)
	}
	m := &monitor{target: target}
	rules := statusRules{failures: 3, recovery: 2}
	finished := at.Add(time.Hour)
	a.resetObservationGap(m, &rules, at, finished.Add(-time.Second), finished)
	if err := s.apply(<-s.events); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		previous := m.target.State
		m.target.State = rules.apply(previous, true)
		if err := s.apply(historyEvent{target: m.target, kind: "result", previous: previous, at: finished.Add(time.Duration(i) * time.Second)}); err != nil {
			t.Fatal(err)
		}
	}
	if deliveryCount(t, s) != 1 {
		t.Fatal("gap fabricated recovery webhook")
	}
}
