package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type WebhookSettings struct {
	Error           string   `json:"error,omitempty"`
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Scope           string   `json:"scope"`
	Groups          []string `json:"groups"`
	MonitorIDs      []string `json:"monitorIds"`
	Enabled         bool     `json:"enabled"`
	Configured      bool     `json:"configured"`
	URL             string   `json:"url"`
	Bearer          string   `json:"bearer"`
	SigningKey      string   `json:"signingKey"`
	AllowHTTP       bool     `json:"allowHTTP"`
	ClearBearer     bool     `json:"clearBearer"`
	ClearSigningKey bool     `json:"clearSigningKey"`
	HasBearer       bool     `json:"hasBearer"`
	HasSigningKey   bool     `json:"hasSigningKey"`
}
type webhookSecret struct {
	URL        string
	Bearer     string
	SigningKey string
	AllowHTTP  bool
}
type WebhookEvent struct {
	SchemaVersion   int    `json:"schemaVersion"`
	ID              string `json:"id"`
	Event           string `json:"event"`
	Time            string `json:"time"`
	IncidentID      int64  `json:"incidentId"`
	Target          Target `json:"target"`
	DurationSeconds int64  `json:"durationSeconds"`
}
type WebhookDelivery struct {
	DestinationID   string `json:"destinationId"`
	DestinationName string `json:"destinationName"`
	Name            string `json:"name"`
	Host            string `json:"host"`
	ID              string `json:"id"`
	Event           string `json:"event"`
	Created         string `json:"created"`
	Attempts        int    `json:"attempts"`
	Status          string `json:"status"`
	Result          string `json:"result"`
}

func validateWebhook(s webhookSecret) error {
	u, err := url.Parse(s.URL)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && s.AllowHTTP)) {
		return fmt.Errorf("Enter a valid HTTPS webhook URL without a username, password, or fragment. HTTP requires explicit permission")
	}
	if len(s.URL) > 8192 || len(s.Bearer) > 8192 || len(s.SigningKey) > 8192 || strings.ContainsAny(s.Bearer, "\r\n") {
		return fmt.Errorf("Webhook settings are too long or contain invalid header characters")
	}
	return nil
}
func decodeWebhook(blob []byte) (webhookSecret, error) {
	var secret webhookSecret
	data, err := unprotectSecret(blob)
	if err != nil {
		return secret, err
	}
	if json.Unmarshal(data, &secret) != nil {
		return secret, fmt.Errorf("Protected webhook settings could not be read")
	}
	return secret, validateWebhook(secret)
}
func (a *App) GetWebhookSettings() (WebhookSettings, error) {
	return a.getWebhookDestination("legacy")
}
func (a *App) getWebhookDestination(id string) (WebhookSettings, error) {
	out := WebhookSettings{Groups: []string{}, MonitorIDs: []string{}}
	if a.history == nil {
		return out, fmt.Errorf("Alert storage is unavailable")
	}
	var blob []byte
	var groups, monitors string
	err := a.history.db.QueryRow("SELECT id,name,enabled,scope,groups_json,monitors_json,secret FROM webhook_destinations WHERE id=?", id).Scan(&out.ID, &out.Name, &out.Enabled, &out.Scope, &groups, &monitors, &blob)
	if err == sql.ErrNoRows {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	out.Configured = true
	if json.Unmarshal([]byte(groups), &out.Groups) != nil || json.Unmarshal([]byte(monitors), &out.MonitorIDs) != nil {
		return out, fmt.Errorf("Webhook routing settings could not be read")
	}
	s, err := decodeWebhook(blob)
	if err != nil {
		return out, err
	}
	out.AllowHTTP = s.AllowHTTP
	out.HasBearer = s.Bearer != ""
	out.HasSigningKey = s.SigningKey != ""
	// URLs often contain tokens. Do not return any persisted secret to the webview.
	return out, nil
}
func (a *App) SaveWebhookSettings(input WebhookSettings) error {
	input.ID = "legacy"
	input.Name = "Default webhook"
	input.Scope = "global"
	return a.SaveWebhookDestination(input)
}
func (a *App) GetWebhookDestinations() ([]WebhookSettings, error) {
	items := []WebhookSettings{}
	if a.history == nil {
		return items, fmt.Errorf("Alert storage is unavailable")
	}
	rows, err := a.history.db.Query("SELECT id FROM webhook_destinations ORDER BY name,id")
	if err != nil {
		return items, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return items, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return items, err
	}
	for _, id := range ids {
		out, e := a.getWebhookDestination(id)
		if e != nil {
			out.Error = e.Error()
		}
		items = append(items, out)
	}
	return items, nil
}
func validateRouting(input *WebhookSettings) error {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len(input.Name) > 120 {
		return fmt.Errorf("Enter a destination name of 1–120 characters")
	}
	if input.Scope != "global" && input.Scope != "selected" {
		return fmt.Errorf("Choose all monitors or selected groups and monitors")
	}
	normalize := func(values []string, trim bool) ([]string, error) {
		out := []string{}
		seen := map[string]bool{}
		if len(values) > 10000 {
			return nil, fmt.Errorf("Too many routing selections")
		}
		for _, v := range values {
			if trim {
				v = strings.TrimSpace(v)
			}
			if len(v) > 256 {
				return nil, fmt.Errorf("Routing selection is too long")
			}
			if !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
		return out, nil
	}
	var err error
	if input.Groups, err = normalize(input.Groups, false); err != nil {
		return err
	}
	if input.MonitorIDs, err = normalize(input.MonitorIDs, true); err != nil {
		return err
	}
	for _, id := range input.MonitorIDs {
		if id == "" {
			return fmt.Errorf("Invalid monitor selection")
		}
	}
	if input.Scope == "global" {
		input.Groups = []string{}
		input.MonitorIDs = []string{}
	} else if len(input.Groups)+len(input.MonitorIDs) == 0 {
		return fmt.Errorf("Select at least one group or monitor")
	}
	return nil
}
func (a *App) SaveWebhookDestination(input WebhookSettings) error {
	if a.history == nil {
		return fmt.Errorf("Alert storage is unavailable")
	}
	if err := validateRouting(&input); err != nil {
		return err
	}
	tx, err := a.history.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var old []byte
	var secret webhookSecret
	creating := input.ID == ""
	if creating {
		input.ID = uuid.NewString()
	}
	err = tx.QueryRow("SELECT secret FROM webhook_destinations WHERE id=?", input.ID).Scan(&old)
	if err == sql.ErrNoRows && !creating && input.ID != "legacy" {
		return fmt.Errorf("Webhook destination no longer exists")
	}
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if len(old) > 0 {
		secret, err = decodeWebhook(old)
		if err != nil && input.URL == "" {
			return err
		}
	}
	if len(old) == 0 {
		var count int
		if err = tx.QueryRow("SELECT count(*) FROM webhook_destinations").Scan(&count); err != nil {
			return err
		}
		if count >= 50 {
			return fmt.Errorf("A maximum of 50 webhook destinations is supported")
		}
	}
	if input.URL != "" {
		secret.URL = strings.TrimSpace(input.URL)
	}
	if input.Bearer != "" {
		secret.Bearer = input.Bearer
	}
	if input.ClearBearer {
		secret.Bearer = ""
	}
	if input.SigningKey != "" {
		secret.SigningKey = input.SigningKey
	}
	if input.ClearSigningKey {
		secret.SigningKey = ""
	}
	secret.AllowHTTP = input.AllowHTTP
	if err = validateWebhook(secret); err != nil {
		return err
	}
	data, _ := json.Marshal(secret)
	blob, err := protectSecret(data)
	if err != nil {
		return err
	}
	groups, _ := json.Marshal(input.Groups)
	monitors, _ := json.Marshal(input.MonitorIDs)
	if _, err = tx.Exec("INSERT INTO webhook_destinations(id,name,enabled,scope,groups_json,monitors_json,secret) VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,enabled=excluded.enabled,scope=excluded.scope,groups_json=excluded.groups_json,monitors_json=excluded.monitors_json,secret=excluded.secret", input.ID, input.Name, input.Enabled, input.Scope, string(groups), string(monitors), blob); err != nil {
		return err
	}
	// Reconfiguration cancels unsent deliveries; never route old alerts to a new destination.
	if _, err = tx.Exec("UPDATE deliveries SET status='cancelled',result='Cancelled by alert settings change' WHERE status='pending' AND destination_id=?", input.ID); err != nil {
		return err
	}
	return tx.Commit()
}
func (a *App) DeleteWebhookDestination(id string) error {
	if a.history == nil {
		return fmt.Errorf("Alert storage is unavailable")
	}
	tx, err := a.history.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("DELETE FROM webhook_destinations WHERE id=?", id); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE deliveries SET status='cancelled',result='Webhook destination removed' WHERE status='pending' AND destination_id=?", id); err != nil {
		return err
	}
	return tx.Commit()
}

// Matching uses an OR across groups and individual monitor IDs, once per destination.
func webhookMatches(scope string, groups, monitors []string, t Target) bool {
	if scope == "global" {
		return true
	}
	if scope != "selected" {
		return false
	}
	for _, group := range groups {
		if group == t.Group {
			return true
		}
	}
	for _, id := range monitors {
		if id == t.ID {
			return true
		}
	}
	return false
}
func enqueueWebhook(ctx context.Context, tx *sql.Tx, t Target, event string, incidentID int64, at time.Time, duration int64) error {
	return enqueueWebhookDestinations(ctx, tx, t, event, incidentID, at, duration, "")
}
func enqueueWebhookDestinations(ctx context.Context, tx *sql.Tx, t Target, event string, incidentID int64, at time.Time, duration int64, onlyID string) error {
	if duration < 0 {
		duration = 0
	}
	type destination struct {
		id, name, scope  string
		groups, monitors []string
		blob             []byte
	}
	rows, err := tx.QueryContext(ctx, "SELECT id,name,scope,groups_json,monitors_json,secret FROM webhook_destinations WHERE enabled=1 ORDER BY id")
	if err != nil {
		return err
	}
	selected := []destination{}
	for rows.Next() {
		var d destination
		var groups, monitors string
		if err = rows.Scan(&d.id, &d.name, &d.scope, &groups, &monitors, &d.blob); err != nil {
			rows.Close()
			return err
		}
		if json.Unmarshal([]byte(groups), &d.groups) != nil || json.Unmarshal([]byte(monitors), &d.monitors) != nil {
			rows.Close()
			return fmt.Errorf("Webhook routing settings could not be read")
		}
		if (onlyID == "" && event != "test" && webhookMatches(d.scope, d.groups, d.monitors, t)) || (onlyID != "" && onlyID == d.id) {
			selected = append(selected, d)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if onlyID != "" && len(selected) == 0 {
		return fmt.Errorf("Save and enable this webhook destination before sending a test")
	}
	var count int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM deliveries WHERE status='pending'").Scan(&count); err != nil {
		return err
	}
	for _, d := range selected {
		if event == "up" {
			var alerted int
			if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM deliveries WHERE incident_id=? AND event='down' AND status!='cancelled' AND destination_id=? AND secret=?", incidentID, d.id, d.blob).Scan(&alerted); err != nil {
				return err
			}
			if alerted == 0 {
				continue
			}
		}
		if count >= 10000 {
			return fmt.Errorf("Webhook queue is full; disable alerts or restore delivery")
		}
		count++
		id := uuid.NewString()
		payload, e := json.Marshal(WebhookEvent{1, id, event, at.UTC().Format(time.RFC3339Nano), incidentID, t, duration})
		if e != nil {
			return e
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO deliveries(id,incident_id,event,payload,secret,created,next_attempt,destination_id,destination_name) VALUES(?,?,?,?,?,?,?,?,?)", id, incidentID, event, payload, d.blob, at.UnixMilli(), at.UnixMilli(), d.id, d.name); err != nil {
			return err
		}
	}
	return nil
}
func (a *App) TestWebhook() error { return a.TestWebhookDestination("legacy") }
func (a *App) TestWebhookDestination(id string) error {
	if a.history == nil {
		return fmt.Errorf("Alert storage is unavailable")
	}
	if id == "" {
		return fmt.Errorf("Choose a saved destination")
	}
	tx, err := a.history.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = enqueueWebhookDestinations(context.Background(), tx, Target{Name: "Hostlight test", State: "test"}, "test", 0, time.Now(), 0, id); err != nil {
		return err
	}
	return tx.Commit()
}
func (a *App) GetWebhookDeliveries() ([]WebhookDelivery, error) {
	items := []WebhookDelivery{}
	if a.history == nil {
		return items, fmt.Errorf("Alert storage is unavailable")
	}
	rows, err := a.history.db.Query("SELECT id,event,created,attempts,status,result,payload,destination_id,destination_name FROM deliveries ORDER BY created DESC,rowid DESC LIMIT 100")
	if err != nil {
		return items, err
	}
	defer rows.Close()
	for rows.Next() {
		var d WebhookDelivery
		var at int64
		var payload []byte
		if err = rows.Scan(&d.ID, &d.Event, &at, &d.Attempts, &d.Status, &d.Result, &payload, &d.DestinationID, &d.DestinationName); err != nil {
			return items, err
		}
		d.Created = time.UnixMilli(at).UTC().Format(time.RFC3339Nano)
		var event WebhookEvent
		if json.Unmarshal(payload, &event) == nil {
			d.Name = event.Target.Name
			d.Host = event.Target.Host
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

// No redirects: credentials and payloads must never be forwarded to another endpoint.
func webhookClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
func sendWebhook(ctx context.Context, client *http.Client, secret webhookSecret, id string, payload []byte) (string, bool, time.Duration) {
	if validateWebhook(secret) != nil {
		return "Invalid protected endpoint settings", false, 0
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, secret.URL, bytes.NewReader(payload))
	if err != nil {
		return "Unable to create request", false, 0
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Hostlight/1")
	req.Header.Set("X-Hostlight-Event-ID", id)
	if secret.Bearer != "" {
		req.Header.Set("Authorization", "Bearer "+secret.Bearer)
	}
	if secret.SigningKey != "" {
		mac := hmac.New(sha256.New, []byte(secret.SigningKey))
		mac.Write(payload)
		req.Header.Set("X-Hostlight-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	res, err := client.Do(req)
	if err != nil {
		return "Network or TLS error (endpoint details hidden)", true, 0
	}
	defer res.Body.Close()
	io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	message := fmt.Sprintf("HTTP %d", res.StatusCode)
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return message, false, 0
	}
	retry := res.StatusCode == 408 || res.StatusCode == 429 || res.StatusCode >= 500
	delay := time.Duration(0)
	if value := res.Header.Get("Retry-After"); value != "" {
		if seconds, e := strconv.Atoi(value); e == nil && seconds > 0 {
			if seconds > 3600 {
				seconds = 3600
			}
			delay = time.Duration(seconds) * time.Second
		} else if at, e := http.ParseTime(value); e == nil {
			delay = time.Until(at)
			if delay < 0 {
				delay = 0
			}
			if delay > time.Hour {
				delay = time.Hour
			}
		}
	}
	return message, retry, delay
}
func (s *incidentStore) deliverOne(ctx context.Context, client *http.Client, now time.Time) (bool, error) {
	var id string
	var payload, blob []byte
	var attempts int
	var created int64
	// FIFO within each destination; a retrying destination never blocks another.
	err := s.db.QueryRowContext(ctx, `SELECT d.id,d.payload,d.secret,d.attempts,d.created FROM deliveries d WHERE d.status='pending' AND d.next_attempt<=? AND NOT EXISTS (SELECT 1 FROM deliveries earlier WHERE earlier.destination_id=d.destination_id AND earlier.status='pending' AND (earlier.created<d.created OR (earlier.created=d.created AND earlier.rowid<d.rowid))) ORDER BY d.next_attempt,d.created,d.rowid LIMIT 1`, now.UnixMilli()).Scan(&id, &payload, &blob, &attempts, &created)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var due int64
	if err = s.db.QueryRowContext(ctx, "SELECT next_attempt FROM deliveries WHERE id=?", id).Scan(&due); err != nil {
		return false, err
	}
	if due > now.UnixMilli() {
		return false, nil
	}
	result := "Delivery expired after 24 hours"
	retry := false
	delay := time.Duration(0)
	if now.Sub(time.UnixMilli(created)) < 24*time.Hour {
		secret, e := decodeWebhook(blob)
		if e != nil {
			result = "Protected endpoint unavailable; save webhook settings again"
		} else {
			result, retry, delay = sendWebhook(ctx, client, secret, id, payload)
		}
	}
	if ctx.Err() != nil {
		return false, nil
	} // Uncertain delivery is retried with the same event ID next session.
	attempts++
	if finished := time.Now(); finished.After(now) {
		now = finished
	}
	status := "failed"
	if strings.HasPrefix(result, "HTTP 2") {
		status = "delivered"
	} else if retry && attempts < 8 {
		status = "pending"
		backoff := time.Duration(1<<uint(attempts-1)) * 5 * time.Second
		backoff += time.Duration(rand.Int64N(int64(backoff/5) + 1))
		if delay < backoff {
			delay = backoff
		}
	}
	_, err = s.db.ExecContext(ctx, "UPDATE deliveries SET attempts=?,status=?,result=?,next_attempt=? WHERE id=? AND status='pending'", attempts, status, result, now.Add(delay).UnixMilli(), id)
	return true, err
}
func (s *incidentStore) deliverWebhooks(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	client := webhookClient()
	defer client.CloseIdleConnections()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if _, err := s.deliverOne(ctx, client, now); err != nil && ctx.Err() == nil {
				s.setFailure(fmt.Errorf("Webhook delivery storage error: %w", err))
			}
		}
	}
}
