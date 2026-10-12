package loadfixture

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFixtureControlsAndCleanup(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "profile")
	f, err := Start(context.Background(), directory, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data, err := os.ReadFile(filepath.Join(directory, "targets.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Targets []struct {
			Target struct {
				Port int `json:"port"`
			} `json:"target"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	address := fmt.Sprintf("127.0.0.1:%d", cfg.Targets[0].Target.Port)
	reachable := func() bool {
		conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}
	wait := func(want bool) {
		t.Helper()
		deadline := time.Now().Add(4 * time.Second)
		for reachable() != want {
			if time.Now().After(deadline) {
				t.Fatalf("endpoint reachability did not become %v", want)
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
	wait(true)
	if err := os.WriteFile(f.Control, []byte("down"), 0600); err != nil {
		t.Fatal(err)
	}
	wait(false)
	if err := os.WriteFile(f.Control, []byte("up"), 0600); err != nil {
		t.Fatal(err)
	}
	wait(true)
	f.Close()
	wait(false)
}

func TestFixturePreservesExistingFiles(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "targets.json")
	if err := os.WriteFile(path, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	if f, err := Start(context.Background(), directory, 3); err == nil {
		f.Close()
		t.Fatal("nonempty directory accepted")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "existing" {
		t.Fatal("existing data changed")
	}
}
