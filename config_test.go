package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCanceledResultCannotOverwriteEditedTarget(t *testing.T) {
	a := NewApp()
	a.configPath = ""
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.startup(ctx)
	started := make(chan chan struct{}, 2)
	a.dial = func(ctx context.Context, _ string, _ time.Duration) (net.Conn, error) {
		release := make(chan struct{})
		started <- release
		select {
		case <-release:
			return nil, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err := a.SaveTarget(fixtureTarget()); err != nil {
		t.Fatal(err)
	}
	waitCall := func() chan struct{} {
		t.Helper()
		select {
		case c := <-started:
			return c
		case <-time.After(time.Second):
			t.Fatal("probe did not start")
			return nil
		}
	}
	old := waitCall()
	target := a.GetTargets()[0]
	target.Name = "Replacement"
	if err := a.SaveTarget(target); err != nil {
		t.Fatal(err)
	}
	replacement := waitCall()
	close(old)
	if got := a.GetTargets()[0]; got.Name != "Replacement" || got.Checked != "" {
		t.Fatalf("old result changed replacement: %+v", got)
	}
	if err := a.PauseTarget(target.ID); err != nil {
		t.Fatal(err)
	}
	close(replacement)
	if got := a.GetTargets()[0]; got.State != "paused" || got.Checked != "" {
		t.Fatalf("canceled result changed pause: %+v", got)
	}
}

func fixtureTarget() Target {
	return Target{Name: "Fixture", Host: "127.0.0.1", Port: 443, Interval: 5, Timeout: 1, Failures: 3, Recovery: 2}
}
func storedApp(t *testing.T, path string) *App {
	t.Helper()
	a := NewApp()
	a.configPath = path
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a.startup(ctx)
	t.Cleanup(func() { a.shutdown(context.Background()) })
	return a
}

func TestConfigurationRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings", "targets.json")
	a := storedApp(t, path)
	if err := a.SaveTarget(fixtureTarget()); err != nil {
		t.Fatal(err)
	}
	id := a.GetTargets()[0].ID
	if err := a.PauseTarget(id); err != nil {
		t.Fatal(err)
	}
	b := storedApp(t, path)
	targets := b.GetTargets()
	if len(targets) != 1 || targets[0].State != "paused" || targets[0].Name != "Fixture" {
		t.Fatalf("unexpected restored targets: %+v", targets)
	}
	edited := targets[0]
	edited.Name = "Edited"
	if err := b.SaveTarget(edited); err != nil {
		t.Fatal(err)
	}
	if b.GetTargets()[0].State != "paused" {
		t.Fatal("editing resumed paused monitor")
	}
	if err := b.ResumeTarget(id); err != nil {
		t.Fatal(err)
	}
	c := storedApp(t, path)
	if c.GetTargets()[0].State != "unknown" || c.GetTargets()[0].Checked != "" {
		t.Fatal("restored stale live state")
	}
	if err := c.SaveTarget(fixtureTarget()); err != nil {
		t.Fatal(err)
	}
	if len(c.GetTargets()) != 2 {
		t.Fatal("restored ID collided with new target")
	}
	if err := c.DeleteTarget(id); err != nil {
		t.Fatal(err)
	}
	if len(storedApp(t, path).GetTargets()) != 1 {
		t.Fatal("deletion was not persisted")
	}
}

func TestInvalidConfigurationPreserved(t *testing.T) {
	for _, data := range []string{"broken json", `{"version":99}`, `{"version":1,"targets":[{"target":{"id":"1"}}]}`} {
		t.Run(data, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "targets.json")
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			a := storedApp(t, path)
			if a.GetConfigurationStatus().Error == "" {
				t.Fatal("missing configuration error")
			}
			if err := a.SaveTarget(fixtureTarget()); err == nil {
				t.Fatal("allowed overwrite of invalid config")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != data {
				t.Fatal("original configuration changed")
			}
		})
	}
}

func TestFailedSaveRollsBack(t *testing.T) {
	a := storedApp(t, "")
	if err := a.SaveTarget(fixtureTarget()); err != nil {
		t.Fatal(err)
	}
	before := a.GetTargets()[0]
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("block"), 0600); err != nil {
		t.Fatal(err)
	}
	a.configPath = filepath.Join(blocker, "targets.json")
	edited := before
	edited.Name = "Should not stick"
	if err := a.SaveTarget(edited); err == nil {
		t.Fatal("save unexpectedly succeeded")
	}
	if a.GetTargets()[0] != before {
		t.Fatal("failed edit changed live target")
	}
	if err := a.PauseTarget(before.ID); err == nil {
		t.Fatal("pause unexpectedly succeeded")
	}
	if a.GetTargets()[0] != before {
		t.Fatal("failed pause changed live target")
	}
	if err := a.DeleteTarget(before.ID); err == nil {
		t.Fatal("delete unexpectedly succeeded")
	}
	if len(a.GetTargets()) != 1 {
		t.Fatal("failed deletion removed target")
	}
}

func TestStatusRules(t *testing.T) {
	rules := statusRules{failures: 3, recovery: 2}
	state := "unknown"
	sequence := []struct {
		success bool
		want    string
	}{{true, "unknown"}, {false, "unknown"}, {true, "unknown"}, {true, "up"}, {false, "up"}, {false, "up"}, {true, "up"}, {false, "up"}, {false, "up"}, {false, "down"}, {true, "down"}, {false, "down"}, {true, "down"}, {true, "up"}}
	for i, step := range sequence {
		state = rules.apply(state, step.success)
		if state != step.want {
			t.Fatalf("check %d: got %s want %s", i, state, step.want)
		}
	}
}
