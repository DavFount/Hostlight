package main

import (
	"context"
	"fmt"
	"hash/fnv"
	"time"
)

type SchedulingSettings struct {
	Concurrency        int `json:"concurrency"`
	SpreadMilliseconds int `json:"spreadMilliseconds"`
}
type SchedulingStatus struct {
	Saved           SchedulingSettings `json:"saved"`
	Active          SchedulingSettings `json:"active"`
	RestartRequired bool               `json:"restartRequired"`
}

func validScheduling(s SchedulingSettings) bool {
	return s.Concurrency >= 1 && s.Concurrency <= 128 && s.SpreadMilliseconds >= 0 && s.SpreadMilliseconds <= 5000
}
func (a *App) GetSchedulingSettings() SchedulingStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return SchedulingStatus{a.scheduling, a.activeScheduling, a.scheduling != a.activeScheduling}
}
func (a *App) SaveSchedulingSettings(s SchedulingSettings) error {
	if !validScheduling(s) {
		return fmt.Errorf("Choose 1–128 simultaneous checks and a start spread of 0–5000 milliseconds")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closing {
		return fmt.Errorf("Hostlight is shutting down")
	}
	previous := a.scheduling
	a.scheduling = s
	if err := a.persistLocked(); err != nil {
		a.scheduling = previous
		return err
	}
	return nil
}

// Stable monitor-specific offsets avoid restarting every monitor in one burst.
// This is load spreading, not a promise of evenly spaced execution.
func probeStartDelay(id string, spread int) time.Duration {
	if spread <= 0 {
		return 0
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(id))
	return time.Duration(h.Sum64()%uint64(spread+1)) * time.Millisecond
}
func waitForProbe(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return ctx.Err() == nil
	}
}
