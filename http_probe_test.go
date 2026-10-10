package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func httpTarget(endpoint string) Target {
	t := fixtureTarget()
	t.Type = "http"
	t.Host = endpoint
	t.Port = 0
	t.Timeout = 1
	t.Interval = 1
	t.Failures = 1
	t.Recovery = 1
	t.ExpectedStatus = 200
	return t
}

func TestHTTPValidation(t *testing.T) {
	for _, endpoint := range []string{"https://example.com/health?region=east", "http://127.0.0.1:8080/", "https://[::1]:443/health"} {
		if err := validate(httpTarget(endpoint)); err != nil {
			t.Fatal(endpoint, err)
		}
	}
	for _, endpoint := range []string{"example.com", "ftp://example.com/", "https://user:password@example.com/", "https://example.com/#fragment", "https://example.com:99999/", "https://example.com:abc/", "https://example.com:", "https:///health", "http://example.com/a b", "https://example.com/%zz", strings.Repeat("a", 2049)} {
		if validate(httpTarget(endpoint)) == nil {
			t.Fatal("accepted invalid URL", endpoint)
		}
	}
	for _, status := range []int{-1, 99, 600} {
		target := httpTarget("https://example.com")
		target.ExpectedStatus = status
		if validate(target) == nil {
			t.Fatal("accepted invalid status", status)
		}
	}
}

func TestHTTPStatusAndMethod(t *testing.T) {
	for _, status := range []int{200, 204, 301, 401, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.Header.Get("User-Agent") != "Hostlight/1" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					t.Error("unexpected request", r.Method, r.Header)
				}
				w.Header().Set("Location", "/other")
				w.WriteHeader(status)
			}))
			defer server.Close()
			result, err := probeHTTP(context.Background(), httpTarget(server.URL))
			if result.StatusCode != status || result.Success != (status == 200) || (err == nil) != (status == 200) || result.Latency < 0 {
				t.Fatalf("incorrect result: %+v %v", result, err)
			}
			target := httpTarget(server.URL)
			target.ExpectedStatus = status
			result, err = probeHTTP(context.Background(), target)
			if err != nil || !result.Success {
				t.Fatalf("explicit expected status rejected: %+v %v", result, err)
			}
		})
	}
}

func TestHTTPDoesNotFollowRedirects(t *testing.T) {
	var followed atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { followed.Add(1); w.WriteHeader(200) }))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 302) }))
	defer source.Close()
	result, err := probeHTTP(context.Background(), httpTarget(source.URL))
	if err == nil || result.StatusCode != 302 || followed.Load() != 0 {
		t.Fatalf("redirect policy failed: %+v %v", result, err)
	}
}

func TestHTTPRejectsOversizedResponseHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Large", strings.Repeat("a", 70<<10))
		w.WriteHeader(200)
	}))
	defer server.Close()
	result, err := probeHTTP(context.Background(), httpTarget(server.URL))
	if err == nil || result.Success {
		t.Fatal("unbounded response headers accepted")
	}
}

func TestHTTPBulkValidationAndVersionOneBoundary(t *testing.T) {
	a := portabilityApp(t)
	defaults := httpTarget("https://example.com")
	defaults.ExpectedStatus = 204
	rows := a.PreviewBulk("Health | https://EXAMPLE.com:443/\nhttps://example.com/\nhttp://example.com/health", defaults)
	if len(rows) != 3 || rows[0].Error != "" || rows[1].Error == "" || rows[2].Error != "" || rows[0].Target.Port != 0 || rows[0].Target.ExpectedStatus != 204 {
		t.Fatalf("bad HTTP bulk preview: %+v", rows)
	}
	if _, err := a.AddBulk("https://example.com\nnot-a-url", defaults); err == nil || len(a.GetTargets()) != 0 {
		t.Fatal("invalid HTTP batch was partially imported")
	}
	p := portableMonitor(defaults)
	data, err := json.Marshal(MonitorBackup{Format: "hostlight-monitors", Version: 1, Monitors: []PortableMonitor{p}})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := a.prepareMonitorImport(data, "legacy.json")
	if err != nil || preview.Invalid != 1 {
		t.Fatal("version-one HTTP backup was accepted", preview, err)
	}
}

func TestHTTPTimeoutCancellationAndTLS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	start := time.Now()
	result, err := probeHTTP(context.Background(), httpTarget(server.URL))
	if err == nil || result.Success || time.Since(start) > 3*time.Second {
		t.Fatal("timeout did not bound request", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start = time.Now()
	_, err = probeHTTP(ctx, httpTarget(server.URL))
	if err == nil || time.Since(start) > time.Second {
		t.Fatal("cancellation not honored")
	}
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer tlsServer.Close()
	result, err = probeHTTP(context.Background(), httpTarget(tlsServer.URL))
	if err == nil || result.Success {
		t.Fatal("untrusted certificate was accepted")
	}
}

func TestHTTPHeadersOnlyAndPreviewHasNoSideEffects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	a := NewApp()
	a.configPath = ""
	a.ctx = context.Background()
	result, err := a.TestHTTPRequest(httpTarget(server.URL))
	if err != nil || !result.Success || len(a.GetTargets()) != 0 || a.next != 0 {
		t.Fatalf("preview mutated workspace or read body: %+v %v", result, err)
	}
	target := httpTarget("not a URL")
	if _, err = a.TestHTTPRequest(target); err == nil {
		t.Fatal("preview accepted invalid URL")
	}
	a.closing = true
	if _, err = a.TestHTTPRequest(httpTarget(server.URL)); err == nil {
		t.Fatal("preview accepted during shutdown")
	}
}

func TestHTTPPreviewDoesNotEchoURLSecretsOnFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := "http://" + listener.Addr().String() + "/?private=do-not-echo"
	listener.Close()
	a := NewApp()
	a.ctx = context.Background()
	result, err := a.TestHTTPRequest(httpTarget(endpoint))
	if err != nil || result.Success || strings.Contains(result.Message, "do-not-echo") {
		t.Fatal("unsafe preview diagnostic", result, err)
	}
	_, probeErr := probeHTTP(context.Background(), httpTarget(endpoint))
	if probeErr == nil || strings.Contains(probeErr.Error(), "do-not-echo") {
		t.Fatal("unsafe monitor diagnostic", probeErr)
	}
}

func TestHTTPLiveEngineDispatchAndRecovery(t *testing.T) {
	var status atomic.Int32
	status.Store(200)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(int(status.Load())) }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := NewApp()
	a.configPath = ""
	a.startup(ctx)
	defer a.shutdown(context.Background())
	a.dial = func(context.Context, string, time.Duration) (net.Conn, error) { panic("HTTP dispatched as TCP") }
	if err := a.SaveTarget(httpTarget(server.URL)); err != nil {
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
	status.Store(503)
	wait("down")
	status.Store(200)
	wait("up")
	if !strings.Contains(a.GetTargets()[0].Message, "Expected status received") {
		t.Fatal("missing HTTP diagnostic")
	}
}

func TestHTTPConfigurationAndBackupRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "targets.json")
	a := storedApp(t, path)
	target := httpTarget("https://example.com/Health?region=east")
	target.ExpectedStatus = 204
	if err := a.SaveTarget(target); err != nil {
		t.Fatal(err)
	}
	b := storedApp(t, path)
	restored := b.GetTargets()[0]
	if restored.Type != "http" || restored.ExpectedStatus != 204 || restored.Port != 0 {
		t.Fatalf("lost HTTP definition: %+v", restored)
	}
	a.mu.Lock()
	backup := a.monitorBackupLocked()
	a.mu.Unlock()
	data, err := json.Marshal(backup)
	if err != nil {
		t.Fatal(err)
	}
	c := portabilityApp(t)
	preview, err := c.prepareMonitorImport(data, "http.json")
	if err != nil || preview.Ready != 1 {
		t.Fatal("HTTP preview failed", preview, err)
	}
	if _, err = c.CommitMonitorImport(preview.Token, false); err != nil {
		t.Fatal(err)
	}
	imported := c.GetTargets()[0]
	if imported.Type != "http" || imported.ExpectedStatus != 204 || imported.State != "paused" {
		t.Fatalf("bad HTTP import: %+v", imported)
	}
	if targetKey(target) != targetKey(httpTarget("https://EXAMPLE.com:443/Health?region=east")) {
		t.Fatal("URL duplicates not normalized")
	}
	if targetKey(target) == targetKey(httpTarget("https://example.com/health?region=east")) {
		t.Fatal("case-sensitive paths collapsed")
	}
}
