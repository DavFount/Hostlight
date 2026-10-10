package main

import (
	"context"
	"fmt"
	"net"
	"time"
)

type probeSetupError struct{ message string }

func (e *probeSetupError) Error() string { return e.message }

// Resolve once per check, prefer IPv4 for dual-stack names, and use IPv6 for
// IPv6-only targets. The timeout includes resolution and the echo request.
func pingHost(ctx context.Context, host string, timeout time.Duration) (float64, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	address, err := resolvePingAddress(ctx, host)
	if err != nil {
		return 0, err
	}
	if err = ctx.Err(); err != nil {
		return 0, err
	}
	deadline, _ := ctx.Deadline()
	remaining := time.Until(deadline)
	return nativePing(ctx, address, remaining)
}
func resolvePingAddress(ctx context.Context, host string) (net.IPAddr, error) {
	if ip := net.ParseIP(host); ip != nil {
		return net.IPAddr{IP: ip}, nil
	}
	// Scoped IPv6 literals such as fe80::1%12 are resolved by the Go resolver.
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return net.IPAddr{}, err
	}
	if len(addresses) == 0 {
		return net.IPAddr{}, &net.DNSError{Err: "No addresses found", Name: host, IsNotFound: true}
	}
	for _, a := range addresses {
		if a.IP.To4() != nil {
			return a, nil
		}
	}
	return addresses[0], nil
}
func pingStatusError(status uint32) error {
	switch status {
	case 0:
		return nil
	case 11010:
		return fmt.Errorf("Ping timed out — no echo reply received")
	case 11002, 11003, 11004, 11005:
		return fmt.Errorf("Destination unreachable (ICMP status %d)", status)
	case 11013:
		return fmt.Errorf("Ping TTL expired")
	default:
		return &probeSetupError{fmt.Sprintf("Ping could not complete (Windows status %d). Check the local network adapter and routing.", status)}
	}
}
