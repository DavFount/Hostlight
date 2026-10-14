# Hostlight 1.0.0 release notes

The first stable standalone Windows feature set combines local availability
monitoring, incident history, and webhook alerts. Final 1.0.0 package checks
passed on 2026-10-13; stable-release promotion is a separate maintainer action.

## Included

- Ping/ICMP, TCP, and HTTP/HTTPS monitoring with configurable status thresholds.
- HTTP response status/text/JSON rules and verified certificate-validity policies.
- Bulk targets, groups, filtering, saved views, and pause/resume controls.
- Session counters, recent charts, persistent probe summaries, and incident CSV export.
- Scoped webhooks with retries/delivery history, snoozes, and maintenance windows.
- Monitor backup/import, sanitized diagnostics, dark/light themes, and Windows tray operation.
- Batched history persistence with visible storage failures and bounded queues.
- Sidebar version identification using the same release metadata as support info.

## Distribution and upgrade

Windows amd64 ZIP; Microsoft Edge WebView2 Runtime is required. Builds are
unsigned. Verify the ZIP against the release's `SHA256SUMS.txt` before use.
Go, Node, and Wails are not needed to run the packaged app.

Quit Hostlight, back up `%APPDATA%\Hostlight`, then extract and launch the new
package. Do not delete application data as an upgrade step. Monitor settings and
history are reused; current-user encrypted webhook secrets remain account-bound.
GPL-3.0 license and a matching source-tag link are included in the package.

## Limits and next steps

Monitoring stops on Quit; this release is not a Windows service. Popups are
in-app only. Native notifications/overlays, webhook improvements, service mode,
then dependency suppression are the prioritized [roadmap](ROADMAP.md).
See [known limitations](KNOWN_LIMITATIONS.md) for retention, metric semantics,
platform scope, accessibility status, and performance-evidence limits.

## Verification

- Shared verification passed: 135 frontend tests, frontend build/type checks,
  Go tests/vet, packaging checks, and Windows desktop build.
- User verified the actual 1.0.0 package: checksum, About/sidebar/executable
  version, preserved settings/history, outage/recovery/alerts, tray/Quit/relaunch,
  and clean-machine startup without Go/Node/Wails.
- Earlier targeted checks cover IPv4/IPv6, HTTP/body/certificate rules, popup
  silencing, backups, diagnostics, and Explorer/second-instance recovery.
- Additional long soak and typical-workstation capacity claims are deferred;
  see known limitations. No exhaustive screen-reader/platform audit is claimed.

Verified ZIP SHA-256:

```text
60d682a927ef889296c258c69950f3285d28e6dbe1b5173ed08ec39f1a6096a1
```

Publish these notes with the existing tested release, replacing generic
prerelease boilerplate. Preserve its tag, ZIP, and checksum; no rebuild is
required merely to promote the tested prerelease to stable.
