# conductor

A production-ready, self-hosted uptime monitoring tool written in Go.

## Features

- HTTP / HTTPS, TCP, Ping, DNS, SMTP, Push/Heartbeat monitor types
- Steam (A2S), GameDig (A2S/Quake), and Real Browser (Chromium) monitor types
- HTTP extended checks: keyword match, JSON path, XPath, custom headers/body, bearer auth, redirect control, TLS certificate expiry alerts
- Database monitors: MySQL/MariaDB, PostgreSQL, MongoDB, Redis
- Server-side rendered dashboard (no JavaScript build step)
- **Per-user SQLite databases** — each user's monitors and notifications live in their own database file, eliminating write-lock contention
- Automatic schema migrations (embedded in binary)
- bcrypt password hashing, HMAC-signed session cookies
- **API key authentication** — generate named tokens; `Authorization: Bearer` header accepted alongside session cookies
- **Two-factor authentication (TOTP)** — QR code setup, per-user opt-in, enforced two-step login
- **Role-based access control** — first user (via startup token) becomes admin; admin-only routes for user management and settings
- **Account registration** — open registration or invite-link only; admin controls from settings page
- Notification providers: Slack, Discord, ntfy, Telegram, Email (SMTP), Webhook
- Public status pages — read-only pages showing selected monitors
- Maintenance windows — suppress alerts during scheduled downtime
- Tags / labels on monitors — color-coded, displayed on dashboard
- Monitor import / export (JSON)
- Dark/light theme toggle
- Latency sparkline charts
- User management admin panel — add, promote/demote, delete, change passwords
- Graceful shutdown
- Docker + Docker Compose support
- Cloudflare Tunnel support via optional `cloudflared` compose service
- Single compiled binary — no runtime dependencies

## Quick Start

```bash
# Run directly
go run ./cmd/server

# Open http://localhost:3001
# Follow the setup wizard to create your admin account
```

## Configuration

All config is via environment variables:

| Variable      | Default                   | Description                          |
|---------------|---------------------------|--------------------------------------|
| `LISTEN_ADDR` | `:3001`                   | HTTP listen address                  |
| `DATA_DIR`    | `./data`                  | Root data directory                  |
| `SECRET_KEY`  | `change-me-in-production` | HMAC key for sessions                |

### Database layout

```
DATA_DIR/
  users.db                          # shared — user accounts + push token index
  users/<username>/data.db           # per-user — monitors, heartbeats, notifications
```

Each user's data is isolated in their own SQLite file. The shared `users.db` holds only the `users` table and a `push_tokens` index (needed for unauthenticated `/push/:token` routing).

## Docker

```bash
docker compose up --build
```

To run behind Cloudflare Tunnel, start the optional profile and provide your tunnel token:

```bash
CLOUDFLARE_TUNNEL_TOKEN=... docker compose --profile tunnel up --build
```

## Project Structure

```
cmd/server/          Entry point
internal/
  config/            Environment config
  database/          SQLite open + migrate + per-user Registry
    migrations_users/  SQL migrations for shared users.db
    migrations_user/   SQL migrations for per-user data.db
  models/            Data models + DB stores
  monitor/           Monitor checker implementations
  scheduler/         Periodic check scheduler + MultiScheduler
  notifier/          Notification providers
  web/
    handlers/        HTTP request handlers
    templates/       HTML templates (SSR)
Dockerfile
compose.yaml
Makefile
```

## Chart API

The monitor latency/downtime chart is driven by a JSON endpoint that can be embedded in any page.

### Endpoints

| Route | Auth | Caching |
|---|---|---|
| `GET /monitors/:id/chart-data?since=` | Session cookie or API key (owner/admin) | None — always realtime |
| `GET /status/:username/:slug/chart-data/:id?since=` | None (public) | 60 s in-memory TTL |

`?since=` accepts: `1h`, `6h`, `24h` (default), `7d`, `30d`.

### Response shape

```json
{
  "points": [
    { "ts": "2026-03-21T10:00:00Z", "latency": 142, "status": 1, "message": "" }
  ],
  "downtime": [
    { "start": "2026-03-21T09:00:00Z", "end": "2026-03-21T09:15:00Z" },
    { "start": "2026-03-21T10:30:00Z", "end": null }
  ]
}
```

- `status`: `1` = UP, `0` = DOWN
- `end: null` means the incident is still ongoing
- `points` are oldest-first, up to 500 entries

### Embedding the chart

Add two elements and call the render function from `dashboard.gohtml`:

```html
<svg id="my-chart-svg" width="100%" style="height:200px;"></svg>
<div id="my-chart-tooltip" style="position:fixed;display:none;"></div>
```

```js
// Copy renderChart(data, svgId, tooltipId) from dashboard.gohtml,
// then call it:
fetch('/status/alice/my-page/chart-data/3?since=24h')
  .then(r => r.json())
  .then(data => renderChart(data, 'my-chart-svg', 'my-chart-tooltip'));
```

**Key constraints when embedding:**
- The public endpoint only serves monitors that are linked to the named status page — arbitrary monitor IDs return 404.
- Call `renderChart` after the SVG element is visible in the DOM; it reads `getBoundingClientRect()` at call time.
- For a new embed location behind auth, add a route inside the `auth` group that calls the same DB queries and returns the same JSON shape.
