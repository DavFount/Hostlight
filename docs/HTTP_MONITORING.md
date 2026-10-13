# HTTP and HTTPS monitoring

[Back to README](../README.md) · [User guide](USER_GUIDE.md)

## HTTP/HTTPS monitoring

Choose **HTTP / HTTPS** when adding a monitor, enter a full URL such as
`https://service.example/health`, and choose the exact expected status (default
200). The URL supplies its port; the separate TCP-port setting is not used.
Checks send GET requests and apply the same failure/recovery thresholds, scoped
webhooks, history, pause/resume, and silencing rules as other monitors. A failed
DNS lookup is a check error; connection/TLS failures, timeouts, and unexpected
status codes are failed availability checks.

**Test HTTP request** makes one real request using the current URL, status, and
timeout without saving a monitor, writing incident history, or sending alerts.
Changing the form clears its result. The preview shows the actual status or a
safe connection/TLS/timeout diagnostic. A successful status-only match on a card shows time
to response headers, including DNS, connection, and TLS work—not page load time.
Failed checks display a dash; the preview distinguishes response-header timing
from elapsed failed-attempt time.

HTTPS certificates are verified using the system trust configuration. Redirects
are not followed: a 301/302 is a mismatch unless explicitly expected. Use the
final health URL when a redirect is not what you want to monitor. Each check
opens a fresh connection; response headers are limited to 64 KiB and response
bodies are closed without inspection for status-only checks. Optional body checks
read a bounded response in memory without storing it. The timeout includes DNS,
TLS, and body reading when enabled.
Go's standard environment proxy rules apply; there is no custom proxy/PAC UI.

HTTP is unencrypted. Do not put passwords or tokens in URLs, including query
parameters: complete URLs are ordinary configuration metadata, saved in plain
text and included in exports, incident records, and webhook target snapshots.
Embedded URL credentials and fragments are rejected. Authentication, arbitrary
headers/methods and redirect options are not implemented yet.

Under **Additional response check**, optionally require:

- **Response contains text:** a case-sensitive substring, such as `service ready`.
  This is not whole-word matching: `healthy` also appears in `unhealthy`. Prefer
  JSON field equality for an exact structured health value.
- **JSON field equals:** a JSON Pointer such as `/status` or `/checks/0/healthy`,
  with an expected text, number, boolean, or null value. Enter text without quotes.
  Leave the path blank to compare the root value. Field names containing `/` use
  `~1`; names containing `~` use `~0`. Missing fields fail, including when null is
  expected. Types are strict: `true` is not `"true"`. Numbers compare by exact
  numeric value, so `1` equals `1.0`, without floating-point rounding.

Both the status and optional body rule must match. JSON must be a single complete
document; arrays/objects cannot be expected values. Body checks read up to 1 MiB
of decoded content; oversized, incomplete, or timed-out responses fail even if
an earlier prefix matched. Status mismatches do not read/inspect the body. Body
content is never displayed, logged, stored, or included in alerts. Expected rules
are configuration metadata and do appear in backups and webhook target snapshots:
do not use secret expected values. Expectations are limited to 4,096 bytes; JSON
Pointer paths to 1,024 characters/32 levels; numeric comparisons to 4,096 characters
and exponents between -4096 and 4096.

The request preview uses these same rules. Body-rule latency measures the full
response check, while status-only latency measures time to headers.

Bulk add accepts one complete URL per line with shared expected-status settings.
Backup duplicate matching normalizes URL host/scheme case, default ports, and an
empty path to `/`; path/query case and content stay significant. Expected status
does not create a distinct duplicate identity for the same URL. Configuration
writes now use version 4; existing version-1/2/3 files remain readable. Older builds
will refuse version 4 rather than silently discard certificate policies.

### HTTPS certificate expiry

HTTPS cards and request previews show the last verified server certificate's expiry
date. Metadata is refreshed on each completed TLS request and cleared when a check
cannot verify a certificate; paused cards retain their last observation. It is not
restored as fresh evidence after restart or exported in monitor backups.

**Minimum certificate validity (days)** defaults to 0 (expiry policy off). Set
1–365 days, such as 30, to require at least that many 24-hour periods remaining.
Less than the configured minimum fails the check, even with HTTP 200; an exact
minimum passes. This is a health policy, not a separate degraded state: the normal
failure threshold establishes Unavailable, and normal scoped outage/recovery
webhooks, snoozing, and initial-discovery rules apply. Renewal above the minimum
can establish recovery after the success threshold.

Certificate trust, hostname, and validity verification always remain enabled,
including when the expiry policy is off. Untrusted or already-expired certificates
fail and do not produce verified expiry metadata. This feature checks the leaf
(server) certificate's future expiry, not intermediate/root future-expiry policies
or a separate TLS-only probe. HTTP URLs cannot use a positive certificate minimum.
Dates are stored/reported as UTC and displayed in local time; certificate metadata
is the certificate seen on this connection, including any trusted inspection proxy.
