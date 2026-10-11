package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Configuration contains definitions only. Live results are never restored as evidence.
type configuration struct {
	Scheduling  *SchedulingSettings `json:"scheduling,omitempty"`
	CloseToTray bool                `json:"closeToTray,omitempty"`
	Version     int                 `json:"version"`
	NextID      uint64              `json:"nextId,omitempty"`
	Targets     []savedTarget       `json:"targets"`
}
type savedTarget struct {
	Target Target `json:"target"`
	Paused bool   `json:"paused"`
}

func (a *App) persistLocked() error {
	if a.configError != "" {
		return fmt.Errorf("Configuration unavailable: %s", a.configError)
	}
	if a.configPath == "" {
		return nil
	} // In-memory fixtures.
	cfg := configuration{Version: 4, NextID: a.next, CloseToTray: a.desktop.closeToTray, Targets: make([]savedTarget, 0, len(a.targets))}
	cfg.Scheduling = &a.scheduling
	for _, m := range a.targets {
		t := m.target
		paused := t.State == "paused"
		t.State = ""
		t.Message = ""
		t.Checked = ""
		t.Latency = 0
		t.CertificateExpires = ""
		cfg.Targets = append(cfg.Targets, savedTarget{t, paused})
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(a.configPath)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("Cannot create settings folder: %w", err)
	}
	f, err := os.CreateTemp(dir, ".targets-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
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
	if err = os.Rename(name, a.configPath); err != nil {
		return fmt.Errorf("Cannot save targets: %w", err)
	}
	return nil
}

func (a *App) loadConfiguration() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.configPath == "" || a.configError != "" {
		return
	}
	data, err := os.ReadFile(a.configPath)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		a.configError = err.Error()
		return
	}
	var cfg configuration
	if err = json.Unmarshal(data, &cfg); err != nil {
		a.configError = "Saved targets could not be read. The original file has been preserved."
		return
	}
	if cfg.Version < 1 || cfg.Version > 4 {
		a.configError = "This configuration version is not supported. The original file has been preserved."
		return
	}
	// Validate the entire file before restoring any monitors.
	if cfg.Scheduling != nil && !validScheduling(*cfg.Scheduling) {
		a.configError = "Saved scheduling settings are invalid. The original file has been preserved."
		return
	}
	if cfg.NextID > a.next {
		a.next = cfg.NextID
	}
	seen := map[string]bool{}
	for _, s := range cfg.Targets {
		id, e := strconv.ParseUint(s.Target.ID, 10, 64)
		if e != nil || id == 0 || seen[s.Target.ID] || validate(s.Target) != nil {
			a.configError = "Saved targets contain invalid or duplicate entries. The original file has been preserved."
			return
		}
		seen[s.Target.ID] = true
		if id > a.next {
			a.next = id
		}
	}
	a.desktop.closeToTray = cfg.CloseToTray
	if cfg.Scheduling != nil {
		a.scheduling = *cfg.Scheduling
		a.activeScheduling = *cfg.Scheduling
		a.slots = make(chan struct{}, cfg.Scheduling.Concurrency)
	}
	for _, s := range cfg.Targets {
		t := s.Target
		if t.Type == "" {
			t.Type = "tcp"
		}
		if t.Type == "http" && t.ExpectedStatus == 0 {
			t.ExpectedStatus = 200
		}
		t.State = "unknown"
		t.Message = "Waiting for fresh checks"
		t.Checked = ""
		t.CertificateExpires = ""
		t.Latency = 0
		if s.Paused {
			t.State = "paused"
			t.Message = "Monitoring paused"
		}
		ctx, cancel := context.WithCancel(a.ctx)
		m := &monitor{target: t, cancel: cancel}
		a.targets[t.ID] = m
		if !s.Paused {
			go a.run(ctx, m, t)
		}
	}
}
