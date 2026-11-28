package main

import (
	"context"
	"testing"
	"time"
)

func TestDarwinPingLoopback(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "::1"} {
		t.Run(address, func(t *testing.T) {
			rtt, err := pingHost(context.Background(), address, 3*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if rtt < 0 {
				t.Fatalf("negative echo RTT: %f", rtt)
			}
		})
	}
}
