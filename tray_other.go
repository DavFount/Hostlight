//go:build !windows

package main

import "fmt"

func startNativeTray(show, quit func(), availability func(bool)) (trayHandle, error) {
	return nil, fmt.Errorf("Tray integration is currently Windows-only")
}
