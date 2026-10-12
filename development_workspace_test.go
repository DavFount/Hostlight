package main

import (
	"context"
	"encoding/json"
	"hostlight/internal/loadfixture"
	"os"
	"path/filepath"
	"testing"
)

func TestDevelopmentWorkspaceIsolation(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOSTLIGHT_DEV_DATA_DIR", directory)
	a := NewApp()
	canonical, err := filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}
	if a.configError != "" || a.configPath != filepath.Join(canonical, "targets.json") || a.developmentWorkspace != canonical {
		t.Fatalf("not isolated: %+v", a)
	}
	if a.windowTitle() == "Hostlight" || a.desktopInstanceID() == desktopInstanceID {
		t.Fatal("development window/instance not separated")
	}
	b := NewApp()
	if a.desktopInstanceID() != b.desktopInstanceID() {
		t.Fatal("same workspace should share one instance")
	}
	c := &App{developmentWorkspace: filepath.Join(directory, "other")}
	if a.desktopInstanceID() == c.desktopInstanceID() {
		t.Fatal("different workspaces share instance lock")
	}
	for _, version := range []string{"0.1.0", "1.0.0"} {
		if _, err := developmentDataDirectory(directory, version); err == nil {
			t.Fatal("versioned build accepted development override")
		}
	}
	if _, err := developmentDataDirectory("relative", "early preview"); err == nil {
		t.Fatal("relative workspace accepted")
	}
	t.Setenv("HOSTLIGHT_DEV_DATA_DIR", "relative")
	invalid := NewApp()
	if invalid.configError == "" || invalid.configPath != "" {
		t.Fatal("invalid override fell back to real configuration")
	}
}

func TestDesktopFixtureConfigurationValid(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "profile")
	fixture, err := loadfixture.Start(context.Background(), directory, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	data, err := os.ReadFile(filepath.Join(directory, "targets.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg configuration
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Version != 4 || cfg.NextID != 3 || len(cfg.Targets) != 3 || cfg.Scheduling == nil || !validScheduling(*cfg.Scheduling) {
		t.Fatalf("invalid fixture: %+v", cfg)
	}
	for _, saved := range cfg.Targets {
		if err := validate(saved.Target); err != nil {
			t.Fatal(err)
		}
		if saved.Target.Host != "127.0.0.1" || saved.Target.Interval != 1 {
			t.Fatal("fixture escaped local workload")
		}
	}
}
