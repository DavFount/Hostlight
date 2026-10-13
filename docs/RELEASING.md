# Release guide

[Back to README](../README.md) · [Development](DEVELOPMENT.md)

## Automated checks and releases

**Windows verification** runs formatting checks, backend tests/vet, packaging-helper
tests, frontend tests/type checking, and a Wails desktop build on pushes and pull
requests. It uses the Go version in `go.mod`, Node 24, and the Wails CLI version
matching the module. Successful runs retain an unsigned verification executable
for seven days. The badge reflects GitHub runs, not local results.

To publish without creating or pushing a Git tag yourself:

1. Push these workflows to `main`.
2. Open **Actions → Publish release → Run workflow** and select `main`.
3. Enter an unused three-part version such as `0.1.0` (no `v` prefix).
4. Keep **prerelease** enabled while release acceptance checks remain incomplete.

The action reruns verification, embeds the version in the executable/support
screen, packages a Windows amd64 ZIP and `SHA256SUMS.txt`, and publishes a GitHub
release. GitHub creates `v<version>` at the exact tested commit automatically.
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
