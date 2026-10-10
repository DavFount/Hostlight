//go:build !windows

package main

import "fmt"

func protectSecret([]byte) ([]byte, error) {
	return nil, fmt.Errorf("Secure webhook storage currently requires Windows")
}
func unprotectSecret([]byte) ([]byte, error) {
	return nil, fmt.Errorf("Secure webhook storage currently requires Windows")
}
