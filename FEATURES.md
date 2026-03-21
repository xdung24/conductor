# Conductor — Feature Roadmap

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
| WebSocket Upgrade | ✅ Done | Dial ws:// or wss://; verify 101 Switching Protocols; optional TLS skip |
| SMTP | ✅ Done | EHLO + optional STARTTLS + optional AUTH PLAIN |
| SNMP | ✅ Done | Get OID value via gosnmp; v1/v2c/v3; optional expected-value assertion |
| MQTT | ✅ Done | Connect to broker, subscribe to topic, wait for message; optional keyword assertion |
| Docker Container | ✅ Done | Container running/healthy via Docker Unix socket or TCP HTTP API; health check status reported |
| MySQL / MariaDB | ✅ Done | Connection string DSN + optional query (default `SELECT 1`) |
| PostgreSQL | ✅ Done | Connection string DSN + optional query (default `SELECT 1`) |
| Microsoft SQL Server | ✅ Done | Connection string DSN + optional query (default `SELECT 1`) |
| MongoDB | ✅ Done | mongo-driver ping + optional admin command (e.g. `{"ping":1}`) |
| Redis | ✅ Done | Raw RESP PING/PONG; optional AUTH (Redis 6+ ACL); `host:port` or `user:pass@host:port` |
| RabbitMQ | ✅ Done | Management API health check (`/api/healthchecks/node`); Basic Auth |
| gRPC Keyword | ✅ Done | Standard `grpc.health.v1.Health/Check`; optional keyword assertion on status string; TLS support |
| SIP Options | ✅ Done | Raw UDP SIP OPTIONS request; checks SIP/2.0 response |
| Radius | ✅ Done | Access-Request; Accept or Reject = UP; shared secret + optional Called-Station-Id |
| Steam | ⬜ Planned | Steam Web API query |
| GameDig | ⬜ Planned | Game server query protocol |
| Tailscale Ping | ✅ Done | `tailscale ping --c 1` subprocess; checks for pong/DERP |
| Globalping | ✅ Done | Globalping API distributed ping check; polls for result |
| Kafka Producer | ✅ Done | Produce a test message to a configurable topic; broker reachability + write check |
| Real Browser (Chromium) | ⬜ Planned | Headless browser via chromedp |
| System Service | ✅ Done | Windows SCM (`sc.exe query`) / systemd (`systemctl is-active`) / launchd (`launchctl list`) |
| Group / Manual | ✅ Done | Group: status derived from children; Manual: static UP/DOWN flag |

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
| Public status page | ✅ Done | Read-only page at `/status/:username/:slug` showing selected monitors with 24h uptime, sparklines, and interactive latency/downtime chart |
| Maintenance windows | ✅ Done | Suppress alerts during scheduled downtime; per-monitor or global |
| Tags / labels on monitors | ✅ Done | Color-coded labels; assign to monitors; displayed on dashboard |
| Proxy management | ✅ Done | Shared proxy config referenced by monitors |
| Docker host management | ✅ Done | `docker_hosts` table + `DockerHostStore` CRUD; Unix socket or TCP HTTP URL; migration 0007; resolved at check time via `DockerHostLookup` callback |
| HTTP client reuse | ✅ Done | One `*http.Client` cached per monitor in scheduler; reuses TCP+TLS connections across checks (HTTP, RabbitMQ) |
| DB connection pool | ✅ Done | One `*sql.DB` cached per monitor in scheduler; reuses connections across checks (MySQL, PostgreSQL, MSSQL) |
| API keys | ✅ Done | SHA-256 hashed tokens; `Authorization: Bearer` auth alongside session cookies |
| 2FA (TOTP) | ✅ Done | `pquerna/otp`; QR code setup page; two-step login flow; per-user opt-in |
| Notification providers: Slack | ✅ Done | |
| Notification providers: Discord | ✅ Done | |
| Notification providers: ntfy | ✅ Done | |
| Notification providers: 360Messenger | ⬜ Planned | |
| Notification providers: 46Elks | ✅ Done | SMS via 46elks API |
| Notification providers: Alerta | ⬜ Planned | |
| Notification providers: AlertNow | ⬜ Planned | |
| Notification providers: Aliyun SMS | ⬜ Planned | |
| Notification providers: Apprise | ⬜ Planned | Meta-provider wrapping 50+ services |
| Notification providers: Bale | ⬜ Planned | |
| Notification providers: Bark | ✅ Done | iOS push via Bark app |
| Notification providers: Bitrix24 | ⬜ Planned | |
| Notification providers: Brevo | ✅ Done | Transactional email/SMS |
| Notification providers: CallMeBot | ✅ Done | WhatsApp / Signal via CallMeBot |
| Notification providers: Cellsynt | ✅ Done | SMS |
| Notification providers: ClickSend SMS | ⬜ Planned | |
| Notification providers: DingDing | ✅ Done | DingTalk webhook |
| Notification providers: Evolution | ✅ Done | WhatsApp via Evolution API |
| Notification providers: Feishu | ✅ Done | Lark / Feishu webhook |
| Notification providers: FlashDuty | ⬜ Planned | |
| Notification providers: Fluxer | ⬜ Planned | |
| Notification providers: FreeMobile | ✅ Done | French SMS |
| Notification providers: GoAlert | ⬜ Planned | |
| Notification providers: Google Chat | ✅ Done | Google Chat webhook |
| Notification providers: Google Sheets | ⬜ Planned | Append rows to a spreadsheet |
| Notification providers: Gorush | ✅ Done | Push via Gorush server |
| Notification providers: Gotify | ✅ Done | Self-hosted push; server URL + app token |
| Notification providers: Grafana OnCall | ⬜ Planned | |
| Notification providers: GTX Messaging | ✅ Done | SMS |
| Notification providers: HaloPSA | ⬜ Planned | |
| Notification providers: Heii On-Call | ⬜ Planned | |
| Notification providers: Home Assistant | ✅ Done | HA notification service |
| Notification providers: Jira Service Management | ⬜ Planned | Create incidents |
| Notification providers: Keep | ⬜ Planned | |
| Notification providers: Kook | ⬜ Planned | |
| Notification providers: LINE | ✅ Done | LINE Notify |
| Notification providers: LunaSea | ✅ Done | Self-hosted push |
| Notification providers: Matrix | ✅ Done | Home server + access token + room ID |
| Notification providers: Mattermost | ✅ Done | Incoming webhook |
| Notification providers: Nextcloud Talk | ⬜ Planned | |
| Notification providers: Nostr | ⬜ Planned | |
| Notification providers: Notifery | ⬜ Planned | |
| Notification providers: Octopush | ✅ Done | SMS |
| Notification providers: OneBot | ✅ Done | QQ via OneBot protocol |
| Notification providers: OneChat | ⬜ Planned | |
| Notification providers: OneSender | ✅ Done | WhatsApp |
| Notification providers: OpsGenie | ⬜ Planned | |
| Notification providers: PagerDuty | ✅ Done | Events API v2; routing key + severity |
| Notification providers: PagerTree | ⬜ Planned | |
| Notification providers: PromoSMS | ✅ Done | SMS |
| Notification providers: Pumble | ⬜ Planned | |
| Notification providers: Pushbullet | ⬜ Planned | |
| Notification providers: PushDeer | ⬜ Planned | |
| Notification providers: Pushover | ✅ Done | User key + API token + optional device |
| Notification providers: PushPlus | ✅ Done | WeChat push |
| Notification providers: Pushy | ⬜ Planned | |
| Notification providers: Resend | ✅ Done | Transactional email via Resend API |
| Notification providers: Rocket.Chat | ✅ Done | Incoming webhook |
| Notification providers: SendGrid | ✅ Done | Transactional email |
| Notification providers: ServerChan | ✅ Done | WeChat push |
| Notification providers: SerwerSMS | ✅ Done | SMS (Poland) |
| Notification providers: Seven.io | ✅ Done | SMS |
| Notification providers: Signal | ✅ Done | Via signal-cli REST API |
| Notification providers: SIGNL4 | ⬜ Planned | |
| Notification providers: SMSC | ✅ Done | SMS |
| Notification providers: SMSEagle | ✅ Done | SMS via SMSEagle device |
| Notification providers: SMS.ir | ✅ Done | SMS (Iran) |
| Notification providers: SMS Manager | ⬜ Planned | |
| Notification providers: SMS Planet | ⬜ Planned | |
| Notification providers: Splunk | ⬜ Planned | |
| Notification providers: SpugPush | ⬜ Planned | |
| Notification providers: Squadcast | ⬜ Planned | |
| Notification providers: Stackfield | ⬜ Planned | |
| Notification providers: Microsoft Teams | ✅ Done | Incoming webhook |
| Notification providers: Techulus Push | ⬜ Planned | |
| Notification providers: Teltonika | ✅ Done | SMS via Teltonika router |
| Notification providers: Threema | ⬜ Planned | |
| Notification providers: Twilio | ✅ Done | SMS / voice |
| Notification providers: WAHA | ✅ Done | WhatsApp HTTP API |
| Notification providers: WebPush | ⬜ Planned | Browser push notifications |
| Notification providers: WeCom | ✅ Done | WeCom (WeChat Work) webhook |
| Notification providers: Whapi | ✅ Done | WhatsApp via Whapi |
| Notification providers: WPush | ⬜ Planned | |
| Notification providers: YZJ | ✅ Done | Yunji via webhook |
| Notification providers: Zoho Cliq | ⬜ Planned | |
| Remote browser config | ⬜ Planned | Chromium endpoint for real-browser checks |
| Cloudflare Tunnel integration | ⬜ Planned | Expose via cloudflared without open port |
| Dark/light theme toggle | ✅ Done | User preference stored in `sm_theme` cookie; toggled from navbar |
| Latency sparkline charts | ✅ Done | Inline SVG polyline of last 50 checks on dashboard and public status page |
| Interactive latency chart | ✅ Done | Modal chart with selectable time spans (1h/6h/24h/7d/30d); latency polyline + downtime band overlay; on dashboard (authenticated, realtime) and public status page (unauthenticated, 60 s TTL cache) |
| Downtime events tracking | ✅ Done | `downtime_events` table records contiguous DOWN periods (started_at, ended_at, duration_s); written by scheduler on state transitions; queried by chart API to render downtime bands |
| Chart JSON API | ✅ Done | `GET /monitors/:id/chart-data?since=` (authenticated, realtime) and `GET /status/:username/:slug/chart-data/:id?since=` (public, cached); both return `{"points":[…],"downtime":[…]}` — see **Embedding the Chart** section in README |
| Public chart cache | ✅ Done | Unauthenticated chart endpoint responses cached in-memory (60 s TTL, `sync.Map` + background eviction) to prevent DB flooding |
| Multi-user support | ✅ Done | Per-user monitors/notifications in isolated SQLite DB files; shared `users.db` for auth + push token routing; `Registry` + `MultiScheduler` for per-user DB and scheduler lifecycle |
| Import / export monitors | ✅ Done | Export a single monitor's config as JSON (`GET /monitors/:id/export`); import via file upload (`POST /monitors/import`) |
| User management admin page | ✅ Done | `/admin/users` — list, add, change password, delete users |
| Account registration | ✅ Done | Open registration (toggleable by admin); startup system token grants admin to first registrant; admin invite links (single-use, no expiry); runtime enable/disable via settings page |
| Role-based access control | ✅ Done | First account (via startup token) becomes admin; admin-only routes gated by `AdminRequired` middleware; normal users isolated to own data by per-user DB design; admin can promote/demote users |

---

## Legend

| Symbol | Meaning |
|---|---|
| ✅ Done | Fully implemented and tested |
| 🚧 In Progress | Currently being worked on |
| ⬜ Planned | Identified, not yet started |
| ❌ Won't do | Out of scope for this project |
