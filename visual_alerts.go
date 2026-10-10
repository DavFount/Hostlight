package main

import (
	"context"
	"database/sql"
	"time"
)

// A bounded, session-only feed preserves transitions between UI refreshes.
// It contains no response bodies or credentials and never replays on restart.
type VisualAlert struct {
	Sequence        uint64 `json:"sequence"`
	Name            string `json:"name"`
	Event           string `json:"event"`
	Time            string `json:"time"`
	Host            string `json:"host"`
	Reason          string `json:"reason"`
	DurationSeconds int64  `json:"durationSeconds"`
}
type VisualAlertPage struct {
	Cursor  uint64        `json:"cursor"`
	Items   []VisualAlert `json:"items"`
	Dropped bool          `json:"dropped"`
}
type visualFeed struct {
	sequence uint64
	items    []VisualAlert
	outages  map[string]int64
}

// The history writer calls this only for committed incident transitions. Silence
// decisions are made in the same transaction as the incident and webhook outbox.
func (s *incidentStore) commitVisual(ctx context.Context, tx *sql.Tx, e historyEvent, id int64, event string) error {
	if id == 0 {
		return tx.Commit()
	}
	silence, err := activeSilence(ctx, tx, e.target.ID, e.at)
	if err != nil {
		return err
	}
	var duration int64
	if event == "up" {
		if err = tx.QueryRowContext(ctx, "SELECT MAX(0,(ended-opened-excluded_ms)/1000) FROM incidents WHERE id=?", id).Scan(&duration); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f := &s.visual
	if f.outages == nil {
		f.outages = make(map[string]int64)
	}
	eligible := id != 0
	if event == "up" {
		eligible = eligible && f.outages[e.target.ID] == id
		delete(f.outages, e.target.ID)
	}
	if !eligible || silence != "" {
		return nil
	}
	if event == "down" {
		f.outages[e.target.ID] = id
	}
	f.sequence++
	f.items = append(f.items, VisualAlert{Sequence: f.sequence, Name: e.target.Name, Host: e.target.Host, Reason: e.target.Message, DurationSeconds: duration, Event: event, Time: e.at.UTC().Format(time.RFC3339Nano)})
	if len(f.items) > 100 {
		f.items = append([]VisualAlert(nil), f.items[len(f.items)-100:]...)
	}
	return nil
}

// baseline avoids stale popups when the UI opens, even on a quiet session.
func (a *App) GetVisualAlerts(cursor uint64, baseline bool) VisualAlertPage {
	page := VisualAlertPage{Items: []VisualAlert{}}
	if a.history == nil {
		return page
	}
	s := a.history
	s.mu.Lock()
	defer s.mu.Unlock()
	page.Cursor = s.visual.sequence
	if baseline || cursor > page.Cursor {
		return page
	}
	if len(s.visual.items) > 0 {
		page.Dropped = cursor+1 < s.visual.items[0].Sequence
	}
	for _, item := range s.visual.items {
		if item.Sequence > cursor {
			page.Items = append(page.Items, item)
		}
	}
	return page
}
