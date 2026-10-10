package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPBodyLiveFailureAndRecovery(t *testing.T) {
	var healthy atomic.Bool
	healthy.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if healthy.Load() {
			w.Write([]byte(`{"healthy":true}`))
		} else {
			w.Write([]byte(`{"healthy":false}`))
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := NewApp()
	a.configPath = ""
	a.startup(ctx)
	defer a.shutdown(context.Background())
	target := httpTarget(server.URL)
	target.BodyMode = "json"
	target.JSONPointer = "/healthy"
	target.JSONType = "boolean"
	target.BodyExpected = "true"
	if err := a.SaveTarget(target); err != nil {
		t.Fatal(err)
	}
	wait := func(state string) {
		t.Helper()
		deadline := time.Now().Add(4 * time.Second)
		for a.GetTargets()[0].State != state {
			if time.Now().After(deadline) {
				t.Fatalf("did not reach %s: %+v", state, a.GetTargets())
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	wait("up")
	if !strings.Contains(a.GetTargets()[0].Message, "body matched") {
		t.Fatal("body success diagnostic lost")
	}
	healthy.Store(false)
	wait("down")
	healthy.Store(true)
	wait("up")
}

func TestHTTPInactiveRulesAreNotSaved(t *testing.T) {
	a := portabilityApp(t)
	target := httpTarget("https://example.com/health")
	target.BodyExpected = "unused value"
	target.JSONPointer = "/unused"
	target.JSONType = "string"
	if err := a.SaveTarget(target); err != nil {
		t.Fatal(err)
	}
	saved := a.GetTargets()[0]
	if saved.BodyExpected != "" || saved.JSONPointer != "" || saved.JSONType != "" {
		t.Fatalf("inactive settings retained: %+v", saved)
	}
}

func TestHTTPBodyMatching(t *testing.T) {
	tests := []struct {
		name, mode, path, kind, want, body string
		match                              bool
	}{
		{"contains", "contains", "", "", "healthy", "service healthy", true},
		{"case sensitive", "contains", "", "", "Healthy", "service healthy", false},
		{"unicode", "contains", "", "", "正常", "服务正常", true},
		{"JSON string", "json", "/status", "string", "healthy", `{"status":"healthy"}`, true},
		{"boolean", "json", "/ok", "boolean", "true", `{"ok":true}`, true},
		{"strict boolean", "json", "/ok", "boolean", "true", `{"ok":"true"}`, false},
		{"null", "json", "/ok", "null", "", `{"ok":null}`, true},
		{"missing is not null", "json", "/ok", "null", "", `{}`, false},
		{"empty string", "json", "/ok", "string", "", `{"ok":""}`, true},
		{"array", "json", "/checks/0/ok", "boolean", "true", `{"checks":[{"ok":true}]}`, true},
		{"escaped keys", "json", "/a~1b/~0/0", "string", "healthy", `{"a/b":{"~":["healthy"]}}`, true},
		{"root", "json", "", "boolean", "false", `false`, true},
		{"number equivalents", "json", "/count", "number", "1e0", `{"count":1.0}`, true},
		{"precise large number", "json", "/count", "number", "9007199254740993", `{"count":9007199254740992}`, false},
		{"number is not text", "json", "/count", "string", "1", `{"count":1}`, false},
		{"unbounded exponent", "json", "/count", "number", "1", `{"count":1e999999999}`, false},
		{"array noncanonical index", "json", "/00", "number", "1", `[1]`, false},
		{"array missing", "json", "/1", "number", "1", `[1]`, false},
		{"object is not string", "json", "/status", "string", "healthy", `{"status":{}}`, false},
		{"invalid JSON", "json", "/status", "string", "healthy", `{"status":`, false},
		{"trailing JSON", "json", "/status", "string", "healthy", `{"status":"healthy"} {}`, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target := httpTarget("http://localhost")
			target.BodyMode = test.mode
			target.BodyExpected = test.want
			target.JSONPointer = test.path
			target.JSONType = test.kind
			if err := validate(target); err != nil {
				t.Fatal(err)
			}
			if got := matchHTTPBody(target, []byte(test.body)); (got == nil) != test.match {
				t.Fatalf("match=%v error=%v", test.match, got)
			}
		})
	}
}

func TestHTTPBodyRuleValidation(t *testing.T) {
	tests := []Target{
		{BodyMode: "contains"}, {BodyMode: "script"}, {BodyMode: "json", JSONPointer: "status"},
		{BodyMode: "json", JSONPointer: "/bad~2"}, {BodyMode: "json", JSONPointer: "/bad~"},
		{BodyMode: "json", JSONType: "number", BodyExpected: "NaN"}, {BodyMode: "json", JSONType: "number", BodyExpected: "1e9999999"},
		{BodyMode: "json", JSONType: "number", BodyExpected: `"1"`}, {BodyMode: "json", JSONType: "boolean", BodyExpected: "yes"},
		{BodyMode: "json", JSONType: "object"}, {BodyMode: "contains", BodyExpected: strings.Repeat("a", 4097)},
		{BodyMode: "json", JSONPointer: strings.Repeat("/a", 33)},
	}
	for _, target := range tests {
		if validateHTTPBodyRule(target) == nil {
			t.Fatalf("accepted invalid rule: %+v", target)
		}
	}
}

func TestHTTPBodyProbeLimitsTimeoutAndPrivacy(t *testing.T) {
	tests := []struct {
		name, body, mode, want, path, kind string
		match                              bool
	}{
		{"text match", "private-content-healthy", "contains", "healthy", "", "", true},
		{"text mismatch", "private-content-unhealthy", "contains", "expected-secret", "", "", false},
		{"JSON match", `{"status":"healthy","private":"response-secret"}`, "json", "healthy", "/status", "string", true},
		{"too large", strings.Repeat("a", maxHTTPBodyBytes+1), "contains", "a", "", "", false},
		{"exact boundary", strings.Repeat("a", maxHTTPBodyBytes), "contains", "a", "", "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(test.body)) }))
			defer server.Close()
			target := httpTarget(server.URL)
			target.BodyMode = test.mode
			target.BodyExpected = test.want
			target.JSONPointer = test.path
			target.JSONType = test.kind
			result, err := probeHTTP(context.Background(), target)
			if result.Success != test.match || (err == nil) != test.match {
				t.Fatalf("bad result %+v %v", result, err)
			}
			if strings.Contains(result.Message, "private-content") || strings.Contains(result.Message, "expected-secret") || strings.Contains(result.Message, "response-secret") {
				t.Fatal("response or expectation leaked")
			}
			if result.Timing == "time to response headers" {
				t.Fatal("body check incorrectly labeled header timing")
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	target := httpTarget(server.URL)
	target.BodyMode = "contains"
	target.BodyExpected = "healthy"
	start := time.Now()
	result, err := probeHTTP(context.Background(), target)
	if err == nil || result.Success || time.Since(start) > 3*time.Second || !strings.Contains(result.Message, "timed out") {
		t.Fatalf("body read not bounded: %+v %v", result, err)
	}
}

func TestHTTPBodyConfigurationAndImport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "targets.json")
	a := storedApp(t, path)
	target := httpTarget("https://service.example/health")
	target.BodyMode = "json"
	target.JSONPointer = "/checks/0/ok"
	target.JSONType = "boolean"
	target.BodyExpected = "true"
	if err := a.SaveTarget(target); err != nil {
		t.Fatal(err)
	}
	b := storedApp(t, path)
	restored := b.GetTargets()[0]
	if restored.BodyMode != "json" || restored.BodyExpected != "true" || restored.JSONPointer != target.JSONPointer || restored.JSONType != "boolean" {
		t.Fatalf("lost rule: %+v", restored)
	}
	a.mu.Lock()
	backup := a.monitorBackupLocked()
	a.mu.Unlock()
	if backup.Version != 3 {
		t.Fatal("body rules require export version 3")
	}
	data, err := json.Marshal(backup)
	if err != nil {
		t.Fatal(err)
	}
	c := portabilityApp(t)
	preview, err := c.prepareMonitorImport(data, "body.json")
	if err != nil || preview.Ready != 1 {
		t.Fatal(preview, err)
	}
	if _, err = c.CommitMonitorImport(preview.Token, false); err != nil {
		t.Fatal(err)
	}
	imported := c.GetTargets()[0]
	if imported.JSONPointer != target.JSONPointer || imported.BodyExpected != "true" || imported.State != "paused" {
		t.Fatalf("bad import: %+v", imported)
	}
	backup.Version = 2
	data, _ = json.Marshal(backup)
	preview, err = c.prepareMonitorImport(data, "old.json")
	if err != nil || preview.Invalid != 1 {
		t.Fatal("legacy backup accepted body rules", preview, err)
	}
}
