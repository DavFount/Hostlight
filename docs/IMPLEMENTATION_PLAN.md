# Hostlight implementation plan

Status reviewed: 2026-10-10. Implementation underway; not release-qualified.
The Windows preview includes ICMP/TCP monitoring, target management and bulk paste,
saved configuration, dark/light themes, incident history, scoped webhook delivery,
per-monitor alert suppression, and reviewed monitor backup/import.

Checklist convention: `[x]` means the stated implementation or verification is
complete; `[ ]` means work or verification remains. Split partially delivered
features into separate items rather than leaving completed work unchecked.
Completed items do not imply that an entire milestone's acceptance criteria pass.
Manual platform checks, load/soak tests, and release checks remain explicit gates.
Update this checklist with each implementation slice before requesting a commit.

Implementation note: target settings currently use an atomically replaced, versioned
JSON file in the OS user configuration directory. SQLite now stores incident history;
sample history and charts remain planned. This keeps configuration readable and separate from
high-volume monitoring data. Existing settings must remain importable when storage evolves.

Incident history implementation: `modernc.org/sqlite` supplies a pure-Go driver,
with schema version 5 (including persisted alert suppression and paused-outage accounting), a background writer, paged queries, CSV export, and configurable
retention. Go 1.26 is now the minimum version. Outages are segmented at
edits, deletion, errors, shutdown/restart, and long observation gaps; interrupted
records end at the last observation. Pauses now preserve the current outage within
the application session, exclude the unobserved pause interval, and retain recovery
alert eligibility after fresh checks. Writes are bounded and errors are visible.

This document is the working implementation plan. Check off milestones only when
their acceptance criteria are met, and update decisions as implementation supplies
new evidence. Milestones describe delivery order, not calendar commitments.

Alert suppression implementation: per-monitor snoozes and one-time maintenance
windows are persisted in SQLite. Checks and incident history continue. Notifications
and queued retries crossing a window are marked suppressed, including intervals
crossed while closed. Suppressed events never replay, and ongoing outages do not
generate catch-up alerts when a window ends. Overlapping windows combine; cancelling
an active window retains its elapsed interval for delayed observations. Test
delivery bypasses suppression. Local-time scheduling is stored as UTC; recurring
schedules and global/group maintenance controls remain future work.

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

Use Wails v2, Vue 3, TypeScript, Vite, and the existing npm lockfile. The preview
uses Vue composition state, component-local typed API declarations, view switching,
and one-second snapshot polling; Pinia, a shared API adapter, routing, and sequenced
events below are proposed architecture, not implemented prerequisites. Introduce
them when shared state or navigation complexity warrants it. Select a chart
library after testing a small latency-chart prototype; avoid installing a broad
UI stack before the first working probe.

Suggested future structure (current Go implementation remains in package `main`,
with frontend screens under `frontend/src/components`):

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

Target contract for further extraction/refactoring; the preview implements these
responsibilities across existing structs and storage records, not all as separate
models. Generation/sequence-based frontend reconciliation remains planned:

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

Planned event-driven design: frontend startup should subscribe to events and obtain a versioned snapshot,
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
- Pause cancels probe work without opening or recovering an outage. An existing
  outage stays open; resume uses fresh thresholds and the same alert association.
  Continued failure does not duplicate the outage alert. Exclude unobserved paused
  time from duration. Deletion closes as interrupted without a recovery webhook.
  Shutdown/restart and configuration edits still interrupt observation.
- Sleep and resume create a monitoring gap, not synthetic failures. Reset pending
  counters and obtain fresh evidence after resume.
- Emit one transition event per established state change. Initial discovery is
  silent by default; recovery alerts require an actual prior outage.
- Interval statistics exclude paused time and monitoring gaps. Document exactly
  what uptime and loss percentages measure, including how unknown/error samples
  are handled. Prefer observed-check availability for v1 over an implied SLA.

## Milestone 0 — Baseline and technical spikes

- [x] Verify frontend production/type-check build and Windows desktop build.
- [ ] Complete a recorded `wails dev` desktop smoke test (startup, bindings, shutdown).
- [x] Establish Go formatting, Go tests/vet, and frontend component tests/type checks.
- [ ] Add basic CI for these checks and the supported Windows build.
- [x] Implement native Windows IP Helper ICMP and document the choice in README.
- [x] Test cancellation, DNS/address selection, IPv4/IPv6 localhost, and error classification.
- [ ] Record standard-user and adapter/permission failure checks on Windows.
- [x] Select pure-Go `modernc.org/sqlite` with versioned transactional migrations.
- [ ] Investigate tray integration if required; document Wails v2/platform limitations.

Acceptance: the starter builds reproducibly and a small probe prototype reports
success, timeout, and local errors correctly without requiring elevation by default.
If a platform requires privileges, report that explicitly rather than silently
elevating the whole desktop app. Confirm library versions at implementation time.

## Milestone 1 — Application shell and target management

- [x] Replace the greeting demo with Hostlight navigation, empty state, and target grid.
- [x] Implement target models and typed Wails methods/bindings for implemented features.
- [x] Add target create/edit/delete, aliases, groups, and bulk paste with validation previews.
- [x] Validate addresses, ports, intervals, timeouts, and threshold ranges in Go.
- [x] Provide common TCP service-port choices with custom-port entry.
- [ ] Introduce shared frontend state/API adapters when complexity warrants them; Pinia remains optional.
- [x] Show readable status labels and color-independent indicators.
- [ ] Audit keyboard navigation, focus management, and screen-reader behavior across screens.

Acceptance: targets can be managed through the UI; invalid input produces useful
messages; frontend build/type checks pass. Mock status is clearly marked until
real monitoring is connected.

## Milestone 2 — Monitoring engine

- [x] Implement cancellable ICMP/TCP probe adapters with stale-result protection.
- [x] Resolve hostnames; document ICMP IPv4 preference and IPv6-only/scoped-address support.
- [x] Bound concurrent probes to 32 and prevent overlapping checks per monitor.
- [ ] Add staggered starts and configurable concurrency/capacity limits; document scheduling behavior.
- [x] Implement tested failure/recovery state decisions and backend incident/alert transitions.
- [ ] Implement sequenced frontend transition events if replacing snapshot polling.
- [x] Wire per-target and filtered bulk pause/resume into the live UI.
- [x] Show last check, latency, and diagnostic errors; show incident durations in history.
- [ ] Add loss statistics and elapsed outage duration to monitor cards.
- [x] Discard stale results after edits, deletion, pause, and shutdown.
- [ ] Verify network changes and sleep/resume, including fresh threshold counters after gaps.

Acceptance: controlled success/failure sequences produce exactly the expected
transitions. Stopping or editing a target cannot apply stale results. A sustained
run has bounded goroutines, memory, and UI buffers. Target count and probe interval
limits are configurable and failures do not stall unrelated targets.

## Milestone 3 — Persistence and history

- [x] Create versioned SQLite migrations and transactional incident/alert storage.
- [x] Save target/group/pause configuration in versioned JSON and theme/density in local storage.
- [x] Persist observed incidents and monitoring interruptions.
- [ ] Persist bounded probe samples and define sample aggregation.
- [x] Implement configurable incident retention, record limits, and scheduled cleanup.
- [x] Add history filters, incident duration, date ranges, paging, and CSV export.
- [ ] Add recent latency/loss charts and precisely labeled statistics.
- [x] Store data in the OS application-data directory and display the configuration location.
- [x] Surface database/queue failures and interrupt unfinished incidents after restart.

Acceptance: targets survive restart; incidents remain consistent across crashes;
retention bounds database growth; migrations preserve existing data. Shutdown and
restart gaps are represented as gaps rather than invented uptime or downtime.

## Milestone 4 — Webhook alerts

Implementation update: up to 50 named generic destinations are implemented, with
global scope or selected groups and/or individual monitors, enable/disable, test
delivery, bearer authentication, HMAC signing, and a recent delivery table with
destination names. Schema v3 preserves existing incidents, migrates the previous
webhook as a global destination, and retains its pending queue. Secrets use
current-user Windows DPAPI encrypted blobs rather than credential-store references;
this deliberately makes the first implementation Windows-only and non-portable.
Initial discovery is silent. Pending deliveries are cancelled on settings changes
for the affected destination only, and recovery alerts require an outage under
that same configuration. Group/monitor matching is additive and deduplicated within
each destination; retries preserve FIFO per destination without head-of-line
blocking across destinations. Monitor ID high-water marks persist to prevent
deleted identities being reused by replacements. Provider presets, transition
selection, and cooldown rules remain future work.

- [x] Configure named generic JSON destinations, global/group/monitor scopes, and test delivery.
- [x] Support bearer authentication and HMAC request headers.
- [ ] Add arbitrary custom headers and configurable transition selection.
- [x] Encrypt endpoint URLs, bearer tokens, and signing keys with current-user Windows DPAPI.
- [ ] Evaluate credential-store references and non-Windows secret adapters before cross-platform support.
- [x] Create a durable delivery queue transactionally with recorded transitions.
- [x] Add stable event IDs, schema version, timestamps, target metadata, and outage duration.
- [x] Deliver asynchronously with request timeouts and bounded concurrency.
- [x] Retry transient network errors, rate limits, and appropriate server errors
  with capped exponential backoff and jitter; honor bounded `Retry-After` values.
- [x] Treat permanent failures explicitly, cap attempts/age, and show delivery history.
- [x] Add per-monitor alert snoozing and one-time scheduled maintenance, with visible suppression and no replay.
- [ ] Define cooldown behavior without dropping the final recovery notification.
- [x] Support optional HMAC signing and redact secrets from logs and exports.
- [ ] Add provider payload presets after generic delivery works.

Acceptance: an outage and recovery each create one logical event, including across
restart. Delivery is at least once: retries may reach the receiver more than once,
so event IDs support receiver deduplication. Webhook failures never block probes.
Tests cover retries, restart recovery, timeouts, redaction, and terminal failures.

## Milestone 5 — Desktop workflow and configuration portability

- [x] Add saved dark-default/light themes and card-density preferences.
- [x] Add a consistent vector Hostlight logo, sidebar/favicon branding, and generated multi-resolution Windows executable icons.
- [x] Add search/status/group filters and transactional bulk actions.
- [x] Make compact cards meaningfully denser and active silencing visibly amber in both themes.
- [x] Clarify skipped/invalid import rows, singular labels, and failed TCP timing.
- [x] Preserve same-session outages across pause/resume with excluded time and recovery routing.
- [ ] Add saved views and customizable layout beyond card density.
- [ ] Add desktop notifications and mute controls where supported.
- [ ] Add polished outage/recovery visual notifications with configurable screen
  position, auto-close duration, and a persistent-until-dismissed option. Keep
  notification health/initial-discovery semantics aligned with webhook transitions;
  investigate desktop overlay feasibility rather than claiming background popups work.
- [ ] Implement tested tray behavior if the platform spike supports it.
- [x] Add versioned monitor configuration import/export with preview and validation.
- [x] Exclude credentials and live history from configuration exports by default.
- [ ] Offer explicit portable mode with writable-path checks and clear credential behavior.
- [ ] Add support information and a sanitized diagnostic export.

Implementation update: **Monitor backups** provides a separate version-1 JSON
allowlist for monitor definitions and pause preferences. Imports are add-only,
skip duplicate host/check combinations, reject invalid batches, and default to
paused. Explicit opt-in starts only source-active monitors. Preview tokens expire
after 15 minutes and bind to the monitor-settings snapshot; commits revalidate
under the application lock and roll back failed writes. Export/import omit IDs,
credentials, webhook rules, maintenance windows, history, and appearance. Limits
are 1,000 monitors and 1 MiB. Native dialogs select files; live storage paths are
protected against export overwrite. Full workspace portability remains future work.

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
3. Recurring and global/group maintenance, acknowledgments, and dependency suppression
   (per-monitor timed silences and one-time windows are already implemented).
4. Route diagnostics and MTR-style monitoring.
5. Saved views, advanced tables, comparison charts, and customizable layouts.
6. A shared-core CLI/headless runner with clear process ownership and local IPC.
7. Verified macOS/Linux packages and platform-specific probe/credential adapters.
8. Signed automatic updates with rollback/recovery behavior.

Do not add a server, cloud accounts, remote agents, or plugin system without a
separate product decision. Keep those possibilities from complicating v1.

API-tooling direction: keep Hostlight focused on monitoring rather than building
a general-purpose Postman/Insomnia replacement. HTTP/HTTPS monitoring is the next
candidate feature after the current slice: status/latency checks first, with a
small test-request workflow shared with webhook diagnostics. Expected body/JSON
checks and TLS expiry can follow. Request collections, scripting, environment
management, and request chaining are not committed scope. None of these HTTP
features is implemented yet; they remain outside the first-release checklist above.

## Verification strategy

Latest completed implementation-slice checks (2026-10-10): `go test ./...`,
`go vet ./...`, 40 frontend tests, frontend production/type-check build, and
`wails build` passed. This is a development verification record, not completion
of milestone 6. The checklist reconciliation itself changes documentation only;
it does not establish new manual desktop, standard-user, or load-test evidence.

User UI/UX verification (2026-10-10): brief failures stayed below the unavailable
threshold; an outage/recovery pair shared its incident ID and delivered HTTP 200;
snoozing suppressed delivery with zero attempts; individual scoped destinations
routed correctly; restart persistence, duplicate-skipping/paused import, and light
theme passed. These are targeted checks, not a full release matrix. The subsequent
fix pass adds automated pause/resume/deletion/migration coverage and UI regressions;
the new paused-outage policy, denser cards, and amber silencing banner still need
a desktop spot check after restarting the backend.

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

Completed: TCP monitoring end to end, followed by native Windows ICMP. Subsequent
slices added persistence, incident history, scoped webhooks, per-monitor suppression,
and monitor backup/import. See the milestone checklists for remaining work; this
completed bootstrap sequence is retained as historical context, not the next task.

## Decision log

| Decision | Starting position | Revisit when |
| --- | --- | --- |
| Desktop framework | Existing Wails v2 project | A required feature cannot be supported cleanly |
| Frontend | Vue 3 + TypeScript + Vite | No planned migration |
| Package manager | npm and committed lockfile | A concrete workflow requires a change |
| Persistence | Atomic versioned JSON for targets; SQLite via modernc.org/sqlite for history/alerts | Build or measured performance requires it |
| Secrets | Current-user Windows DPAPI encrypted blobs; not Credential Manager references | Cross-platform support or portable credentials are required |
| Frontend updates | One-second snapshots with Vue composition state; Pinia/router not installed | Shared state, update load, or navigation complexity warrants a change |
| First platform | Windows | Windows acceptance criteria pass |
| Monitoring ownership | Go process independent of views | Headless execution is implemented |
| Alert delivery | Durable queue, at-least-once semantics | External integrations require additional guarantees |

Record significant library selections and deviations here or in short architecture
decision records under `docs/` so future changes retain their rationale.
