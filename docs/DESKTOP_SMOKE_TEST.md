# Windows desktop smoke test

Run this checklist against a recorded commit before calling a standalone build
release-ready. Automated component tests and a successful compilation do not
prove desktop integration. Use disposable monitors and a test webhook receiver;
never intentionally disrupt a production service. Review backups before sharing.

## Record the environment

- Commit/build:
- Date:
- Windows edition/version and architecture:
- Standard user or administrator:
- Go, Node, npm, Wails, and WebView2 versions:
- Result: not run / pass / fail (link failure notes):

Start as a standard user with no other Hostlight instance running. For native
window lifecycle changes, restart `wails dev` or test the production verification
executable. Browser previews cannot validate native dialogs, tray, or shutdown.

## Startup and configuration

- [ ] Launch; dark is the default on a fresh profile and no startup errors appear.
- [ ] Add ICMP, TCP, and HTTP monitors using controlled reachable targets.
- [ ] Reject invalid settings without starting checks or overwriting valid entries.
- [ ] Edit/pause/resume; restart and confirm definitions and pause state persist.
- [ ] Switch light/dark and compact/comfortable; confirm preferences persist.
- [ ] Sidebar contains only monitoring features (Monitors and Incident history);
      File opens Settings and Help opens About Hostlight. Settings contains General,
      webhook setup/testing/deliveries, history retention, and monitor backups.
- [ ] File/View/Help menu clicks and Ctrl+, / Ctrl+1 / Ctrl+2 reach the correct
      pages; Ctrl+Q quits even with close-to-tray enabled. About shows diagnostics.
- [ ] Visual alerts continue across page changes; controls appear under General.
- [ ] General sections are open without extra clicks; its table of contents
      scrolls/focuses Appearance, Visual alerts, Check scheduling, and Desktop behavior.
      Check narrow-window layout and both themes.
- [ ] Save scheduling limits; active values stay unchanged until Quit/reopen,
      then saved values become active. Hiding to tray must not apply changes.

## Availability, incidents, and alerting

- [ ] Brief failure below the configured threshold does not declare an outage.
- [ ] Use a disposable TCP monitor at `127.0.0.1` on a confirmed unused port:
      refused connections show a plain-language reason and expandable Technical
      details preserve the original diagnostic. Failed latency remains a dash.
      Check the outage popup after its failure threshold and use Tab/Enter to
      expand details. Verify wrapping in both themes and card densities.
- [ ] Recover or pause that monitor; successful/paused messages remain readable
      and stale technical details disappear. HTTP status/body/certificate messages
      retain their existing detail. Do not stop production services to test this.
- [ ] Confirmed failure creates one incident; repeated failures do not duplicate it.
- [ ] Confirmed recovery resolves that incident; duration matches observed time.
- [ ] Pause a down monitor; resume while still down, then recover. Preserve the
      incident and exclude paused time; no duplicate outage alert.
- [ ] Delete a down monitor; interrupt the incident without a recovery webhook.
- [ ] Test global/group/individual webhook routing and successful test delivery.
- [ ] Verify outage/recovery share an incident ID; event IDs deduplicate retries.
- [ ] Snooze/maintenance suppress alerts without catch-up after the window ends.
- [ ] Check visual alert host/reason/duration, positions, timer, persistent dismissal.

## HTTP and certificates

- [ ] Preview a request without saving a monitor or generating an incident/alert.
- [ ] Check expected/unexpected HTTP statuses and refusal to follow redirects.
- [ ] Exercise text/JSON body rules, missing fields, and wrong primitive types.
- [ ] Verify trusted certificate expiry display and minimum-validity policy.
- [ ] Reject untrusted/expired certificates without an insecure bypass.

## Native Windows lifecycle

- [ ] Tray icon appears (check the overflow area) and click restores the window.
- [ ] Close-to-tray is off by default; normal close stops the app.
- [ ] Enable close-to-tray, close, and restore; fresh check timestamps prove
      monitoring continued. Produce an outage/recovery while hidden and verify
      webhook deliveries, not just the service/process running.
- [ ] Explicit Hide to tray works without changing the close preference.
- [ ] Starting a second instance restores the existing instance; checks do not double.
- [ ] No duplicate sidebar/settings Quit action; File → Quit Hostlight and the
      tray Quit action stop monitoring and clean up the tray icon.
- [ ] Tray Quit exits, removes its icon, and allows a clean restart.
- [ ] Close-to-tray preference persists across restart.
- [ ] On a disposable test session, verify Explorer-restart recovery. Do not restart
      Explorer on a production admin session just for this check.
- [ ] Sleep/resume, adapter loss, and sign-out/shutdown behave safely. Record gaps
      rather than implying continuous observations or inventing recovery.

## Backups, saved views, support

- [ ] Export/review/import; duplicates remain unchanged, new monitors default paused.
- [ ] Invalid/oversized backups fail without partial imports; cancel dialogs safely.
- [ ] Saved views switch filters, persist, and preserve a missing group's empty result.
- [ ] Update/rename/delete views never edits or deletes monitors.
- [ ] Support refresh succeeds; diagnostic save/cancel behaves correctly.
- [ ] Diagnostic JSON excludes names, hosts, URLs, paths, raw errors, and credentials.

## Release gates still separate from this smoke test

Record measured load/soak results, fresh-machine WebView2 startup, upgrade/uninstall,
artifact versioning/checksums, and signing/distribution decisions separately. A
200-target benchmark is not a capacity claim. CI binaries are unsigned development
verification artifacts, not production releases.
