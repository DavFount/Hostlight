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
ICMP on macOS/Linux, incident/metric history, webhook alerts, and advanced platform
error handling are still planned.
This preview is intended for development and evaluation.

Initial goals:

- A responsive, color-coded grid of monitored targets.
- ICMP ping and TCP port checks with configurable intervals and timeouts.
- Status transitions based on consecutive failures and successes.
- Webhook notifications for outages and recovery.
- Saved targets, friendly names, and groups.
- Persistent incident history and latency charts.

Additional probe types, maintenance windows, and advanced alert rules may follow.

See the [implementation plan](docs/IMPLEMENTATION_PLAN.md) for architecture,
delivery milestones, acceptance criteria, and the planned first release scope.

## Prerequisites

- Go 1.25 or newer, as declared in `go.mod`.
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
```

Backend tests cover local TCP connectivity, failure and recovery thresholds,
cancellation, input validation, configuration round trips, unreadable settings,
and rollback when settings cannot be saved. ICMP tests include probe dispatch,
type-specific validation, legacy configuration compatibility, error classification,
and native Windows IPv4/IPv6 localhost replies.
Bulk-operation tests cover previews, duplicate detection, limits, persistence,
stale selections, and rollback when a group change or batch cannot be saved.

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
