package main

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

const desktopInstanceID = "hostlight-desktop-91be6b13-35d0-4f18-aad5-402d19b3aa49"

func developmentDataDirectory(path, version string) (string, error) {
	if version != "early preview" {
		return "", fmt.Errorf("Development workspace overrides are unavailable in versioned builds")
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("Development workspace must use an absolute directory")
	}
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path, nil
}

func (a *App) desktopInstanceID() string {
	if a.developmentWorkspace == "" {
		return desktopInstanceID
	}
	path := a.developmentWorkspace
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	return fmt.Sprintf("%s-dev-%x", desktopInstanceID, sha256.Sum256([]byte(path)))
}

func (a *App) windowTitle() string {
	if a.developmentWorkspace != "" {
		return "Hostlight — Development workspace"
	}
	return "Hostlight"
}
