# Hostlight implementation plan

Status: implementation underway. A Windows ICMP/TCP monitoring preview is available with dark/light
themes, target editing, filtering, pause/resume, configurable thresholds, and saved
target definitions. Bulk paste now includes validation previews and duplicate checks;
group filtering and transactional pause/resume apply to the shown monitors. The
milestones below remain the full release acceptance checklist.

Implementation note: target settings currently use an atomically replaced, versioned
JSON file in the OS user configuration directory. SQLite now stores incident history;
sample history and charts remain planned. This keeps configuration readable and separate from
high-volume monitoring data. Existing settings must remain importable when storage evolves.

Incident history implementation: `modernc.org/sqlite` supplies a pure-Go driver,
with schema version 1, a background writer, paged queries, CSV export, and configurable
retention. Go 1.26 is now the minimum version. Outages are segmented at pauses,
edits, deletion, errors, shutdown/restart, and long observation gaps; interrupted
records end at the last observation. Writes are bounded and errors are visible.

This document is the working implementation plan. Check off milestones only when
their acceptance criteria are met, and update decisions as implementation supplies
new evidence. Milestones describe delivery order, not calendar commitments.

## Product goal

Build a focused desktop utility for monitoring multiple hosts and services at a
glance, with trustworthy outage detection, useful history, and webhook alerts.
Preserve vmPing's fast workflow: paste targets, start monitoring, see what changes.

Windows is the first release platform. Keep the monitoring core portable and
verify macOS/Linux support separately before claiming those platforms are supported.

## First release scope

- ICMP and TCP probes with IPv4/IPv6 support.
- Target CRUD, aliases, groups, bulk input, and saved configuration.
- Responsive status grid and searchable target list.
- Configurable probe intervals, timeouts, and failure/recovery thresholds.
- Distinct unknown, up, down, paused, and error states.
- Generic JSON webhooks, recovery alerts, retries, and delivery history.
- SQLite incident history, recent latency/loss charts, and bounded retention.
- Application settings, export/import, and a reproducible Windows build.

Defer HTTP/DNS/TLS probes, degraded-state policies, MTR, dependency suppression,
advanced dashboards, remote agents, and headless service operation until the core
release is dependable. Tray integration is a separate platform feasibility task;
it must not delay a working monitoring application.

## Architecture and ownership

Go owns target validation, scheduling, probes, status decisions, persistence, and
alert delivery. Vue owns presentation and transient interaction state. Monitoring
must continue when the view changes or the window is minimized.

Use Wails v2, Vue 3, TypeScript, Vite, and the existing npm lockfile. Add Pinia for
shared frontend state. Add routing when multiple screens exist. Select a chart
library after testing a small latency-chart prototype; avoid installing a broad
UI stack before the first working probe.

Suggested structure:

```text
app.go                         Thin Wails facade and lifecycle wiring
main.go                        Desktop startup
internal/
  domain/                      Targets, results, states, incidents, policies
  probe/                       Probe interface and ICMP/TCP implementations
  monitor/                     Scheduler and status state machine
  storage/                     SQLite repositories and migrations
  alert/                       Rules, durable delivery queue, webhook transport
  config/                      Settings, import/export, credential references
  platform/                    OS integration and credential-store adapters
frontend/src/
  components/                  Shared visual components
  features/targets/            Grid, list, editor, bulk input
  features/history/            Incidents and charts
  features/alerts/             Alert rules and delivery history
  features/settings/           Application preferences
  stores/                      Pinia state and event reconciliation
  services/                    Wails API/event adapter
docs/                          Project decisions and implementation notes
```

Keep Go packages independent of Wails where possible. Inject clocks, probes,
repositories, and transports where deterministic tests need them. Expose a small
typed API through the facade; treat generated `frontend/wailsjs` files as generated.

## Data and event contracts

Define these models before adding persistence or alerts:

| Model | Required responsibilities |
| --- | --- |
| Target | Stable ID, name, address, probe type, optional port, group, enabled flag, policy |
| Probe policy | Interval, timeout, consecutive failure/recovery thresholds |
| Probe result | Target ID, run generation, sequence, timestamp, duration, outcome, diagnostic |
| Target status | Current state, transition time, last result, consecutive counters |
| Incident | Target ID, opened/resolved timestamps, cause, policy snapshot |
| Alert rule | Target/group scope, selected transitions, endpoint reference, cooldown |
| Delivery | Stable event ID, destination, attempts, next attempt time, terminal outcome |

Store timestamps in UTC; localize them in the UI. Measure probe duration with a
monotonic clock. Separate immutable result/transition records from current status.

Frontend startup should subscribe to events and obtain a versioned snapshot,
reconciling buffered events by sequence. Include a run generation so results from
a stopped or edited target cannot overwrite its new state. Batch high-frequency
result updates, deliver transitions reliably, and expose snapshot refresh when the
UI detects a gap. Bound frontend logs and chart buffers.

## Status semantics

- New or resumed targets begin as unknown until enough evidence establishes state.
- Consecutive failed checks establish down; consecutive successes establish up.
- A successful result resets the failure counter; a failed result resets the success counter.
- Timeouts, refused TCP connections, and unreachable responses are probe failures.
- Invalid configuration, missing privileges, or an unavailable local adapter are
  diagnostic errors; distinguish them from evidence that the remote target is down.
- Pause/stop cancels work and does not open an outage or trigger an outage webhook.
- Sleep and resume create a monitoring gap, not synthetic failures. Reset pending
  counters and obtain fresh evidence after resume.
- Emit one transition event per established state change. Initial discovery is
  silent by default; recovery alerts require an actual prior outage.
- Interval statistics exclude paused time and monitoring gaps. Document exactly
  what uptime and loss percentages measure, including how unknown/error samples
  are handled. Prefer observed-check availability for v1 over an implied SLA.

## Milestone 0 — Baseline and technical spikes

- [ ] Verify `wails dev`, frontend production build, and Windows desktop build.
- [ ] Establish formatting, type checks, Go checks, and basic CI.
- [ ] Prototype ICMP on Windows as a standard user and record the implementation choice.
- [ ] Check cancellation, DNS behavior, IPv6, and local permission/error reporting.
- [ ] Choose a maintained SQLite driver and migration approach compatible with the build.
- [ ] Investigate tray integration if required; document Wails v2/platform limitations.

Acceptance: the starter builds reproducibly and a small probe prototype reports
success, timeout, and local errors correctly without requiring elevation by default.
If a platform requires privileges, report that explicitly rather than silently
elevating the whole desktop app. Confirm library versions at implementation time.

## Milestone 1 — Application shell and target management

- [ ] Replace the greeting demo with Hostlight navigation, empty state, and target grid.
- [ ] Implement domain models and a typed Wails facade.
- [ ] Add target create/edit/delete, aliases, groups, and bulk paste.
- [ ] Validate addresses, ports, intervals, timeouts, and threshold ranges in Go.
- [ ] Add Pinia stores and a frontend API adapter with mock data for UI development.
- [ ] Support keyboard navigation, readable status labels, and color-independent indicators.

Acceptance: targets can be managed through the UI; invalid input produces useful
messages; frontend build/type checks pass. Mock status is clearly marked until
real monitoring is connected.

## Milestone 2 — Monitoring engine

- [ ] Implement a cancellable probe interface with ICMP and TCP adapters.
- [ ] Resolve hostnames and apply explicit IPv4/IPv6 selection rules.
- [ ] Schedule checks with bounded concurrency and staggered starts.
- [ ] Prevent overlapping checks for a target; define missed-tick behavior.
- [ ] Implement the state machine and transition events.
- [ ] Wire start/stop all and per-target pause/resume into the live UI.
- [ ] Show last response, latency, loss, diagnostic errors, and elapsed outage duration.
- [ ] Handle edits, deletion, shutdown, network changes, and sleep/resume safely.

Acceptance: controlled success/failure sequences produce exactly the expected
transitions. Stopping or editing a target cannot apply stale results. A sustained
run has bounded goroutines, memory, and UI buffers. Target count and probe interval
limits are configurable and failures do not stall unrelated targets.

## Milestone 3 — Persistence and history

- [ ] Create versioned SQLite migrations and transactional repositories.
- [ ] Save target/group configuration and application preferences.
- [ ] Persist transitions/incidents and bounded probe samples.
- [ ] Define retention, sample aggregation, and cleanup schedules.
- [ ] Add history filters, incident duration, date ranges, and CSV export.
- [ ] Add recent latency/loss charts and precisely labeled statistics.
- [ ] Store data in the OS application-data directory; show its location in settings.
- [ ] Handle database failures visibly and recover unfinished incidents after restart.

Acceptance: targets survive restart; incidents remain consistent across crashes;
retention bounds database growth; migrations preserve existing data. Shutdown and
restart gaps are represented as gaps rather than invented uptime or downtime.

## Milestone 4 — Webhook alerts

- [ ] Configure generic JSON endpoints, headers, transition selection, and test delivery.
- [ ] Keep endpoint secrets in the OS credential store; persist only references.
- [ ] Create a durable delivery queue transactionally with recorded transitions.
- [ ] Add stable event IDs, schema version, timestamps, target metadata, and outage duration.
- [ ] Deliver asynchronously with request timeouts and bounded concurrency.
- [ ] Retry transient network errors, rate limits, and appropriate server errors
  with capped exponential backoff and jitter; honor bounded `Retry-After` values.
- [ ] Treat permanent failures explicitly, cap attempts/age, and show delivery history.
- [ ] Define cooldown behavior without dropping the final recovery notification.
- [ ] Support optional HMAC signing and redact secrets from logs and exports.
- [ ] Add provider payload presets after generic delivery works.

Acceptance: an outage and recovery each create one logical event, including across
restart. Delivery is at least once: retries may reach the receiver more than once,
so event IDs support receiver deduplication. Webhook failures never block probes.
Tests cover retries, restart recovery, timeouts, redaction, and terminal failures.

## Milestone 5 — Desktop workflow and configuration portability

- [ ] Add theme preferences, saved layout, search/filter, and bulk actions.
- [ ] Add desktop notifications and mute controls where supported.
- [ ] Implement tested tray behavior if the platform spike supports it.
- [ ] Add versioned configuration import/export with preview and validation.
- [ ] Exclude credentials and live history from configuration exports by default.
- [ ] Offer explicit portable mode with writable-path checks and clear credential behavior.
- [ ] Add support information and a sanitized diagnostic export.

Acceptance: reopening restores the intended workspace; imports cannot silently
replace configuration; exported configuration contains no secrets. Monitoring
continues when minimized. Closing behavior is explicit and platform-tested.

## Milestone 6 — First release verification

- [ ] Exercise a target matrix: IP/hostname, IPv4/IPv6, reachable/unreachable,
  refused port, DNS failure, invalid settings, adapter loss, sleep/resume, and restart.
- [ ] Benchmark an initial 200-target workload at one-second intervals on a recorded
  Windows reference machine; measure CPU, memory, scheduling delay, and UI responsiveness.
- [ ] Run an extended soak and verify retention, queue bounds, and cleanup.
- [ ] Produce release artifacts, version metadata, checksums, and release notes.
- [ ] Verify fresh-machine startup, WebView2 requirements, upgrade, and uninstall behavior.
- [ ] Decide distribution format and code-signing approach before public release.
- [ ] Document supported platforms, configuration location, limits, and known issues.

Acceptance: release checks pass on the supported Windows platform, no critical
monitoring/alerting defects remain, and the README reflects implemented behavior.
Treat 200 targets as an initial benchmark, not a supported capacity claim until measured.

## Later milestones

Prioritize these using actual usage after the first release:

1. HTTP/HTTPS status/body checks, DNS expected-answer checks, and TLS expiry.
2. Degraded-state latency/loss policies with rolling windows and hysteresis.
3. Maintenance schedules, timed silences, acknowledgments, and dependency suppression.
4. Route diagnostics and MTR-style monitoring.
5. Saved views, advanced tables, comparison charts, and customizable layouts.
6. A shared-core CLI/headless runner with clear process ownership and local IPC.
7. Verified macOS/Linux packages and platform-specific probe/credential adapters.
8. Signed automatic updates with rollback/recovery behavior.

Do not add a server, cloud accounts, remote agents, or plugin system without a
separate product decision. Keep those possibilities from complicating v1.

## Verification strategy

- Unit-test state transitions, scheduling/cancellation, validation, retention, and
  retry decisions with fake clocks and controlled outcomes.
- Integration-test SQLite migrations/transactions and HTTP delivery against local
  fixtures. Use controlled local endpoints rather than public Internet hosts in CI.
- Test probe adapters on each supported OS; platform behavior cannot be proven by mocks.
- Component-test critical target forms and status/event reconciliation with Vitest.
- Add a small desktop smoke-test checklist for Wails startup, bindings, window
  behavior, and shutdown. Browser-only tests do not prove desktop integration.
- Run `go test ./...`, `go vet ./...`, frontend checks/build, and `wails build` in
  CI where supported. Run Go's race detector where its platform toolchain is available.

## First implementation slice

Deliver one TCP target end to end: add it in Vue, validate it in Go, probe a local
test listener, apply failure/recovery thresholds, and update the status card.
Then add ICMP using the platform spike results. Complete this slice before expanding
navigation, charting, or notification integrations.

## Decision log

| Decision | Starting position | Revisit when |
| --- | --- | --- |
| Desktop framework | Existing Wails v2 project | A required feature cannot be supported cleanly |
| Frontend | Vue 3 + TypeScript + Vite | No planned migration |
| Package manager | npm and committed lockfile | A concrete workflow requires a change |
| Persistence | SQLite; driver selected in milestone 0 | Build or measured performance requires it |
| First platform | Windows | Windows acceptance criteria pass |
| Monitoring ownership | Go process independent of views | Headless execution is implemented |
| Alert delivery | Durable queue, at-least-once semantics | External integrations require additional guarantees |

Record significant library selections and deviations here or in short architecture
decision records under `docs/` so future changes retain their rationale.
