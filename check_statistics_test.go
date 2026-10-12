package main

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"
	"time"
)

func TestCheckWindowKeepsOnlyRecentChecks(t *testing.T) {
	var window checkWindow
	window.record(true)
	for i := 0; i < 99; i++ {
		window.record(false)
	}
	if window.count != 100 || window.fails != 1 {
		t.Fatalf("unexpected full window: %+v", window)
	}
	window.record(false)
	if window.count != 100 || window.fails != 0 {
		t.Fatal("oldest failure was not evicted")
	}
	if window.sessionChecks != 101 || window.sessionFailed != 1 {
		t.Fatal("recent eviction erased session totals")
	}
	for i := 0; i < 200; i++ {
		window.record(true)
	}
	if window.count != 100 || window.fails != 100 {
		t.Fatal("window did not stay bounded across wraparounds")
	}
}

func TestCheckStatisticsSurvivePauseAndResetOnEdit(t *testing.T) {
	a := NewApp()
	a.ctx = context.Background()
	a.configPath = ""
	// Prevent a resumed probe from completing during the test.
	a.dial = func(ctx context.Context, _ string, _ time.Duration) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	target := Target{ID: "1", Name: "Fixture", Host: "127.0.0.1", Type: "tcp", Port: 1, Interval: 1, Timeout: 1, Failures: 3, Recovery: 2, State: "up"}
	m := &monitor{target: target, cancel: func() {}}
	m.checks.record(true)
	m.checks.record(false)
	m.checks.recordSetupError()
	m.checks.started = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	a.targets["1"] = m
	before := a.GetCheckStatistics()[0]
	beforeSamples, _ := a.GetRecentProbeSamples("1")
	defer a.shutdown(context.Background())
	for _, paused := range []bool{true, false, true} {
		if err := a.SetTargetsPaused([]string{"1"}, paused); err != nil {
			t.Fatal(err)
		}
		if got := a.GetCheckStatistics()[0]; got != before {
			t.Fatalf("pause/resume changed checks: %+v", got)
		}
		if got, err := a.GetRecentProbeSamples("1"); err != nil || !reflect.DeepEqual(got, beforeSamples) {
			t.Fatalf("pause/resume changed samples: %+v %v", got, err)
		}
	}
	target.Name = "Edited"
	if err := a.SaveTarget(target); err != nil {
		t.Fatal(err)
	}
	if got := a.GetCheckStatistics()[0]; got.Checks != 0 || got.Failed != 0 || got.SessionChecks != 0 || got.SetupErrors != 0 || got.Started == before.Started {
		t.Fatalf("edit retained old observations: %+v", got)
	}
	if got, err := a.GetRecentProbeSamples("1"); err != nil || len(got) != 0 {
		t.Fatalf("edit retained samples: %+v %v", got, err)
	}
}

func TestCheckStatisticsExcludeSetupErrorsAndCancelledResults(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want CheckStatistics
	}{
		{"success", nil, CheckStatistics{ID: "1", Checks: 1, SessionChecks: 1, SessionPassed: 1}},
		{"failure", errors.New("connection refused"), CheckStatistics{ID: "1", Checks: 1, Failed: 1, SessionChecks: 1, SessionFailed: 1}},
		{"dns", &net.DNSError{Err: "no such host"}, CheckStatistics{ID: "1", SetupErrors: 1}},
		{"setup", &probeSetupError{"adapter unavailable"}, CheckStatistics{ID: "1", SetupErrors: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewApp()
			s := historyFixture(t)
			a.history = s
			a.configPath = ""
			a.activeScheduling.SpreadMilliseconds = 0
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			a.dial = func(context.Context, string, time.Duration) (net.Conn, error) { return nil, tc.err }
			m := &monitor{target: Target{ID: "1", Type: "tcp", Interval: 3600, Failures: 3, Recovery: 2, State: "unknown"}, cancel: cancel}
			a.targets["1"] = m
			done := make(chan struct{})
			go func() { a.run(ctx, m, m.target); close(done) }()
			deadline := time.Now().Add(2 * time.Second)
			for a.GetTargets()[0].Checked == "" {
				if time.Now().After(deadline) {
					t.Fatal("check did not complete")
				}
				time.Sleep(time.Millisecond)
			}
			cancel()
			<-done
			tc.want.Started = a.GetCheckStatistics()[0].Started
			if tc.want.Started == "" {
				t.Fatal("session start not set")
			}
			if got := a.GetCheckStatistics()[0]; got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			// A cancelled probe must not add another observation.
			a.run(ctx, m, m.target)
			if got := a.GetCheckStatistics()[0]; got != tc.want {
				t.Fatalf("cancelled result counted: %+v", got)
			}
			if samples, err := a.GetRecentProbeSamples("1"); err != nil || len(samples) != tc.want.Checks {
				t.Fatalf("sample inclusion differs from completed checks: %+v %v", samples, err)
			}
			for len(s.events) > 0 {
				if err := s.apply(<-s.events); err != nil {
					t.Fatal(err)
				}
			}
			var saved int
			if err := s.db.QueryRow("SELECT COALESCE(SUM(checks),0) FROM probe_minutes").Scan(&saved); err != nil || saved != tc.want.Checks {
				t.Fatalf("persisted inclusion differs: %d %v", saved, err)
			}
		})
	}
}

func TestSessionStatisticsKeepOvernightTotals(t *testing.T) {
	window := newCheckWindow()
	start := window.started
	for i := 0; i < 20000; i++ {
		window.record(false)
	}
	for i := 0; i < 20; i++ {
		window.record(true)
	}
	for i := 0; i < 3; i++ {
		window.recordSetupError()
	}
	a := NewApp()
	a.targets["1"] = &monitor{checks: window}
	got := a.GetCheckStatistics()[0]
	if got.SessionChecks != 20020 || got.SessionPassed != 20000 || got.SessionFailed != 20 || got.SetupErrors != 3 {
		t.Fatalf("lost overnight totals: %+v", got)
	}
	if got.Checks != 100 || got.Failed != 20 || window.started != start {
		t.Fatal("session totals changed the recent window or start time")
	}
	// The API returns values rather than mutable internal state.
	got.SessionFailed = 0
	if a.GetCheckStatistics()[0].SessionFailed != 20 {
		t.Fatal("snapshot mutated internal counts")
	}
}

func TestSessionStatisticsResetOnConfigurationReload(t *testing.T) {
	a := NewApp()
	a.ctx = context.Background()
	a.configPath = t.TempDir() + "/targets.json"
	target := Target{ID: "1", Name: "Fixture", Host: "127.0.0.1", Type: "tcp", Port: 1, Interval: 1, Timeout: 1, Failures: 3, Recovery: 2, State: "paused"}
	m := &monitor{target: target, cancel: func() {}, checks: newCheckWindow()}
	m.checks.started = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	m.checks.record(true)
	m.checks.recordSetupError()
	a.targets["1"] = m
	if err := a.persistLocked(); err != nil {
		t.Fatal(err)
	}
	b := NewApp()
	b.ctx = context.Background()
	b.configPath = a.configPath
	b.loadConfiguration()
	defer b.shutdown(context.Background())
	got := b.GetCheckStatistics()[0]
	if got.SessionChecks != 0 || got.SetupErrors != 0 || got.Checks != 0 {
		t.Fatalf("restored stale totals: %+v", got)
	}
	if got.Started == "" || got.Started == a.GetCheckStatistics()[0].Started {
		t.Fatal("restart did not reset session start")
	}
}
