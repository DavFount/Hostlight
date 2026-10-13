# Storage verification

Reviewed: 2026-10-13. Tests use temporary profiles, SQLite databases, and local
endpoints; no user workspace or production service is modified.

## Automated coverage

| Concern | Evidence |
| --- | --- |
| Incident retention | Age cleanup removes completed incidents and preserves open incidents; 50,000-record retention fixture passes. |
| Delivery retention | Expired completed deliveries are removed; pending deliveries and delivery markers attached to active incidents are preserved for eventual recovery. |
| Silence cleanup | Expired historical windows are removed while recent and future windows remain. |
| Probe retention | Seven-day boundary test retains the cutoff minute and newer samples, removes older samples, and verifies remaining check/failure totals. |
| Probe row bound | Seeded 50,000-summary fixture evicts the oldest row for a new summary; updating an existing minute does not evict another row. |
| Ordered writes | Batched pause/resume/recovery preserves incident identity and excluded time; failed transactions publish no visual state. |
| Shutdown draining | Closing the writer queue with 1,000 pending samples saves all 1,000 across 200 monitor summaries before completion. |
| Queue overflow | Queue stays bounded at 1,024 events and overflow produces a visible warning rather than blocking probes. |
| Persisted count consistency | Opt-in 200-monitor load test pauses producers and compares each monitor's saved check/failure sums with its session counters. |
| Migrations/restart | Existing schema upgrade, saved-summary restart, and incident interruption tests remain in the full Go suite. |

The full Go suite and `go vet ./...` passed on 2026-10-13. Tests simulate old
timestamps to exercise retention now, rather than waiting seven days. The normal
writer invokes the same pruning function at startup and hourly; these fixtures
do not establish that a live desktop session crossed every cleanup boundary.

## Recorded integration result

2026-10-13, Windows amd64 / Go 1.27.2, 200 monitors, one-second interval,
32 concurrent probes and 250 ms startup spread: 11,980 measurement-period checks
in 60.010 seconds (59–60 per monitor), 20 resolved incidents, 40 delivered
webhooks, and 400 stored minute summaries. All per-monitor persisted check and
failure totals matched session counters after pausing producers. No history
error was reported; peak queue length at one-second samples was 0/1,024.
Peak sampled Go heap was 7,423,128 bytes; snapshot batch P95 was 6.796 ms.
These are backend/test-process measurements and exclude the GUI/WebView.

## Meaning of bounds

Retention bounds retained records, not the exact size of SQLite files. SQLite can
keep freed pages for reuse, and WAL/temporary space can fluctuate. No physical
file-size ceiling or automatic shrinking claim is established by these tests.
Active incidents and pending deliveries are deliberately protected from age
cleanup; other queue limits and visible errors still apply.

The load fixture is a short, local TCP/storage/webhook integration check, not an
overnight soak, remote-network capacity guarantee, or GUI memory measurement.
See [load verification](LOAD_VERIFICATION.md) for results and manual follow-ups.

## Reproduce

```powershell
go test ./... -count=1 -timeout=3m
go vet ./...
./scripts/measure-load.ps1 -Seconds 60
```

The load report includes `persistedCountsMatchSessions: true` only after every
monitor's persisted totals match. Measurement-period throughput excludes the
initial baseline; persisted reconciliation includes all samples in the fresh
profile, including startup and final drain/recovery checks.
