# Hostlight

See what's reachable. Know when it changes.

Hostlight is a desktop network-monitoring application being built with Go, Wails,
Vue 3, and TypeScript. Inspired by [vmPing](https://github.com/r-smith/vmPing),
it aims to make the status of multiple hosts and services easy to see at a glance.

## Project status

The first working preview includes a Ping/ICMP and TCP monitor grid, target editing and removal,
search and status filters, pause/resume controls, and configurable timing and
failure/recovery thresholds. Dark is the default theme; light mode and card spacing
preferences are saved locally.

Use **Add multiple** to paste up to 200 hosts at a time, one per line. Optionally
assign a friendly name with `Office gateway | 192.168.1.1`. Choose a shared check
type, port (for TCP), group, and timing policy, then review validation before adding.
Duplicates within the list or existing workspace are highlighted; the same host
can still have distinct ICMP and TCP monitors or TCP monitors on different ports.
An invalid line or failed settings write leaves the entire batch unapplied.

The group selector narrows the grid. **Pause shown** and **Resume shown** apply to
the monitors matching the current group, search, and status filters. Resuming a
mixed selection leaves already-active monitors running without resetting them.

Targets and pause settings are saved automatically in a versioned JSON file under
the OS user configuration directory (`%AppData%\Hostlight\targets.json` on Windows).
The UI displays the exact path. Active monitors restart with fresh checks when
Hostlight opens; paused monitors stay paused. Settings writes use a temporary file
and replacement, and unreadable configurations are preserved with a visible error.

New monitors default to **Ping (ICMP)**, which requires only a hostname/IP address.
Select **TCP service** to check a specific port. Existing saved TCP definitions
retain their type and port. Windows ping uses the native IP Helper API for IPv4
and IPv6, without spawning a command or parsing localized output. Dual-stack DNS
names prefer IPv4; IPv6-only names and scoped IPv6 addresses are supported.

An ICMP timeout can mean a firewall blocks echo requests, even when a service is
available. Ping measures echo round-trip time; TCP measures connection-attempt time.
Windows native ping calls finish within the configured timeout; pause, edits, and
shutdown discard their results even if the native request is still finishing.
ICMP on macOS/Linux, metric history/charts, and advanced platform
error handling are still planned.
This preview is intended for development and evaluation.

## Incident history

Open **Incident history** to review outages, recovery, and interrupted monitoring.
History is saved in `history.db` alongside `targets.json`, using SQLite with a
versioned schema and a pure-Go driver. No separate database server is required.
Only one desktop instance runs at a time to keep monitoring and storage ownership clear.

- An incident opens when the configured failure threshold establishes unavailable status.
- Recovery closes it only after the configured success threshold is met.
- Pausing stops probe traffic but preserves an open outage and its alert association.
  Resuming gathers fresh threshold evidence: continued failure keeps the same outage
  without another outage notification; confirmed recovery can send its recovery webhook.
  The unobserved interval from the last check to resume is excluded from observed duration.
  History labels open paused outages explicitly. This continuity applies within the
  current application session; shutdown/restart still interrupts observation.
- Deletion, settings changes, local check errors, and shutdown interrupt
  the incident at its last observation; they do not imply recovery.
- A restart or long observation gap also interrupts the previous incident.
  A gap means no results for more than the greater of 30 seconds or twice the
  sum of the configured interval and timeout. A continued outage starts a new observed segment.
- Durations measure the observed status period, not an SLA or proof of continuous
  downtime. Initial failures before the failure threshold are not counted.

Search by monitor, host, or group; filter by outcome and date range; and export
matching records to CSV using the native save dialog. The grid shows 100 records
per page; exports include all matching records up to 50,000, with UTC timestamps.
CSV text is escaped to prevent spreadsheet formulas from being interpreted.

Completed history defaults to 90 days. The history screen offers 7, 30, 90, and
365 days; reducing retention requests confirmation before removing older completed
incidents. Cleanup runs on startup, after retention changes, and hourly. Completed
records are also bounded to approximately the most recent 50,000 records; active
incidents are preserved. SQLite may reuse freed pages rather than shrink the file.

History writes use a bounded background queue so disk operations do not block
probe checks. Storage errors or queue overflow are shown in the application;
history may be incomplete after such an error. Closing drains pending writes for
up to five seconds. An abrupt exit can lose queued observations, and unfinished
incidents are marked interrupted on the next start.

## Webhook alerts

Open **Webhook alerts**, enter your JSON receiver's HTTPS endpoint, enable alerts,
and save. **Send test using saved settings** queues a real test request; its outcome
appears in delivery history. Add up to 50 named destinations, each with optional
bearer authentication and HMAC signing. Slack, Discord,
and Teams-specific payload formats are not implemented yet.

For each destination choose **All monitors (global)** or **Selected groups and/or
individual monitors**. You can select multiple groups, multiple individual
monitors, or both. Selections are combined: a destination sends once when either
the group or monitor matches, even if both match. Global and scoped destinations
are additive, not overrides. Separate destinations remain independent, even if
you enter the same URL in both; only selection overlap within a destination is
deduplicated.

Group rules automatically include future monitors in that group. Groups are
matched by their exact name; renaming or moving a monitor's group does not rename
the routing rule. **Ungrouped** selects monitors without a group. Individual
assignments use stable monitor IDs rather than hostnames; renaming a monitor
preserves its assignment, while deleting it does not transfer that assignment to
a replacement. Removed selections remain visible in the editor for cleanup.
The previous single webhook migrates automatically to **Default webhook** with
global scope; saved credentials and pending deliveries are preserved.

- A confirmed `up` → `down` transition queues an outage event; the corresponding
  recovery queues an `up` event. Existing monitor thresholds apply.
- Initial discovery is silent, even if a monitor starts down. Pausing is not recovery;
  an existing outage remains eligible for recovery after resume. Edits, removals,
  local check errors, observation gaps, and restarts interrupt rather than recover it.
- URLs, tokens, and signing keys are encrypted using current-user Windows DPAPI
  before storage in SQLite. They are not returned to the webview. This is not a
  portable credential backup or Windows Credential Manager entry; other platforms
  cannot save webhook settings yet. See [Microsoft's DPAPI documentation](https://learn.microsoft.com/en-us/windows/win32/api/dpapi/nf-dpapi-cryptprotectdata).
- HTTPS verifies certificates. HTTP requires explicit permission and should only
  be used on trusted internal networks. Redirects are never followed.
- Saving settings cancels pending deliveries for that destination only, including when disabling alerts.
  Requests already in flight may complete. A recovery is only queued for an outage
  alerted under the same saved settings; old outages never switch destinations.
  Removing a destination cancels its pending deliveries and retains its delivery
  history. Tests target the selected saved destination regardless of scope.

The incident change and its delivery are committed in one SQLite transaction.
A separate worker sends one request at a time with a 10-second timeout. Network/TLS
failures, HTTP 408, 429, and 5xx retry with exponential backoff (starting at five
seconds), jitter, and bounded `Retry-After` support (up to one hour). Other non-2xx
responses are terminal. Delivery stops after eight attempts or 24 hours. Pending
requests resume after restart; Hostlight must be open to send them. FIFO delivery
preserves outage/recovery order within each destination. A destination awaiting
retry does not block another destination's due events; a single worker still sends
one HTTP request at a time.

Delivery is **at least once**, not exactly once. Receivers must deduplicate the
stable `id` / `X-Hostlight-Event-ID` when a response is lost. JSON schema version 1
contains `schemaVersion`, `id`, `event` (`down`, `up`, or `test`), UTC `time`,
`incidentId`, a `target` snapshot, and `durationSeconds` (observed outage duration
on recovery). With a signing key, `X-Hostlight-Signature` is `sha256=` followed by
the hex HMAC-SHA256 of the exact request body.

The screen shows the latest 100 deliveries, including destination and monitor names, without response bodies or endpoint
details. Completed delivery records are retained for 30 days, bounded to about
10,000, except outage markers needed for active incidents. The pending queue is
limited to 10,000. Storage errors/overflow are visible in the app; the in-memory
observation queue can still lose transitions during a crash or disk failure.

## Maintenance and alert snoozing

Choose **Silence alerts** on a monitor card to snooze its notifications now or
schedule a one-time maintenance window. Snoozes accept 1 minute to 7 days;
maintenance windows may last up to 30 days and start within the next year.
An optional reason is shown in the window list and suppressed delivery records.
These controls apply to every webhook destination that matches that monitor.

Checks, availability status, and incident history continue normally. Cards show
active suppression or the next scheduled window without changing the monitor's
availability indicator. Scheduled times are entered in the desktop's local time
zone and saved as absolute UTC timestamps; the window list includes the local
time-zone abbreviation. These are not recurring schedules. Review times carefully
around daylight-saving changes. Windows persist across application restarts.

Suppression policy:

- Window starts are inclusive and ends are exclusive. Overlapping windows combine;
  ending one does not end another. Active and upcoming windows can be cancelled.
- Outage notifications during a window are recorded as `suppressed`, with zero
  send attempts. A recovery for an entirely suppressed outage is not sent.
- A recovery during a window is also suppressed if its outage was already alerted.
- Pending notifications and retries that cross a window are permanently suppressed,
  including when the window passes while Hostlight is closed. Suppression applies
  before sending; an HTTP request already in flight may still complete.
- Suppressed notifications are never replayed, even when a window is cancelled.
  There is **no catch-up notification** if a monitor remains down at the end.
  Inspect availability before ending planned work; the next actual confirmed
  outage/recovery cycle uses normal alert routing.
- Explicit **Send test** requests bypass silencing, since they are operator-initiated.

SQLite schema v4 preserves existing destinations, incident history, and queued
deliveries. Ended windows are retained internally for 30 days so delayed records
and retries still respect the interval. Up to 50 active/upcoming windows per
monitor and 10,000 total are supported. Reasons are ordinary operational metadata,
not encrypted secrets; do not put passwords or tokens in them.

Schema v5 adds paused-outage tracking and excluded observation time, preserving
existing incident records and webhook delivery associations.

Monitor cards show successful latency only; failed TCP attempts display a dash
and explain the elapsed attempt time in the diagnostic text/tooltip. The existing
webhook schema-v1 `target.latency` field remains the latest TCP attempt duration
in milliseconds, including failed attempts; it is not proof of a remote reply.
Use `target.message` and the transition event to interpret the result. A pending
failure/recovery threshold can leave the established availability state unchanged
even when the latest individual check has the opposite outcome.

## Monitor backups and import

Open **Monitor backups** to export monitor definitions to a versioned JSON backup,
or choose an exported backup to review before importing. This is a monitor-only
backup, not a complete workspace or a copy of the internal `targets.json` format.

Included: names, hosts, groups, ICMP/TCP settings, timing/threshold rules, and source
pause preferences. Excluded: monitor IDs, live results, webhook destinations and
credentials, maintenance windows, incident/delivery history, and appearance settings.
The file still contains internal hostnames/IP addresses; store and share it accordingly.

Import is add-only. Existing monitors are never overwritten. Duplicate host/check
combinations within the file or already in the workspace are skipped, using the
same hostname/IP normalization as bulk add. Different TCP ports and ICMP remain
distinct checks. Invalid settings block the whole import; nothing is partially added.
The preview shows check settings, source pause preferences, and each entry's result.

Imported monitors receive fresh IDs and stay **paused by default**. An explicit
checkbox starts monitors that were not paused in the source backup; source-paused
monitors stay paused. Starting performs real network checks. Existing destination
rules for all monitors or matching groups apply to future availability changes;
individual monitor assignments are not transferred.

The preview uses an in-memory copy of the selected file and expires after 15 minutes.
Changing monitor definitions or pause preferences requires choosing the file again;
routine background check results do not invalidate it. Imports are saved as one
transactional settings replacement, with rollback on failure. Exports also use a
temporary file and replacement; live configuration/database paths are protected.

Limits: 1,000 monitors and 1 MiB per backup. Unsupported versions, unknown JSON
fields, and trailing documents are rejected rather than guessed. Windows UTF-8
BOM files are accepted. JSON files edited by hand must use the exported format
(`format: hostlight-monitors`, `version: 1`); existing internal settings files are
still loaded normally at application startup.

Initial goals:

- A responsive, color-coded grid of monitored targets.
- ICMP ping and TCP port checks with configurable intervals and timeouts.
- Status transitions based on consecutive failures and successes.
- Webhook notifications for outages and recovery.
- Saved targets, friendly names, and groups.
- Persistent incident history and latency charts.

Additional probe types, recurring maintenance schedules, and advanced alert rules may follow.

See the [implementation plan](docs/IMPLEMENTATION_PLAN.md) for architecture,
delivery milestones, acceptance criteria, and the planned first release scope.

## Prerequisites

- Go 1.26 or newer, as declared in `go.mod` (required by the SQLite driver).
- Node.js 24 LTS and npm (the configured frontend package manager).
- Wails v2 CLI; use the version required by `go.mod`.
- Git.
- Platform dependencies described in the [Wails installation guide](https://wails.io/docs/gettingstarted/installation/).
  On Windows, this includes the Microsoft Edge WebView2 runtime.

Install the CLI for the current project version:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
wails doctor
```

Ensure the `bin` directory inside your Go workspace is on `PATH`. On Windows,
the default is `%USERPROFILE%\go\bin`. Restart your terminal after changing `PATH`.

## Development

Run these commands from the project root, which contains `wails.json`:

```powershell
cd frontend
npm ci
cd ..
wails dev
```

Wails runs the Go application and Vite development server together, with live
frontend updates. It also generates bindings under `frontend/wailsjs` for calling
Go methods from TypeScript. Update the Go API and let Wails regenerate these
bindings rather than editing them manually.

Application settings and frontend commands are configured in `wails.json`.

## Build

```powershell
wails build
```

Build output is placed in `build/bin/`. On Windows, the executable is
`build/bin/hostlight.exe`. Build and verify releases on each target operating system.

## Checks

Check Go code:

```powershell
go test ./...
go vet ./...
```

Type-check and build the frontend:

```powershell
cd frontend
npm run build
npm test
```

Backend tests cover local TCP connectivity, failure and recovery thresholds,
cancellation, input validation, configuration round trips, unreadable settings,
and rollback when settings cannot be saved. ICMP tests include probe dispatch,
type-specific validation, legacy configuration compatibility, error classification,
and native Windows IPv4/IPv6 localhost replies.
Bulk-operation tests cover previews, duplicate detection, limits, persistence,
stale selections, and rollback when a group change or batch cannot be saved.
History tests cover schema initialization, outage/recovery, monitoring gaps,
restart interruption, retention, filters, queue overflow, and CSV formatting.
Vue component tests verify history presentation, paging, export, and retention controls.

## Project layout

```text
hostlight/
├── app.go             Go methods exposed to the frontend
├── main.go            Desktop application entry point
├── go.mod             Go dependencies and minimum Go version
├── wails.json         Wails project configuration
├── build/             Icons, platform metadata, and installer sources
│   └── bin/           Generated application output (ignored)
└── frontend/
    ├── src/           Vue components, TypeScript, and styles
    ├── wailsjs/       Generated Go bindings and Wails runtime
    ├── package.json   Frontend dependencies and scripts
    └── dist/          Generated frontend bundle (ignored except placeholder)
```

Commit dependency lockfiles (`go.sum` and `frontend/package-lock.json`) and build
resources. Dependencies, compiled bundles, executable output, logs, and local
environment secrets are excluded by `.gitignore`.

## References

- [Wails documentation](https://wails.io/docs/introduction/)
- [Vue documentation](https://vuejs.org/guide/introduction.html)
- [Go documentation](https://go.dev/doc/)
- [vmPing](https://github.com/r-smith/vmPing)
