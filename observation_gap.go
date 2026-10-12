package main

import "time"

func observationGapLimit(t Target) time.Duration {
	gap := 2 * time.Duration(t.Interval+t.Timeout) * time.Second
	if gap < 30*time.Second {
		gap = 30 * time.Second
	}
	return gap
}

// Called with a.mu held. Use wall time: a monotonic clock may stop during sleep.
// A check spanning the gap is inconclusive and must not create a failure sample.
func (a *App) resetObservationGap(m *monitor, rules *statusRules, last, started, finished time.Time) bool {
	limit := observationGapLimit(m.target).Milliseconds()
	spanned := finished.UnixMilli()-started.UnixMilli() > limit
	if !spanned && finished.UnixMilli()-last.UnixMilli() <= limit {
		return false
	}
	a.recordHistoryLocked(m.target, "Monitoring gap: no observations received")
	rules.good, rules.bad = 0, 0
	m.target.State = "unknown"
	m.target.Checked = ""
	m.target.Latency = 0
	m.target.CertificateExpires = ""
	m.target.Message = "Monitoring gap detected. Waiting for fresh checks."
	return spanned
}
