package main

import (
	"fmt"
	"math"
	"time"
)

const recentCheckLimit = 100

// A bounded, session-only window. Local setup errors are not availability checks.
type checkWindow struct {
	failed        [recentCheckLimit]bool
	samples       [recentCheckLimit]ProbeSample
	breakBefore   bool
	count         int
	next          int
	fails         int
	started       time.Time
	sessionChecks uint64
	sessionFailed uint64
	setupErrors   uint64
}

func newCheckWindow() checkWindow { return checkWindow{started: time.Now().UTC()} }

func (w *checkWindow) start() {
	if w.started.IsZero() {
		w.started = time.Now().UTC()
	}
}

func (w *checkWindow) recordSetupError() {
	w.start()
	w.setupErrors++
	w.breakBefore = true
}

func (w *checkWindow) record(failed bool) {
	w.recordSample(failed, 0, time.Now())
}

func (w *checkWindow) recordSample(failed bool, latency float64, at time.Time) {
	w.start()
	w.sessionChecks++
	if failed {
		w.sessionFailed++
	}
	if w.count == recentCheckLimit {
		if w.failed[w.next] {
			w.fails--
		}
	} else {
		w.count++
	}
	w.failed[w.next] = failed
	if failed || math.IsNaN(latency) || math.IsInf(latency, 0) || latency < 0 {
		latency = 0
	}
	w.samples[w.next] = ProbeSample{Time: at.UTC().Format(time.RFC3339Nano), Failed: failed, LatencyMS: latency, BreakBefore: w.breakBefore}
	w.breakBefore = false
	if failed {
		w.fails++
	}
	w.next = (w.next + 1) % recentCheckLimit
}

type ProbeSample struct {
	Time        string  `json:"time"`
	Failed      bool    `json:"failed"`
	LatencyMS   float64 `json:"latencyMS"`
	BreakBefore bool    `json:"breakBefore"`
}

// GetRecentProbeSamples returns a detached oldest-first window for one expanded
// chart. Samples are session-only and bounded to the same 100 checks as statistics.
func (a *App) GetRecentProbeSamples(id string) ([]ProbeSample, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	m, ok := a.targets[id]
	if !ok {
		return nil, fmt.Errorf("Monitor no longer exists")
	}
	w := &m.checks
	out := make([]ProbeSample, 0, w.count)
	start := (w.next - w.count + recentCheckLimit) % recentCheckLimit
	for i := 0; i < w.count; i++ {
		out = append(out, w.samples[(start+i)%recentCheckLimit])
	}
	return out, nil
}

type CheckStatistics struct {
	ID            string `json:"id"`
	Checks        int    `json:"checks"`
	Failed        int    `json:"failed"`
	Started       string `json:"started"`
	SessionChecks uint64 `json:"sessionChecks"`
	SessionPassed uint64 `json:"sessionPassed"`
	SessionFailed uint64 `json:"sessionFailed"`
	SetupErrors   uint64 `json:"setupErrors"`
}

// GetCheckStatistics returns actual completed checks, independent of thresholds.
// Pauses add no observations; edits and application restarts create fresh windows.
func (a *App) GetCheckStatistics() []CheckStatistics {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]CheckStatistics, 0, len(a.targets))
	for id, m := range a.targets {
		started := ""
		if !m.checks.started.IsZero() {
			started = m.checks.started.Format(time.RFC3339Nano)
		}
		out = append(out, CheckStatistics{ID: id, Checks: m.checks.count, Failed: m.checks.fails,
			Started: started, SessionChecks: m.checks.sessionChecks,
			SessionPassed: m.checks.sessionChecks - m.checks.sessionFailed,
			SessionFailed: m.checks.sessionFailed, SetupErrors: m.checks.setupErrors})
	}
	return out
}
