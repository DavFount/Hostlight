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

- [ ] On a disposable monitor, Remove opens the styled in-app confirmation.
      Cancel gets initial focus; Escape/Cancel keeps the monitor. Tab/Shift+Tab
      stays inside the dialog and the background cannot be clicked. Confirmation
      removes the monitor once; keyboard focus remains useful afterward.
- [ ] Repeat cancellation/confirmation for a disposable webhook destination.
      Cancel restores focus to Remove; successful deletion moves focus to Add
      destination. Pending deliveries are cancelled only on confirmed removal.
- [ ] In a disposable profile, reduce history retention: Cancel restores the
      previous value, confirmation applies it once, and increasing retention needs
      no destructive prompt. Confirmed reduction can delete older completed history.
      Check modal layout in both themes and at a narrow window width.
- [ ] Verify session availability totals continue beyond 100 while recent checks
      remain capped. Passed + failed must equal session availability checks.
      Use an invalid `.invalid` hostname to confirm DNS/setup errors increase
      separately without changing the failure percentage. On disposable monitors,
      pause/resume preserves totals/start time; edit/full restart resets both.
      Confirm the displayed start includes a local date/time and statistics remain
      readable in light/dark and compact/comfortable layouts.
- [ ] On disposable reachable and refused TCP monitors, check recent failed-check
      counts and percentages. Counts reflect each completed check, even before
      availability thresholds change the status; the window stops growing at 100.
      Pause freezes counts, resume keeps them, and edit/full restart resets them.
      DNS/setup errors do not count as availability failures. Check both themes
      and card densities; HTTP policy mismatches count as failed checks.
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

## Observed outage duration

- Use a disposable failing monitor and compare its observed outage duration with incident history. It advances with observations, not every wall-clock second.
- Pause for at least 30 seconds: the duration holds. Resume and confirm paused time is excluded while checks continue.
- Recover the monitor and confirm the duration disappears. Edit a failing monitor and confirm its old incident duration is cleared.
- Check compact/comfortable density and both themes. DNS/setup errors alone must not display an outage duration.

## Fresh evidence after monitoring gaps

- Set failure/recovery thresholds above one, with interval 5 seconds and timeout 3 seconds. Sleep Windows for at least one minute, then resume.
- Confirm status uses fresh checks to establish availability. A probe spanning sleep must not increment session totals or create a synthetic failed sample.
- Repeat with an unavailable disposable monitor: its previous incident ends as interrupted at its last observation. Continued failure starts a new incident only after fresh failure evidence.
- Confirm session totals survive the gap and no recovery notification/webhook is fabricated for the interrupted incident.
- Disconnect/reconnect the network and verify actual completed failures still count normally; a network failure alone does not imply a monitoring gap.

## Recent probe charts

- Expand Recent chart for passing, failing, and mixed-result monitors. Confirm successful latency mean/max excludes failures and failed-check percentage matches the same recent window.
- Let the window exceed 100 checks: chart and table stay capped while session totals continue.
- Pause/resume: samples remain, no samples arrive while paused, and the latency line breaks after resume. DNS/setup errors add no chart samples and break the next successful segment.
- Edit/restart: chart samples reset. Recovering failures appear as crosses with no connecting successful-latency line through them.
- Verify keyboard expansion, check-values table scrolling, and readable narrow cards in both themes and densities. Chart refresh can lag statistics by up to five seconds.
