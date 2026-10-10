# Hostlight

See what's reachable. Know when it changes.

Hostlight is a desktop network-monitoring application being built with Go, Wails,
Vue 3, and TypeScript. Inspired by [vmPing](https://github.com/r-smith/vmPing),
it aims to make the status of multiple hosts and services easy to see at a glance.

## Project status

The project currently contains the generated Wails Vue/TypeScript starter.
Monitoring and alerting features are planned and are not implemented yet.

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

No application tests have been added to the starter yet.

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
