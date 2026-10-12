package main

import (
	"context"
	"errors"
	"net"
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
	a.targets["1"] = m
	defer a.shutdown(context.Background())
	for _, paused := range []bool{true, false, true} {
		if err := a.SetTargetsPaused([]string{"1"}, paused); err != nil {
			t.Fatal(err)
		}
		if got := a.GetCheckStatistics()[0]; got != (CheckStatistics{ID: "1", Checks: 2, Failed: 1}) {
			t.Fatalf("pause/resume changed checks: %+v", got)
		}
	}
	target.Name = "Edited"
	if err := a.SaveTarget(target); err != nil {
		t.Fatal(err)
	}
	if got := a.GetCheckStatistics()[0]; got.Checks != 0 || got.Failed != 0 {
		t.Fatalf("edit retained old observations: %+v", got)
	}
}

func TestCheckStatisticsExcludeSetupErrorsAndCancelledResults(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want CheckStatistics
	}{
		{"success", nil, CheckStatistics{ID: "1", Checks: 1}},
		{"failure", errors.New("connection refused"), CheckStatistics{ID: "1", Checks: 1, Failed: 1}},
		{"dns", &net.DNSError{Err: "no such host"}, CheckStatistics{ID: "1"}},
		{"setup", &probeSetupError{"adapter unavailable"}, CheckStatistics{ID: "1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewApp()
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
			if got := a.GetCheckStatistics()[0]; got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			// A cancelled probe must not add another observation.
			a.run(ctx, m, m.target)
			if got := a.GetCheckStatistics()[0]; got != tc.want {
				t.Fatalf("cancelled result counted: %+v", got)
			}
		})
	}
}
