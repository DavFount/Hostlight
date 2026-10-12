// Package loadfixture supplies disposable loopback targets for desktop testing.
package loadfixture

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Fixture struct {
	Directory string `json:"directory"`
	Control   string `json:"control"`
	Count     int    `json:"count"`
	cancel    context.CancelFunc
	done      chan struct{}
	listeners []net.Listener
	addresses []string
	acceptors sync.WaitGroup
}

func Start(ctx context.Context, directory string, count int) (*Fixture, error) {
	if !filepath.IsAbs(directory) || count < 1 || count > 200 {
		return nil, fmt.Errorf("Use an absolute empty directory and 1–200 targets")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	if len(entries) != 0 {
		return nil, fmt.Errorf("Fixture directory must be empty; existing files are preserved")
	}
	ctx, cancel := context.WithCancel(ctx)
	f := &Fixture{Directory: directory, Control: filepath.Join(directory, "control.txt"), Count: count, cancel: cancel, done: make(chan struct{}), listeners: make([]net.Listener, count), addresses: make([]string, count)}
	closeListeners := func() {
		for _, listener := range f.listeners {
			if listener != nil {
				listener.Close()
			}
		}
		f.acceptors.Wait()
	}
	targets := make([]map[string]any, 0, count)
	for i := 0; i < count; i++ {
		if err := f.listen(i, "127.0.0.1:0"); err != nil {
			cancel()
			closeListeners()
			return nil, err
		}
		targets = append(targets, map[string]any{"target": map[string]any{"id": fmt.Sprint(i + 1), "name": fmt.Sprintf("Load %03d", i+1), "host": "127.0.0.1", "type": "tcp", "port": f.listeners[i].Addr().(*net.TCPAddr).Port, "group": "Load test", "interval": 1, "timeout": 1, "failures": 2, "recovery": 2}, "paused": false})
	}
	cfg := map[string]any{"version": 4, "nextId": count, "closeToTray": false, "scheduling": map[string]int{"concurrency": 32, "spreadMilliseconds": 250}, "targets": targets}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err == nil {
		err = writeNew(filepath.Join(directory, "targets.json"), data)
	}
	if err == nil {
		err = writeNew(f.Control, []byte("up\n"))
	}
	if err != nil {
		cancel()
		closeListeners()
		return nil, err
	}
	go func() {
		defer close(f.done)
		defer closeListeners()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		down := false
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				data, err := os.ReadFile(f.Control)
				if err != nil {
					continue
				}
				command := strings.TrimSpace(string(data))
				limit := 20
				if limit > count {
					limit = count
				}
				if command == "down" && !down {
					for i := 0; i < limit; i++ {
						f.listeners[i].Close()
						f.listeners[i] = nil
					}
					down = true
				}
				if command == "up" && down {
					complete := true
					for i := 0; i < limit; i++ {
						if f.listeners[i] == nil {
							if err := f.listen(i, f.addresses[i]); err != nil {
								complete = false
							}
						}
					}
					if complete {
						down = false
					}
				}
			}
		}
	}()
	return f, nil
}

func writeNew(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func (f *Fixture) listen(index int, address string) error {
	listener, err := net.Listen("tcp4", address)
	if err != nil {
		return err
	}
	f.listeners[index] = listener
	f.addresses[index] = listener.Addr().String()
	f.acceptors.Add(1)
	go func() {
		defer f.acceptors.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// This disposable endpoint only tests connection establishment.
			// Reset accepted sockets on close so sustained loopback traffic does
			// not accumulate TIME_WAIT sockets in the Windows dynamic port pool.
			if tcp, ok := conn.(*net.TCPConn); ok {
				tcp.SetLinger(0)
			}
			conn.Close()
		}
	}()
	return nil
}

func (f *Fixture) Close() { f.cancel(); <-f.done }
