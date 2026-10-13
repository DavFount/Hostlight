# Hostlight 1.0.0 release notes — candidate draft

This is the proposed first stable standalone Windows feature set. Final candidate
verification and publication are not yet complete; this document does not
announce an available 1.0.0 release.

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

## Before publication

- Complete the remaining targeted standard-user/IPv6 and live-popup checks in
  the implementation plan; reconcile any failures before selecting stable status.
- Run the shared verification pipeline on the final source commit.
- Verify the actual 1.0.0 archive hash, About/sidebar version, startup, retained
  data, and Quit/reopen. Prior v0.2.0 results do not validate a new artifact.
- Finalize these notes and publish them with the release instead of the generic
  prerelease notes. Preserve the source tag and matching package/checksum files.
