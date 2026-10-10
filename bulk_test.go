package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBulkPreviewAndAtomicValidation(t *testing.T) {
	a := storedApp(t, "")
	defaults := fixtureTarget()
	defaults.Type = "icmp"
	defaults.Group = "Office"
	rows := a.PreviewBulk("Gateway | 192.168.1.1\r\n\nserver.example.com\nBad | https://example.com", defaults)
	if len(rows) != 3 || rows[0].Target.Name != "Gateway" || rows[0].Target.Port != 0 || rows[1].Line != 3 || rows[2].Error == "" {
		t.Fatalf("unexpected preview: %+v", rows)
	}
	if len(a.GetTargets()) != 0 {
		t.Fatal("preview mutated workspace")
	}
	if _, err := a.AddBulk("good.example.com\nhttps://bad.example.com", defaults); err == nil {
		t.Fatal("accepted invalid batch")
	}
	if len(a.GetTargets()) != 0 {
		t.Fatal("invalid batch partly committed")
	}
	n, err := a.AddBulk("Gateway | 192.168.1.1\nserver.example.com", defaults)
	if err != nil || n != 2 {
		t.Fatalf("add: %d %v", n, err)
	}
	rows = a.PreviewBulk("SERVER.EXAMPLE.COM.\n::ffff:192.168.1.1", defaults)
	if rows[0].Error == "" || rows[1].Error == "" {
		t.Fatal("did not recognize equivalent hosts")
	}
	defaults.Type = "tcp"
	defaults.Port = 443
	if rows = a.PreviewBulk("server.example.com", defaults); rows[0].Error != "" {
		t.Fatal("distinct service rejected")
	}
}

func TestBulkLimitsAndDuplicateLines(t *testing.T) {
	a := storedApp(t, "")
	defaults := fixtureTarget()
	if _, err := a.AddBulk(" \n", defaults); err == nil {
		t.Fatal("accepted empty input")
	}
	if rows := a.PreviewBulk("one.example.com\nONE.example.com", defaults); rows[1].Error == "" {
		t.Fatal("accepted duplicate line")
	}
	var input strings.Builder
	for i := 0; i < 201; i++ {
		input.WriteString(strings.Repeat("a", i+1) + ".example.com\n")
	}
	rows := a.PreviewBulk(input.String(), defaults)
	if len(rows) != 201 || rows[200].Error == "" {
		t.Fatal("missing batch limit")
	}
}

func TestBulkPersistenceAndGroupControls(t *testing.T) {
	path := filepath.Join(t.TempDir(), "targets.json")
	a := storedApp(t, path)
	defaults := fixtureTarget()
	if _, err := a.AddBulk("one.example.com\ntwo.example.com", defaults); err != nil {
		t.Fatal(err)
	}
	targets := a.GetTargets()
	ids := []string{targets[0].ID, targets[1].ID}
	if err := a.SetTargetsPaused(ids, true); err != nil {
		t.Fatal(err)
	}
	b := storedApp(t, path)
	for _, m := range b.GetTargets() {
		if m.State != "paused" {
			t.Fatal("pause not persisted")
		}
	}
	if err := b.SetTargetsPaused([]string{ids[0], "missing"}, false); err == nil {
		t.Fatal("accepted stale selection")
	}
	if b.GetTargets()[0].State != "paused" {
		t.Fatal("stale selection partly applied")
	}
	if err := b.SetTargetsPaused([]string{ids[0], ids[0]}, false); err != nil {
		t.Fatal(err)
	}
	restored := storedApp(t, path).GetTargets()
	if restored[0].State != "unknown" || restored[1].State != "paused" {
		t.Fatal("resume changed unselected monitor")
	}
	b.mu.Lock()
	b.targets[ids[0]].target.State = "up"
	b.targets[ids[0]].target.Checked = "2026-10-09T00:00:00Z"
	b.mu.Unlock()
	if err := b.SetTargetsPaused(ids, false); err != nil {
		t.Fatal(err)
	}
	if got := b.GetTargets()[0]; got.State != "up" || got.Checked != "2026-10-09T00:00:00Z" {
		t.Fatal("resuming a mixed group reset an active monitor")
	}
}

func TestBulkWriteFailureRollback(t *testing.T) {
	a := storedApp(t, "")
	if err := a.SaveTarget(fixtureTarget()); err != nil {
		t.Fatal(err)
	}
	before := a.GetTargets()[0]
	file := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	a.configPath = filepath.Join(file, "targets.json")
	if _, err := a.AddBulk("one.example.com\ntwo.example.com", fixtureTarget()); err == nil {
		t.Fatal("write unexpectedly succeeded")
	}
	if len(a.GetTargets()) != 1 {
		t.Fatal("failed batch changed targets")
	}
	if err := a.SetTargetsPaused([]string{before.ID}, true); err == nil {
		t.Fatal("pause write unexpectedly succeeded")
	}
	if a.GetTargets()[0] != before {
		t.Fatal("failed group operation changed live target")
	}
}
