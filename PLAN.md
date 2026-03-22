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

### 5.1 Proxy Management ✅
- Migration 0014: `proxies(id, name, url)` + `proxy_id INTEGER NOT NULL DEFAULT 0` on monitors
- `monitor.ProxyLookup` callback (like `DockerHostLookup`) resolves proxy FK to URL at schedule time
- `NewHTTPClient(m, proxyURL string)`: configures `http.Transport.Proxy` when proxyURL non-empty
- Scheduler resolves proxy URL for HTTP monitors when building the cached `*http.Client`
- Handlers: `/proxies/*` CRUD (ProxyList, ProxyNew, ProxyCreate, ProxyEdit, ProxyUpdate, ProxyDelete)
- Template: `proxies.html` management page; navbar link added
- Monitor form: proxy dropdown in HTTP fields section (`AllProxies` data key)

### 5.2 HTTP Client Reuse ✅
- Cache one `*http.Client` per monitor inside the `Scheduler` (keyed by monitor ID) via `monitor.NewHTTPClient(m)`
- Client built at `Schedule()` time; evicted at `Unschedule()`/`Stop()` (Transport releases idle sockets)
- Eliminates fresh TCP + TLS handshake on every HTTP check; Transport connection pool reused across checks
- Applies to HTTP/HTTPS and RabbitMQ monitor types
- `monitor.Cache` struct threads the optional cached client/connection from scheduler → checkerFor → checker

### 5.3 Database Connection Pool ✅
- Cache one `*sql.DB` per monitor inside the `Scheduler` via `monitor.NewDBConn(m)`
- Pool built at `Schedule()` time; explicitly closed and evicted at `Unschedule()`/`Stop()`
- Eliminates `sql.Open()` + TCP handshake + auth round-trip on every DB check
- Applies to MySQL, PostgreSQL, and MSSQL monitor types
- Pool settings: `MaxOpenConns(1)`, `MaxIdleConns(1)`, `ConnMaxLifetime(5m)`, `ConnMaxIdleTime(2m)`

---

## Phase 6 — Monitor Types (B) ✅ Complete

### 6.1 SNMP ✅
- Migration 0010: add `snmp_community`, `snmp_version`, `snmp_oid`, `snmp_expected`
- Dependency: `github.com/gosnmp/gosnmp`
- `SNMPChecker`: GET OID + optional `compareExpectedValue` assertion

### 6.2 RabbitMQ ✅
- No new columns (URL = management API endpoint)
- `RabbitMQChecker`: `GET {url}/api/healthchecks/node`, check `status == "ok"`

### 6.3 Kafka Producer ✅
- Migration 0011: add `kafka_topic TEXT`
- Dependency: `github.com/twmb/franz-go` (pure Go)
- `KafkaChecker`: dial brokers, produce a test message, confirm delivery

### 6.4 SIP Options ✅
- No new columns (URL = `host:port`)
- `SIPChecker`: raw UDP SIP OPTIONS, expect SIP/2.0 response

### 6.5 Radius ✅
- Migration 0013: add `radius_secret TEXT`, `radius_called_station_id TEXT`; reuse `http_username`/`http_password` for credentials
- Dependency: `layeh.com/radius`
- `RadiusChecker`: Access-Request; Access-Accept or Access-Reject = UP, no response = DOWN

### 6.6 System Service ✅
- Migration 0010: add `service_name TEXT`
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

---

## Phase 8 — User Management V2 & System Email

All decisions resolved. Ready to implement.

### Resolved Decisions

| Q | Decision |
|---|---|
| Existing non-email usernames (e.g. `admin`) | No forced migration. Show a warning banner on the admin settings page if any account's username fails email validation. Login is never blocked. |
| "Remove 2FA" / "2FA Not Set" display | Show grey "2FA Not Set" (non-clickable) when inactive. Show an orange "Remove 2FA" button with a `confirm()` dialog when active. |
| Recipient email for invite link | **Mandatory.** Admin must enter the recipient's email. The link is also shown in the flash message as a copy-paste fallback. |
| System SMTP config location | **Environment variables only** (`SYSTEM_SMTP_*`) — no DB storage. |
| Email format | **HTML** with inline CSS matching the app's brand. A plain-text `multipart/alternative` fallback is always included. |
| Email errors | **Fire-and-forget** goroutine. SMTP errors log a `[WARNING]` line and never affect the HTTP response. `SYSTEM_SMTP_BCC` adds a BCC to every message for delivery verification. |

### 8.1 Remove "Make/Revoke Admin" Button

- Remove role-toggle button from `users.html`
- Remove `UserSetAdmin` handler from `handlers/users.go`
- Remove `POST /admin/users/:username/role` route from `router.go`
- The `is_admin` column and `SetAdmin` store method are kept for future CLI use

### 8.2 Remove "Delete User" Button

- Remove Delete button from `users.html`
- Remove `UserDelete` handler from `handlers/users.go`
- Remove `POST /admin/users/:username/delete` route from `router.go`
- The Disable feature (V1) is the intended replacement

### 8.3 "Remove 2FA" / "2FA Not Set" Cell

**Model change:** Add `TOTPEnabled bool` to the `User` struct; extend `GetByUsername` + `ListAll` SQL to include `totp_enabled`. No new migration — column exists.

**New route:** `POST /admin/users/:username/remove-2fa`  
**New handler:** `UserRemove2FA`
1. Reject if target does not exist.
2. Reject if `target == currentUser`.
3. Call `users.DisableTOTP(target)` — clears `totp_secret`, sets `totp_enabled=0`.
4. Send `2fa_removed` system email to the target.
5. Redirect `/admin/users?flash=…`

**Template (`users.html`):**
```
if .TOTPEnabled → <form POST /admin/users/{{.Username}}/remove-2fa onsubmit="confirm(...)">
                    <button class="btn btn-warning btn-sm">Remove 2FA</button>
                  </form>
else            → <span style="color:#64748b; font-size:0.8rem;">2FA Not Set</span>
```

### 8.4 Username = Email Address

No new migration. Enforcement is at the application layer only.

**`validateEmail(s string) error`** — new file `internal/web/handlers/validation.go`:

| Rule | Prevents |
|---|---|
| Non-empty, max 254 chars | RFC 5321 limit |
| ASCII-only (no Unicode) | Homograph attacks |
| No `+` in local part | Plus-alias account farming |
| No whitespace | Smuggling / display confusion |
| Exactly one `@` | Basic structure |
| Local allow-list: `a-zA-Z0-9._-`; no leading/trailing `.` or `-`; no `..` | Malformed addresses |
| Domain: `a-zA-Z0-9-.`; at least one `.`; no leading/trailing `.` or `-` | Valid hostname |
| Lowercased before storing | Case-variant duplicates |

Applied in: `UserCreate`, `RegisterSubmit`, `InviteGenerate` (recipient field).

**UI label changes:** "Username" → "Email" in `login.html`, `register.html`, `user_form.html`, `users.html` (column header).

**Non-email existing accounts:** Admin settings page shows a subtle banner if any account's username is not a valid email (advisory only, no enforcement).

### 8.5 User List: Search Filter + Pagination

**URL params:** `?q=<email substring>` + `?page=<1-based>`

**New store method:** `ListPaged(q string, page, pageSize int) ([]*User, int, error)`  
- `WHERE username LIKE '%'||?||'%'` + `LIMIT ? OFFSET ?`
- Returns: user slice, total matching count, error

**Handler (`UserList`):** Reads `q` + `page`; passes `Users`, `Total`, `Page`, `PageSize`, `TotalPages`, `Q` to template.

**Template (`users.html`):**
- Search bar above the table (GET form, `name="q"`)
- Pagination bar below: Previous / Next links + "Page X of Y"
- 10 items per page

### 8.6 System SMTP Configuration

New env vars added to `internal/config/config.go`:

| Variable | Default | Notes |
|---|---|---|
| `SYSTEM_SMTP_HOST` | _(empty = disabled)_ | Setting this enables system email |
| `SYSTEM_SMTP_PORT` | `587` | |
| `SYSTEM_SMTP_USERNAME` | | |
| `SYSTEM_SMTP_PASSWORD` | | |
| `SYSTEM_SMTP_FROM` | | e.g. `noreply@example.com` |
| `SYSTEM_SMTP_TLS` | `"true"` | `"false"` = plain SMTP |
| `SYSTEM_SMTP_BCC` | _(empty)_ | Added to every outgoing email |

### 8.7 System Mailer Package

New package: `internal/mailer/`

```
internal/mailer/
  mailer.go      — Mailer struct; send(); SendAsync()
  templates.go   — RenderXxx() HTML email helpers
```

`Mailer` fields mirror the `SYSTEM_SMTP_*` env vars. `SendAsync(to, subject, htmlBody)` spawns a goroutine; on error logs `[WARNING] system email to <to> failed: <err>`. If `SYSTEM_SMTP_HOST` is empty, `SendAsync` is a no-op.

**HTML email design** (inline CSS only — email clients strip `<link>`):
- White card on `#f8fafc` background
- Conductor text header in `#38bdf8`
- Body text `#1e293b`, muted text `#64748b`
- Muted footer: "This is an automated message from Conductor. Do not reply."

**Email catalogue:**

| ID | Subject |
|---|---|
| `invite` | You've been invited to Conductor |
| `password_reset` | Reset your Conductor password |
| `account_disabled` | Your Conductor account has been disabled |
| `account_enabled` | Your Conductor account has been re-enabled |
| `2fa_removed` | Two-factor authentication removed |
| `2fa_enabled` | Two-factor authentication enabled |
| `password_changed_admin` | Your password has been changed |
| `password_changed_reset` | Your password has been changed |

### 8.8 Transactional Email Triggers

| # | Handler | Trigger | Email ID | Recipient |
|---|---|---|---|---|
| 1 | `InviteGenerate` | Invite link created | `invite` | `recipient_email` form field (mandatory) |
| 2 | `UserGenerateResetLink` | Reset link created | `password_reset` | Target user's username |
| 3 | `UserToggleDisabled` | Account disabled | `account_disabled` | Target user's username |
| 4 | `UserToggleDisabled` | Account enabled | `account_enabled` | Target user's username |
| 5 | `UserRemove2FA` | Admin removes 2FA | `2fa_removed` | Target user's username |
| 6 | `TwoFAVerify` (account.go) | User enables 2FA | `2fa_enabled` | Logged-in user's username |
| 7 | `UserChangePassword` | Admin changes password | `password_changed_admin` | Target user's username |
| 8 | `ResetPasswordSubmit` | Reset link redeemed | `password_changed_reset` | Username on the reset token |

### 8.9 Implementation Order

| Step | Work | Checkpoint |
|---|---|---|
| 1 | Add `TOTPEnabled bool` to `User`; update `GetByUsername` + `ListAll` SQL | `go build ./...` |
| 2 | Add `ListPaged` to `UserStore` | `go build ./...` |
| 3 | Remove `UserDelete` + `UserSetAdmin` handlers + routes | `go test ./...` |
| 4 | Add `UserRemove2FA` handler + route | `go test ./...` |
| 5 | Create `validation.go`; add `validateEmail` to `UserCreate`, `RegisterSubmit`, `InviteGenerate` | `go test ./...` |
| 6 | Update `users.html`: remove buttons; add 2FA cell; search + pagination; "Email" label | `go test ./internal/web/...` |
| 7 | Update `login.html`, `register.html`, `user_form.html`: Username → Email | `go test ./internal/web/...` |
| 8 | Update invite form in `register.html`/`users.html`: mandatory `recipient_email` | `go test ./internal/web/...` |
| 9 | Add `SYSTEM_SMTP_*` fields to `config.go` | `go build ./...` |
| 10 | Create `internal/mailer/mailer.go` + `templates.go` | `go build ./...` |
| 11 | Wire `*mailer.Mailer` into `handlers.New()` + `cmd/server/main.go` | `go build ./...` |
| 12 | Email triggers: `InviteGenerate` (8.1) | `go test ./...` |
| 13 | Email triggers: `UserGenerateResetLink`, `UserToggleDisabled`, `UserRemove2FA` (8.2–8.5) | `go test ./...` |
| 14 | Email triggers: `TwoFAVerify`, `UserChangePassword`, `ResetPasswordSubmit` (8.6–8.8) | `go test ./...` |
| 15 | Update `admin_settings.html`: SMTP status indicator + non-email username warning | `go test ./internal/web/...` |
| 16 | Update `FEATURES.md` | — |

### 8.10 Files Changed

**New files:**

| File | Purpose |
|---|---|
| `internal/mailer/mailer.go` | Mailer struct, `send`, `SendAsync` |
| `internal/mailer/templates.go` | HTML email render helpers |
| `internal/web/handlers/validation.go` | `validateEmail(s string) error` |

**Modified files:**

| File | Changes |
|---|---|
| `internal/config/config.go` | Add `SystemSMTP*` fields |
| `internal/models/user.go` | Add `TOTPEnabled bool` |
| `internal/models/store.go` | Update `GetByUsername`, `ListAll` (add `totp_enabled`); add `ListPaged` |
| `internal/web/handlers/dashboard.go` | Accept `*mailer.Mailer` in `Handler`; update `handlers.New()` signature |
| `internal/web/handlers/users.go` | Remove `UserDelete`, `UserSetAdmin`; add `UserRemove2FA`; call `validateEmail`; email triggers |
| `internal/web/handlers/account.go` | Email trigger in `TwoFAVerify` |
| `internal/web/handlers/register.go` | `validateEmail` in `RegisterSubmit`; mandatory `recipient_email` in `InviteGenerate` |
| `internal/web/router.go` | Remove deleted routes; add `/remove-2fa` route |
| `internal/web/templates/users.html` | Remove admin/delete buttons; 2FA cell; search + pagination; "Email" label |
| `internal/web/templates/login.html` | "Username" → "Email" |
| `internal/web/templates/register.html` | "Username" → "Email"; mandatory recipient email field |
| `internal/web/templates/user_form.html` | "Username" → "Email" |
| `internal/web/templates/admin_settings.html` | SMTP status indicator; non-email username warning |
| `cmd/server/main.go` | Construct `*mailer.Mailer`; pass to `handlers.New()` |
| `FEATURES.md` | Update status rows |
