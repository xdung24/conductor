# Service Monitor — Feature Roadmap

This document tracks which features are implemented, in progress, or planned.
**Update this file whenever a feature is implemented or a new one is identified.**

---

## Monitor Types

| Type | Status | Notes |
|---|---|---|
| HTTP / HTTPS | ✅ Done | Status code 2xx–3xx check |
| TCP Port | ✅ Done | Dial connect check |
| Ping | ✅ Done | TCP-proxy reachability (port 80/443) |
| DNS | ✅ Done | A/AAAA/CNAME/MX/NS/TXT/PTR; optional expected-value assertion |
| Push / Heartbeat | ✅ Done | `GET /push/:token` endpoint; token stored on monitor; scheduler skips push monitors |
| HTTP — Keyword match | ✅ Done | Scan response body for substring; optional invert flag |
| HTTP — JSON Query | ✅ Done | JSONPath expression + optional expected value assertion |
| HTTP — XML / SOAP XPath Query | ✅ Done | XPath expression on XML/SOAP response body + optional expected value assertion |
| HTTP — Accepted status codes | ✅ Done | Comma-separated list, e.g. `200,404`; empty = 2xx/3xx |
| WebSocket Upgrade | ⬜ Planned | Verify WS handshake succeeds |
| SMTP | ✅ Done | EHLO + optional STARTTLS + optional AUTH PLAIN |
| SNMP | ⬜ Planned | Get OID value, optional assert |
| MQTT | ⬜ Planned | Subscribe, verify broker responds |
| Docker Container | ⬜ Planned | Container running/healthy via Docker socket or HTTP API |
| MySQL / MariaDB | ✅ Done | Connection string DSN + optional query (default `SELECT 1`) |
| PostgreSQL | ✅ Done | Connection string DSN + optional query (default `SELECT 1`) |
| Microsoft SQL Server | ⬜ Planned | TCP + simple query |
| MongoDB | ✅ Done | mongo-driver ping + optional admin command (e.g. `{"ping":1}`) |
| Redis | ✅ Done | Raw RESP PING/PONG; optional AUTH (Redis 6+ ACL); `host:port` or `user:pass@host:port` |
| RabbitMQ | ⬜ Planned | Management API health check |
| gRPC Keyword | ⬜ Planned | gRPC call, keyword match on response |
| SIP Options | ⬜ Planned | SIP OPTIONS request |
| Radius | ⬜ Planned | Authentication request |
| Steam | ⬜ Planned | Steam Web API query |
| GameDig | ⬜ Planned | Game server query protocol |
| Tailscale Ping | ⬜ Planned | `tailscale ping` subprocess |
| Globalping | ⬜ Planned | Globalping API distributed check |
| Kafka Producer | ⬜ Planned | Produce a test message |
| Real Browser (Chromium) | ⬜ Planned | Headless browser via chromedp |
| System Service | ⬜ Planned | Windows SCM / systemd unit status |
| Group / Manual | ⬜ Planned | Logical grouping / manual status |

---

## HTTP Check Capabilities (extensions to HTTP type)

| Feature | Status | Notes |
|---|---|---|
| Status code check | ✅ Done | |
| Custom accepted status codes | ✅ Done | e.g. treat 404 as UP |
| Ignore TLS/SSL errors | ✅ Done | `InsecureSkipVerify` flag (user opt-in) |
| TLS certificate expiry alert | ✅ Done | Alert N days before expiry; configurable per monitor |
| Basic auth | ✅ Done | Username + password on monitor |
| Bearer token auth | ✅ Done | `Authorization: Bearer …` header (takes priority over basic auth) |
| Custom request headers | ✅ Done | `Key: Value` per line; set on HTTP request before auth headers |
| Custom request body | ✅ Done | HTTP method select + raw body textarea for POST/PUT/PATCH |
| Keyword match in body | ✅ Done | (see Monitor Types above) |
| JSON path query | ✅ Done | (see Monitor Types above) |
| Redirect follow control | ✅ Done | Max redirects (0 = no-follow) |
| Proxy per-monitor | ⬜ Planned | HTTP/SOCKS5 proxy per check |

---

## Platform / Infrastructure Features

| Feature | Status | Notes |
|---|---|---|
| Uptime % (24 h / 30 d) | ✅ Done | |
| Heartbeat history page | ✅ Done | |
| Notification send history | ✅ Done | |
| Custom DNS server per monitor | ✅ Done | |
| Notification providers: Webhook | ✅ Done | |
| Notification providers: Telegram | ✅ Done | |
| Notification providers: Email (SMTP) | ✅ Done | |
| Public status page | ✅ Done | Read-only page at `/status/:username/:slug` showing selected monitors with 24h uptime |
| Maintenance windows | ✅ Done | Suppress alerts during scheduled downtime; per-monitor or global |
| Tags / labels on monitors | ✅ Done | Color-coded labels; assign to monitors; displayed on dashboard |
| Proxy management | ⬜ Planned | Shared proxy config referenced by monitors |
| Docker host management | ⬜ Planned | Registered Docker daemons for Docker monitor |
| API keys | ⬜ Planned | Token-based API access |
| 2FA (TOTP) | ⬜ Planned | TOTP QR code + enforced on login |
| Additional notification providers | ✅ Done (Slack, Discord, ntfy); ⬜ Planned (PagerDuty, Gotify, others) |
| Remote browser config | ⬜ Planned | Chromium endpoint for real-browser checks |
| Cloudflare Tunnel integration | ⬜ Planned | Expose via cloudflared without open port |
| Dark/light theme toggle | ✅ Done | User preference stored in `sm_theme` cookie; toggled from navbar |
| Latency sparkline charts | ✅ Done | Inline SVG polyline of last 50 checks on dashboard |
| Multi-user support | ✅ Done | Per-user monitors/notifications in isolated SQLite DB files; shared `users.db` for auth + push token routing; `Registry` + `MultiScheduler` for per-user DB and scheduler lifecycle |
| Import / export monitors | ✅ Done | Export a single monitor's config as JSON (`GET /monitors/:id/export`); import via file upload (`POST /monitors/import`) |
| User management admin page | ✅ Done | `/admin/users` — list, add, change password, delete users |

---

## Legend

| Symbol | Meaning |
|---|---|
| ✅ Done | Fully implemented and tested |
| 🚧 In Progress | Currently being worked on |
| ⬜ Planned | Identified, not yet started |
| ❌ Won't do | Out of scope for this project |
