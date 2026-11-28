# Release guide

[Back to README](../README.md) · [Development](DEVELOPMENT.md)

For stable release preparation, review the [candidate notes](RELEASE_NOTES.md)
and [known limitations](KNOWN_LIMITATIONS.md). Finalize the notes and ensure the
published release uses them; the current workflow's generic notes do not substitute
for the curated 1.0 announcement. Test the actual final archive, including version,
checksum, existing-data preservation, and Quit/relaunch, before stable publication.
An additional long Threadripper soak is deferred by user decision; do not convert
that deferral into a completed test or a supported-capacity claim.

## Automated checks and releases

**Windows verification** runs formatting checks, backend tests/vet, packaging-helper
tests, frontend tests/type checking, and a Wails desktop build on pushes and pull
requests. It uses the Go version in `go.mod`, Node 24, and the Wails CLI version
matching the module. Successful runs retain an unsigned verification executable
for seven days. The badge reflects GitHub runs, not local results.

**macOS verification** runs the frontend and Go checks on Apple Silicon, builds
an arm64 app, and validates the architecture and ad-hoc bundle signature.

To publish without creating or pushing a Git tag yourself:

1. Push these workflows to `main`.
2. Open **Actions → Publish release → Run workflow** and select `main`.
3. Enter an unused three-part version such as `0.1.0` (no `v` prefix).
4. For macOS, enable **macos_preview** and **prerelease**. The workflow leaves
   both platform ZIPs and a shared `SHA256SUMS.txt` in a draft release.
5. For a draft, download the actual macOS ZIP from the release page in a browser,
   verify its SHA-256, and run the Mac acceptance checks below before publishing
   the draft as a prerelease.

The action reruns verification, embeds the version in the app and About screen,
and packages the Windows amd64 ZIP and optional macOS ZIP. Windows-only runs
publish automatically after asset upload. GitHub creates `v<version>` at the
exact tested commit automatically.
Release tags are still used by GitHub, but no manual tag commands are needed.
Existing tags/releases are not overwritten. Uploads happen into a draft before
publication; an interrupted upload may leave a draft and tag requiring manual cleanup
or a new version number. The workflow never deletes them automatically.
Repository/tag-protection policies must permit the workflow token to create releases.

The ZIP contains the executable, launch notes, and GPL-3.0 license; it is **not an installer**.
Launch notes link to the matching tagged source. Publish the source tag with each
binary release; a local packaging command does not create that tag.
Settings remain in `%APPDATA%\Hostlight`. Packages are unsigned and require
WebView2. Code signing is deferred; unsigned releases will remain clearly labeled.
Fresh-machine/upgrade/soak verification remain release gates.
Hostlight will remain free and open source.
Nothing is published merely by pushing to `main`; those pushes produce CI builds.
For automatic version selection later, release-PR automation can build on this flow.

Local verification (Go, Node/npm, Git, and the matching Wails CLI on PATH):

```powershell
./scripts/verify.ps1
# Optional packaging after verification; never publishes anything:
./scripts/package-release.ps1 -Version 0.1.0
```

See [the desktop smoke checklist](DESKTOP_SMOKE_TEST.md) for native manual
checks that CI does not prove. Native tray interaction is intentionally excluded
from hosted CI; its short local smoke requires `HOSTLIGHT_TRAY_SMOKE=1`.

## macOS preview acceptance

1. Run `bash scripts/verify-macos.sh 1.0.1` locally with Xcode Command Line
   Tools, Git, Go, Node/npm, and the matching Wails CLI. Substitute the planned
   version. The ZIP appears under `build/bin/release/<version>/`.
2. On the draft downloaded from GitHub, compare `shasum -a 256 <zip>` with
   the matching entry in `SHA256SUMS.txt`. Extract with Finder and open
   `Hostlight.app`. Check the first-launch Gatekeeper behavior and the README
   instructions on a Mac without an existing exception. Confirm the expected
   version under **Help → About Hostlight**.
3. Verify IPv4 and IPv6 ICMP loopback checks on `127.0.0.1` and `::1`.
   Check TCP and HTTP/HTTPS against controlled endpoints, including failure,
   recovery, and incident history.
4. Enable close-to-tray, close the window, and confirm checks continue.
   Restore the window with **Show Hostlight** in the menu bar. Save a test
   webhook, approve Keychain access if prompted, and verify delivery before
   and after quitting from the menu bar and reopening the app. Test upgrades
   against a backed-up profile when one exists.
5. Record the macOS versions tested on Apple Silicon before publishing.
