package main

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"
)

const probeMinuteLimit = 50000

type storedProbeSample struct {
	Session string
	Failed  bool
	Latency float64
}

// Upsert within the incident transaction: probes never perform disk I/O directly.
func storeProbeMinute(ctx context.Context, tx *sql.Tx, e historyEvent) error {
	s := e.sample
	failed := 0
	var latency any
	sum := 0.0
	if s.Failed {
		failed = 1
	} else if !math.IsNaN(s.Latency) && !math.IsInf(s.Latency, 0) && s.Latency >= 0 {
		latency = s.Latency
		sum = s.Latency
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO probe_minutes(target_id,session,name,probe_type,minute,checks,failed,latency_sum,latency_min,latency_max)
 VALUES(?,?,?,?,?,1,?,?,?,?) ON CONFLICT(target_id,session,minute) DO UPDATE SET
 checks=checks+1, failed=failed+excluded.failed, latency_sum=latency_sum+excluded.latency_sum,
 latency_min=CASE WHEN latency_min IS NULL THEN excluded.latency_min WHEN excluded.latency_min IS NULL THEN latency_min ELSE MIN(latency_min,excluded.latency_min) END,
 latency_max=CASE WHEN latency_max IS NULL THEN excluded.latency_max WHEN excluded.latency_max IS NULL THEN latency_max ELSE MAX(latency_max,excluded.latency_max) END`,
		e.target.ID, s.Session, e.target.Name, e.target.Type, e.at.Unix()/60, failed, sum, latency, latency)
	if err != nil {
		return err
	}
	// IDs increase only for newly inserted summaries. The indexed cutoff gives a
	// hard global bound without scanning/counting the entire table on every check.
	_, err = tx.ExecContext(ctx, "DELETE FROM probe_minutes WHERE id <= (SELECT MAX(id)-? FROM probe_minutes)", probeMinuteLimit)
	return err
}

type ProbeHistoryMonitor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type ProbeHistoryBucket struct {
	Time   string  `json:"time"`
	Checks int64   `json:"checks"`
	Failed int64   `json:"failed"`
	MeanMS float64 `json:"meanMS"`
	MinMS  float64 `json:"minMS"`
	MaxMS  float64 `json:"maxMS"`
	Types  string  `json:"types"`
}
type ProbeHistoryPage struct {
	Items         []ProbeHistoryBucket `json:"items"`
	BucketMinutes int                  `json:"bucketMinutes"`
	Error         string               `json:"error"`
	From          string               `json:"from"`
	Until         string               `json:"until"`
}

func (a *App) GetProbeHistoryMonitors() ([]ProbeHistoryMonitor, error) {
	if a.history == nil {
		return nil, fmt.Errorf("History storage is unavailable")
	}
	rows, err := a.history.db.Query(`SELECT target_id,name FROM probe_minutes WHERE id IN (SELECT MAX(id) FROM probe_minutes GROUP BY target_id) ORDER BY name,target_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProbeHistoryMonitor{}
	for rows.Next() {
		var m ProbeHistoryMonitor
		if err := rows.Scan(&m.ID, &m.Name); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (a *App) GetProbeHistory(id string, hours int) (ProbeHistoryPage, error) {
	if a.history == nil {
		return ProbeHistoryPage{}, fmt.Errorf("History storage is unavailable")
	}
	return a.history.probeHistory(id, hours, time.Now())
}

func (s *incidentStore) probeHistory(id string, hours int, now time.Time) (ProbeHistoryPage, error) {
	page := ProbeHistoryPage{Items: []ProbeHistoryBucket{}, Error: s.errorMessage()}
	if hours != 1 && hours != 24 && hours != 168 {
		return page, fmt.Errorf("Choose a valid probe history range")
	}
	step := hours * 60 / 120
	if step < 1 {
		step = 1
	}
	page.BucketMinutes = step
	page.From = time.Unix((now.Add(-time.Duration(hours)*time.Hour).Unix()/60/int64(step))*int64(step)*60, 0).UTC().Format(time.RFC3339)
	page.Until = time.Unix((now.Unix()/60/int64(step)+1)*int64(step)*60, 0).UTC().Format(time.RFC3339)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `SELECT (minute/?)*?, SUM(checks),SUM(failed),
 COALESCE(SUM(latency_sum)/NULLIF(SUM(checks-failed),0),0),COALESCE(MIN(latency_min),0),COALESCE(MAX(latency_max),0),GROUP_CONCAT(DISTINCT probe_type)
 FROM probe_minutes WHERE target_id=? AND minute>=? AND minute<=? GROUP BY minute/? ORDER BY minute/?`, step, step, id, now.Add(-time.Duration(hours)*time.Hour).Unix()/60, now.Unix()/60, step, step)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var b ProbeHistoryBucket
		var minute int64
		if err := rows.Scan(&minute, &b.Checks, &b.Failed, &b.MeanMS, &b.MinMS, &b.MaxMS, &b.Types); err != nil {
			return page, err
		}
		b.Time = time.Unix(minute*60, 0).UTC().Format(time.RFC3339)
		page.Items = append(page.Items, b)
	}
	return page, rows.Err()
}
