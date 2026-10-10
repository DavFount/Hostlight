package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type AlertSilence struct {
	ID       string `json:"id"`
	TargetID string `json:"targetId"`
	Kind     string `json:"kind"`
	Starts   string `json:"starts"`
	Ends     string `json:"ends"`
	Reason   string `json:"reason"`
}
type rowQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// Half-open intervals: start is inclusive, end is exclusive. Latest ending wins
// the display label; all overlapping intervals still suppress independently.
func activeSilence(ctx context.Context, q rowQuerier, targetID string, at time.Time) (string, error) {
	var reason, kind string
	err := q.QueryRowContext(ctx, "SELECT kind,reason FROM alert_silences WHERE target_id=? AND starts<=? AND ends>? ORDER BY ends DESC,id LIMIT 1", targetID, at.UnixMilli(), at.UnixMilli()).Scan(&kind, &reason)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	label := "Alerts snoozed"
	if kind == "maintenance" {
		label = "Scheduled maintenance"
	}
	if reason != "" {
		label += " · " + reason
	}
	return label, nil
}

func suppressPendingWith(ctx context.Context, tx *sql.Tx, now time.Time) error {
	// Include windows crossed while the app was closed or the receiver was retrying.
	_, err := tx.ExecContext(ctx, `UPDATE deliveries SET status='suppressed',result=COALESCE((SELECT CASE WHEN kind='maintenance' THEN 'Scheduled maintenance' ELSE 'Alerts snoozed' END || CASE WHEN reason='' THEN '' ELSE ' · ' || reason END FROM alert_silences WHERE target_id=deliveries.target_id AND starts<=? AND ends>deliveries.created ORDER BY ends DESC,id LIMIT 1),'Alerts silenced') WHERE status='pending' AND event!='test' AND EXISTS (SELECT 1 FROM alert_silences WHERE target_id=deliveries.target_id AND starts<=? AND ends>deliveries.created)`, now.UnixMilli(), now.UnixMilli())
	return err
}
func (s *incidentStore) suppressPending(ctx context.Context, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = suppressPendingWith(ctx, tx, now); err != nil {
		return err
	}
	return tx.Commit()
}
func (a *App) GetAlertSilences() ([]AlertSilence, error) {
	items := []AlertSilence{}
	if a.history == nil {
		return items, fmt.Errorf("Alert storage is unavailable")
	}
	rows, err := a.history.db.Query("SELECT id,target_id,kind,starts,ends,reason FROM alert_silences WHERE ends>? ORDER BY starts,id", time.Now().UnixMilli())
	if err != nil {
		return items, err
	}
	defer rows.Close()
	for rows.Next() {
		var s AlertSilence
		var start, end int64
		if err = rows.Scan(&s.ID, &s.TargetID, &s.Kind, &start, &end, &s.Reason); err != nil {
			return items, err
		}
		s.Starts = time.UnixMilli(start).UTC().Format(time.RFC3339Nano)
		s.Ends = time.UnixMilli(end).UTC().Format(time.RFC3339Nano)
		items = append(items, s)
	}
	return items, rows.Err()
}
func (a *App) SnoozeTargetAlerts(id string, minutes int, reason string) error {
	if minutes < 1 || minutes > 10080 {
		return fmt.Errorf("Snooze alerts for 1 minute to 7 days")
	}
	now := time.Now().UTC()
	return a.addAlertSilence(AlertSilence{TargetID: id, Kind: "snooze", Starts: now.Format(time.RFC3339Nano), Ends: now.Add(time.Duration(minutes) * time.Minute).Format(time.RFC3339Nano), Reason: reason}, now)
}
func (a *App) ScheduleTargetMaintenance(id, starts, ends, reason string) error {
	return a.addAlertSilence(AlertSilence{TargetID: id, Kind: "maintenance", Starts: starts, Ends: ends, Reason: reason}, time.Now())
}
func (a *App) addAlertSilence(input AlertSilence, now time.Time) error {
	if a.history == nil {
		return fmt.Errorf("Alert storage is unavailable")
	}
	a.mu.Lock()
	_, exists := a.targets[input.TargetID]
	a.mu.Unlock()
	if !exists {
		return fmt.Errorf("Monitor no longer exists")
	}
	input.Reason = strings.TrimSpace(input.Reason)
	if len(input.Reason) > 200 {
		return fmt.Errorf("Use a reason of at most 200 characters")
	}
	if input.Kind != "snooze" && input.Kind != "maintenance" {
		return fmt.Errorf("Choose a snooze or maintenance window")
	}
	start, err := time.Parse(time.RFC3339Nano, input.Starts)
	if err != nil {
		return fmt.Errorf("Choose a valid start date and time")
	}
	end, err := time.Parse(time.RFC3339Nano, input.Ends)
	if err != nil {
		return fmt.Errorf("Choose a valid end date and time")
	}
	if !end.After(start) || !end.After(now) || start.Before(now.Add(-5*time.Minute)) || start.After(now.AddDate(1, 0, 0)) || end.Sub(start) > 30*24*time.Hour {
		return fmt.Errorf("Choose an upcoming window of up to 30 days, starting within the next year")
	}
	tx, err := a.history.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var targetCount, total int
	if err = tx.QueryRow("SELECT count(*),COALESCE(sum(CASE WHEN target_id=? THEN 1 ELSE 0 END),0) FROM alert_silences WHERE ends>?", input.TargetID, now.UnixMilli()).Scan(&total, &targetCount); err != nil {
		return err
	}
	if targetCount >= 50 || total >= 10000 {
		return fmt.Errorf("Too many active or scheduled windows (50 per monitor, 10,000 total)")
	}
	if _, err = tx.Exec("INSERT INTO alert_silences(id,target_id,kind,starts,ends,reason) VALUES(?,?,?,?,?,?)", uuid.NewString(), input.TargetID, input.Kind, start.UnixMilli(), end.UnixMilli(), input.Reason); err != nil {
		return err
	}
	if err = suppressPendingWith(context.Background(), tx, now); err != nil {
		return err
	}
	return tx.Commit()
}
func (a *App) CancelAlertSilence(id string) error {
	if a.history == nil {
		return fmt.Errorf("Alert storage is unavailable")
	}
	tx, err := a.history.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now()
	if err = suppressPendingWith(context.Background(), tx, now); err != nil {
		return err
	}
	// Keep an ended interval for delayed observations and pending deliveries that
	// crossed it. Cancelling must not resurrect previously suppressed events.
	if _, err = tx.Exec("UPDATE alert_silences SET ends=? WHERE id=? AND starts<? AND ends>?", now.UnixMilli(), id, now.UnixMilli(), now.UnixMilli()); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM alert_silences WHERE id=? AND starts>=?", id, now.UnixMilli()); err != nil {
		return err
	}
	return tx.Commit()
}
