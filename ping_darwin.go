package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var darwinPingRTT = regexp.MustCompile(`\btime[=<]([0-9]+(?:\.[0-9]+)?)\s+ms\b`)

func nativePing(ctx context.Context, address net.IPAddr, _ time.Duration) (float64, error) {
	command := "/sbin/ping6"
	if address.IP.To4() != nil {
		command = "/sbin/ping"
	}

	// pingHost has already resolved the address and set the deadline. Pass a
	// numeric destination directly and measure the echo RTT reported by ping.
	cmd := exec.CommandContext(ctx, command, "-n", "-c", "1", address.String())
	cmd.Env = append(cmd.Environ(), "LC_ALL=C")
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	if err != nil {
		var exit *exec.ExitError
		message := strings.ToLower(string(output))
		switch {
		case errors.As(err, &exit) && exit.ExitCode() == 2:
			return 0, fmt.Errorf("Ping timed out — no echo reply received")
		case strings.Contains(message, "unreachable") || strings.Contains(message, "no route to host") ||
			strings.Contains(message, "host is down"):
			return 0, fmt.Errorf("Destination unreachable")
		default:
			return 0, &probeSetupError{"macOS could not send the ICMP echo request. Check local network access and the network adapter."}
		}
	}
	match := darwinPingRTT.FindSubmatch(output)
	if match == nil {
		return 0, &probeSetupError{"macOS ping returned an unreadable echo time"}
	}
	return strconv.ParseFloat(string(match[1]), 64)
}
