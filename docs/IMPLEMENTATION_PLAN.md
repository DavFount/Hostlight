# Hostlight implementation plan

Status reviewed: 2026-10-12. Implementation underway; not release-qualified.
The Windows preview includes ICMP/TCP/HTTP monitoring, target management and bulk paste,
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
recent charts use a bounded session-only sample window. SQLite also stores bounded
minute summaries for saved probe-history charts across aggregated time ranges.
This keeps configuration readable and separate from
high-volume monitoring data. Existing settings must remain importable when storage evolves.

Incident history implementation: `modernc.org/sqlite` supplies a pure-Go driver,
with schema version 6 (including persisted alert suppression, paused-outage accounting,
and probe-minute summaries), a background writer, paged queries, CSV export, and configurable
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
Hostlight will remain free and open source. Paid code signing is deferred and
does not block continued development or clearly labeled unsigned releases.

Windows is the first release platform. Keep the monitoring core portable and
verify macOS/Linux support separately before claiming those platforms are supported.

## First release scope

- ICMP and TCP probes with IPv4/IPv6 support.
- HTTP/HTTPS GET probes with exact status, optional text/JSON matching and certificate minimum validity, plus a non-saving request preview.
- Target CRUD, aliases, groups, bulk input, and saved configuration.
- Responsive status grid and searchable target list.
- Configurable probe intervals, timeouts, and failure/recovery thresholds.
- Distinct unknown, up, down, paused, and error states.
- Generic JSON webhooks, recovery alerts, retries, and delivery history.
- SQLite incident history, recent latency/loss charts, and bounded retention.
- Application settings, export/import, and a reproducible Windows build.

Defer DNS probes, certificate-chain expiry policies, degraded-state policies, MTR, dependency suppression,
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
- [x] Add Windows GitHub Actions and a shared fail-fast local verification script
  for formatting, Go tests/vet, package helper tests, frontend tests/build, and Wails build.
- [x] Record the first successful hosted CI run after pushing the workflows.
  User confirmed GitHub Actions verification completed successfully (2026-10-12).
- [x] Implement native Windows IP Helper ICMP and document the choice in README.
- [x] Test cancellation, DNS/address selection, IPv4/IPv6 localhost, and error classification.
- [ ] Record standard-user and adapter/permission failure checks on Windows.
- [x] Select pure-Go `modernc.org/sqlite` with versioned transactional migrations.
- [x] Verify native Windows tray feasibility with Shell_NotifyIcon and a dedicated
  message-pump thread, Wails OnBeforeClose, and explicit Quit lifecycle handling.

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
- [x] Add deterministic staggered starts (default 250 ms, configurable 0–5000 ms)
  and saved concurrency limits (1–128, default 32), applied after a full restart.
  HTTP previews share the active limit. Intervals begin after completion; checks
  wait for capacity without overlap or catch-up bursts.
- [ ] Measure supported target capacity and verify scheduling controls in the desktop UI.
- [x] Implement tested failure/recovery state decisions and backend incident/alert transitions.
- [ ] Implement sequenced frontend transition events if replacing snapshot polling.
- [x] Wire per-target and filtered bulk pause/resume into the live UI.
- [x] Show last check, latency, and diagnostic errors; show incident durations in history.
- [x] Add bounded, session-only recent failed-check statistics to monitor cards:
  last 100 completed availability checks, excluding DNS/local setup errors and
  cancelled/stale results. Pause/resume preserves the window; edits/restarts reset
  it. Label failed checks rather than packet loss or uptime, including HTTP policy failures.
- [x] User verified increasing recent-check counts, working pause behavior, and
  the rolling window capped at 100 checks (2026-10-12).
- [x] User verified resume continuity, edit/restart resets, and card layout in
  both themes and densities (2026-10-12).
- [x] Add per-monitor session totals alongside the
  recent 100-check window for overnight stability checks. Show completed
  availability checks, successes, failures, failure percentage, and an explicit
  session start timestamp. Track DNS/local setup errors separately; cancelled or
  stale results add nothing. Use counters rather than retaining every sample.
  Pause/resume preserves totals; edits and full application restarts reset totals
  and their start timestamp. Label the session scope and reset behavior clearly;
  persistence across restarts and uptime claims are outside this slice.
- [x] User verified session totals beyond 100 checks, independent recent/session
  percentages, preserved totals/start time across
  pause/resume, edit/restart resets, and readable layouts in both themes/densities.
  Light-theme screenshot shows 109 session checks with recent counts capped at
  100, for both successful and refused-connection monitors (2026-10-12).
- [x] User verified separate DNS/setup-error counts using `missing-host.invalid`:
  screenshot shows 2 setup errors, 0 availability checks, and no failure percentage
  (dash), with Check error status (2026-10-12).
- [x] Compact session statistics into labeled Passed/Failed/Failure rate columns
  and one recent-check row. Put start/reset/exclusion details behind Session details;
  keep nonzero setup-error counts visible in the collapsed summary. User confirmed
  the revised dark-theme card layout is clearer (2026-10-12); screenshot shows the
  Session details keyboard focus outline. User subsequently confirmed both themes,
  densities, keyboard expansion, and details staying open during updates.
- [x] Add elapsed observed outage duration to monitor cards, using open incident
  records through the last observation and excluding paused time. Keep the value
  held while paused; show pending evidence after resume and hide resolved/error
  states. Do not extrapolate a wall-clock timer between observations.
- [x] Manually verify card outage duration advances with checks, holds while
  paused, excludes the pause after resume, and disappears after recovery/edit;
  compare against incident history in both themes and densities.
  User confirmed all checks passed on 2026-10-12, including recovery/edit clearing
  and no outage duration for DNS/setup errors alone.
- [x] Discard stale results after edits, deletion, pause, and shutdown.
- [x] Reset live status and failure/recovery threshold counters after an observation
  gap, using the same boundary as incident history: greater than 30 seconds or
  twice the sum of interval and timeout, whichever is larger. Interrupt the old
  incident; discard checks spanning a gap
  without incrementing statistics. Preserve completed session totals and require
  fresh threshold evidence without fabricating a recovery across the gap.
- [x] Verify network changes and sleep/resume in Wails, including fresh threshold
  counters after gaps and no synthetic failed sample from a probe spanning sleep.
  User confirmed the slice's manual checks passed on 2026-10-12: session totals
  preserved, old outages interrupted, no fabricated recovery popups/webhooks,
  and actual completed failures counted during network disconnection/reconnection.

Acceptance: controlled success/failure sequences produce exactly the expected
transitions. Stopping or editing a target cannot apply stale results. A sustained
run has bounded goroutines, memory, and UI buffers. Target count and probe interval
limits are configurable and failures do not stall unrelated targets.

## Milestone 3 — Persistence and history

- [x] Create versioned SQLite migrations and transactional incident/alert storage.
- [x] Save target/group/pause configuration in versioned JSON and theme/density in local storage.
- [x] Persist observed incidents and monitoring interruptions.
- [x] Persist per-monitor/session minute summaries of completed availability
  checks, failures, and successful duration sum/min/max in SQLite schema 6, using
  the existing bounded background writer and incident transaction. Retain seven
  days with hourly age cleanup and a hard global cap of 50,000 summaries. Do not
  store response bodies, per-check diagnostics, or setup-error samples.
- [x] Implement configurable incident retention, record limits, and scheduled cleanup.
- [x] Add history filters, incident duration, date ranges, paging, and CSV export.
- [x] Add an expandable session-only SVG chart of the last 100 completed
  availability checks, successful latency mean/maximum, and failed-check markers.
  Fetch only expanded charts, provide accessible summaries and a check-values
  table, and break lines at failures, pauses, setup errors, and observation gaps.
  Use equal check spacing with timestamps, not an implied elapsed-time axis.
- [x] User verified updating recent charts capped at 100 while session totals
  continue, DNS/setup-error exclusion, pause/resume preserving data, edit resets,
  and working keyboard interaction (2026-10-12). Screenshots show dark-theme
  TCP/ICMP charts and a single fresh TCP sample after editing.
- [x] Complete recent-chart desktop checks for restart resets, mixed/all-failed
  windows, line breaks after pause/setup errors, successful-only latency averages,
  and layouts in both themes and densities.
  User confirmed all remaining checks passed on 2026-10-12.
- [x] Add saved probe-history charts in Incident history, with monitor selection
  (including deleted monitors while retained), last-hour/24-hour/7-day ranges,
  weighted successful mean and min/max, failed-check percentage, elapsed-time
  positioning, and accessible interval values. Show no observations as gaps;
  never join aggregate intervals with a continuous line. Include prior edits and
  sessions for the same monitor ID and disclose mixed check types.
- [x] User verified saved history includes previous edits and remains after
  deletion, while a replacement monitor has separate history (2026-10-12).
  Test 3 (#3) retained its Google/Yahoo observations after deletion; Test 4 (#6)
  shows separate successful Yahoo observations. Screenshots also confirm mixed
  failure counts, passing results, and monitor selection in the dark theme.
- [x] User verified saved probe history survives restart, existing incidents and
  settings remain intact, counts do not increase while paused (including a
  setup-error monitor), and keyboard operation works (2026-10-12).
- [x] User verified no-data selector behavior: monitors are absent until saved
  observations exist (2026-10-12). This covers initial no-data behavior; an empty
  date range for a monitor with older retained data was not separately exercised.
- [x] User screenshot verifies all-failed chart rendering: All Fail (#8) shows
  3 completed checks, 3 failed, 100.00% failed, a full-height failure bar, and no
  successful-duration marks (2026-10-12).
- [x] Complete saved probe-history checks for active setup-error exclusion,
  range switching, interval-table values/access, and both themes at narrow sizes.
  User confirmed all remaining listed checks passed on 2026-10-12.
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

- [x] Add a reusable in-app confirmation/alert dialog component to replace browser
  native dialogs. Start with monitor deletion, webhook destination deletion, and
  history-retention reduction; use consistent light/dark styling, clear action
  labels, and destructive-action emphasis. Support accessible dialog naming,
  keyboard focus trapping/restoration, Escape/cancel, safe initial focus, and
  prevention of duplicate submissions. Verify cancellation leaves data unchanged
  and confirmation performs the intended action once. Keep native file dialogs.
- [x] Manually verify custom confirmations in Wails: Cancel/Escape, safe initial
  focus, Tab/Shift+Tab containment, restored focus, one confirmed operation,
  background interaction blocking, and both themes at narrow window sizes.
  User confirmed all manual checks passed on 2026-10-12.

- [x] Keep primary navigation focused on monitors and incident history; group
  appearance, scheduling, desktop behavior, history retention, webhook configuration/delivery logs/tests,
  and monitor backups under Settings. Put support diagnostics in About Hostlight.
- [x] Add native File/View/Help menus with settings, backups, page navigation,
  explicit Quit, and About Hostlight; clean up frontend navigation listeners.
- [x] Remove duplicate sidebar Settings/About and settings-page Quit commands.
  Keep File/Help and tray actions; show General settings sections without collapsed
  controls, with responsive section links and keyboard-focusable scroll targets.
- [ ] Verify native menu navigation/shortcuts and the reorganized pages in Wails.

- [x] Add saved dark-default/light themes and card-density preferences.
- [x] Add a consistent vector Hostlight logo, sidebar/favicon branding, and generated multi-resolution Windows executable icons.
- [x] Add search/status/group filters and transactional bulk actions.
- [x] Make compact cards meaningfully denser and active silencing visibly amber in both themes.
- [x] Clarify skipped/invalid import rows, singular labels, and failed TCP timing.
- [x] Preserve same-session outages across pause/resume with excluded time and recovery routing.
- [x] Add named local saved views for search, status, and group filters with explicit
  updates, rename/delete controls, validation, and persistence tests.
- [ ] Add customizable layout beyond card density.
- [ ] Manually verify saved-view switching, restart persistence, and missing-group behavior.
- [x] Explain recognized platform check errors in cards/popups using plain-language
  reasons, retaining original diagnostics in expandable technical details. Cover
  refused connections, timeouts, routing, DNS, permissions, and ping setup failures;
  unknown messages remain intact and HTTP diagnostics retain their safe wording.
- [ ] Manually verify failure explanations and keyboard-accessible technical details
  in cards/popups, both themes, and compact/comfortable layouts.
  Partial user verification (2026-10-12): screenshot confirms a refused localhost
  TCP check displays the explanation, failed-latency dash, and collapsed Technical
  details control. User reports keyboard interaction works as expected and Ctrl+Q
  closes the window. Popup, theme/density, and pause/recovery checks remain open.
- [ ] Add desktop notifications and mute controls where supported.
- [x] Add session-only in-app outage/recovery popups with four corner positions,
  persisted enable/duration controls, preview, dismissal, and bounded transition feed.
  Respect maintenance/snoozing; suppress initial discovery and orphan recoveries.
- [x] Include monitor name, host, and check failure reason in outage popups;
  include observed outage duration (excluding pauses) in recovery popups.
- [ ] Manually verify in-app popups in both themes, persistent dismissal, timing,
  pause/resume recovery, and silence windows using the running Wails application.
- [ ] Add polished outage/recovery visual notifications with configurable screen
  position, auto-close duration, and a persistent-until-dismissed option. Keep
  notification health/initial-discovery semantics aligned with webhook transitions;
  investigate desktop overlay feasibility rather than claiming background popups work.
- [x] Implement Windows tray Show/Quit controls, explicit hide, persisted opt-in
  close-to-tray, single-instance restore, icon availability fallback, and cleanup.
  Native smoke covers add/remove, activation, and icon-loss recovery without
  restarting Explorer. Other platforms retain ordinary close-to-exit behavior.
- [ ] Manually verify Wails close/hide/restore, checks and webhook delivery while
  hidden, second-launch restore, real Explorer restart, and explicit quit cleanup.
- [x] User confirmed minimizing to the system tray works (2026-10-11).
  This does not establish the remaining hidden-monitoring and lifecycle checks.
- [x] Add versioned monitor configuration import/export with preview and validation.
- [x] Exclude credentials and live history from configuration exports by default.
- [ ] Offer explicit portable mode with writable-path checks and clear credential behavior.
- [x] Add support information and an explicit sanitized JSON diagnostic export:
  allowlisted runtime/platform, health flags, aggregate counts, and queue sizes;
  no monitor addresses, names, secrets, paths, raw errors, or logs. Protect live
  storage export paths and test privacy, storage failures, and cancelled exports.
- [ ] Manually verify support refresh and native diagnostic save/cancel dialogs.

Implementation update: **Monitor backups** provides a separate version-4 JSON
allowlist for monitor definitions and pause preferences. Imports are add-only,
skip duplicate host/check combinations, reject invalid batches, and default to
paused. Explicit opt-in starts only source-active monitors. Preview tokens expire
after 15 minutes and bind to the monitor-settings snapshot; commits revalidate
under the application lock and roll back failed writes. Export/import omit IDs,
credentials, webhook rules, maintenance windows, history, and appearance. Limits
are 1,000 monitors and 1 MiB. Native dialogs select files; live storage paths are
protected against export overwrite. Version-1 ICMP/TCP backups remain readable;
version 2 carries HTTP expected-status settings and remains readable; version 3
carries response-body rules and remains readable. Version 4 adds certificate minimum
validity; live certificate observations are excluded. Full workspace portability remains future work.

Acceptance: reopening restores the intended workspace; imports cannot silently
replace configuration; exported configuration contains no secrets. Monitoring
continues when minimized. Closing behavior is explicit and platform-tested.

## Milestone 6 — First release verification

Release-readiness follow-up (2026-10-12): user observed a full history queue
during desktop testing. Treat this as a release blocker pending repeat testing.
The writer now drains up to 64 queued events per transaction, preserving event
order and publishing visual transitions only after commit. Regression coverage
checks pause/recovery ordering, full rollback without popup publication, and
draining 1,000 queued samples on shutdown. Queue capacity remains bounded at
1,024 and dropped-observation warnings remain visible. Desktop soak after this
change, remaining platform checks, and release documentation must pass before
release qualification. Build the post-release roadmap after those gates close.
Automated follow-up passed: full shared verification and 11,904 checks in a
60-second backend repeat, 20 resolved incidents/40 delivered webhooks, no history
errors, and zero queue length at one-second samples. Desktop repeat remains open.
User desktop repeat subsequently passed the 30-minute warning-free gate:
200 monitors shown, visible cards at 1,827–1,828 checks, no reported error.
Visible Load 001–003 were available with 117 failures each; controlled-outage
attribution and resolved incident confirmation after batching remain pending.
This does not close longer soak/retention or remaining release platform gates.
Subsequent screenshot showed 20 matching incidents and visible recovered Load
007/019 entries with expected connection-refused/recovery-threshold reasons.
Controlled outage/recovery desktop follow-up after batching accepted as passed;
the screenshot is supporting evidence rather than a full incident CSV audit.

- [ ] Exercise a target matrix: IP/hostname, IPv4/IPv6, reachable/unreachable,
  refused port, DNS failure, invalid settings, adapter loss, sleep/resume, and restart.
- [ ] Benchmark an initial 200-target workload at one-second intervals on a recorded
  Windows reference machine; measure CPU, memory, scheduling delay, and UI responsiveness.
- [x] Add and run an isolated, opt-in 200-monitor Windows TCP loopback workload
  with real SQLite storage, controlled failures/recoveries, delivered webhooks,
  process CPU, sampled Go heap/queue/goroutines, cadence overhead, and API timing.
  Recorded 11,811 checks in 60 seconds (59–60 per monitor), 20 resolved incidents,
  and 40 delivered webhooks without storage errors (2026-10-12).
  User's local repeat passed with 12,000 checks (60 per monitor), 40 delivered
  webhooks, queue peak 76/1,024, and snapshot API P95 4.008 ms. User reported
  normal browser/ticket-work responsiveness during the run (2026-10-12).
  See `docs/LOAD_VERIFICATION.md`; this does not establish desktop responsiveness,
  process working set, remote-network capacity, or an extended soak.
- [ ] Run an extended soak and verify retention, queue bounds, and cleanup.
- [x] Add manual-dispatch release automation: tested Windows ZIP, embedded version,
  SHA-256 checksums, draft-first upload/publication, and GitHub-created tags without
  local tag/push commands. Default to prerelease; never overwrite existing versions.
- [x] Verify the first hosted publication and basic downloaded-package startup.
  User confirmed the 0.1.0 prerelease runs as expected from the extracted ZIP
  (2026-10-12). This does not establish checksum/version, fresh-machine, or soak checks.
- [ ] Add an installer and curated release notes.
- [ ] Revisit optional code signing when funding or a suitable free option is available;
  keep unsigned distribution explicitly labeled in the meantime.
- [ ] Verify fresh-machine startup, WebView2 requirements, upgrade, and uninstall behavior.
- [x] Use standalone ZIP distribution for the initial prerelease; defer paid code
  signing while keeping Hostlight free and open source (2026-10-12).
- [ ] Document supported platforms, configuration location, limits, and known issues.

Acceptance: release checks pass on the supported Windows platform, no critical
monitoring/alerting defects remain, and the README reflects implemented behavior.
Treat 200 targets as an initial benchmark, not a supported capacity claim until measured.

Desktop load fixture slice (2026-10-12): added a disposable 200-monitor loopback
workspace, separate development data/WebView/instance identity, and controlled
20-endpoint outage/recovery commands. Automated isolation/configuration/socket
checks cover the helper. Shared verification passed (131 frontend tests, Go
tests/vet, release-tool checks, and Windows Wails build); a standalone helper
smoke test connected to all 200 endpoints. After adjusting fixture socket cleanup,
the user confirmed responsive desktop/background work with 200 monitors; visible
cards showed 424 successes each, zero failures, and incident history was empty
(approximately seven minutes). Short desktop responsiveness passed. Extended soak,
recorded desktop CPU/working set remain pending. Fixture outage/recovery passed:
the user's CSV contained exactly 20 resolved incidents for Load 001–020, each
with 44 seconds observed duration, and none for Load 021–200;
follow `docs/LOAD_VERIFICATION.md` before marking the full benchmark complete.

## HTTP monitoring — delivered slices

- [x] Add full HTTP/HTTPS URL validation, exact expected status (default 200), and GET probes.
- [x] Apply existing thresholds, pause/resume, scoped alerts, silencing, and history semantics.
- [x] Verify HTTPS certificates, refuse redirects, bound timeouts/header size, and avoid body storage.
- [x] Add a one-request preview that does not save monitors or create incidents/alerts.
- [x] Add HTTP bulk input and preserve legacy configuration/backup read compatibility.
- [x] Test local status/redirect/timeout/TLS/cancellation behavior, live recovery, and portability.
- [ ] Manually verify HTTP outage/recovery, preview, and backup import in the Windows desktop UI.
- [x] Add case-sensitive text containment and typed JSON-Pointer equality assertions.
- [x] Bound body reads to 1 MiB and the probe timeout; fail oversized/incomplete responses without retaining content.
- [x] Measure full-check timing for body rules and preserve header timing for status-only probes.
- [x] Add version-3 settings/backups with version-1/2 read compatibility and legacy body-rule rejection.
- [x] Test strict types, exact numeric comparison, missing fields, pointer escapes/arrays, body limits/timeouts, and round trips.
- [ ] Manually verify text/JSON health changes and body-rule backup imports in the Windows desktop UI.
- [x] Show verified leaf-certificate expiry in cards and request previews without restoring/exporting stale observations.
- [x] Add optional 0–365-day HTTPS minimum-validity policy using normal health thresholds and alerts.
- [x] Keep certificate trust/hostname/time validation enabled and test trusted local TLS, expiry boundaries, expired/untrusted certificates, and portability.
- [x] Add version-4 settings/backups with version-1/2/3 read compatibility and legacy certificate-policy rejection.
- [ ] Manually verify expiry-policy failure/recovery and certificate displays in the Windows desktop UI.
- [ ] Add optional authentication and certificate-chain expiry policies in later slices.
- [ ] Reuse/extend request diagnostics for webhook response inspection without exposing secrets.

## Later milestones

Prioritize these using actual usage after the first release:

1. Extend HTTP checks with authentication and optional redirect policies; add DNS expected-answer checks and certificate-chain expiry diagnostics.
2. Degraded-state latency/loss policies with rolling windows and hysteresis.
3. Recurring and global/group maintenance, acknowledgments, and dependency suppression
   (per-monitor timed silences and one-time windows are already implemented).
4. Route diagnostics and MTR-style monitoring.
5. Saved views, advanced tables, comparison charts, and customizable layouts.
6. Release 2: optional Windows service using a shared core and secured local IPC;
   keep standalone mode. Service owns probes/history/webhooks and the GUI attaches
   without starting a second engine. Include boot startup, RMM health/heartbeat,
   installation IDs for ticket correlation, and explicit service-account credential
   provisioning. No service execution or account migration is part of release 1.
7. Verified macOS/Linux packages and platform-specific probe/credential adapters.
8. Signed automatic updates with rollback/recovery behavior.

Do not add a server, cloud accounts, remote agents, or plugin system without a
separate product decision. Keep those possibilities from complicating v1.

API-tooling direction: keep Hostlight focused on monitoring rather than building
a general-purpose Postman/Insomnia replacement. HTTP/HTTPS status/latency checks
with optional text/JSON assertions and a monitor-specific test-request preview are
implemented, along with verified server-certificate expiry and minimum-validity
policies. Sharing that workflow with webhook diagnostics remains
planned. Request collections, scripting, environment
management, and request chaining are not committed scope. The delivered HTTP
slice extends the original first-release scope by explicit user direction.

## Verification strategy

Load-verification slice (2026-10-12): isolated 60-second 200-monitor TCP workload
passed, including actual persistent summaries, bounded queue health, outage and
recovery transitions, and 40 delivered local webhook events. The opt-in Windows
test is skipped during ordinary verification. Measured backend results and their
scope are recorded in `docs/LOAD_VERIFICATION.md`; desktop/load-soak gates remain open.
The shared `scripts/verify.ps1` pipeline also passed: formatting, 131 frontend
tests, production/type checks, Go tests/vet, release-tool checks, and Windows
desktop build. The load test is opt-in and skipped by the ordinary suite.

Persistent probe-history slice (2026-10-12): 131 frontend tests, frontend
production/type checks, full Go tests/vet, and Windows Wails build passed.
Storage tests cover minute
aggregation, weighted means, successful-only durations, monitor isolation,
restart persistence, schema-5 migration preserving incidents, hard global cap,
and seven-day retention. Legacy v1–v5 migration fixtures reconstruct their old
schemas and pass the full regression suite. User confirmed the listed desktop
acceptance checks passed on 2026-10-12: restart persistence, existing incidents
and settings, edit/deletion retention and replacement isolation, paused counts,
initial no-data behavior, all-failed rendering, active setup-error exclusion,
range switching, interval-table access, keyboard operation, and both themes at
narrow sizes. Raw recent charts and session totals still reset
on edit/restart; saved minute summaries are separate and remain subject to retention.

Recent chart slice (2026-10-12): 126 frontend tests, frontend production/type
checks, Go tests/vet, and Windows Wails build passed. Coverage includes ring
ordering/eviction, detached API snapshots, pause/edit lifecycle, setup/gap line
breaks, successful-only latency summaries, empty/all-failed/error states, stale
response rejection, and stopping polling when collapsed. User verified chart
updates/capping, setup-error exclusion, pause/resume preservation, edit resets,
and keyboard interaction on 2026-10-12. User also confirmed all remaining desktop
checks passed: restart resets, mixed/all-failed windows, line breaks,
successful-only averages, and both themes and densities.
Samples are session-only;
persistent sample storage and aggregation remain unfinished first-release work.

Observation-gap slice (2026-10-12): Go tests/vet and Windows Wails build
(including frontend production/type checks) passed. Additional targeted tests
passed for fresh threshold evidence, exact gap boundaries, discarded spanning
checks, preserved session totals, interrupted duration, and absence of fabricated
recovery popups/webhooks. User confirmed Windows sleep/resume and network-change
manual checks passed on 2026-10-12, including fresh threshold evidence, preserved
session totals, interrupted outage history, and no fabricated recovery alerts.
This is elapsed-observation-gap handling, not native power-event detection.

Observed outage card slice (2026-10-12): Go tests/vet, 123 frontend tests,
frontend production/type-check build, and Windows Wails build passed. Additional
targeted Go tests passed for gap and edit interruption. Coverage verifies paused
time exclusion, held observations, recovery removal, unavailable storage, and
duration formatting. User confirmed all desktop acceptance checks passed on
2026-10-12: history agreement, pause/resume exclusion, recovery/edit clearing,
DNS/setup-error behavior, and both themes and densities.

Custom confirmation slice (2026-10-12): 121 frontend tests, frontend
production/type-check build, and Windows Wails build passed. Coverage includes monitor/webhook deletion,
retention cancellation/confirmation/failure, duplicate-submit protection, safe
initial focus, restored focus after busy-state changes, escaped content, optional
acknowledgement mode, and cancellation on navigation/unmount. User confirmed all
desktop manual acceptance checks passed on 2026-10-12, including native WebView
focus containment, cancellation, focus restoration, background interaction blocking,
single confirmed operations, and both themes at narrow window sizes.

Session-total statistics slice (2026-10-12): Go tests/vet, 109 frontend tests,
frontend production/type-check build, and Windows Wails build passed. Tests cover
20,000 successes plus 20 failures, independent recent-window eviction, separate
setup-error counts, pause/resume preservation, edit/configuration-reload resets,
local start-time display, nonzero failure percentages below 0.01%, and collapsed
details that preserve their open state during polling updates. User confirmed the
session-total, pause/resume, edit/restart, keyboard, and theme/density checks;
screenshot confirms 109 session checks versus 100 recent checks (2026-10-12).
User also verified separate DNS/setup-error counting: `missing-host.invalid`
shows 2 setup errors with 0 availability checks and a dash for failure rate.

Recent-check statistics slice (2026-10-12): Go tests/vet, 105 frontend tests,
the frontend production/type-check build, and the Windows Wails build passed. Coverage includes bounded
window eviction, pause/resume continuity, reset on edit, excluded setup/cancelled
results, and unavailable-versus-empty statistics. User confirmed pause behavior,
increasing counts, and the 100-check cap (2026-10-12). Resume continuity,
edit/restart resets, and theme/density layout verification remain open. Session
totals were deferred from this rolling-window slice to the subsequent slice.

Failure-message slice (2026-10-12): 101 frontend tests and the frontend
production/type-check build and Windows Wails build passed. Component coverage includes preserved raw
diagnostics, escaped markup, card threshold state, and outage popup explanations.
Native layout and keyboard verification remain on the desktop smoke checklist.

CI follow-up (2026-10-12): Go sources now require LF checkouts via
`.gitattributes` so Windows CRLF conversion does not fail `gofmt` checks.
Verification builds frontend assets before Go tests because `main.go` embeds
`frontend/dist`. User confirmed GitHub Actions verification completed successfully
on 2026-10-12 and the app ran via `wails dev` and was verified locally.
This records successful local use; individual startup/bindings/shutdown, menu,
tray lifecycle, and release-matrix checks remain open until specifically recorded.

Latest completed implementation-slice checks (2026-10-11): `go test ./...`,
`go vet ./...`, 83 frontend tests, frontend production/type-check build, and
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
| Probe charts | Native SVG with accessible summaries/tables; bounded recent samples and persisted minute summaries | Chart interaction requirements exceed the small bounded plots |
| Monitoring ownership | Go process independent of views | Headless execution is implemented |
| Alert delivery | Durable queue, at-least-once semantics | External integrations require additional guarantees |

Record significant library selections and deviations here or in short architecture
decision records under `docs/` so future changes retain their rationale.
