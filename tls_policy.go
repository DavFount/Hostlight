package main

import (
	"fmt"
	"time"
)

func checkCertificateValidity(t Target, expires string, now time.Time) error {
	if t.TLSMinDays == 0 {
		return nil
	}
	deadline, err := time.Parse(time.RFC3339, expires)
	if err != nil {
		return fmt.Errorf("HTTPS certificate expiry information was unavailable")
	}
	if deadline.Before(now.Add(time.Duration(t.TLSMinDays) * 24 * time.Hour)) {
		return fmt.Errorf("HTTPS certificate expires within %d days · Minimum validity policy failed", t.TLSMinDays)
	}
	return nil
}
