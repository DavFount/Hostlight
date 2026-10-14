# Hostlight

[![Windows verification](https://github.com/DavFount/Hostlight/actions/workflows/verify.yml/badge.svg?branch=main)](https://github.com/DavFount/Hostlight/actions/workflows/verify.yml)

Hostlight is a free, open-source Windows desktop tool for monitoring hosts and services. Inspired by [vmPing](https://github.com/r-smith/vmPing), it combines at-a-glance availability with useful history and webhook alerts.

**1.0.0 package verification passed** for the standalone Windows release. Builds are unsigned. Hostlight must stay running to monitor; Windows service/start-at-boot support is planned. See the [release notes](docs/RELEASE_NOTES.md) and [known limitations](docs/KNOWN_LIMITATIONS.md).

## What it does

- Ping/ICMP, TCP port, and HTTP/HTTPS checks with configurable timing and thresholds.
- HTTP status, text/JSON response matching, and HTTPS certificate-validity policies.
- Groups, bulk add, search, saved views, and individual or filtered pause/resume.
- Session totals, recent charts, saved probe summaries, incident history, and CSV export.
- Scoped outage/recovery webhooks, retries, delivery history, and maintenance silencing.
- Dark/light themes, monitor backups, and Windows tray monitoring.

## Get started

1. Download a Windows amd64 ZIP from [Releases](https://github.com/DavFount/Hostlight/releases).
2. Extract it and run `Hostlight/hostlight.exe`. Microsoft Edge WebView2 Runtime is required.
3. Choose **Add monitor** or **Add multiple**, select a check type, and enter your targets.

Packages include `SHA256SUMS.txt` for checksum verification. Unsigned builds may trigger a Windows SmartScreen warning; use this repository's release artifacts. The ZIP is not an installer or portable-data mode. Settings and history are saved under `%APPDATA%\Hostlight` and survive replacing the executable.

Open **File → Settings** for appearance, alerts, scheduling, and backups. **Help → About Hostlight** contains support information and sanitized diagnostics. Close-to-tray is optional; use **Quit Hostlight** to stop monitoring explicitly.

## Things to know

- Initial discovery is silent. Recovery notifications require an eligible prior outage.
- In-app popups appear inside Hostlight; webhooks continue while hidden in the tray.
- Failed-check percentages describe observed checks, not SLA uptime or packet loss.
- Saved probe summaries retain up to seven days, subject to a global record cap.
- Monitor backups exclude credentials and history. Stored/exported URLs may be sensitive.
- Windows is the tested preview platform; other platforms are not release-qualified.

## Documentation

| Guide | Contents |
| --- | --- |
| [User guide](docs/USER_GUIDE.md) | Monitors, statistics/charts, saved views, tray behavior, diagnostics, and backups |
| [HTTP/HTTPS](docs/HTTP_MONITORING.md) | Status/body rules, request previews, certificates, and limits |
| [Alerts and history](docs/ALERTS_AND_HISTORY.md) | Incident semantics, webhooks, maintenance, delivery, and retention |
| [Development](docs/DEVELOPMENT.md) | Prerequisites, local setup, builds, tests, and project layout |
| [Release guide](docs/RELEASING.md) | Versioning, packaging, checksums, and publication |
| [Known limitations](docs/KNOWN_LIMITATIONS.md) | Platform support, data handling, and verification limits |
| [Implementation status](docs/IMPLEMENTATION_PLAN.md) | Delivered work, verification evidence, and remaining release gates |
| [Roadmap](docs/ROADMAP.md) | Windows notifications, webhooks, service mode, then dependency suppression |

## Contributing

Bug reports, documentation improvements, feature discussions, and pull requests are welcome. Start with [CONTRIBUTING.md](CONTRIBUTING.md) for setup, verification, and how to propose changes. Discuss larger features before investing in an implementation.

When opening an issue, include the version/build, reproduction steps, and expected versus actual behavior. Review attachments and remove credentials, internal addresses, and other sensitive information. Please keep discussions respectful.

## License

Hostlight is licensed under [GNU GPL version 3](LICENSE) (`GPL-3.0-only`). Dependencies retain their respective licenses.
