package main

import (
	"fmt"
	"testing"
	"time"
)

func TestHistoryBatchPreservesTransitionsAndPause(t *testing.T) {
	s := historyFixture(t)
	a := &App{history: s}
	at := time.Now()
	down := Target{ID: "batch", State: "down", Interval: 5, Timeout: 3}
	up := down
	up.State = "up"
	events := []historyEvent{
		{target: down, kind: "result", previous: "up", at: at},
		{target: down, kind: "Monitoring paused", at: at.Add(time.Second)},
		{target: down, kind: "Monitoring resumed", at: at.Add(time.Hour)},
		{target: up, kind: "result", previous: "unknown", at: at.Add(time.Hour + time.Second)},
	}
	if err := s.applyBatch(events); err != nil {
		t.Fatal(err)
	}
	items := historyItems(t, s)
	if len(items) != 1 || items[0].Status != "resolved" || items[0].DurationSeconds != 1 {
		t.Fatalf("batch lost pause/recovery ordering: %+v", items)
	}
	alerts := a.GetVisualAlerts(0, false).Items
	if len(alerts) != 2 || alerts[0].Event != "down" || alerts[1].Event != "up" || alerts[1].DurationSeconds != 1 {
		t.Fatalf("batch lost committed alert ordering: %+v", alerts)
	}
}

func TestHistoryBatchRollbackDoesNotPublish(t *testing.T) {
	s := historyFixture(t)
	_, err := s.db.Exec(`CREATE TRIGGER reject_batch BEFORE INSERT ON probe_minutes BEGIN SELECT RAISE(ABORT, 'test failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	target := Target{ID: "batch", State: "down", Interval: 1, Timeout: 1}
	at := time.Now()
	err = s.applyBatch([]historyEvent{
		{target: target, kind: "result", previous: "up", at: at},
		{target: target, kind: "result", at: at.Add(time.Second), sample: &storedProbeSample{Session: "batch", Failed: true}},
	})
	if err == nil {
		t.Fatal("expected rollback")
	}
	if len(historyItems(t, s)) != 0 || len((&App{history: s}).GetVisualAlerts(0, false).Items) != 0 || len(s.visual.outages) != 0 {
		t.Fatal("failed batch leaked incident or visual state")
	}
}

func TestHistoryWriterDrainsQueuedSamplesOnShutdown(t *testing.T) {
	s := historyFixture(t)
	at := time.Now()
	for i := 0; i < 1000; i++ {
		target := Target{ID: fmt.Sprint(i % 200), State: "up", Interval: 1, Timeout: 1}
		s.events <- historyEvent{target: target, kind: "result", at: at, sample: &storedProbeSample{Session: "batch", Latency: 1}}
	}
	close(s.events)
	go s.writer()
	select {
	case <-s.done:
	case <-time.After(10 * time.Second):
		t.Fatal("writer did not drain bounded backlog")
	}
	var checks, summaries int
	if err := s.db.QueryRow("SELECT SUM(checks), COUNT(*) FROM probe_minutes").Scan(&checks, &summaries); err != nil {
		t.Fatal(err)
	}
	if checks != 1000 || summaries != 200 || s.errorMessage() != "" {
		t.Fatalf("lost samples: checks=%d summaries=%d error=%s", checks, summaries, s.errorMessage())
	}
}
