# Hostlight roadmap

Reviewed: 2026-10-12. Free and open source; Windows first.

This roadmap records product direction, not delivery dates or implemented
features. The initial release must pass the outstanding gates in
[IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md) before these tracks begin.
Changes will ship as reviewable slices with automated checks, focused manual
verification, and commit checkpoints.

## Priority and sequence

User-approved sequence:

1. Webhook presets, custom headers, event selection, and cooldowns.
2. Windows service and start at boot.
3. Dependency suppression.

Route diagnostics/MTR is also a major requested feature, but has no fixed order
after those three. Certificate-chain expiry is interesting, lower-priority work.
No provider selection, version number, or delivery date is committed yet.

The service and dependency tracks support the same sysadmin workflow: monitor
infrastructure continuously, then avoid a flood of downstream alerts when a VPN
or upstream connection fails. Service operation alone does not provide dependency
suppression; clearly disclose that until track 3 ships. Prepare stable monitor
identities and policy/storage interfaces during service work so dependencies do
not require a second monitoring engine later.

## Track 1 — Webhook integrations

Why: connect Hostlight to existing incident/ticket/chat workflows with less
receiver-side translation. Existing generic JSON, scopes, bearer authentication,
HMAC, durable retries, and delivery history remain supported.

Suggested delivery slices:

- Add validated custom headers per destination. Encrypt secret values, redact
  diagnostics, and define reserved-header precedence. Avoid exporting credentials.
- Add selected outage/recovery events with explicit routing and recovery
  eligibility, settings migration, and a preview/test delivery workflow.
- Add provider payload presets after choosing the first providers with the user.
  Keep generic JSON available; validate actual payloads against provider fixtures.
- Add per-destination/monitor cooldowns with an explicit flapping policy and a
  final recovery notification that is not silently lost. Distinguish logical
  alert suppression from retrying the same durable delivery.
- Extend safe webhook response diagnostics if needed for troubleshooting.

Acceptance: settings survive restart; existing destinations retain behavior;
payload/header previews redact secrets; scopes and selected events route
correctly; retries preserve event identity; cooldown/recovery behavior is tested
across restart, maintenance, destination edits, and rapid state changes.

## Track 2 — Windows service and start at boot

Why: infrastructure monitoring must survive closing the GUI and user logout,
and start without an interactive login. Startup of the GUI at login is not a
substitute for service monitoring at boot.

Suggested delivery slices:

- Extract a shared monitoring/storage/alert core with no dependency on a Wails
  window. Retain standalone mode.
- Add service install/start/stop/status/uninstall and boot-start configuration,
  with explicit administrative actions and bounded, graceful shutdown.
- Make the service the sole owner of probes, history, and webhook delivery in
  service mode. Attach the GUI over authenticated, access-controlled local IPC;
  reconnect without starting duplicate monitors or alert workers.
- Define service-account permissions, data-directory ACLs, ICMP/network behavior,
  and credential provisioning. Current-user DPAPI secrets cannot simply be copied
  into a different service account and assumed readable. Migration must be
  explicit, recoverable, and preserve the existing standalone workspace.
- Add actionable service health/status, restart recovery, diagnostic export,
  and documented upgrade/removal behavior. Consider RMM heartbeat and installation
  identity as separate follow-up slices.

Acceptance: monitoring and webhook delivery continue through GUI close/logout;
boot starts exactly one engine; GUI reconnect works for an authorized user;
unauthorized IPC clients cannot read secrets or change configuration; stop and
upgrade drain writes; crash recovery avoids duplicate logical transitions;
service and standalone migration is tested on a fresh Windows machine.

## Track 3 — Dependency suppression

Why: when a VPN fails, downstream infrastructure should not generate thousands
of separate outage alerts. Service monitoring makes this especially valuable.

Agreed baseline behavior:

- Continue downstream probes and incident/history recording while the dependency
  is down. Suppress downstream alerts, rather than hiding observed failures.
- Show the upstream cause in the UI, such as “Alerts suppressed by VPN outage.”
- Do not replay suppressed downstream alerts when the dependency recovers.
- Apply dependency decisions consistently to webhook routing, retries, and
  visual alerts. Preserve suppression reasons for troubleshooting.

Suggested delivery slices:

- Define directed dependency relationships using stable monitor IDs, starting
  with one upstream dependency and extending to multiple/recursive dependencies
  only with explicit semantics. Reject cycles and invalid references.
- Persist dependency configuration and suppression decisions with transitions.
  Define ordering for near-simultaneous upstream/downstream results, including
  whether a bounded grace period is needed to catch downstream failures observed
  just before the upstream failure threshold is reached.
- Define unknown/error/paused/deleted upstream behavior, restart initialization,
  multiple upstream failures, and interactions with maintenance/cooldowns.
- Define recovery eligibility: avoid orphan recovery alerts for an outage whose
  notification was suppressed. Decide how to notify about a downstream monitor
  that remains unavailable after upstream recovery; do not replay a backlog.
- Add dependency editing, visible suppression state/cause, and diagnostics.

Acceptance: a controlled VPN outage produces the upstream notification without
downstream alert fan-out, including already-queued retries under the agreed
policy. Downstream history remains accurate, independent failures are still
actionable, cycles are rejected, and recovery/restart never releases an alert
backlog. Verify a representative large fan-out workload before capacity claims.

## Route diagnostics and MTR

Why: move from “this endpoint is failing” toward useful path investigation.
Recommended first slice is bounded, cancellable, on-demand route diagnostics
from a monitor, with explicit timestamps and copy/export of results. Continuous
MTR-style sampling follows only if useful and measured.

Validate Windows probe support, privileges, IPv4/IPv6, hop timeouts, rate limits,
and DNS lookup behavior. Explain that intermediate routers may ignore or limit
responses; missing hop replies alone do not establish an endpoint outage.
Diagnostics must not block routine probes, change availability state, or silently
send route output through webhooks. Bound concurrency, duration, and retained data.

## Certificate-chain expiry — lower priority

Existing HTTPS checks verify trust/hostname/time and show leaf-certificate
expiry with an optional minimum-validity policy. Explore showing the verified
chain and identifying the earliest relevant expiry, including intermediates.
Define alternate-chain selection and trust-anchor treatment before adding
alert policies. Keep normal TLS validation enabled and test local certificate
chains; do not infer a chain problem from the leaf expiry alone.

## Additional backlog

User request (2026-10-13): add a discoverable in-app keyboard-shortcut reference,
preferably under Help or a small help page. List shortcuts and their actions,
explain context-specific keys where relevant, and keep the reference synchronized
with native menus and actual bindings. This improves discoverability without
changing the approved webhook/service/dependency priority order.

User request (2026-10-13): make version/build identification useful in support
info. The current About report and diagnostic JSON already expose `release`,
and release packaging embeds the requested version; ordinary local builds show
`early preview`. Add a clear build identifier (for example commit revision and
clean/modified source status where available), alongside release version, in
About and sanitized diagnostics. Label development builds explicitly, avoid
personal build paths, and verify release metadata matches the packaged artifact.
Track this as release-support polish, separate from the three major feature tracks.

No fixed ordering after the three priority tracks:

- Authenticated HTTP probes and explicit redirect policies; DNS expected-answer checks.
- Degraded-state latency/failure-rate policies with clear windows and hysteresis.
- Recurring/group maintenance and acknowledgments.
- Background Windows notifications; existing popups only appear inside Hostlight.
- Advanced tables, comparison charts, customizable layouts. Saved views already exist.
- Portable/full-workspace migration with explicit credential handling.
- Installer and optional signing; unsigned ZIP releases remain acceptable.
- Verified macOS/Linux support and credential adapters.
- Signed automatic updates with recovery/rollback.

Cloud accounts, remote agents, a server product, and a plugin system need a
separate product decision. This roadmap does not authorize those additions.
