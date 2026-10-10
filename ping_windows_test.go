package main

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestWindowsICMPLoopback(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1"} {
		t.Run(host, func(t *testing.T) {
			latency, err := nativePing(context.Background(), net.IPAddr{IP: net.ParseIP(host)}, time.Second)
			if err != nil {
				t.Fatalf("native loopback ping: %v", err)
			}
			if latency < 0 {
				t.Fatal("negative latency")
			}
		})
	}
}
