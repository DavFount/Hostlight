# Supported platform and known limitations

This document describes the standalone Windows release scope. It does not claim
that every platform or accessibility combination has been verified.

## Installation and local data

- Windows amd64 is the tested package target. macOS, Linux, and Windows ARM64
  packages are not release-qualified.
- Microsoft Edge WebView2 Runtime is required. The app can prompt to install it;
  restricted/offline environments may require an administrator to provision it.
- ZIP packages are unsigned. Windows may display publisher/SmartScreen warnings.
- There is no installer. Extract the entire ZIP before running the executable.
- Settings/history live under `%APPDATA%\Hostlight`, not beside the executable.
  Development and release builds normally share this profile.
- Quit before upgrading or backing up the data folder. Keep a backup before
  opening existing data with a newer build; schema upgrades may prevent downgrade.
- To remove the application, Quit and remove its extracted folder/shortcuts.
  User data is retained. Removing `%APPDATA%\Hostlight` separately permanently
  removes that workspace; it is not required for an ordinary upgrade.

## Monitoring and notifications

- Hostlight must remain running. Tray hiding continues checks/webhooks; Quit,
  logout, and shutdown stop monitoring. Windows service/boot startup is planned.
- Current popups appear only inside Hostlight. Native Windows notifications and
  desktop overlays are the first post-release roadmap priority.
- Initial discovery is silent. Recovery alerts require an eligible prior outage.
- Pauses, restarts, edits, and observation gaps have explicit incident semantics;
  they do not establish continuous uptime. See [alerts/history](ALERTS_AND_HISTORY.md).
- HTTP authentication/custom headers, redirect following, DNS-specific checks,
  dependency suppression, and MTR diagnostics are not implemented.
- Scheduling changes apply after Quit/reopen. A Windows reboot is unnecessary.

## Storage, metrics, and privacy

- Session counters and raw recent charts reset on monitor edit/application restart.
- Saved probe summaries retain up to seven days, with a global 50,000-row cap;
  busy workspaces can retain less. Incident retention is configured separately.
- Failed-check percentages are observed-check statistics, not SLA uptime or
  packet-loss measurements. DNS/local setup errors are excluded.
- Storage errors/queue overflow can leave gaps. An abrupt exit can lose queued
  observations. SQLite can reuse freed pages without shrinking its files.
- Webhooks are at least once; receivers should deduplicate using event IDs.
- URLs/configuration/history can contain sensitive information. Monitor backups
  exclude history and webhook secrets but are not anonymized.
- Webhook secrets use Windows current-user encryption and cannot be assumed
  portable to another account/machine. See [user guide](USER_GUIDE.md).

## Verification limits

The 200-monitor loopback workload validates local scheduler/storage/webhook
behavior. It is not a supported-capacity promise, a remote-network performance
claim, or a typical-PC benchmark. An additional long Threadripper soak is not a
1.0 release requirement by user decision (2026-10-13). Testing on modest hardware
and extended resource/soak measurements remain follow-up work. Automated retention,
count-reconciliation, and shutdown tests remain required.

Keyboard checks cover the recorded workflows; a comprehensive screen-reader
audit is not complete. Report accessibility problems with reproduction details.
