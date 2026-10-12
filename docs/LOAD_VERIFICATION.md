# Windows load verification

This opt-in workload measures 200 TCP monitors at one-second intervals, using
32 concurrent probes and a 250 ms start spread. It runs the real scheduler,
loopback connections, SQLite history/minute summaries, snapshot APIs, and webhook
delivery. Twenty endpoints stop accepting connections one-third of the way
through and recover at two-thirds, producing 20 outage/recovery pairs and
40 delivered webhook events.

The workload creates temporary configuration/database files, loopback listeners,
and a local HTTP webhook receiver. It does not open or change the saved Hostlight
workspace. Ordinary tests skip it unless explicitly enabled. Windows is required
for process CPU counters and the current credential encryption adapter.

Run normal verification first so embedded frontend assets exist, then:

```powershell
./scripts/measure-load.ps1 -Seconds 60
```

For a longer backend soak, use `-Seconds 3600`; supported durations are 60 seconds
through 24 hours. Setup, warmup, transition checks, and teardown add time beyond
the measurement duration. A one-minute pass does not establish an extended soak.

The `HOSTLIGHT_LOAD_REPORT` JSON line records CPU identity, logical processors,
Go concurrency, completed-check throughput, per-monitor counts, process CPU,
peak Go heap/goroutines, sampled history queue occupancy, snapshot-call latency,
interval cadence overhead, and persisted/delivered record counts.

- CPU percentage uses **one logical core** as 100%; this includes the test's local
  servers and sampling overhead, and is not Task Manager's whole-machine CPU percentage.
- Go heap is allocated Go memory sampled once a second, not process working set
  or WebView memory. Goroutines also include local test servers.
- Snapshot latency covers sequential target/statistics/configuration/silence/
  active-outage/recent-sample API calls. It excludes Wails IPC and Vue rendering.
- Cadence overhead measures from the previous dial's completion to the next
  dial's start, minus the configured one-second interval. It includes result
  processing, timer delay, and waiting for capacity. P95 is a histogram upper
  bound; maximum is directly observed. Storage normally follows via the queue.
- Queue peaks are sampled once a second and can miss short bursts. Queue overflow
  or storage errors fail the workload. Every monitor must complete at least half
  the nominal checks; this is a stall check, not a promised throughput threshold.
- All 200 monitors must recover, all 20 incidents must resolve, all 40 webhook
  deliveries must succeed, and persistent summaries must respect their global cap.

This is a **backend loopback baseline**, not a supported-capacity claim. Remote
timeouts, ICMP/HTTP behavior, WebView memory, desktop responsiveness, and an
extended retention/cleanup soak require separate evidence. Record other machine
activity when comparing runs.

## Desktop follow-up

Use a disposable workspace for 200 authorized local targets at one-second
intervals. Record machine details, scheduling settings, process working set and
CPU in Task Manager, and behavior while searching, filtering, expanding charts,
switching pages, minimizing/restoring, and pausing/resuming shown monitors. Check
that status/history remain fresh and unrelated monitors continue during failures.
Do not import a test workload into a workspace containing important monitors.

## Recorded baseline

Measured on 2026-10-12, Windows amd64, Go 1.27.2, 48 logical processors /
GOMAXPROCS 48. CPU identifier: AMD64 Family 23 Model 8 Stepping 2, AuthenticAMD.
Other machine activity was not controlled.

| Measurement | Result |
| --- | --- |
| Measurement duration | 60.001 seconds |
| Completed checks | 11,811 (196.85/second) |
| Per-monitor checks | 59–60 |
| Process CPU | 51.109 CPU seconds; 85.18% of one logical core |
| Peak sampled Go heap | 7,263,528 bytes (6.93 MiB) |
| Peak sampled goroutines | 409, including test servers |
| Peak sampled history queue | 116 / 1,024 |
| Snapshot API batch P95 | 7.051 ms |
| Interval overhead P95 upper bound | 25 ms |
| Maximum interval overhead | 35.235 ms |
| Resolved incidents / delivered webhooks | 20 / 40 |
| Persisted minute summaries | 400 |

The workload passed with no reported storage/queue errors.
Desktop measurements and the extended soak remain pending.

## User local repeat

User confirmed PASS on 2026-10-12 using Windows amd64, Go 1.27.2, the same
reported CPU identifier, 48 logical processors, and GOMAXPROCS 48.

| Measurement | Result |
| --- | --- |
| Measurement duration | 60.003 seconds |
| Completed checks | 12,000 (199.99/second) |
| Per-monitor checks | 60 for every monitor |
| Process CPU | 24.922 CPU seconds; 41.53% of one logical core |
| Peak sampled Go heap | 7,552,656 bytes (7.20 MiB) |
| Peak sampled goroutines | 409, including test servers |
| Peak sampled history queue | 76 / 1,024 |
| Snapshot API batch P95 | 4.008 ms |
| Interval overhead P95 upper bound | 2 ms |
| Maximum interval overhead | 11.228 ms |
| Resolved incidents / delivered webhooks | 20 / 40 |
| Persisted minute summaries | 400 |

User reported no noticeable CPU/memory jump and continued browser/ticket work
during the run. An Ethernet spike coincided with a browser reload; the workload
uses loopback, so this observation does not attribute physical-adapter traffic
to Hostlight. This supports ordinary computer responsiveness during the backend
workload; it does not measure the Hostlight WebView rendering 200 monitor cards.
