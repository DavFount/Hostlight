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

Release-blocking follow-up: the user subsequently observed a history queue-full
warning. Earlier short passes do not establish sustained history reliability.
The history writer now batches up to 64 already-queued events per transaction;
the queue stays bounded and does not hide past dropped observations. Restart in
a fresh fixture workspace and repeat the desktop run, including history queries
and outage/recovery. Record elapsed time and verify no history/storage warnings.
If the warning returns, export sanitized About diagnostics while it is present
to capture current queue length/capacity and storage health. Existing gaps cannot
be reconstructed by restarting or by this change.

Automated follow-up passed on 2026-10-12: shared verification (131 frontend
tests, Go tests/vet, release-tool checks, Windows Wails build) and a 60-second
200-monitor backend repeat. The latter completed 11,904 checks (59–60 per
monitor), resolved 20 incidents, delivered 40 webhooks, and stored 400 minute
summaries without reported history errors. Queue length at one-second samples
peaked at zero; this does not measure transient peaks between samples. The run
overlapped other verification work. Desktop repeat remains pending.

Desktop batching repeat (2026-10-12): user reported no warning at 1,200 checks
and no error at 1,827–1,828 checks per visible monitor (approximately 30.5 minutes
at one-second intervals), with 200 monitors shown. Visible Load 001–003 cards
were available, each with 117 failed session checks. Attribution of those
failures to the controlled outage and confirmation of resolved incident records
remain pending. The 30-minute warning-free desktop follow-up passed; longer soak,
retention/cleanup verification, and other release checks remain open.

The user then supplied incident-history evidence showing 20 matching incidents;
visible Load 007 and Load 019 entries were recovered with connection-refused
reasons and recovery threshold reached. Together with the available monitor
cards, this supports passing the controlled outage/recovery desktop follow-up
after batching. The screenshot does not enumerate every incident or expose all
filter values; the earlier CSV remains the complete 20-monitor baseline.

Close any existing `wails dev` session, then run from the repository:

```powershell
./scripts/desktop-load.ps1
```

Requires Go, Node/npm, and Wails on PATH. The script creates 200 loopback TCP
listeners and a fresh temporary profile, then launches Wails dev. The title must
say **Hostlight — Development workspace**. Configuration, SQLite history,
WebView preferences, and the instance lock are separate from your normal app.
The development-only `HOSTLIGHT_DEV_DATA_DIR` override requires an absolute
directory and is rejected by versioned release builds.

- Confirm 200 monitors become available and counters keep increasing.
- Search/filter, expand recent charts, switch to history, and minimize/restore.
- Pause shown monitors; confirm counts stop, then resume and confirm they advance.
- Use the printed `control.txt` commands in a second terminal: `down` fails the
  first 20 endpoints; `up` recovers them. Allow several seconds for thresholds.
  Confirm the other 180 continue and incident/history views reflect the outage.
- Record machine details, Hostlight and WebView CPU/working set in Task Manager,
  run duration, and any delays while doing normal desktop work.
- Quit the test app and exit Wails dev. Reopen the normal app and confirm your
  original monitors/settings/history remain intact.

The helper stops when the script exits or after one hour (use `-Hours 8` for a
longer session, maximum 24). After expiry endpoints stop responding; end the test
before interpreting those failures. Temporary files are retained at the printed
path for inspection. This fixture does not exercise webhook delivery; the backend
workload above covers that. Desktop performance and extended soak remain pending
until results are recorded.

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

Desktop follow-up: user reported responsive UI with 200 monitors on 2026-10-12.
Screenshots also showed unplanned Windows socket-address errors across monitors
outside the controlled outage group. This confirms observed responsiveness, but
does not qualify a clean sustained connection run. The disposable TCP helper now
resets accepted sockets on close to avoid accumulating TIME_WAIT sockets.

The user repeated the desktop run after that change on 2026-10-12. Screenshots
show 200 monitors, 424 successful checks and zero failures on each visible card
(approximately seven minutes at one-second intervals), and zero incidents with
no search/outcome restrictions. The user continued background work and reported
no noticeable slowness. The unexpected socket-address incidents did not recur in
this observed run. This passes the short desktop responsiveness follow-up;
desktop process/WebView CPU and working set were not recorded, and an extended
soak remains a separate check.

Controlled outage/recovery also passed on 2026-10-12. The user's incident CSV
contained exactly 20 resolved incidents, one each for Load 001–020, with 44
seconds observed duration each and connection-refused reasons followed by
recovery threshold reached. No incidents for Load 021–200 appeared in the export.

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
