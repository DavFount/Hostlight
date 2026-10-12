package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	_ "modernc.org/sqlite"
)

type Incident struct {
	ID              int64  `json:"id"`
	TargetID        string `json:"targetId"`
	Name            string `json:"name"`
	Host            string `json:"host"`
	Group           string `json:"group"`
	Type            string `json:"type"`
	Port            int    `json:"port"`
	Opened          string `json:"opened"`
	LastSeen        string `json:"lastSeen"`
	Ended           string `json:"ended"`
	Status          string `json:"status"`
	Reason          string `json:"reason"`
	DurationSeconds int64  `json:"durationSeconds"`
}
type HistoryPage struct {
	Items         []Incident `json:"items"`
	Total         int        `json:"total"`
	Error         string     `json:"error"`
	Path          string     `json:"path"`
	RetentionDays int        `json:"retentionDays"`
}
type historyEvent struct {
	target   Target
	kind     string
	at       time.Time
	previous string
	sample   *storedProbeSample
}
type incidentStore struct {
	visual  visualFeed
	db      *sql.DB
	path    string
	mu      sync.Mutex
	failure string
	events  chan historyEvent
	done    chan struct{}
}

func openIncidentStore(path string, now time.Time) (*incidentStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	fail := func(err error) (*incidentStore, error) { db.Close(); return nil, err }
	if _, err = db.Exec("PRAGMA busy_timeout=1500; PRAGMA journal_mode=WAL;"); err != nil {
		return fail(err)
	}
	var version int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fail(err)
	}
	if version > 6 {
		return fail(fmt.Errorf("History database version is newer than this application supports"))
	}
	if version == 0 {
		tx, err := db.Begin()
		if err != nil {
			return fail(err)
		}
		_, err = tx.Exec(`CREATE TABLE incidents (
 id INTEGER PRIMARY KEY AUTOINCREMENT, target_id TEXT NOT NULL, name TEXT NOT NULL,
 host TEXT NOT NULL, group_name TEXT NOT NULL, probe_type TEXT NOT NULL, port INTEGER NOT NULL,
 opened INTEGER NOT NULL, last_seen INTEGER NOT NULL, ended INTEGER,
 status TEXT NOT NULL CHECK(status IN ('open','resolved','interrupted')), reason TEXT NOT NULL);
 CREATE UNIQUE INDEX one_open_incident ON incidents(target_id) WHERE status='open';
 CREATE INDEX incident_time ON incidents(opened DESC);
 CREATE TABLE settings(key TEXT PRIMARY KEY, value INTEGER NOT NULL);
 INSERT INTO settings VALUES('retention_days',90);
 PRAGMA user_version=1;`)
		if err != nil {
			tx.Rollback()
			return fail(err)
		}
		if err = tx.Commit(); err != nil {
			return fail(err)
		}
	}
	if version < 2 {
		if _, err = db.Exec(`BEGIN;
 CREATE TABLE webhook_config(id INTEGER PRIMARY KEY CHECK(id=1), enabled INTEGER NOT NULL, secret BLOB NOT NULL);
 CREATE TABLE deliveries(id TEXT PRIMARY KEY, incident_id INTEGER NOT NULL, event TEXT NOT NULL, payload BLOB NOT NULL, secret BLOB NOT NULL,
 created INTEGER NOT NULL, next_attempt INTEGER NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL DEFAULT 'pending', result TEXT NOT NULL DEFAULT 'Waiting to send');
 CREATE INDEX delivery_due ON deliveries(status,next_attempt);
 PRAGMA user_version=2; COMMIT;`); err != nil {
			return fail(err)
		}
	}
	if version < 3 {
		if _, err = db.Exec(`BEGIN;
 CREATE TABLE webhook_destinations(id TEXT PRIMARY KEY,name TEXT NOT NULL,enabled INTEGER NOT NULL,scope TEXT NOT NULL,groups_json TEXT NOT NULL,monitors_json TEXT NOT NULL,secret BLOB NOT NULL);
 INSERT INTO webhook_destinations SELECT 'legacy','Default webhook',enabled,'global','[]','[]',secret FROM webhook_config;
 ALTER TABLE deliveries ADD COLUMN destination_id TEXT NOT NULL DEFAULT 'legacy';
 ALTER TABLE deliveries ADD COLUMN destination_name TEXT NOT NULL DEFAULT 'Default webhook';
 CREATE INDEX delivery_destination ON deliveries(destination_id,status,created);
 DELETE FROM webhook_config;
 PRAGMA user_version=3; COMMIT;`); err != nil {
			return fail(err)
		}
	}
	if version < 4 {
		if _, err = db.Exec(`BEGIN;
 CREATE TABLE alert_silences(id TEXT PRIMARY KEY,target_id TEXT NOT NULL,kind TEXT NOT NULL,starts INTEGER NOT NULL,ends INTEGER NOT NULL,reason TEXT NOT NULL);
 CREATE INDEX silence_active ON alert_silences(target_id,starts,ends);
 ALTER TABLE deliveries ADD COLUMN target_id TEXT NOT NULL DEFAULT '';
 UPDATE deliveries SET target_id=COALESCE(json_extract(CAST(payload AS TEXT),'$.target.id'),'');
 PRAGMA user_version=4; COMMIT;`); err != nil {
			return fail(err)
		}
	}
	if version < 5 {
		if _, err = db.Exec(`BEGIN;
 ALTER TABLE incidents ADD COLUMN excluded_ms INTEGER NOT NULL DEFAULT 0;
 ALTER TABLE incidents ADD COLUMN paused INTEGER NOT NULL DEFAULT 0;
 PRAGMA user_version=5; COMMIT;`); err != nil {
			return fail(err)
		}
	}
	if version < 6 {
		if _, err = db.Exec(`BEGIN;
 CREATE TABLE probe_minutes(id INTEGER PRIMARY KEY, target_id TEXT NOT NULL, session TEXT NOT NULL, name TEXT NOT NULL, probe_type TEXT NOT NULL, minute INTEGER NOT NULL,
 checks INTEGER NOT NULL, failed INTEGER NOT NULL, latency_sum REAL NOT NULL, latency_min REAL, latency_max REAL,
 UNIQUE(target_id,session,minute));
 CREATE INDEX probe_minute_time ON probe_minutes(minute);
 CREATE INDEX probe_target_time ON probe_minutes(target_id,minute);
 PRAGMA user_version=6; COMMIT;`); err != nil {
			return fail(err)
		}
	}
	// An unfinished prior session cannot establish outage duration while closed.
	if _, err = db.Exec("UPDATE incidents SET status='interrupted', ended=last_seen, reason=reason || ' · Monitoring interrupted by application restart' WHERE status='open'"); err != nil {
		return fail(err)
	}
	s := &incidentStore{db: db, path: path, events: make(chan historyEvent, 1024), done: make(chan struct{})}
	if err = s.prune(now); err != nil {
		return fail(err)
	}
	return s, nil
}
func (s *incidentStore) setFailure(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failure = err.Error()
}
func (s *incidentStore) errorMessage() string { s.mu.Lock(); defer s.mu.Unlock(); return s.failure }
func (s *incidentStore) writer() {
	defer close(s.done)
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case e, ok := <-s.events:
			if !ok {
				return
			}
			if err := s.apply(e); err != nil {
				s.setFailure(fmt.Errorf("History could not be saved: %w", err))
			}
		case now := <-ticker.C:
			if err := s.prune(now); err != nil {
				s.setFailure(err)
			}
		}
	}
}
func (s *incidentStore) apply(e historyEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	t := e.target
	at := e.at.UnixMilli()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if e.sample != nil {
		if err := storeProbeMinute(ctx, tx, e); err != nil {
			return err
		}
	}
	if e.kind == "Monitoring paused" {
		_, err = tx.ExecContext(ctx, "UPDATE incidents SET paused=1 WHERE target_id=? AND status='open'", t.ID)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	if e.kind == "Monitoring resumed" {
		// Exclude the unobserved interval since the last check, retaining the
		// incident identity and original alert destination association.
		_, err = tx.ExecContext(ctx, "UPDATE incidents SET excluded_ms=excluded_ms+MAX(0,?-last_seen), last_seen=?, paused=0 WHERE target_id=? AND status='open' AND paused=1", at, at, t.ID)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	if e.kind == "result" {
		var paused int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM incidents WHERE target_id=? AND status='open' AND paused=1", t.ID).Scan(&paused); err != nil {
			return err
		}
		if paused > 0 {
			return tx.Commit()
		}
		gap := observationGapLimit(t)
		if _, err := tx.ExecContext(ctx, "UPDATE incidents SET status='interrupted', ended=last_seen, reason=reason || ' · Monitoring gap: no observations received' WHERE target_id=? AND status='open' AND last_seen<?", t.ID, e.at.Add(-gap).UnixMilli()); err != nil {
			return err
		}
	}
	if e.kind == "result" && t.State == "down" {
		var visualID int64
		result, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO incidents(target_id,name,host,group_name,probe_type,port,opened,last_seen,status,reason) VALUES(?,?,?,?,?,?,?,?,'open',?)`, t.ID, t.Name, t.Host, t.Group, t.Type, t.Port, at, at, t.Message)
		if err != nil {
			return err
		}
		if count, _ := result.RowsAffected(); count == 1 && e.previous == "up" {
			id, _ := result.LastInsertId()
			visualID = id
			if err = enqueueWebhook(ctx, tx, t, "down", id, e.at, 0); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, "UPDATE incidents SET last_seen=? WHERE target_id=? AND status='open'", at, t.ID); err != nil {
			return err
		}
		return s.commitVisual(ctx, tx, e, visualID, "down")
	}
	if e.kind == "result" && t.State == "up" {
		var id, opened, excluded int64
		err := tx.QueryRowContext(ctx, "SELECT id,opened,excluded_ms FROM incidents WHERE target_id=? AND status='open'", t.ID).Scan(&id, &opened, &excluded)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == nil {
			if err = enqueueWebhook(ctx, tx, t, "up", id, e.at, (at-opened-excluded)/1000); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, "UPDATE incidents SET status='resolved', last_seen=?, ended=?, reason=reason || ' · Recovery threshold reached' WHERE target_id=? AND status='open'", at, at, t.ID); err != nil {
			return err
		}
		return s.commitVisual(ctx, tx, e, id, "up")
	}
	if e.kind == "result" && t.State != "error" {
		// Fresh threshold evidence after resume is still observed time, even
		// while the card remains unknown until the threshold is reached.
		if _, err = tx.ExecContext(ctx, "UPDATE incidents SET last_seen=? WHERE target_id=? AND status='open'", at, t.ID); err != nil {
			return err
		}
		return tx.Commit()
	}
	reason := e.kind
	if reason == "result" {
		reason = "Check error: " + t.Message
	}
	_, err = tx.ExecContext(ctx, "UPDATE incidents SET status='interrupted', ended=last_seen, reason=reason || ' · ' || ? WHERE target_id=? AND status='open'", reason, t.ID)
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.visual.outages, t.ID)
	s.mu.Unlock()
	return nil
}
func (s *incidentStore) retention() (int, error) {
	var days int
	err := s.db.QueryRow("SELECT value FROM settings WHERE key='retention_days'").Scan(&days)
	return days, err
}
func (s *incidentStore) prune(now time.Time) error {
	if _, err := s.db.Exec("DELETE FROM probe_minutes WHERE minute<?", now.Add(-7*24*time.Hour).Unix()/60); err != nil {
		return err
	}
	days, err := s.retention()
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM incidents WHERE status!='open' AND (ended<? OR id NOT IN (SELECT id FROM incidents ORDER BY id DESC LIMIT 50000))`, now.AddDate(0, 0, -days).UnixMilli())
	if err != nil {
		return err
	}
	// Preserve the outage marker for active incidents so eventual recoveries can alert.
	_, err = s.db.Exec(`DELETE FROM deliveries WHERE status!='pending' AND incident_id NOT IN (SELECT id FROM incidents WHERE status='open') AND (created<? OR id NOT IN (SELECT id FROM deliveries ORDER BY created DESC,rowid DESC LIMIT 10000))`, now.AddDate(0, 0, -30).UnixMilli())
	if err != nil {
		return err
	}
	_, err = s.db.Exec("DELETE FROM alert_silences WHERE ends<?", now.AddDate(0, 0, -30).UnixMilli())
	return err
}
func historyWhere(search, status string, days int) (string, []any, error) {
	if status != "all" && status != "open" && status != "resolved" && status != "interrupted" {
		return "", nil, fmt.Errorf("Choose a valid incident status")
	}
	if days < 0 || days > 3650 {
		return "", nil, fmt.Errorf("History range must be 0–3650 days")
	}
	clauses := []string{"1=1"}
	args := []any{}
	if search = strings.ToLower(strings.TrimSpace(search)); search != "" {
		clauses = append(clauses, "(instr(lower(name),?)>0 OR instr(lower(host),?)>0 OR instr(lower(group_name),?)>0)")
		args = append(args, search, search, search)
	}
	if status != "all" {
		clauses = append(clauses, "status=?")
		args = append(args, status)
	}
	if days > 0 {
		clauses = append(clauses, "last_seen>=?")
		args = append(args, time.Now().AddDate(0, 0, -days).UnixMilli())
	}
	return " WHERE " + strings.Join(clauses, " AND "), args, nil
}
func (s *incidentStore) query(search, status string, days, offset, limit int) (HistoryPage, error) {
	page := HistoryPage{Items: []Incident{}, Path: s.path, Error: s.errorMessage()}
	where, args, err := historyWhere(search, status, days)
	if err != nil {
		return page, err
	}
	if offset < 0 {
		return page, fmt.Errorf("Invalid history page")
	}
	if err = s.db.QueryRow("SELECT count(*) FROM incidents"+where, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	page.RetentionDays, err = s.retention()
	if err != nil {
		return page, err
	}
	queryArgs := append(append([]any{}, args...), limit, offset)
	rows, err := s.db.Query(`SELECT id,target_id,name,host,group_name,probe_type,port,opened,last_seen,ended,status,reason,excluded_ms,paused FROM incidents`+where+" ORDER BY opened DESC,id DESC LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var item Incident
		var opened, last int64
		var ended sql.NullInt64
		var excluded int64
		var paused bool
		if err = rows.Scan(&item.ID, &item.TargetID, &item.Name, &item.Host, &item.Group, &item.Type, &item.Port, &opened, &last, &ended, &item.Status, &item.Reason, &excluded, &paused); err != nil {
			return page, err
		}
		item.Opened = time.UnixMilli(opened).UTC().Format(time.RFC3339Nano)
		item.LastSeen = time.UnixMilli(last).UTC().Format(time.RFC3339Nano)
		end := last
		if ended.Valid {
			end = ended.Int64
			item.Ended = time.UnixMilli(end).UTC().Format(time.RFC3339Nano)
		}
		item.DurationSeconds = (end - opened - excluded) / 1000
		if paused && item.Status == "open" {
			item.Reason += " · Monitoring paused; outage remains open"
		}
		if item.DurationSeconds < 0 {
			item.DurationSeconds = 0
		}
		page.Items = append(page.Items, item)
	}
	return page, rows.Err()
}

// Called with the application lock held, preserving event order across edits,
// results, pause, and deletion without disk I/O on the probe path.
func (a *App) recordHistoryLocked(t Target, kind string, previous ...string) {
	if a.history == nil || a.closing {
		return
	}
	prior := ""
	if len(previous) > 0 {
		prior = previous[0]
	}
	select {
	case a.history.events <- historyEvent{target: t, kind: kind, at: time.Now(), previous: prior}:
	default:
		a.history.setFailure(fmt.Errorf("History queue is full. Some observations were not recorded; history may contain gaps"))
	}
}

func (a *App) recordProbeHistoryLocked(t Target, previous string, sample *storedProbeSample, at time.Time) {
	if a.history == nil || a.closing {
		return
	}
	select {
	case a.history.events <- historyEvent{target: t, kind: "result", previous: previous, at: at, sample: sample}:
	default:
		a.history.setFailure(fmt.Errorf("History queue is full. Some observations were not recorded; history may contain gaps"))
	}
}
func (a *App) GetIncidents(search, status string, days, offset int) (HistoryPage, error) {
	if a.history == nil {
		return HistoryPage{Items: []Incident{}, Error: a.historyError}, nil
	}
	return a.history.query(search, status, days, offset, 100)
}
func (a *App) SetHistoryRetention(days int) error {
	if days < 1 || days > 3650 {
		return fmt.Errorf("Keep history for 1–3650 days")
	}
	if a.history == nil {
		return fmt.Errorf("History storage is unavailable")
	}
	if _, err := a.history.db.Exec("UPDATE settings SET value=? WHERE key='retention_days'", days); err != nil {
		return err
	}
	return a.history.prune(time.Now())
}
func csvCell(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if strings.HasPrefix(value, "\t") || strings.HasPrefix(value, "\r") || strings.HasPrefix(value, "\n") || strings.HasPrefix(trimmed, "=") || strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "@") {
		return "'" + value
	}
	return value
}
func writeIncidentCSV(w io.Writer, items []Incident) error {
	csvWriter := csv.NewWriter(w)
	if err := csvWriter.Write([]string{"Monitor", "Host", "Group", "Check", "Port", "Status", "Opened (UTC)", "Ended (UTC)", "Last observed (UTC)", "Observed duration (seconds)", "Reason"}); err != nil {
		return err
	}
	for _, item := range items {
		row := []string{item.Name, item.Host, item.Group, item.Type, fmt.Sprint(item.Port), item.Status, item.Opened, item.Ended, item.LastSeen, fmt.Sprint(item.DurationSeconds), item.Reason}
		for i := range row {
			row[i] = csvCell(row[i])
		}
		if err := csvWriter.Write(row); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return csvWriter.Error()
}
func (a *App) ExportIncidents(search, status string, days int) (string, error) {
	if a.history == nil {
		return "", fmt.Errorf("History storage is unavailable")
	}
	page, err := a.history.query(search, status, days, 0, 50000)
	if err != nil {
		return "", err
	}
	if page.Total > 50000 {
		return "", fmt.Errorf("Narrow the history filters to export at most 50,000 incidents")
	}
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{Title: "Export incident history", DefaultFilename: "hostlight-incidents.csv", Filters: []wruntime.FileFilter{{DisplayName: "CSV files", Pattern: "*.csv"}}})
	if err != nil || path == "" {
		return "", err
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	err = writeIncidentCSV(f, page.Items)
	closeErr := f.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	return path, nil
}
