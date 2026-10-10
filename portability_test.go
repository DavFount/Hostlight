package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func portableFixture(host string) PortableMonitor {
	return PortableMonitor{Name: "Office gateway", Host: host, Type: "icmp", Port: 0, Group: "Office", Interval: 5, Timeout: 3, Failures: 3, Recovery: 2}
}
func backupBytes(t *testing.T, monitors ...PortableMonitor) []byte {
	t.Helper()
	data, err := json.Marshal(MonitorBackup{Format: "hostlight-monitors", Version: 1, Monitors: monitors})
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func portabilityApp(t *testing.T) *App {
	t.Helper()
	a := NewApp()
	a.configPath = ""
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a.ctx = ctx
	t.Cleanup(func() { a.shutdown(context.Background()) })
	return a
}
func TestMonitorBackupAllowlistAndRoundTrip(t *testing.T) {
	a := portabilityApp(t)
	target := portableFixture("gateway.example").target()
	target.ID = "77"
	target.State = "paused"
	target.Checked = time.Now().Format(time.RFC3339)
	target.Latency = 123
	target.Message = "do-not-export-secret"
	a.next = 77
	a.targets[target.ID] = &monitor{target: target, cancel: func() {}}
	backup := a.monitorBackupLocked()
	path := filepath.Join(t.TempDir(), "backup.json")
	if err := writeMonitorBackup(path, backup); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"id":`, `"state":`, `"latency":`, `"checked":`, `"message":`, `"nextId":`, `"secret":`, "do-not-export-secret"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("backup exposed runtime/private field %s", forbidden)
		}
	}
	restored := portabilityApp(t)
	preview, err := restored.prepareMonitorImport(data, "backup.json")
	if err != nil || preview.Ready != 1 {
		t.Fatalf("bad roundtrip preview: %+v %v", preview, err)
	}
	result, err := restored.CommitMonitorImport(preview.Token, true)
	if err != nil || result.Added != 1 || result.Paused != 1 {
		t.Fatalf("bad roundtrip commit: %+v %v", result, err)
	}
	got := restored.GetTargets()[0]
	if got.ID == "77" || got.State != "paused" || got.Name != target.Name || got.Host != target.Host || got.Interval != 5 || got.Checked != "" || got.Latency != 0 {
		t.Fatalf("bad restored monitor: %+v", got)
	}
}
func TestMonitorImportDeduplicatesAndNeverReplaces(t *testing.T) {
	a := portabilityApp(t)
	target := portableFixture("existing.example").target()
	if err := a.SaveTarget(target); err != nil {
		t.Fatal(err)
	}
	duplicate := portableFixture("EXISTING.EXAMPLE.")
	duplicate.Name = "Do not rename existing monitor"
	fresh := portableFixture("new.example")
	repeated := fresh
	repeated.Host = "NEW.EXAMPLE."
	tcp := fresh
	tcp.Type = "tcp"
	tcp.Port = 443
	preview, err := a.prepareMonitorImport(backupBytes(t, duplicate, fresh, repeated, tcp), "backup.json")
	if err != nil || preview.Ready != 2 || preview.Skipped != 2 || preview.Invalid != 0 {
		t.Fatalf("bad duplicate preview: %+v %v", preview, err)
	}
	result, err := a.CommitMonitorImport(preview.Token, false)
	if err != nil || result.Added != 2 || result.Skipped != 2 || result.Paused != 2 {
		t.Fatalf("bad import result: %+v %v", result, err)
	}
	targets := a.GetTargets()
	if len(targets) != 3 || targets[0].Name != target.Name {
		t.Fatal("existing monitor was changed")
	}
	if _, err = a.CommitMonitorImport(preview.Token, false); err == nil {
		t.Fatal("consumed token reused")
	}
}
func TestMonitorImportInvalidEntryBlocksEntireBatch(t *testing.T) {
	a := portabilityApp(t)
	invalid := portableFixture("bad.example")
	invalid.Interval = 0
	preview, err := a.prepareMonitorImport(backupBytes(t, portableFixture("good.example"), invalid), "backup.json")
	if err != nil || preview.Invalid != 1 {
		t.Fatal("invalid entry not shown in preview")
	}
	if _, err = a.CommitMonitorImport(preview.Token, false); err == nil || len(a.targets) != 0 || a.next != 0 {
		t.Fatal("invalid batch partially imported")
	}
}
func TestMonitorImportRollbackAndRetry(t *testing.T) {
	a := portabilityApp(t)
	a.next = 100
	preview, err := a.prepareMonitorImport(backupBytes(t, portableFixture("one.example"), portableFixture("two.example")), "backup.json")
	if err != nil {
		t.Fatal(err)
	}
	// A directory cannot be replaced by the atomic settings file rename.
	a.configPath = filepath.Join(t.TempDir(), "settings-directory")
	if err = os.Mkdir(a.configPath, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = a.CommitMonitorImport(preview.Token, false); err == nil || len(a.targets) != 0 || a.next != 100 {
		t.Fatal("failed settings write left partial targets or consumed identities")
	}
	a.configPath = ""
	if _, err = a.CommitMonitorImport(preview.Token, false); err != nil {
		t.Fatal(err)
	}
	if a.GetTargets()[0].ID != "101" || len(a.targets) != 2 {
		t.Fatal("retry failed to restore atomic import")
	}
}
func TestMonitorImportRejectsStaleExpiredAndDismissedPreviews(t *testing.T) {
	a := portabilityApp(t)
	data := backupBytes(t, portableFixture("imported.example"))
	preview, err := a.prepareMonitorImport(data, "backup.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = a.SaveTarget(portableFixture("concurrent.example").target()); err != nil {
		t.Fatal(err)
	}
	if _, err = a.CommitMonitorImport(preview.Token, false); err == nil || len(a.targets) != 1 {
		t.Fatal("stale preview committed")
	}
	preview, err = a.prepareMonitorImport(data, "backup.json")
	if err != nil {
		t.Fatal(err)
	}
	a.monitorImport.created = time.Now().Add(-16 * time.Minute)
	if _, err = a.CommitMonitorImport(preview.Token, false); err == nil {
		t.Fatal("expired preview committed")
	}
	preview, err = a.prepareMonitorImport(data, "backup.json")
	if err != nil {
		t.Fatal(err)
	}
	a.DismissMonitorImport(preview.Token)
	if _, err = a.CommitMonitorImport(preview.Token, false); err == nil {
		t.Fatal("dismissed preview committed")
	}
}
func TestMonitorImportPausePolicyAndLiveChanges(t *testing.T) {
	a := portabilityApp(t)
	existing := portableFixture("existing.example").target()
	if err := a.SaveTarget(existing); err != nil {
		t.Fatal(err)
	}
	paused := portableFixture("paused.example")
	paused.Paused = true
	preview, err := a.prepareMonitorImport(backupBytes(t, portableFixture("active.example"), paused), "backup.json")
	if err != nil {
		t.Fatal(err)
	}
	// Background check results must not invalidate a settings-only review.
	a.targets["1"].target.State = "up"
	a.targets["1"].target.Latency = 12
	a.targets["1"].target.Checked = time.Now().Format(time.RFC3339)
	result, err := a.CommitMonitorImport(preview.Token, true)
	if err != nil || result.Paused != 1 {
		t.Fatalf("live result invalidated preview: %+v %v", result, err)
	}
	if a.targets["2"].target.State != "unknown" || a.targets["3"].target.State != "paused" {
		t.Fatal("source pause preference was not preserved")
	}
}
func TestMonitorBackupStrictParsingAndLimits(t *testing.T) {
	valid := backupBytes(t, portableFixture("example.com"))
	for _, data := range [][]byte{[]byte("null"), []byte(`{"format":"hostlight-monitors","version":2,"monitors":[]}`), []byte(`{"format":"hostlight-monitors","version":1,"webhook":"secret","monitors":[]}`), append(append([]byte{}, valid...), []byte(" {}")...), []byte(strings.Repeat(" ", maxMonitorImportBytes+1))} {
		if _, err := decodeMonitorBackup(data); err == nil {
			t.Fatalf("invalid backup accepted: %.100s", data)
		}
	}
	if _, err := decodeMonitorBackup(append([]byte{0xef, 0xbb, 0xbf}, valid...)); err != nil {
		t.Fatal("Windows UTF-8 BOM rejected")
	}
	excessive := make([]PortableMonitor, maxMonitorImportCount+1)
	if _, err := decodeMonitorBackup(backupBytes(t, excessive...)); err == nil {
		t.Fatal("excessive monitor count accepted")
	}
}
func TestMonitorBackupProtectedPathsAndFailedExport(t *testing.T) {
	a := portabilityApp(t)
	a.configPath = filepath.Join(t.TempDir(), "targets.json")
	if a.validateMonitorExportPath(a.configPath) == nil {
		t.Fatal("live configuration can be overwritten by export")
	}
	if a.validateMonitorExportPath(filepath.Join(filepath.Dir(a.configPath), "history.db")) == nil {
		t.Fatal("live database can be overwritten when history is unavailable")
	}
	path := filepath.Join(t.TempDir(), "preserve.json")
	if err := os.WriteFile(path, []byte("existing content"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeMonitorBackup(path, MonitorBackup{}); err == nil {
		t.Fatal("invalid backup exported")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "existing content" {
		t.Fatal("failed export destroyed previous file")
	}
}
func TestMonitorImportPersistsAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "targets.json")
	a := storedApp(t, path)
	preview, err := a.prepareMonitorImport(backupBytes(t, portableFixture("persisted.example")), "backup.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.CommitMonitorImport(preview.Token, false); err != nil {
		t.Fatal(err)
	}
	a.shutdown(context.Background())
	b := storedApp(t, path)
	targets := b.GetTargets()
	if len(targets) != 1 || targets[0].Host != "persisted.example" || targets[0].State != "paused" {
		t.Fatal("import did not persist safely")
	}
}
