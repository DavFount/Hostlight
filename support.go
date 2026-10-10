package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Deliberately separate from Target/configuration: never serialize user strings,
// paths, raw errors, destinations, delivery payloads, or environment variables.
type SupportReport struct {
	Format               string         `json:"format"`
	Version              int            `json:"version"`
	Created              string         `json:"created"`
	Release              string         `json:"release"`
	OS                   string         `json:"os"`
	Architecture         string         `json:"architecture"`
	GoVersion            string         `json:"goVersion"`
	WailsVersion         string         `json:"wailsVersion"`
	ConfigurationHealth  string         `json:"configurationHealth"`
	HistoryHealth        string         `json:"historyHealth"`
	StorageSummary       string         `json:"storageSummary"`
	MonitorCount         int            `json:"monitorCount"`
	States               map[string]int `json:"states"`
	Checks               map[string]int `json:"checks"`
	OpenIncidents        int            `json:"openIncidents"`
	PendingDeliveries    int            `json:"pendingDeliveries"`
	ProbeConcurrency     int            `json:"probeConcurrency"`
	HistoryQueueLength   int            `json:"historyQueueLength"`
	HistoryQueueCapacity int            `json:"historyQueueCapacity"`
}

func (a *App) GetSupportReport() SupportReport {
	r := SupportReport{Format: "hostlight-diagnostics", Version: 1, Created: time.Now().UTC().Format(time.RFC3339), Release: "early preview", OS: runtime.GOOS, Architecture: runtime.GOARCH, GoVersion: runtime.Version(), WailsVersion: "unavailable", ConfigurationHealth: "ready", HistoryHealth: "unavailable", StorageSummary: "unavailable", States: map[string]int{}, Checks: map[string]int{}}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, d := range info.Deps {
			if d.Path == "github.com/wailsapp/wails/v2" {
				r.WailsVersion = d.Version
			}
		}
	}
	a.mu.Lock()
	r.ProbeConcurrency = cap(a.slots)
	if a.configPath == "" {
		r.ConfigurationHealth = "not persisted"
	}
	if a.configError != "" {
		r.ConfigurationHealth = "error"
	}
	r.MonitorCount = len(a.targets)
	for _, m := range a.targets {
		switch m.target.State {
		case "up", "down", "unknown", "paused", "error":
			r.States[m.target.State]++
		default:
			r.States["other"]++
		}
		switch m.target.Type {
		case "icmp", "tcp", "http":
			r.Checks[m.target.Type]++
		default:
			r.Checks["other"]++
		}
	}
	s := a.history
	a.mu.Unlock()
	if s == nil {
		return r
	}
	r.HistoryHealth = "ready"
	if s.errorMessage() != "" {
		r.HistoryHealth = "error"
	}
	r.HistoryQueueLength = len(s.events)
	r.HistoryQueueCapacity = cap(s.events)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM incidents WHERE status='open'").Scan(&r.OpenIncidents); err != nil {
		r.StorageSummary = "unavailable"
		return r
	}
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM deliveries WHERE status='pending'").Scan(&r.PendingDeliveries); err != nil {
		r.StorageSummary = "unavailable"
		return r
	}
	r.StorageSummary = "ready"
	return r
}

func writeSupportReport(path string, report SupportReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".hostlight-diagnostics-*.tmp")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, err = f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

func (a *App) ExportSupportReport() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("Open Hostlight through Wails to export diagnostics")
	}
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{Title: "Export sanitized Hostlight diagnostics", DefaultFilename: "hostlight-diagnostics.json", Filters: []wruntime.FileFilter{{DisplayName: "JSON diagnostics", Pattern: "*.json"}}})
	if err != nil || path == "" {
		return "", err
	}
	if err = a.validateMonitorExportPath(path); err != nil {
		return "", err
	}
	if err = writeSupportReport(path, a.GetSupportReport()); err != nil {
		return "", err
	}
	return path, nil
}
