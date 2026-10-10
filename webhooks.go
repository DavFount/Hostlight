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
	Enabled         bool   `json:"enabled"`
	Configured      bool   `json:"configured"`
	URL             string `json:"url"`
	Bearer          string `json:"bearer"`
	SigningKey      string `json:"signingKey"`
	AllowHTTP       bool   `json:"allowHTTP"`
	ClearBearer     bool   `json:"clearBearer"`
	ClearSigningKey bool   `json:"clearSigningKey"`
	HasBearer       bool   `json:"hasBearer"`
	HasSigningKey   bool   `json:"hasSigningKey"`
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
	Name     string `json:"name"`
	Host     string `json:"host"`
	ID       string `json:"id"`
	Event    string `json:"event"`
	Created  string `json:"created"`
	Attempts int    `json:"attempts"`
	Status   string `json:"status"`
	Result   string `json:"result"`
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
	var out WebhookSettings
	if a.history == nil {
		return out, fmt.Errorf("Alert storage is unavailable")
	}
	var blob []byte
	err := a.history.db.QueryRow("SELECT enabled,secret FROM webhook_config WHERE id=1").Scan(&out.Enabled, &blob)
	if err == sql.ErrNoRows {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	s, err := decodeWebhook(blob)
	if err != nil {
		return out, err
	}
	out.Configured = true
	out.AllowHTTP = s.AllowHTTP
	out.HasBearer = s.Bearer != ""
	out.HasSigningKey = s.SigningKey != ""
	// URLs often contain tokens. Do not return any persisted secret to the webview.
	return out, nil
}
func (a *App) SaveWebhookSettings(input WebhookSettings) error {
	if a.history == nil {
		return fmt.Errorf("Alert storage is unavailable")
	}
	tx, err := a.history.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var old []byte
	var secret webhookSecret
	err = tx.QueryRow("SELECT secret FROM webhook_config WHERE id=1").Scan(&old)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if len(old) > 0 {
		secret, err = decodeWebhook(old)
		if err != nil && input.URL == "" {
			return err
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
	if _, err = tx.Exec("INSERT INTO webhook_config(id,enabled,secret) VALUES(1,?,?) ON CONFLICT(id) DO UPDATE SET enabled=excluded.enabled,secret=excluded.secret", input.Enabled, blob); err != nil {
		return err
	}
	// Reconfiguration cancels unsent deliveries; never route old alerts to a new destination.
	if _, err = tx.Exec("UPDATE deliveries SET status='cancelled',result='Cancelled by alert settings change' WHERE status='pending'"); err != nil {
		return err
	}
	return tx.Commit()
}
func enqueueWebhook(ctx context.Context, tx *sql.Tx, t Target, event string, incidentID int64, at time.Time, duration int64) error {
	if duration < 0 {
		duration = 0
	}
	var blob []byte
	err := tx.QueryRowContext(ctx, "SELECT secret FROM webhook_config WHERE id=1 AND enabled=1").Scan(&blob)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	// Bound the durable queue. Explicitly fail rather than silently discard alerts.
	var count int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM deliveries WHERE status='pending'").Scan(&count); err != nil {
		return err
	}
	if count >= 10000 {
		return fmt.Errorf("Webhook queue is full; disable alerts or restore delivery")
	}
	id := uuid.NewString()
	payload, err := json.Marshal(WebhookEvent{1, id, event, at.UTC().Format(time.RFC3339Nano), incidentID, t, duration})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO deliveries(id,incident_id,event,payload,secret,created,next_attempt) VALUES(?,?,?,?,?,?,?)", id, incidentID, event, payload, blob, at.UnixMilli(), at.UnixMilli())
	return err
}
func (a *App) TestWebhook() error {
	if a.history == nil {
		return fmt.Errorf("Alert storage is unavailable")
	}
	tx, err := a.history.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Tests use the same durable queue and retry policy as alerts, and require enabled settings.
	var enabled bool
	if err = tx.QueryRow("SELECT enabled FROM webhook_config WHERE id=1").Scan(&enabled); err != nil || !enabled {
		return fmt.Errorf("Save and enable the webhook before sending a test")
	}
	if err = enqueueWebhook(context.Background(), tx, Target{Name: "Hostlight test", State: "test"}, "test", 0, time.Now(), 0); err != nil {
		return err
	}
	return tx.Commit()
}
func (a *App) GetWebhookDeliveries() ([]WebhookDelivery, error) {
	items := []WebhookDelivery{}
	if a.history == nil {
		return items, fmt.Errorf("Alert storage is unavailable")
	}
	rows, err := a.history.db.Query("SELECT id,event,created,attempts,status,result,payload FROM deliveries ORDER BY created DESC,rowid DESC LIMIT 100")
	if err != nil {
		return items, err
	}
	defer rows.Close()
	for rows.Next() {
		var d WebhookDelivery
		var at int64
		var payload []byte
		if err = rows.Scan(&d.ID, &d.Event, &at, &d.Attempts, &d.Status, &d.Result, &payload); err != nil {
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
	// FIFO ordering preserves outage-before-recovery, including across retry/restart.
	err := s.db.QueryRowContext(ctx, "SELECT id,payload,secret,attempts,created FROM deliveries WHERE status='pending' ORDER BY created,rowid LIMIT 1").Scan(&id, &payload, &blob, &attempts, &created)
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
