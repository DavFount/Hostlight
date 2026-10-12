package main

import "time"

const recentCheckLimit = 100

// A bounded, session-only window. Local setup errors are not availability checks.
type checkWindow struct {
	failed        [recentCheckLimit]bool
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
}

func (w *checkWindow) record(failed bool) {
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
	if failed {
		w.fails++
	}
	w.next = (w.next + 1) % recentCheckLimit
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
