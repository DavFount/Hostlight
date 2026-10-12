package main

import (
	"testing"
	"time"
)

func TestRecentSamplesBoundedOrderedAndDetached(t *testing.T) {
	a := NewApp()
	m := &monitor{}
	a.targets["1"] = m
	at := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 205; i++ {
		m.checks.recordSample(i%2 == 0, float64(i), at.Add(time.Duration(i)*time.Second))
	}
	items, err := a.GetRecentProbeSamples("1")
	if err != nil || len(items) != 100 {
		t.Fatalf("window: %d %v", len(items), err)
	}
	for i, s := range items {
		index := i + 105
		if s.Time != at.Add(time.Duration(index)*time.Second).Format(time.RFC3339Nano) || s.Failed != (index%2 == 0) {
			t.Fatalf("order: %+v", s)
		}
		if s.Failed && s.LatencyMS != 0 {
			t.Fatal("failure retained a successful latency")
		}
	}
	items[0].Time = "mutated"
	again, _ := a.GetRecentProbeSamples("1")
	if again[0].Time == "mutated" {
		t.Fatal("API exposed mutable ring")
	}
	if m.checks.sessionChecks != 205 {
		t.Fatal("window eviction reset totals")
	}
	if _, err := a.GetRecentProbeSamples("missing"); err == nil {
		t.Fatal("missing monitor not reported")
	}
}

func TestRecentSamplesSetupAndGapBreaks(t *testing.T) {
	a := NewApp()
	m := &monitor{target: Target{Interval: 5, Timeout: 3}}
	a.targets["1"] = m
	at := time.Now()
	m.checks.recordSample(false, 12, at)
	m.checks.recordSetupError()
	m.checks.recordSample(false, 15, at.Add(time.Second))
	rules := statusRules{}
	a.resetObservationGap(m, &rules, at, at.Add(time.Hour), at.Add(time.Hour))
	m.checks.recordSample(false, 18, at.Add(time.Hour))
	items, _ := a.GetRecentProbeSamples("1")
	if len(items) != 3 || items[0].BreakBefore || !items[1].BreakBefore || !items[2].BreakBefore {
		t.Fatalf("lost discontinuities: %+v", items)
	}
	if m.checks.sessionChecks != 3 || m.checks.setupErrors != 1 {
		t.Fatal("setup error added availability sample")
	}
}
