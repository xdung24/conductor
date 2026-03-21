# Conductor — Implementation Plan

Complete feature roadmap for the Go rewrite, organized into 7 phases.
See `FEATURES.md` for current status of each item.

---

## Phase 1 — Core Platform & UX Features

### 1.1 TLS Certificate Expiry Alert
- Migration 0003: add `cert_expiry_alert_days INT DEFAULT 0` to monitors table
- Add `CertExpiryAlertDays` field to Monitor struct (`internal/models/monitor.go`)
- Update all SQL queries in `internal/models/store.go` to include the column
- `HTTPChecker.Check()`: after successful response, if `CertExpiryAlertDays > 0` and `resp.TLS != nil`, inspect the leaf certificate's `NotAfter`; if it expires within the alert window, return DOWN
- Add field to `monitor_form.html` (HTTP section only)
- Add to `monitorFromForm()` parser

### 1.2 Public Status Page
- Migration 0004: `status_pages(id, name, slug UNIQUE, description, created_at, updated_at)` + `status_page_monitors(page_id FK, monitor_id FK, position)`
- `internal/models/status_page.go` — StatusPage, StatusPageStore
- Handlers: CRUD at `/status-pages/*`; public read-only at `/status/:username/:slug` (no auth)
- Templates: `status_page_list.html`, `status_page_form.html`, `status_page_public.html`
- Public page shows monitor name, current UP/DOWN badge, 24h uptime, last-check time

### 1.3 Tags / Labels
- Migration 0005: `tags(id, name UNIQUE, color)` + `monitor_tags(monitor_id FK, tag_id FK)`
- `internal/models/tag.go` — Tag, TagStore
- Tags multi-select in monitor form; `?tag=` filter query on dashboard
- Handlers: `/tags/*` CRUD routes

### 1.4 Maintenance Windows
- Migration 0006: `maintenance_windows(id, name, start_time, end_time, active, created_at, updated_at)` + `monitor_maintenance(window_id FK, monitor_id FK)`
- `internal/models/maintenance.go` — MaintenanceWindow, MaintenanceStore
- Scheduler: before running a check, query `IsInMaintenance(monitorID, now)`; if true, skip
- Handlers: `/maintenance/*` CRUD routes

### 1.5 Dark/Light Theme Toggle
- `sm_theme` cookie (light/dark); JS reads it and sets `data-theme` on `<html>` element
- CSS: `[data-theme="light"]` override block in `partials.html` styles
- Toggle button in navbar; `POST /settings/theme` handler sets or clears the cookie

### 1.6 Latency Sparkline Charts
- `HeartbeatStore.LatencyHistory(monitorID, limit)` returns latencies newest-first
- Dashboard handler: compute per-monitor sparkline SVG in Go and pass as `map[int64]template.HTML`
- Template: add `{{index $.Sparklines .ID}}` column to dashboard table

---

## Phase 2 — Security Features ✅ Complete

### 2.1 API Keys ✅
- Migration `migrations_users/0002_api_keys`: `api_keys(id, username, name, token_hash, created_at, last_used_at)`
- Generate random 32-byte token; display once; store SHA-256 hash
- Auth middleware: accept `Authorization: Bearer <token>` alongside session cookie
- Handlers: `/api-keys/*` CRUD

### 2.2 Two-Factor Auth (TOTP) ✅
- Migration `migrations_users/0003_2fa`: add `totp_secret TEXT`, `totp_enabled INT DEFAULT 0` to users table
- Dependency: `github.com/pquerna/otp`
- Login becomes two-step when enabled; `/account/2fa` setup page with QR code (embedded `data:` URI)
- Pending cookie (`sm_pending`) gates the TOTP verification step

### 2.3 Account Registration ✅
- Migration `migrations_users/0004_registration_tokens` + `0005_token_expiry_settings`
- On first startup (no users) a 30-minute system token is printed to the console; registering with it grants admin
- `app_settings` table stores `registration_enabled` (default `true`); admin can toggle from `/admin/settings`
- Admin can generate unlimited invite links (`created_by = admin username`, no expiry) from `/admin/users`
- Open registration (no token) always creates a normal user

### 2.4 Authorization ✅
- Migration `migrations_users/0006_roles`: `is_admin INTEGER NOT NULL DEFAULT 0`; existing first user promoted on upgrade
- `AdminRequired()` middleware gates `/admin/users/*` and `/admin/settings` routes
- `UserStore.SetAdmin()` allows promote/demote; first startup-token registrant auto-promoted
- Normal users are already isolated to their own data by the per-user DB design
- Navbar hides Users/Settings links for non-admins; Users page shows Role column + Make Admin / Revoke Admin buttons

---

## Phase 3 — New Monitor Types (A) ✅ Complete

### 3.1 WebSocket Upgrade ✅
- No new DB columns (uses `url` field, `ws://` or `wss://`)
- Add `MonitorTypeWebSocket` constant
- `WebSocketChecker`: dial, verify 101 Switching Protocols (use `nhooyr.io/websocket`)

### 3.2 Docker Container Monitor ✅
- Migration 0007: `docker_hosts(id, name, socket_path, http_url)` + add `docker_host_id INT`, `docker_container_id TEXT` to monitors
- `DockerChecker`: raw HTTP to Docker daemon API; check `State.Running` + optional health check status
- Handlers: `/docker-hosts/*` CRUD; `DockerHostLookup` callback threads per-user DB to checker at runtime

### 3.3 Microsoft SQL Server ✅
- No new DB columns (reuses `url` + `db_query`)
- Dependency: `github.com/microsoft/go-mssqldb`
- `MSSQLChecker` in `checker_db.go` — same pattern as MySQL/Postgres

### 3.4 MQTT ✅
- Migration 0008: add `mqtt_topic TEXT`, `mqtt_username TEXT`, `mqtt_password TEXT`
- Dependency: `github.com/eclipse/paho.mqtt.golang`
- `MQTTChecker`: connect, subscribe, wait for message within timeout; optional keyword assertion

### 3.5 gRPC Keyword ✅
- Migration 0009: add `grpc_protobuf TEXT`, `grpc_service_name TEXT`, `grpc_method TEXT`, `grpc_body TEXT`, `grpc_enable_tls INT`
- Dependency: `google.golang.org/grpc`
- `GRPCChecker`: standard `grpc.health.v1.Health/Check`; optional keyword assertion on status string; TLS support

---

## Phase 4 — More Notification Providers ✅ Complete

All 48 providers are implemented and registered in `internal/notifier/notifier.go`.
Every provider has unit tests (field-validation + `httptest` HTTP roundtrip).

### 4.1 Webhook / Messaging Platforms
| Key | Provider |
|-----|---------|
| `webhook` | Generic JSON webhook |
| `telegram` | Telegram Bot API |
| `slack` | Slack Incoming Webhooks |
| `discord` | Discord Webhooks |
| `ntfy` | ntfy self-hosted push |
| `mattermost` | Mattermost Webhooks |
| `rocketchat` | Rocket.Chat Webhooks |
| `dingding` | DingTalk (钉钉) |
| `feishu` | Feishu / Lark |
| `googlechat` | Google Chat Spaces |
| `teams` | Microsoft Teams Webhooks |
| `wecom` | WeCom (企业微信) |
| `yzj` | YZJ (云之家) |
| `lunasea` | LunaSea push |

### 4.2 Mobile / Desktop Push
| Key | Provider |
|-----|---------|
| `gotify` | Gotify self-hosted |
| `bark` | Bark (iOS) |
| `gorush` | Gorush push gateway |
| `pushover` | Pushover |
| `pushplus` | PushPlus (微信) |
| `serverchan` | Server酱 (ServerChan) |
| `line` | LINE Notify |
| `homeassistant` | Home Assistant |

### 4.3 Multi-Field / Complex Providers
| Key | Provider |
|-----|---------|
| `pagerduty` | PagerDuty Events API v2 |
| `matrix` | Matrix (Element) |
| `signal` | Signal via signal-cli-rest-api |
| `waha` | WAHA WhatsApp HTTP API |
| `whapi` | Whapi.cloud WhatsApp |
| `onesender` | OneSender |
| `onebot` | OneBot (QQ) |
| `evolution` | Evolution API (WhatsApp) |

### 4.4 Email
| Key | Provider |
|-----|---------|
| `email` | SMTP |
| `sendgrid` | SendGrid |
| `resend` | Resend |
| `twilio` | Twilio SMS |

### 4.5 SMS
| Key | Provider |
|-----|---------|
| `46elks` | 46elks |
| `brevo` | Brevo (Sendinblue) SMS |
| `callmebot` | CallMeBot (WhatsApp/Signal) |
| `cellsynt` | Cellsynt |
| `freemobile` | Free Mobile (France) |
| `gtxmessaging` | GTX Messaging |
| `octopush` | Octopush |
| `promosms` | PromoSMS (Poland) |
| `serwersms` | SerwerSMS (Poland) |
| `sevenio` | seven.io (sms77) |
| `smsc` | SMSC.ru |
| `smseagle` | SMSEagle hardware |
| `smsir` | SMS.ir (Iran) |
| `teltonika` | Teltonika router SMS |

### 4.6 File Structure
- `internal/notifier/notifier.go` — `Registry` map + `Notifier.Send()`
- `internal/notifier/webhook_providers.go` — Mattermost, RocketChat, DingDing, Feishu, GoogleChat, Teams, WeCom, YZJ, LunaSea
- `internal/notifier/token_providers.go` — Gotify, Bark, Gorush, PushPlus, ServerChan, LINE, HomeAssistant
- `internal/notifier/multifield_providers.go` — PagerDuty, Pushover, Matrix, Signal, WAHA, Whapi, OneSender, OneBot, Evolution
- `internal/notifier/email_providers.go` — SendGrid, Resend, Twilio
- `internal/notifier/sms_providers.go` — 46elks, Brevo, CallMeBot, Cellsynt, FreeMobile, GTXMessaging, Octopush
- `internal/notifier/sms_providers2.go` — PromoSMS, SerwerSMS, SevenIO, SMSC, SMSEagle, SMS.ir, Teltonika

---

## Phase 5 — Infrastructure

### 5.1 Proxy Management
- Migration 0018: `proxies(id, name, protocol, host, port, username, password)` + add `proxy_id INT` to monitors
- In `HTTPChecker`: if `proxy_id` set, configure `http.Transport.Proxy`
- Handlers: `/proxies/*` CRUD

---

## Phase 6 — Monitor Types (B)

### 6.1 SNMP
- Migration 0019: add `snmp_community`, `snmp_version`, `snmp_oid`, `snmp_expected`
- Dependency: `github.com/gosnmp/gosnmp`
- `SNMPChecker`: GET OID + optional `compareExpectedValue` assertion

### 6.2 RabbitMQ
- No new columns (URL = management API endpoint)
- `RabbitMQChecker`: `GET {url}/api/healthchecks/node`, check `status == "ok"`

### 6.3 Kafka Producer
- Migration 0020: add `kafka_ssl INT DEFAULT 0`
- Dependency: `github.com/twmb/franz-go` (pure Go)
- `KafkaChecker`: dial brokers, produce a test message, confirm delivery

### 6.4 SIP Options
- No new columns (URL = `sip:host:port`)
- `SIPChecker`: raw UDP SIP OPTIONS, expect 200 OK

### 6.5 Radius
- Migration 0021: add `radius_secret TEXT`, `radius_called_station_id TEXT`; reuse `http_username`/`http_password` for credentials
- Dependency: `layeh.com/radius`
- `RadiusChecker`: Access-Request; Access-Accept or Access-Reject = UP, no response = DOWN

### 6.6 System Service
- Migration 0022: add `service_name TEXT`
- OS build tags: `systemctl` via D-Bus on Linux; SCM via `golang.org/x/sys/windows/svc/mgr`

---

## Phase 7 — Niche & Advanced

### 7.1 Steam Game Server  
A2S_INFO UDP protocol (manual implementation, no external lib).

### 7.2 GameDig  
A2S + Quake UDP subsets.

### 7.3 Tailscale Ping  
`exec.CommandContext("tailscale", "ping", "--c", "1", host)`.

### 7.4 Globalping  
POST to `api.globalping.io/v1/measurements`, poll for result.

### 7.5 Group / Manual Monitor  
- `group`: status derived from child monitors (all UP = UP)
- `manual`: status toggled via UI button; no checker

### 7.6 Real Browser (Chromium)  
- Dependency: `github.com/chromedp/chromedp`
- Navigate URL + optional keyword match in DOM

### 7.7 Remote Browser Config  
- Migration 0023: `remote_browsers(id, name, endpoint_url)`
- `BrowserChecker` connects via remote DevTools WebSocket

### 7.8 Cloudflare Tunnel  
- Docs + `compose.yaml` service only; no app code changes

---

## Key Files Reference

| File | Purpose |
|------|---------|
| `internal/models/monitor.go` | Monitor struct — add new type enum values + fields for each phase |
| `internal/models/store.go` | MonitorStore / HeartbeatStore — update SQL for new columns |
| `internal/monitor/checker.go` | Core checker dispatch + HTTP checker cert-expiry logic |
| `internal/monitor/checker_db.go` | DB checkers (MySQL, Postgres, Redis, MongoDB, MSSQL) |
| `internal/notifier/` | One file per notification provider |
| `internal/scheduler/scheduler.go` | Maintenance window skip logic |
| `internal/web/handlers/` | One handler file per feature group |
| `internal/web/templates/` | SSR HTML templates (dark/light CSS) |
| `internal/web/router.go` | Register all routes here |
| `internal/database/migrations_user/` | Per-user DB migrations (0003 onwards) |
| `internal/database/migrations_users/` | Shared users DB migrations (0002+ for API keys, 2FA) |
| `go.mod` | Add new dependencies as needed |

---

## Verification Checklist (Each Phase)

1. `go build ./...` — zero build errors
2. `go test ./...` — all existing tests pass
3. Write unit test for each new checker (follow `checker_smtp_test.go` pattern)
4. Manual: `go run cmd/server/main.go`, exercise feature in browser
5. Update `FEATURES.md`: change ⬜ → ✅ after implementation

---

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Public status page URL | `/status/:username/:slug` | Unambiguous DB lookup without global slug search |
| Cloudflare Tunnel | Docs only | No code changes needed; handled by infrastructure |
| Proxy support | HTTP/HTTPS first | SOCKS5 is stretch goal |
| Real Browser | Optional | Requires Chrome binary; document requirement |
| Steam/GameDig | Manual protocol impl | Avoids large game-query library dependencies |
| Tailscale Ping | `exec` subprocess | Simplest reliable approach; depends on `tailscale` CLI |
| Globalping | Free public API | No API key required for basic use |
| API keys | Coexist with session auth | Same handler functions serve both |
| 2FA | Per-user opt-in | Admin cannot force 2FA on others (for now) |
