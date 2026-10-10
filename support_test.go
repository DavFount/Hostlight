package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSupportReportSanitization(t *testing.T) {
	a := NewApp()
	a.configPath = `C:\Users\PRIVATE-USER\Hostlight\targets.json`
	a.configError = "PRIVATE-CONFIG-ERROR"
	a.historyError = "PRIVATE-HISTORY-ERROR"
	a.targets["private-id"] = &monitor{target: Target{Name: "PRIVATE-NAME", Host: "https://PRIVATE-HOST/?token=PRIVATE-SECRET", Group: "PRIVATE-GROUP", Message: "PRIVATE-DIAGNOSTIC", State: "down", Type: "http", BodyExpected: "PRIVATE-BODY"}}
	a.targets["other"] = &monitor{target: Target{State: "PRIVATE-STATE", Type: "PRIVATE-TYPE"}}
	report := a.GetSupportReport()
	if report.MonitorCount != 2 || report.States["down"] != 1 || report.States["other"] != 1 || report.Checks["http"] != 1 || report.ConfigurationHealth != "error" || report.HistoryHealth != "unavailable" {
		t.Fatalf("summary: %+v", report)
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "PRIVATE") {
		t.Fatalf("private data leaked: %s", data)
	}
	path := filepath.Join(t.TempDir(), "diagnostics.json")
	if err = writeSupportReport(path, report); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "PRIVATE") {
		t.Fatal("private data in exported file")
	}
	var decoded SupportReport
	if err = json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Format != "hostlight-diagnostics" || decoded.Version != 1 {
		t.Fatal("report format")
	}
}

func TestSupportReportStorageHealth(t *testing.T) {
	s := historyFixture(t)
	a := &App{history: s, targets: map[string]*monitor{}}
	r := a.GetSupportReport()
	if r.HistoryHealth != "ready" || r.StorageSummary != "ready" {
		t.Fatalf("storage summary: %+v", r)
	}
	s.setFailure(os.ErrPermission)
	r = a.GetSupportReport()
	if r.HistoryHealth != "error" {
		t.Fatal("missing storage failure")
	}
	s.db.Close()
	r = a.GetSupportReport()
	if r.StorageSummary != "unavailable" {
		t.Fatal("failed reads claimed success")
	}
}
