package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// App struct
type App struct {
	monitorImport    *pendingMonitorImport
	ctx              context.Context
	mu               sync.Mutex
	targets          map[string]*monitor
	next             uint64
	configPath       string
	configError      string
	slots            chan struct{}
	scheduling       SchedulingSettings
	activeScheduling SchedulingSettings
	dial             func(context.Context, string, time.Duration) (net.Conn, error)
	ping             func(context.Context, string, time.Duration) (float64, error)
	history          *incidentStore
	historyError     string
	closing          bool
	alertCancel      context.CancelFunc
	alertDone        chan struct{}
	desktop          desktopState
}

// NewApp creates a new App application struct
func NewApp() *App {
	a := &App{targets: make(map[string]*monitor), slots: make(chan struct{}, 32)}
	a.scheduling = SchedulingSettings{Concurrency: 32, SpreadMilliseconds: 250}
	a.activeScheduling = a.scheduling
	a.ping = pingHost
	a.dial = func(ctx context.Context, address string, timeout time.Duration) (net.Conn, error) {
		d := net.Dialer{Timeout: timeout}
		return d.DialContext(ctx, "tcp", address)
	}
	if dir, err := os.UserConfigDir(); err == nil {
		a.configPath = filepath.Join(dir, "Hostlight", "targets.json")
	} else {
		a.configError = err.Error()
	}
	return a
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.configPath != "" {
		store, err := openIncidentStore(filepath.Join(filepath.Dir(a.configPath), "history.db"), time.Now())
		if err != nil {
			a.historyError = "History unavailable: " + err.Error()
		} else {
			a.history = store
			go store.writer()
			alertCtx, cancel := context.WithCancel(context.Background())
			a.alertCancel = cancel
			a.alertDone = make(chan struct{})
			go func() { defer close(a.alertDone); store.deliverWebhooks(alertCtx) }()
		}
	}
	a.loadConfiguration()
}

func (a *App) shutdown(context.Context) {
	a.mu.Lock()
	if a.closing {
		a.mu.Unlock()
		return
	}
	for _, m := range a.targets {
		m.cancel()
		a.recordHistoryLocked(m.target, "Monitoring stopped when Hostlight closed")
	}
	a.closing = true
	tray := a.desktop.tray
	store := a.history
	if store != nil {
		close(store.events)
	}
	a.mu.Unlock()
	if tray != nil {
		tray.Close()
	}
	if a.alertCancel != nil {
		a.alertCancel()
		<-a.alertDone
	}
	if store != nil {
		select {
		case <-store.done:
			store.db.Close()
		case <-time.After(5 * time.Second):
			go func() { <-store.done; store.db.Close() }()
		}
	}
}

type ConfigurationStatus struct {
	Path         string `json:"path"`
	Error        string `json:"error"`
	HistoryError string `json:"historyError"`
}

func (a *App) GetConfigurationStatus() ConfigurationStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	historyError := a.historyError
	if a.history != nil {
		historyError = a.history.errorMessage()
	}
	return ConfigurationStatus{Path: a.configPath, Error: a.configError, HistoryError: historyError}
}

type Target struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Host               string  `json:"host"`
	Type               string  `json:"type"`
	Port               int     `json:"port"`
	Group              string  `json:"group"`
	Interval           int     `json:"interval"`
	Timeout            int     `json:"timeout"`
	Failures           int     `json:"failures"`
	Recovery           int     `json:"recovery"`
	State              string  `json:"state"`
	Latency            float64 `json:"latency"`
	Message            string  `json:"message"`
	Checked            string  `json:"checked"`
	ExpectedStatus     int     `json:"expectedStatus,omitempty"`
	BodyMode           string  `json:"bodyMode,omitempty"`
	BodyExpected       string  `json:"bodyExpected,omitempty"`
	JSONPointer        string  `json:"jsonPointer,omitempty"`
	JSONType           string  `json:"jsonType,omitempty"`
	TLSMinDays         int     `json:"tlsMinDays,omitempty"`
	CertificateExpires string  `json:"certificateExpires,omitempty"`
}
type monitor struct {
	target Target
	cancel context.CancelFunc
}

func validate(t Target) error {
	if t.Type != "http" && t.TLSMinDays != 0 {
		return fmt.Errorf("Certificate expiry policies require an HTTPS monitor")
	}
	if t.Type != "http" && t.BodyMode != "" {
		return fmt.Errorf("Response-body checks require an HTTP/HTTPS monitor")
	}
	if t.Name == "" || t.Host == "" {
		return fmt.Errorf("Enter a name and host address")
	}
	_, ipErr := netip.ParseAddr(t.Host)
	if t.Type == "http" {
		if err := validateHTTPDefinition(t); err != nil {
			return err
		}
	} else if strings.ContainsAny(t.Host, "/ :|,\t\r\n") && ipErr != nil {
		return fmt.Errorf("Enter a hostname or IP address without a protocol or port")
	}
	if t.Type != "" && t.Type != "tcp" && t.Type != "icmp" && t.Type != "http" {
		return fmt.Errorf("Choose Ping (ICMP), TCP service, or HTTP/HTTPS")
	}
	if t.Type != "icmp" && t.Type != "http" && (t.Port < 1 || t.Port > 65535) {
		return fmt.Errorf("Port must be between 1 and 65535")
	}
	if t.Interval < 1 || t.Interval > 3600 || t.Timeout < 1 || t.Timeout > 60 {
		return fmt.Errorf("Interval must be 1–3600 seconds and timeout 1–60 seconds")
	}
	if t.Failures < 1 || t.Failures > 100 || t.Recovery < 1 || t.Recovery > 100 {
		return fmt.Errorf("Thresholds must be 1–100 checks")
	}
	return nil
}
func (a *App) SaveTarget(t Target) error {
	return a.saveTarget(t, false)
}
func (a *App) saveTarget(t Target, resume bool) error {
	t.Name = strings.TrimSpace(t.Name)
	t.Host = strings.TrimSpace(t.Host)
	t.Group = strings.TrimSpace(t.Group)
	// Empty type identifies TCP definitions written by the first preview.
	if t.Type == "" {
		t.Type = "tcp"
	}
	if t.Type == "icmp" || t.Type == "http" {
		t.Port = 0
	}
	if t.Type == "http" && t.ExpectedStatus == 0 {
		t.ExpectedStatus = 200
	}
	if t.Type != "http" {
		t.ExpectedStatus = 0
		t.BodyMode = ""
		t.BodyExpected = ""
		t.JSONPointer = ""
		t.JSONType = ""
		t.TLSMinDays = 0
	}
	t = normalizeHTTPBodyFields(t)
	if err := validate(t); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.configError != "" {
		return fmt.Errorf("Configuration unavailable: %s", a.configError)
	}
	var old *monitor
	if t.ID != "" {
		var ok bool
		old, ok = a.targets[t.ID]
		if !ok {
			return fmt.Errorf("Target no longer exists")
		}
	} else {
		a.next++
		t.ID = fmt.Sprint(a.next)
	}
	t.State = "unknown"
	t.Message = "Waiting for checks"
	t.Checked = ""
	t.Latency = 0
	t.CertificateExpires = ""
	if old != nil && old.target.State == "paused" && !resume {
		t.State = "paused"
		t.Message = "Monitoring paused"
	}
	ctx, cancel := context.WithCancel(a.ctx)
	m := &monitor{target: t, cancel: cancel}
	a.targets[t.ID] = m
	if err := a.persistLocked(); err != nil {
		cancel()
		if old != nil {
			a.targets[t.ID] = old
		} else {
			delete(a.targets, t.ID)
		}
		return err
	}
	if old != nil {
		old.cancel()
		a.recordHistoryLocked(old.target, "Monitor settings changed")
	}
	if t.State != "paused" {
		go a.run(ctx, m, t)
	}
	return nil
}
func (a *App) GetTargets() []Target {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]Target, 0, len(a.targets))
	for _, m := range a.targets {
		out = append(out, m.target)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (a *App) DeleteTarget(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if m, ok := a.targets[id]; ok {
		delete(a.targets, id)
		if err := a.persistLocked(); err != nil {
			a.targets[id] = m
			return err
		}
		m.cancel()
		a.recordHistoryLocked(m.target, "Monitor removed")
	}
	return nil
}
func (a *App) PauseTarget(id string) error {
	return a.SetTargetsPaused([]string{id}, true)
}
func (a *App) ResumeTarget(id string) error {
	return a.SetTargetsPaused([]string{id}, false)
}
func (a *App) run(ctx context.Context, m *monitor, t Target) {
	a.mu.Lock()
	spread := a.activeScheduling.SpreadMilliseconds
	a.mu.Unlock()
	if !waitForProbe(ctx, probeStartDelay(t.ID, spread)) {
		return
	}
	rules := statusRules{failures: t.Failures, recovery: t.Recovery}
	for {
		select {
		case a.slots <- struct{}{}:
		case <-ctx.Done():
			return
		}
		started := time.Now()
		var err error
		var latency float64
		var httpMessage string
		var certificateExpires string
		if t.Type == "http" {
			result, probeErr := probeHTTP(ctx, t)
			latency, err = result.Latency, probeErr
			httpMessage = result.Message
			certificateExpires = result.CertificateExpires
		} else if t.Type == "icmp" {
			latency, err = a.ping(ctx, t.Host, time.Duration(t.Timeout)*time.Second)
		} else {
			var conn net.Conn
			conn, err = a.dial(ctx, net.JoinHostPort(t.Host, fmt.Sprint(t.Port)), time.Duration(t.Timeout)*time.Second)
			latency = float64(time.Since(started).Microseconds()) / 1000
			if conn != nil {
				conn.Close()
			}
		}
		<-a.slots
		if ctx.Err() != nil {
			return
		}
		a.mu.Lock()
		if a.closing || a.targets[t.ID] != m || m.target.State == "paused" {
			a.mu.Unlock()
			return
		}
		previous := m.target.State
		m.target.Checked = time.Now().UTC().Format(time.RFC3339)
		m.target.Latency = latency
		m.target.CertificateExpires = certificateExpires
		var dns *net.DNSError
		var local *probeSetupError
		localError := errors.As(err, &dns) || errors.As(err, &local)
		if localError {
			rules.good = 0
			rules.bad = 0
			m.target.State = "error"
			if dns != nil {
				m.target.Message = "DNS lookup failed. Check the hostname and local DNS connection."
			} else {
				m.target.Message = err.Error()
			}
		} else if err == nil {
			m.target.Message = "TCP connection successful"
			if t.Type == "icmp" {
				m.target.Message = "Ping reply received"
			}
			if t.Type == "http" {
				m.target.Message = httpMessage
			}
			m.target.State = rules.apply(m.target.State, true)
		} else {
			m.target.Message = err.Error()
			m.target.State = rules.apply(m.target.State, false)
		}
		a.recordHistoryLocked(m.target, "result", previous)
		a.mu.Unlock()
		timer := time.NewTimer(time.Duration(t.Interval) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

type statusRules struct{ good, bad, failures, recovery int }

func (s *statusRules) apply(state string, success bool) string {
	if state == "error" {
		state = "unknown"
	}
	if success {
		s.bad = 0
		s.good++
		if s.good >= s.recovery {
			return "up"
		}
	} else {
		s.good = 0
		s.bad++
		if s.bad >= s.failures {
			return "down"
		}
	}
	return state
}
