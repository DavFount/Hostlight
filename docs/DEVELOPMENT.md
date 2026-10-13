# Development guide

[Back to README](../README.md) · [Contributing](../CONTRIBUTING.md) · [Release guide](RELEASING.md)

Windows is the tested development and release platform. Other platforms require separate verification.

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

## Verification and load testing

Run the shared pipeline from the repository root:

```powershell
./scripts/verify.ps1
```

Build frontend assets before standalone Go checks on a fresh checkout: main.go embeds frontend/dist. The shared pipeline handles this order.

See [desktop smoke checks](DESKTOP_SMOKE_TEST.md), [load testing](LOAD_VERIFICATION.md), and [storage verification](STORAGE_VERIFICATION.md). The 200-monitor fixture uses a disposable profile; do not import load targets into your normal workspace.

The architecture and implementation state are documented in [the implementation plan](IMPLEMENTATION_PLAN.md). [The roadmap](ROADMAP.md) records agreed priorities.
