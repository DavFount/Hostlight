package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// Opt-in real TCP/storage workload, excluded from ordinary tests and CI timing.
// All listeners bind loopback; all application files live in t.TempDir().
func TestWindowsLoad200(t *testing.T) {
	value := os.Getenv("HOSTLIGHT_LOAD_SECONDS")
	if value == "" {
		t.Skip("opt in with scripts/measure-load.ps1")
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds < 60 || seconds > 86400 {
		t.Fatal("HOSTLIGHT_LOAD_SECONDS must be 60–86400")
	}
	duration := time.Duration(seconds) * time.Second
	listeners := make([]net.Listener, 200)
	addresses := make([]string, 200)
	listen := func(index int, address string) {
		t.Helper()
		listener, err := net.Listen("tcp4", address)
		if err != nil {
			t.Fatal(err)
		}
		listeners[index] = listener
		addresses[index] = listener.Addr().String()
		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				conn.Close()
			}
		}()
	}
	t.Cleanup(func() {
		for _, listener := range listeners {
			if listener != nil {
				listener.Close()
			}
		}
	})
	cfg := configuration{Version: 4, NextID: 200, Scheduling: &SchedulingSettings{Concurrency: 32, SpreadMilliseconds: 250}}
	for i := range listeners {
		listen(i, "127.0.0.1:0")
		cfg.Targets = append(cfg.Targets, savedTarget{Target: Target{ID: strconv.Itoa(i + 1), Name: fmt.Sprintf("Loopback %03d", i+1), Host: "127.0.0.1", Type: "tcp", Port: listeners[i].Addr().(*net.TCPAddr).Port, Interval: 1, Timeout: 1, Failures: 2, Recovery: 2}})
	}
	a := NewApp()
	var cadenceMu sync.Mutex
	previousFinish := map[string]time.Time{}
	var measureCadence bool
	var overheadMax time.Duration
	var overheadCount uint64
	limits := []time.Duration{time.Millisecond / 10, time.Millisecond / 2, time.Millisecond, 2 * time.Millisecond, 5 * time.Millisecond, 10 * time.Millisecond, 25 * time.Millisecond, 50 * time.Millisecond, 100 * time.Millisecond, 500 * time.Millisecond}
	histogram := make([]uint64, len(limits)+1)
	dial := a.dial
	a.dial = func(ctx context.Context, address string, timeout time.Duration) (net.Conn, error) {
		start := time.Now()
		cadenceMu.Lock()
		if previous, ok := previousFinish[address]; ok && measureCadence {
			overhead := start.Sub(previous) - time.Second
			if overhead < 0 {
				overhead = 0
			}
			if overhead > overheadMax {
				overheadMax = overhead
			}
			index := sort.Search(len(limits), func(i int) bool { return overhead <= limits[i] })
			histogram[index]++
			overheadCount++
		}
		cadenceMu.Unlock()
		conn, err := dial(ctx, address, timeout)
		cadenceMu.Lock()
		previousFinish[address] = time.Now()
		cadenceMu.Unlock()
		return conn, err
	}
	a.configPath = filepath.Join(t.TempDir(), "targets.json")
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.configPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.startup(ctx)
	defer a.shutdown(context.Background())
	if a.history == nil {
		t.Fatal(a.historyError)
	}
	waitUntil := func(description string, condition func() bool) {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for !condition() {
			if time.Now().After(deadline) {
				t.Fatal(description)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	waitUntil("monitors did not warm up", func() bool {
		for _, target := range a.GetTargets() {
			if target.State != "up" {
				return false
			}
		}
		return len(a.GetTargets()) == 200
	})
	var webhookRequests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookRequests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	if err := a.SaveWebhookSettings(WebhookSettings{Enabled: true, URL: server.URL, AllowHTTP: true}); err != nil {
		t.Fatal(err)
	}
	baseline := map[string]uint64{}
	for _, s := range a.GetCheckStatistics() {
		baseline[s.ID] = s.SessionChecks
	}
	cpu := func() time.Duration {
		t.Helper()
		handle, err := syscall.GetCurrentProcess()
		if err != nil {
			t.Fatal(err)
		}
		var created, exited, kernel, user syscall.Filetime
		if err := syscall.GetProcessTimes(handle, &created, &exited, &kernel, &user); err != nil {
			t.Fatal(err)
		}
		ticks := (uint64(kernel.HighDateTime)<<32 | uint64(kernel.LowDateTime)) + (uint64(user.HighDateTime)<<32 | uint64(user.LowDateTime))
		return time.Duration(ticks * 100)
	}
	cpuStart := cpu()
	started := time.Now()
	cadenceMu.Lock()
	measureCadence = true
	cadenceMu.Unlock()
	peakQueue, peakGoroutines := 0, 0
	var peakHeap uint64
	var snapshotMS []float64
	closed, reopened := false, false
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for time.Since(started) < duration {
		<-ticker.C
		elapsed := time.Since(started)
		if !closed && elapsed >= duration/3 {
			for i := 0; i < 20; i++ {
				listeners[i].Close()
			}
			closed = true
		}
		if closed && !reopened && elapsed >= 2*duration/3 {
			for i := 0; i < 20; i++ {
				listen(i, addresses[i])
			}
			reopened = true
		}
		before := time.Now()
		a.GetTargets()
		a.GetCheckStatistics()
		a.GetConfigurationStatus()
		if _, err := a.GetAlertSilences(); err != nil {
			t.Fatal(err)
		}
		if _, err := a.GetActiveOutages(); err != nil {
			t.Fatal(err)
		}
		if _, err := a.GetRecentProbeSamples("1"); err != nil {
			t.Fatal(err)
		}
		snapshotMS = append(snapshotMS, float64(time.Since(before).Microseconds())/1000)
		if len(a.history.events) > peakQueue {
			peakQueue = len(a.history.events)
		}
		if runtime.NumGoroutine() > peakGoroutines {
			peakGoroutines = runtime.NumGoroutine()
		}
		var memory runtime.MemStats
		runtime.ReadMemStats(&memory)
		if memory.HeapAlloc > peakHeap {
			peakHeap = memory.HeapAlloc
		}
		if message := a.history.errorMessage(); message != "" {
			t.Fatal(message)
		}
	}
	wallSeconds := time.Since(started).Seconds()
	cpuSeconds := (cpu() - cpuStart).Seconds()
	cadenceMu.Lock()
	measureCadence = false
	p95Overhead := overheadMax
	var cumulative uint64
	for i, count := range histogram {
		cumulative += count
		if overheadCount > 0 && cumulative*100 >= overheadCount*95 {
			if i < len(limits) {
				p95Overhead = limits[i]
			}
			break
		}
	}
	cadenceMu.Unlock()
	var completed uint64
	minimum, maximum := uint64(^uint64(0)), uint64(0)
	for _, s := range a.GetCheckStatistics() {
		count := s.SessionChecks - baseline[s.ID]
		completed += count
		if count < minimum {
			minimum = count
		}
		if count > maximum {
			maximum = count
		}
	}
	waitUntil("monitors did not recover", func() bool {
		for _, target := range a.GetTargets() {
			if target.State != "up" {
				return false
			}
		}
		return true
	})
	waitUntil("webhooks did not drain", func() bool {
		var count int
		err := a.history.db.QueryRow("SELECT count(*) FROM deliveries WHERE status='pending'").Scan(&count)
		return err == nil && count == 0 && webhookRequests.Load() >= 40
	})
	// Freeze producers before reconciling persisted totals. Counting minute rows
	// alone cannot detect missing observations within an otherwise present row.
	var ids []string
	for _, target := range a.GetTargets() {
		ids = append(ids, target.ID)
	}
	if err := a.SetTargetsPaused(ids, true); err != nil {
		t.Fatal(err)
	}
	statistics := a.GetCheckStatistics()
	waitUntil("saved probe counts did not match session totals", func() bool {
		for _, stats := range statistics {
			var checks, failed uint64
			err := a.history.db.QueryRow("SELECT COALESCE(SUM(checks),0), COALESCE(SUM(failed),0) FROM probe_minutes WHERE target_id=?", stats.ID).Scan(&checks, &failed)
			if err != nil || checks != stats.SessionChecks || failed != stats.SessionFailed {
				return false
			}
		}
		return true
	})
	var incidents, deliveries, buckets int
	if err := a.history.db.QueryRow("SELECT count(*) FROM incidents WHERE status='resolved'").Scan(&incidents); err != nil {
		t.Fatal(err)
	}
	if err := a.history.db.QueryRow("SELECT count(*) FROM deliveries WHERE status='delivered'").Scan(&deliveries); err != nil {
		t.Fatal(err)
	}
	if err := a.history.db.QueryRow("SELECT count(*) FROM probe_minutes").Scan(&buckets); err != nil {
		t.Fatal(err)
	}
	if buckets > probeMinuteLimit {
		t.Fatal("probe summary cap exceeded")
	}
	if incidents != 20 || deliveries != 40 {
		t.Fatalf("unexpected transitions: incidents=%d delivered=%d", incidents, deliveries)
	}
	if minimum < uint64(seconds/2) {
		t.Fatalf("a monitor completed fewer than half the nominal checks: %d", minimum)
	}
	sort.Float64s(snapshotMS)
	report := map[string]any{
		"scope":                              "backend TCP loopback, SQLite and webhook workload; excludes GUI/WebView and remote networks",
		"os":                                 runtime.GOOS,
		"architecture":                       runtime.GOARCH,
		"go":                                 runtime.Version(),
		"cpu":                                os.Getenv("PROCESSOR_IDENTIFIER"),
		"logicalCPUs":                        runtime.NumCPU(),
		"goMaxProcs":                         runtime.GOMAXPROCS(0),
		"intervalOverheadP95UpperBoundMS":    float64(p95Overhead.Microseconds()) / 1000,
		"intervalOverheadMaxMS":              float64(overheadMax.Microseconds()) / 1000,
		"monitors":                           200,
		"intervalSeconds":                    1,
		"concurrency":                        32,
		"spreadMilliseconds":                 250,
		"durationSeconds":                    wallSeconds,
		"completedChecks":                    completed,
		"checksPerSecond":                    float64(completed) / wallSeconds,
		"minChecksPerMonitor":                minimum,
		"maxChecksPerMonitor":                maximum,
		"processCPUSeconds":                  cpuSeconds,
		"cpuPercentOfOneCore":                100 * cpuSeconds / wallSeconds,
		"peakGoHeapBytes":                    peakHeap,
		"peakGoroutines":                     peakGoroutines,
		"peakHistoryQueueAtOneSecondSamples": peakQueue,
		"historyQueueCapacity":               cap(a.history.events),
		"snapshotBatchP95MS":                 snapshotMS[(len(snapshotMS)-1)*95/100],
		"resolvedIncidents":                  incidents,
		"deliveredWebhookDeliveries":         deliveries,
		"storedMinuteSummaries":              buckets,
		"persistedCountsMatchSessions":       true,
	}
	result, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("HOSTLIGHT_LOAD_REPORT " + string(result))
}
