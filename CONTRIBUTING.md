# Contributing to Hostlight

Bug reports, documentation, usability feedback, tests, and code contributions
are welcome. The project is moving from private development toward public
collaboration; release qualification is still in progress. There is no guaranteed
response time or release schedule.

## Report a problem

Search [existing issues](https://github.com/DavFount/Hostlight/issues) first.
Include:

- Hostlight release/build and Windows version/architecture.
- Steps to reproduce and expected versus actual behavior.
- Relevant check type, timing, and threshold settings.
- Whether it happens after restart or only while hidden in the tray.
- Sanitized diagnostics from **Help → About Hostlight**, when relevant.

Do not include tokens, webhook endpoints, passwords, private network addresses,
or unreviewed configuration/history exports. Diagnostics omit monitor identities
and secrets, but screenshots, backups, and incident exports can contain them.
For a suspected vulnerability, do not post exploit details or secrets publicly;
use GitHub's private vulnerability reporting if enabled. If it is unavailable,
ask the maintainer for a private reporting route without disclosing details.

## Propose a feature

Review [the roadmap](docs/ROADMAP.md) and open an issue describing your workflow,
the problem, and an example of expected behavior. Discuss substantial changes
before implementing them so scope and architecture can be agreed.

The approved post-release sequence is Windows notifications/overlays, webhook
integrations, Windows service/start at boot, then dependency suppression.
Initial release reliability comes first.
Small independent fixes and documentation improvements need not wait for a roadmap
milestone. Public feedback can change priorities through discussion.

## Develop locally

Follow [the development guide](docs/DEVELOPMENT.md). Windows is the verified
platform; Go, Node/npm, Git, WebView2, and the matching Wails v2 CLI are required.

Fork the repository, create a focused branch, and run from the project root:

```powershell
cd frontend
npm ci
cd ..
wails dev
```

Use disposable monitors and local test endpoints. Development builds normally
use your current Windows profile, so do not experiment against important saved
data. The isolated desktop-load fixture is documented in
[load verification](docs/LOAD_VERIFICATION.md); it is intended for load testing.

## Verify a change

Run the shared verification pipeline before submitting code changes:

```powershell
./scripts/verify.ps1
```

It checks Go formatting/tests/vet, frontend tests/types/build, release helpers,
and a Windows desktop build. It builds frontend assets before Go compilation
because the app embeds them. Use `gofmt` on changed Go files.

Add focused regression coverage for behavior changes. Native dialogs, tray,
keyboard/focus, and lifecycle changes also need relevant
[desktop smoke checks](docs/DESKTOP_SMOKE_TEST.md). Browser tests alone cannot
verify native integration. Load tests are opt-in; run them when a change affects
scheduler/storage throughput. Do not claim unperformed checks passed.

Documentation-only changes need accuracy and local-link review rather than a
full desktop build. If a check cannot run, explain the limitation in the PR.

## Submit a pull request

- Keep the change focused and explain the problem and resulting behavior.
- Reference the issue when there is one; include verification and remaining limitations.
- Update relevant user/developer guides when behavior changes.
- Preserve configuration/backup compatibility or supply an explicit migration.
- Do not commit dependencies, executables, local settings, logs, or secrets.
- Keep dependency lockfiles intentional. Regenerate Wails bindings from Go APIs;
  do not hand-edit generated files to change behavior.
- Avoid unrelated formatting or dependency churn.

Maintainers may request revisions or defer features outside current scope.
Feedback should be specific, constructive, and respectful.

## License

Hostlight uses [GNU GPL version 3](LICENSE), identified as `GPL-3.0-only`.
Submit original work or material you have permission to contribute under that
license. Identify third-party code and its license so compatibility can be reviewed.
