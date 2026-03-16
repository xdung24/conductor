# Service Monitor — Implementation Plan

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

## Phase 2 — Security Features

### 2.1 API Keys
- Migration 0013 (users DB `migrations_users/`): `api_keys(id, username, name, token_hash, created_at, last_used_at)`
- Generate random 32-byte token; display once; store bcrypt hash
- Auth middleware: accept `Authorization: Bearer <token>` alongside session cookie
- Handlers: `/api-keys/*` CRUD

### 2.2 Two-Factor Auth (TOTP)
- Migration 0014 (users DB): add `totp_secret TEXT`, `totp_enabled INT DEFAULT 0` to users table
- Dependency: `github.com/pquerna/otp`
- Login becomes two-step when enabled; `/account/2fa` setup page with QR code
- Session: set a "needs-2fa" semaphore before full login

### 2.3 Account registration
- By default, new user can register account.
- Admin can disable account registration from config.
- Admin can generate a unique link to allow single registration.

### 2.4 Authorization
- Only admin can manage users
- Only admin can manage monitors and notifications of all users.
- Normal user can only see their monitors and notifications

---

## Phase 3 — New Monitor Types (A)

### 3.1 WebSocket Upgrade
- No new DB columns (uses `url` field, `ws://` or `wss://`)
- Add `MonitorTypeWebSocket` constant
- `WebSocketChecker`: dial, verify 101 Switching Protocols (use `nhooyr.io/websocket`)

### 3.2 Docker Container Monitor
- Migration 0015: `docker_hosts(id, name, socket_path, http_url, tls_cert, tls_key, tls_ca)` + add `docker_host_id INT`, `docker_container_id TEXT` to monitors
- `DockerContainerChecker`: raw HTTP to Docker daemon API; check `State.Running`
- Handlers: `/docker-hosts/*` CRUD

### 3.3 Microsoft SQL Server
- No new DB columns (reuses `url` + `db_query`)
- Dependency: `github.com/microsoft/go-mssqldb`
- `MSSQLChecker` in `checker_db.go` — same pattern as MySQL/Postgres

### 3.4 MQTT
- Migration 0016: add `mqtt_topic TEXT`, `mqtt_username TEXT`, `mqtt_password TEXT`
- Dependency: `github.com/eclipse/paho.mqtt.golang`
- `MQTTChecker`: connect, subscribe, wait for message within timeout

### 3.5 gRPC Keyword
- Migration 0017: add `grpc_proto_file TEXT` (optional)
- Dependency: `google.golang.org/grpc`
- `GRPCChecker`: gRPC health check protocol + optional keyword match in response

---

## Phase 4 — More Notification Providers *(parallel with Phase 3)*

### 4.1 PagerDuty
- `notifier/pagerduty.go`; routing key + severity; Events API v2

### 4.2 Gotify
- `notifier/gotify.go`; server URL + app token + priority

### 4.3 Pushover
- `notifier/pushover.go`; user key + API token + optional device

### 4.4 Matrix
- `notifier/matrix.go`; home server + access token + room ID

Register all in `notifier/registry.go` + `notification_form.html`.

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
