package main

const recentCheckLimit = 100

// A bounded, session-only window. Local setup errors are not availability checks.
type checkWindow struct {
	failed [recentCheckLimit]bool
	count  int
	next   int
	fails  int
}

func (w *checkWindow) record(failed bool) {
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
	ID     string `json:"id"`
	Checks int    `json:"checks"`
	Failed int    `json:"failed"`
}

// GetCheckStatistics returns actual completed checks, independent of thresholds.
// Pauses add no observations; edits and application restarts create fresh windows.
func (a *App) GetCheckStatistics() []CheckStatistics {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]CheckStatistics, 0, len(a.targets))
	for id, m := range a.targets {
		out = append(out, CheckStatistics{ID: id, Checks: m.checks.count, Failed: m.checks.fails})
	}
	return out
}
