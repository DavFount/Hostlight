package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const maxMonitorImportBytes = 1024 * 1024
const maxMonitorImportCount = 1000

// A separate allowlist prevents adding new runtime Target fields to exports by accident.
type PortableMonitor struct {
	Name           string `json:"name"`
	Host           string `json:"host"`
	Type           string `json:"type"`
	Port           int    `json:"port"`
	Group          string `json:"group"`
	Interval       int    `json:"interval"`
	Timeout        int    `json:"timeout"`
	Failures       int    `json:"failures"`
	Recovery       int    `json:"recovery"`
	Paused         bool   `json:"paused"`
	ExpectedStatus int    `json:"expectedStatus,omitempty"`
	BodyMode       string `json:"bodyMode,omitempty"`
	BodyExpected   string `json:"bodyExpected,omitempty"`
	JSONPointer    string `json:"jsonPointer,omitempty"`
	JSONType       string `json:"jsonType,omitempty"`
}
type MonitorBackup struct {
	Format     string            `json:"format"`
	Version    int               `json:"version"`
	ExportedAt string            `json:"exportedAt"`
	Monitors   []PortableMonitor `json:"monitors"`
}
type MonitorImportRow struct {
	Row     int             `json:"row"`
	Monitor PortableMonitor `json:"monitor"`
	Status  string          `json:"status"`
	Detail  string          `json:"detail"`
}
type MonitorImportPreview struct {
	Token   string             `json:"token"`
	Source  string             `json:"source"`
	Rows    []MonitorImportRow `json:"rows"`
	Ready   int                `json:"ready"`
	Skipped int                `json:"skipped"`
	Invalid int                `json:"invalid"`
}
type MonitorImportResult struct {
	Added   int `json:"added"`
	Skipped int `json:"skipped"`
	Paused  int `json:"paused"`
}
type pendingMonitorImport struct {
	token     string
	source    string
	backup    MonitorBackup
	signature [32]byte
	created   time.Time
}

func portableMonitor(t Target) PortableMonitor {
	t = normalizeHTTPBodyFields(t)
	if t.Type == "" {
		t.Type = "tcp"
	}
	if t.Type == "icmp" || t.Type == "http" {
		t.Port = 0
	}
	return PortableMonitor{Name: t.Name, Host: t.Host, Type: t.Type, Port: t.Port, Group: t.Group, Interval: t.Interval, Timeout: t.Timeout, Failures: t.Failures, Recovery: t.Recovery, Paused: t.State == "paused", ExpectedStatus: t.ExpectedStatus, BodyMode: t.BodyMode, BodyExpected: t.BodyExpected, JSONPointer: t.JSONPointer, JSONType: t.JSONType}
}
func (p PortableMonitor) target() Target {
	if p.Type == "http" && p.ExpectedStatus == 0 {
		p.ExpectedStatus = 200
	}
	if p.Type != "http" {
		p.ExpectedStatus = 0
	}
	return normalizeHTTPBodyFields(Target{Name: strings.TrimSpace(p.Name), Host: strings.TrimSpace(p.Host), Type: p.Type, Port: p.Port, Group: strings.TrimSpace(p.Group), Interval: p.Interval, Timeout: p.Timeout, Failures: p.Failures, Recovery: p.Recovery, ExpectedStatus: p.ExpectedStatus, BodyMode: p.BodyMode, BodyExpected: p.BodyExpected, JSONPointer: p.JSONPointer, JSONType: p.JSONType})
}
func (a *App) monitorBackupLocked() MonitorBackup {
	backup := MonitorBackup{Format: "hostlight-monitors", Version: 3, ExportedAt: time.Now().UTC().Format(time.RFC3339), Monitors: []PortableMonitor{}}
	ids := make([]string, 0, len(a.targets))
	for id := range a.targets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		backup.Monitors = append(backup.Monitors, portableMonitor(a.targets[id].target))
	}
	return backup
}
func (a *App) workspaceSignatureLocked() [32]byte {
	type entry struct {
		ID      string
		Monitor PortableMonitor
	}
	entries := []entry{}
	for id, m := range a.targets {
		entries = append(entries, entry{id, portableMonitor(m.target)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	data, _ := json.Marshal(struct {
		Next    uint64
		Entries []entry
	}{a.next, entries})
	return sha256.Sum256(data)
}
func decodeMonitorBackup(data []byte) (MonitorBackup, error) {
	var backup MonitorBackup
	if len(data) > maxMonitorImportBytes {
		return backup, fmt.Errorf("Choose a monitor backup smaller than 1 MiB")
	}
	// A UTF-8 BOM is common when a JSON file has been saved by Windows editors.
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&backup); err != nil {
		return backup, fmt.Errorf("This is not a valid Hostlight monitor backup. Unknown fields and malformed JSON are not accepted")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return backup, fmt.Errorf("The backup contains unexpected data after the JSON document")
	}
	if backup.Format != "hostlight-monitors" || backup.Version < 1 || backup.Version > 3 {
		return backup, fmt.Errorf("Choose a supported Hostlight monitor backup (format hostlight-monitors, version 1, 2, or 3)")
	}
	if len(backup.Monitors) == 0 || len(backup.Monitors) > maxMonitorImportCount {
		return backup, fmt.Errorf("A backup must contain 1–1,000 monitors")
	}
	return backup, nil
}
func (a *App) previewMonitorBackupLocked(backup MonitorBackup) MonitorImportPreview {
	out := MonitorImportPreview{Rows: []MonitorImportRow{}}
	existing, seen := map[string]bool{}, map[string]bool{}
	for _, m := range a.targets {
		existing[targetKey(m.target)] = true
	}
	for i, p := range backup.Monitors {
		t := p.target()
		row := MonitorImportRow{Row: i + 1, Monitor: p, Status: "ready", Detail: "Add as a new monitor"}
		// Require an explicit type rather than guessing for manually edited backups.
		var err error
		if t.Type != "icmp" && t.Type != "tcp" && t.Type != "http" {
			err = fmt.Errorf("Choose icmp, tcp, or http as the check type")
		} else if t.BodyMode != "" && backup.Version < 3 {
			err = fmt.Errorf("HTTP response-body rules require backup version 3")
		} else if t.Type == "http" && (backup.Version < 2 || t.Port != 0) {
			err = fmt.Errorf("HTTP monitors require backup version 2 and port 0 (the URL supplies its port)")
		} else if t.Type == "icmp" && t.Port != 0 {
			err = fmt.Errorf("ICMP monitors must have port 0")
		} else {
			err = validate(t)
		}
		key := targetKey(t)
		if err != nil {
			row.Status = "invalid"
			row.Detail = err.Error()
			out.Invalid++
		} else if existing[key] {
			row.Status = "skipped"
			row.Detail = "This host and check already exist; no settings will be changed"
			out.Skipped++
		} else if seen[key] {
			row.Status = "skipped"
			row.Detail = "Duplicate host and check within this backup"
			out.Skipped++
		} else {
			seen[key] = true
			out.Ready++
		}
		out.Rows = append(out.Rows, row)
	}
	return out
}
func (a *App) prepareMonitorImport(data []byte, source string) (MonitorImportPreview, error) {
	backup, err := decodeMonitorBackup(data)
	if err != nil {
		return MonitorImportPreview{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closing {
		return MonitorImportPreview{}, fmt.Errorf("Hostlight is closing")
	}
	pending := &pendingMonitorImport{token: uuid.NewString(), source: filepath.Base(source), backup: backup, signature: a.workspaceSignatureLocked(), created: time.Now()}
	a.monitorImport = pending
	out := a.previewMonitorBackupLocked(backup)
	out.Token = pending.token
	out.Source = pending.source
	return out, nil
}
func (a *App) ChooseMonitorImport() (MonitorImportPreview, error) {
	path, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Choose Hostlight monitor backup", Filters: []wruntime.FileFilter{{DisplayName: "Hostlight monitor backup", Pattern: "*.json"}}})
	if err != nil || path == "" {
		return MonitorImportPreview{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return MonitorImportPreview{}, fmt.Errorf("The backup file could not be opened: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxMonitorImportBytes+1))
	if err != nil {
		return MonitorImportPreview{}, fmt.Errorf("The backup file could not be read: %w", err)
	}
	return a.prepareMonitorImport(data, filepath.Base(path))
}
func (a *App) DismissMonitorImport(token string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.monitorImport != nil && a.monitorImport.token == token {
		a.monitorImport = nil
	}
}
func (a *App) CommitMonitorImport(token string, startMonitors bool) (MonitorImportResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	var result MonitorImportResult
	if a.closing {
		return result, fmt.Errorf("Hostlight is closing")
	}
	if a.configError != "" {
		return result, fmt.Errorf("Configuration unavailable: %s", a.configError)
	}
	pending := a.monitorImport
	if pending == nil || token == "" || pending.token != token || time.Since(pending.created) > 15*time.Minute {
		return result, fmt.Errorf("This import preview expired or was replaced. Choose the backup again")
	}
	if pending.signature != a.workspaceSignatureLocked() {
		return result, fmt.Errorf("Monitor settings changed after the preview. Choose the backup again to review the current workspace")
	}
	preview := a.previewMonitorBackupLocked(pending.backup)
	if preview.Invalid > 0 {
		return result, fmt.Errorf("Fix the invalid monitor settings in the backup and choose it again; nothing was imported")
	}
	if preview.Ready == 0 {
		return result, fmt.Errorf("There are no new monitors to import")
	}
	if uint64(preview.Ready) > math.MaxUint64-a.next {
		return result, fmt.Errorf("Monitor identifiers are exhausted")
	}
	oldNext := a.next
	added := []*monitor{}
	contexts := []context.Context{}
	for _, row := range preview.Rows {
		if row.Status != "ready" {
			continue
		}
		t := row.Monitor.target()
		a.next++
		t.ID = fmt.Sprint(a.next)
		t.State = "paused"
		t.Message = "Monitoring paused"
		if startMonitors && !row.Monitor.Paused {
			t.State = "unknown"
			t.Message = "Waiting for fresh checks"
		} else {
			result.Paused++
		}
		ctx, cancel := context.WithCancel(a.ctx)
		m := &monitor{target: t, cancel: cancel}
		a.targets[t.ID] = m
		added = append(added, m)
		contexts = append(contexts, ctx)
	}
	if err := a.persistLocked(); err != nil {
		for _, m := range added {
			m.cancel()
			delete(a.targets, m.target.ID)
		}
		a.next = oldNext
		return MonitorImportResult{}, err
	}
	for i, m := range added {
		if m.target.State != "paused" {
			go a.run(contexts[i], m, m.target)
		}
	}
	result.Added = len(added)
	result.Skipped = preview.Skipped
	a.monitorImport = nil
	return result, nil
}
func writeMonitorBackup(path string, backup MonitorBackup) error {
	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return err
	}
	if len(data)+1 > maxMonitorImportBytes {
		return fmt.Errorf("The monitor backup exceeds the 1 MiB import limit")
	}
	// Do not emit a backup that the importer would refuse to read.
	if _, err = decodeMonitorBackup(data); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".hostlight-backup-*.tmp")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if _, err = file.Write(append(data, '\n')); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = os.Rename(temp, path); err != nil {
		return fmt.Errorf("Could not save the backup: %w", err)
	}
	return nil
}
func (a *App) ExportMonitorBackup() (string, error) {
	a.mu.Lock()
	backup := a.monitorBackupLocked()
	a.mu.Unlock()
	if len(backup.Monitors) == 0 {
		return "", fmt.Errorf("Add a monitor before exporting a backup")
	}
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{Title: "Export Hostlight monitor backup", DefaultFilename: "hostlight-monitors.json", Filters: []wruntime.FileFilter{{DisplayName: "Hostlight monitor backup", Pattern: "*.json"}}})
	if err != nil || path == "" {
		return "", err
	}
	if err = a.validateMonitorExportPath(path); err != nil {
		return "", err
	}
	if err = writeMonitorBackup(path, backup); err != nil {
		return "", err
	}
	return path, nil
}
func (a *App) validateMonitorExportPath(path string) error {
	absolute, err := canonicalBackupPath(path)
	if err != nil {
		return err
	}
	protected := []string{a.configPath}
	if a.configPath != "" {
		dbPath := filepath.Join(filepath.Dir(a.configPath), "history.db")
		protected = append(protected, dbPath, dbPath+"-wal", dbPath+"-shm")
	}
	if a.history != nil {
		protected = append(protected, a.history.path, a.history.path+"-wal", a.history.path+"-shm")
	}
	for _, p := range protected {
		if p == "" {
			continue
		}
		resolved, e := canonicalBackupPath(p)
		if e == nil && strings.EqualFold(filepath.Clean(absolute), filepath.Clean(resolved)) {
			return fmt.Errorf("Choose a backup filename outside Hostlight's live configuration and database files")
		}
	}
	return nil
}
func canonicalBackupPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, e := filepath.EvalSymlinks(absolute); e == nil {
		return resolved, nil
	}
	if parent, e := filepath.EvalSymlinks(filepath.Dir(absolute)); e == nil {
		return filepath.Join(parent, filepath.Base(absolute)), nil
	}
	return filepath.Clean(absolute), nil
}
