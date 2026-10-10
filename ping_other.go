//go:build !windows

package main

import (
	"context"
	"net"
	"time"
)

func nativePing(context.Context, net.IPAddr, time.Duration) (float64, error) {
	return 0, &probeSetupError{"ICMP is currently available on Windows. Use a TCP service check on this platform."}
}
