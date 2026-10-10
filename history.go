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
	target Target
	kind   string
	at     time.Time
}
type incidentStore struct {
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
	if version > 1 {
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
	if e.kind == "result" {
		gap := 2 * time.Duration(t.Interval+t.Timeout) * time.Second
		if gap < 30*time.Second {
			gap = 30 * time.Second
		}
		if _, err := s.db.ExecContext(ctx, "UPDATE incidents SET status='interrupted', ended=last_seen, reason=reason || ' · Monitoring gap: no observations received' WHERE target_id=? AND status='open' AND last_seen<?", t.ID, e.at.Add(-gap).UnixMilli()); err != nil {
			return err
		}
	}
	if e.kind == "result" && t.State == "down" {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO incidents(target_id,name,host,group_name,probe_type,port,opened,last_seen,status,reason) VALUES(?,?,?,?,?,?,?,?,'open',?)`, t.ID, t.Name, t.Host, t.Group, t.Type, t.Port, at, at, t.Message)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE incidents SET last_seen=? WHERE target_id=? AND status='open'", at, t.ID); err != nil {
			return err
		}
		return tx.Commit()
	}
	if e.kind == "result" && t.State == "up" {
		_, err := s.db.ExecContext(ctx, "UPDATE incidents SET status='resolved', last_seen=?, ended=?, reason=reason || ' · Recovery threshold reached' WHERE target_id=? AND status='open'", at, at, t.ID)
		return err
	}
	if e.kind == "result" && t.State != "error" {
		return nil
	}
	reason := e.kind
	if reason == "result" {
		reason = "Check error: " + t.Message
	}
	_, err := s.db.ExecContext(ctx, "UPDATE incidents SET status='interrupted', ended=last_seen, reason=reason || ' · ' || ? WHERE target_id=? AND status='open'", reason, t.ID)
	return err
}
func (s *incidentStore) retention() (int, error) {
	var days int
	err := s.db.QueryRow("SELECT value FROM settings WHERE key='retention_days'").Scan(&days)
	return days, err
}
func (s *incidentStore) prune(now time.Time) error {
	days, err := s.retention()
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM incidents WHERE status!='open' AND (ended<? OR id NOT IN (SELECT id FROM incidents ORDER BY id DESC LIMIT 50000))`, now.AddDate(0, 0, -days).UnixMilli())
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
	rows, err := s.db.Query(`SELECT id,target_id,name,host,group_name,probe_type,port,opened,last_seen,ended,status,reason FROM incidents`+where+" ORDER BY opened DESC,id DESC LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var item Incident
		var opened, last int64
		var ended sql.NullInt64
		if err = rows.Scan(&item.ID, &item.TargetID, &item.Name, &item.Host, &item.Group, &item.Type, &item.Port, &opened, &last, &ended, &item.Status, &item.Reason); err != nil {
			return page, err
		}
		item.Opened = time.UnixMilli(opened).UTC().Format(time.RFC3339Nano)
		item.LastSeen = time.UnixMilli(last).UTC().Format(time.RFC3339Nano)
		end := last
		if ended.Valid {
			end = ended.Int64
			item.Ended = time.UnixMilli(end).UTC().Format(time.RFC3339Nano)
		}
		item.DurationSeconds = (end - opened) / 1000
		if item.DurationSeconds < 0 {
			item.DurationSeconds = 0
		}
		page.Items = append(page.Items, item)
	}
	return page, rows.Err()
}

// Called with the application lock held, preserving event order across edits,
// results, pause, and deletion without disk I/O on the probe path.
func (a *App) recordHistoryLocked(t Target, kind string) {
	if a.history == nil || a.closing {
		return
	}
	select {
	case a.history.events <- historyEvent{t, kind, time.Now()}:
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
