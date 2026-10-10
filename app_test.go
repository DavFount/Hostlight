package main

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestTCPMonitorAndPause(t *testing.T) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	go func() {
		for {
			c, e := l.Accept()
			if e != nil {
				return
			}
			c.Close()
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := NewApp()
	a.configPath = ""
	a.startup(ctx)
	target := Target{Name: "Local fixture", Host: "127.0.0.1", Port: l.Addr().(*net.TCPAddr).Port, Interval: 1, Timeout: 1, Failures: 1, Recovery: 1}
	if e := a.SaveTarget(target); e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(3 * time.Second)
	for a.GetTargets()[0].State != "up" {
		if time.Now().After(deadline) {
			t.Fatal("did not become available")
		}
		time.Sleep(10 * time.Millisecond)
	}
	id := a.GetTargets()[0].ID
	a.PauseTarget(id)
	if a.GetTargets()[0].State != "paused" {
		t.Fatal("did not pause")
	}
	a.DeleteTarget(id)
	if len(a.GetTargets()) != 0 {
		t.Fatal("did not remove")
	}
}
func TestValidation(t *testing.T) {
	if validate(Target{Name: "Invalid", Host: "https://example.com", Port: 443, Interval: 5, Timeout: 3, Failures: 3, Recovery: 2}) == nil {
		t.Fatal("accepted URL")
	}
}

func TestFailureThreshold(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := NewApp()
	a.configPath = ""
	a.startup(ctx)
	if err := a.SaveTarget(Target{Name: "Closed fixture", Host: "127.0.0.1", Port: port, Interval: 1, Timeout: 1, Failures: 2, Recovery: 1}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(4 * time.Second)
	for a.GetTargets()[0].Checked == "" {
		if time.Now().After(deadline) {
			t.Fatal("first check did not complete")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if a.GetTargets()[0].State != "unknown" {
		t.Fatal("changed state before failure threshold")
	}
	for a.GetTargets()[0].State != "down" {
		if time.Now().After(deadline) {
			t.Fatal("did not become unavailable")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
