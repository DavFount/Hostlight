package main

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
)

const maxBulkTargets = 200

type BulkRow struct {
	Line   int    `json:"line"`
	Target Target `json:"target"`
	Error  string `json:"error"`
}

func targetKey(t Target) string {
	host := strings.ToLower(strings.TrimSuffix(t.Host, "."))
	if ip, err := netip.ParseAddr(host); err == nil {
		host = ip.Unmap().String()
	}
	return fmt.Sprintf("%s|%s|%d", t.Type, host, t.Port)
}

// Each nonempty line is a host or a friendly name followed by | and a host.
// Preview and commit share validation, including duplicate checks under the lock.
func (a *App) PreviewBulk(input string, defaults Target) []BulkRow {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.previewBulkLocked(input, defaults)
}
func (a *App) previewBulkLocked(input string, defaults Target) []BulkRow {
	rows := make([]BulkRow, 0)
	seen := map[string]bool{}
	for _, m := range a.targets {
		seen[targetKey(m.target)] = true
	}
	for i, line := range strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		t := defaults
		t.ID = ""
		t.State = ""
		t.Message = ""
		t.Checked = ""
		t.Latency = 0
		t.Group = strings.TrimSpace(t.Group)
		if t.Type == "" {
			t.Type = "icmp"
		}
		if t.Type == "icmp" {
			t.Port = 0
		}
		name, host, hasName := strings.Cut(line, "|")
		if !hasName {
			host = name
		}
		t.Host = strings.TrimSpace(host)
		t.Name = strings.TrimSpace(name)
		row := BulkRow{Line: i + 1, Target: t}
		if err := validate(t); err != nil {
			row.Error = err.Error()
		} else if seen[targetKey(t)] {
			row.Error = "This host and check already appear in the workspace or this list"
		} else {
			seen[targetKey(t)] = true
		}
		rows = append(rows, row)
		if len(rows) > maxBulkTargets {
			rows[len(rows)-1].Error = fmt.Sprintf("Add at most %d monitors at once", maxBulkTargets)
			break
		}
	}
	return rows
}

func (a *App) AddBulk(input string, defaults Target) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	rows := a.previewBulkLocked(input, defaults)
	if len(rows) == 0 {
		return 0, fmt.Errorf("Enter at least one host")
	}
	for _, row := range rows {
		if row.Error != "" {
			return 0, fmt.Errorf("Line %d: %s", row.Line, row.Error)
		}
	}
	oldNext := a.next
	added := make([]*monitor, 0, len(rows))
	contexts := make([]context.Context, 0, len(rows))
	for _, row := range rows {
		t := row.Target
		a.next++
		t.ID = fmt.Sprint(a.next)
		t.State = "unknown"
		t.Message = "Waiting for checks"
		ctx, cancel := context.WithCancel(a.ctx)
		m := &monitor{target: t, cancel: cancel}
		a.targets[t.ID] = m
		added = append(added, m)
		contexts = append(contexts, ctx)
	}
	if err := a.persistLocked(); err != nil {
		for _, m := range added {
			m.cancel()
			delete(a.targets, m.target.ID)
		}
		a.next = oldNext
		return 0, err
	}
	for i, m := range added {
		go a.run(contexts[i], m, m.target)
	}
	return len(added), nil
}

// Apply a pause/resume change as a single persisted transaction. Already-active
// monitors are left running when resuming a mixed selection.
func (a *App) SetTargetsPaused(ids []string, paused bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, id := range ids {
		if _, ok := a.targets[id]; !ok {
			return fmt.Errorf("A selected monitor no longer exists. Refresh and try again")
		}
	}
	old := map[string]*monitor{}
	contexts := map[string]context.Context{}
	for _, id := range ids {
		m := a.targets[id]
		if _, ok := old[id]; ok {
			continue
		}
		if (m.target.State == "paused") == paused {
			continue
		}
		old[id] = m
		t := m.target
		if paused {
			t.State = "paused"
			t.Message = "Monitoring paused"
		} else {
			t.State = "unknown"
			t.Message = "Waiting for fresh checks"
			t.Checked = ""
			t.Latency = 0
		}
		ctx, cancel := context.WithCancel(a.ctx)
		a.targets[id] = &monitor{target: t, cancel: cancel}
		contexts[id] = ctx
	}
	if len(old) == 0 {
		return nil
	}
	if err := a.persistLocked(); err != nil {
		for id, m := range old {
			a.targets[id].cancel()
			a.targets[id] = m
		}
		return err
	}
	for id, m := range old {
		m.cancel()
		if !paused {
			current := a.targets[id]
			go a.run(contexts[id], current, current.target)
		}
	}
	return nil
}
