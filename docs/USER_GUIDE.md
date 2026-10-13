# Using Hostlight

[Back to README](../README.md) · [HTTP/HTTPS](HTTP_MONITORING.md) · [Alerts and history](ALERTS_AND_HISTORY.md)

Settings contains appearance, visual alerts, scheduling, desktop behavior,
webhook setup/testing/delivery logs, and monitor backups. About Hostlight contains
support information and sanitized diagnostic exports. Native File/View/Help menus
provide navigation; Ctrl+, opens Settings and Ctrl+Q explicitly quits.
The sidebar is reserved for monitoring features. Open Settings from File and
About Hostlight from Help. General settings are visible without expanding panels;
section links jump to appearance, visual alerts, scheduling, and desktop behavior.

Monitor removal, webhook destination removal, and shorter history retention use
styled in-app confirmation dialogs. Cancel is focused first; Escape cancels, and
keyboard focus stays within the dialog while it is open. Native file selection
dialogs are still used for imports and exports.

Check scheduling is configurable from Settings → General: 1–128 simultaneous checks
(default 32) and a 0–5000 ms first-check spread (default 250 ms). Saved changes
apply after quitting and reopening, not hiding to tray. The spread applies on
start/resume; intervals are delays after completion, not fixed-rate deadlines.
Busy checks wait for capacity; HTTP previews share the same limit. Increasing
concurrency increases network/CPU load, not a guaranteed supported target count.
These application settings are not included in monitor backups.

### Windows tray behavior

Hostlight adds its icon to the Windows notification area while running (Windows
may put it in the overflow area). Click it to show the window, or right-click for
**Show Hostlight** and **Quit Hostlight (stop monitoring)**.

Open **File → Settings → General → Desktop behavior** to enable **Close to tray**. It is off
by default: closing normally quits. When enabled, the window’s close button hides
it and checks, incident recording, and webhook delivery continue. **Hide to tray**
works without changing the close preference. Use **Quit Hostlight** to stop the
app; the native menu and tray provide explicit Quit commands. Launching Hostlight again
restores the existing instance rather than starting another monitor engine.

The preference is saved with local settings, not monitor backups. Close-to-tray
only hides while a reachable tray icon is available. If the icon disappears,
Hostlight attempts to restore the window and re-add the icon. The native icon
health check runs every five seconds. If tray setup fails, closing still quits.
There is no start-hidden, login startup, service mode, or desktop-overlay alert
behavior in this slice. In-app popups are not visible while hidden; use webhooks
for unattended alerts. Other platforms do not yet have tray integration.

Implementation references: [Wails lifecycle options](https://v2.wails.io/docs/reference/options/)
and [Microsoft notification-area APIs](https://learn.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-shell_notifyicona).

### Support and diagnostics

Open **Help → About Hostlight** for platform/runtime information, component health, aggregate
monitor counts, and incident/webhook queue statistics. Use **Refresh snapshot**
to update it. Unavailable statistics are not reported as zero.

**Export sanitized diagnostics** saves a versioned JSON file through the native
save dialog. It includes aggregate counts, runtime versions, health flags, queue
sizes, and a UTC timestamp. It excludes monitor names/addresses, URLs, groups,
response bodies, check messages, incident records, webhook destinations and
credentials, local paths/usernames, raw errors/logs, and saved views. Nothing is
uploaded automatically. Counts can change during collection; review the file
before sharing. The export cannot overwrite Hostlight’s live settings/history.

### Saved monitoring views

Set the monitor search, status, and group filters, then expand **Save or manage
views** and give the view a name. Select it later from **Saved view** to reapply
those filters. Changing filters does not overwrite a saved view; **Update selected**
explicitly saves changes and can rename it. **Save as new** keeps the original.
Deletion requires confirmation and never deletes monitors. **Clear filters** shows
all monitors without changing saved views.

Up to 20 views are stored locally on this computer. Views store filters only—not
monitor definitions, monitoring settings, or alert rules—and are not part of monitor
backups. All groups and Ungrouped are distinct. A view for a group with no current
monitors remains saved and displays no matches until matching monitors exist.

### In-app visual alerts

Open **File → Settings → General → Visual alerts** to configure outage/recovery popups.
Choose any corner, 5/10/30/60-second dismissal, or **Until dismissed**. Controls
are saved locally; use **Preview popup** to check positioning. These popups appear
inside Hostlight only—not over other applications or while it is minimized.
Native desktop overlays and OS notifications remain planned.
Outage popups include the monitor name, host/address, and the actual check failure
reason (including HTTP or certificate-policy failures). Recovery popups include
the monitor name, host/address, and observed outage duration, excluding paused time.

Popups follow confirmed incident transitions, not individual failed checks. Initial
discovery is silent. Snoozing and maintenance suppress popups without replay, and
a suppressed outage does not generate an orphan recovery popup. Same-session
pause/resume retains recovery association. Restarting or reopening the UI does
not replay old alerts. The backend retains 100 recent transitions, with a visible
warning if the UI falls behind; at most 20 popups remain visible. Persistent means
until dismissed within that limit and the current UI session. Incident history is
the complete record. Visual alerts require working incident storage.

## Monitors and check statistics

The working preview includes Ping/ICMP, TCP, and HTTP/HTTPS monitors, target editing and removal,
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
ICMP on macOS/Linux remains unverified; Windows is the supported preview platform.
This preview is intended for development and evaluation.

Monitor cards and outage popups explain recognized connection, timeout, routing,
DNS, permission, and ping setup failures in plain language. Expand **Technical
details** to read the original diagnostic when an explanation is shown. Unknown
messages remain intact; these explanations do not change availability thresholds
or incident/webhook records.

Cards also show failed checks as a count and percentage over the last 100 completed
availability checks, plus session-wide availability totals, passed/failed counts,
failure percentage, and the monitor session's local start date/time. Session totals
continue beyond 100 checks without storing every result. These are individual results, independent of
the failure/recovery thresholds. HTTP status, body, and certificate-policy mismatches
count as failures; DNS and local setup errors, cancelled checks, and paused time
add no availability observations. DNS/local setup errors have a separate session
counter and are excluded from the failure percentage. Pause/resume keeps both sets
of counts and the start time; editing a monitor or restarting the app resets them.
Cards use compact Passed/Failed/Failure rate columns and a recent-check summary.
Expand **Session details** for the start time, setup-error count, and reset rules;
nonzero setup errors also appear in its collapsed summary.
These counts are not uptime or ICMP packet-loss measurements,
are not saved/exported, and may update one snapshot apart from the status card.

Expand **Recent chart** to see the last 100 completed availability checks, with
successful latency, failed-check markers, and mean/maximum successful latency.
The horizontal axis spaces checks equally rather than measuring elapsed time;
timestamps and an expandable **Check values** table provide individual results.
Failures never appear as zero-millisecond successes. Lines break at failures,
pauses, local setup errors, and monitoring gaps. Charts refresh every five seconds
while expanded, preserve samples across pause/resume, and reset on edit/restart.
Samples are bounded in memory and are not persisted or exported. TCP latency
measures connection time, ICMP measures echo-reply time, and HTTP uses the same
header/full-body timing as its card reading. Failed-check percentages are not uptime
or packet loss; local setup errors remain excluded.

In **Incident history**, expand **Probe history** for saved observations across
restarts. Choose a monitor and the last hour, 24 hours, or seven days. The upper
plot shows successful check-duration min/max and a weighted mean; the lower plot
shows failed-check percentages. Positions represent elapsed time. Empty intervals
have no observations, and separate intervals are not joined by a line. Expand
**Interval values** for exact counts and duration summaries.

Storage uses one summary per monitor/session/minute, retaining seven days with
hourly cleanup and a hard global cap of 50,000 summaries. Busy workspaces can
therefore retain less than seven days. The displayed ranges aggregate to 1, 12,
or 84 minutes; range edges have minute precision. Summaries include earlier edits
and sessions for the same monitor ID, and deleted monitors remain selectable while
their summaries are retained. Duration semantics follow each check type, so edits
that change check types can produce mixed intervals, identified in the values table.
DNS/setup errors, cancelled probes, pauses, and monitoring gaps add no observations.
Saved charts refresh every 30 seconds while expanded and can lag live card totals.
Probe summaries are separate from incident retention settings and are not included
in monitor backups or incident CSV exports. Schema 6 preserves existing incident
history; older builds do not support the upgraded database.

## Monitor backups and import

Open **Monitor backups** to export monitor definitions to a versioned JSON backup,
or choose an exported backup to review before importing. This is a monitor-only
backup, not a complete workspace or a copy of the internal `targets.json` format.

Included: names, hosts/URLs, groups, ICMP/TCP/HTTP settings, expected HTTP status/body rules, certificate minimum-validity policy, timing/threshold rules, and source
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
(`format: hostlight-monitors`, `version: 4` for new exports). Version-1 ICMP/TCP,
version-2 HTTP status, and version-3 body-rule backups are still accepted. Body
rules require at least version 3; positive certificate policies require version 4.
Verified certificate expiry observations are not exported. Older versions of
Hostlight cannot import version-4 backups.
Existing internal settings files are
still loaded normally at application startup.

Initial goals:

- A responsive, color-coded grid of monitored targets.
- ICMP ping and TCP port checks with configurable intervals and timeouts.
- Status transitions based on consecutive failures and successes.
- Webhook notifications for outages and recovery.
- Saved targets, friendly names, and groups.
- Persistent incident history and latency charts.

Additional probe types, recurring maintenance schedules, and advanced alert rules may follow.

See the [implementation plan](IMPLEMENTATION_PLAN.md) for architecture,
delivery milestones, acceptance criteria, and the planned first release scope.
