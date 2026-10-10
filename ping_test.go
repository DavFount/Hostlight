package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProbeTypeValidation(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		port       int
		valid      bool
	}{{"ping without port", "icmp", 0, true}, {"tcp without port", "tcp", 0, false}, {"tcp port", "tcp", 443, true}, {"legacy TCP", "", 443, true}, {"unknown type", "http", 80, false}} {
		t.Run(tc.name, func(t *testing.T) {
			target := fixtureTarget()
			target.Type = tc.kind
			target.Port = tc.port
			if (validate(target) == nil) != tc.valid {
				t.Fatalf("unexpected validation result for %+v", target)
			}
		})
	}
	target := fixtureTarget()
	target.Type = "icmp"
	target.Host = "fe80::1%12"
	target.Port = 0
	if err := validate(target); err != nil {
		t.Fatalf("scoped IPv6 rejected: %v", err)
	}
}

func TestLegacyTCPAndPingConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "targets.json")
	legacy := fixtureTarget()
	legacy.ID = "1"
	data, err := json.Marshal(configuration{Version: 1, Targets: []savedTarget{{Target: legacy, Paused: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	a := storedApp(t, path)
	if got := a.GetTargets()[0]; got.Type != "tcp" || got.Port != 443 {
		t.Fatalf("legacy TCP changed: %+v", got)
	}
	target := fixtureTarget()
	target.Type = "icmp"
	target.Port = 999
	if err = a.SaveTarget(target); err != nil {
		t.Fatal(err)
	}
	restored := storedApp(t, path).GetTargets()
	if len(restored) != 2 || restored[1].Type != "icmp" || restored[1].Port != 0 {
		t.Fatalf("unexpected ping definition: %+v", restored)
	}
}

func TestICMPMonitorDispatch(t *testing.T) {
	a := NewApp()
	a.configPath = ""
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.startup(ctx)
	calls := make(chan string, 1)
	a.ping = func(_ context.Context, host string, _ time.Duration) (float64, error) { calls <- host; return 2.5, nil }
	target := fixtureTarget()
	target.Type = "icmp"
	target.Port = 0
	target.Recovery = 1
	if err := a.SaveTarget(target); err != nil {
		t.Fatal(err)
	}
	select {
	case host := <-calls:
		if host != target.Host {
			t.Fatal("wrong ping host")
		}
	case <-time.After(time.Second):
		t.Fatal("ping adapter not called")
	}
	deadline := time.Now().Add(time.Second)
	for {
		got := a.GetTargets()[0]
		if got.State == "up" {
			if got.Latency != 2.5 || got.Message != "Ping reply received" {
				t.Fatalf("wrong ping result: %+v", got)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("ping result not applied")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestResolvePingLiteralAndCancellation(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1"} {
		addr, err := resolvePingAddress(context.Background(), host)
		if err != nil || addr.IP.String() != host {
			t.Fatalf("resolve %s: %v %v", host, addr, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := pingHost(ctx, "127.0.0.1", time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestPingStatusClassification(t *testing.T) {
	if pingStatusError(0) != nil {
		t.Fatal("successful reply rejected")
	}
	for _, status := range []uint32{11010, 11002, 11003, 11013} {
		err := pingStatusError(status)
		var local *probeSetupError
		if err == nil || errors.As(err, &local) {
			t.Fatalf("status %d should be remote failure", status)
		}
	}
	var local *probeSetupError
	if !errors.As(pingStatusError(11050), &local) {
		t.Fatal("general failure should be check error")
	}
}
