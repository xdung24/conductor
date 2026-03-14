# service-monitor

A production-ready, self-hosted uptime monitoring tool written in Go.

## Features

- HTTP / HTTPS, TCP, Ping monitor types (more coming)
- Server-side rendered dashboard (no JavaScript build step)
- SQLite database with automatic migrations
- bcrypt password hashing, HMAC-signed session cookies
- Graceful shutdown
- Docker + Docker Compose support
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

| Variable      | Default                          | Description              |
|---------------|----------------------------------|--------------------------|
| `LISTEN_ADDR` | `:3001`                          | HTTP listen address      |
| `DB_PATH`     | `./data/service-monitor.db`      | SQLite database path     |
| `DATA_DIR`    | `./data`                         | Data directory           |
| `SECRET_KEY`  | `change-me-in-production`        | HMAC key for sessions    |

## Docker

```bash
docker compose up --build
```

## Build

```bash
make build   # compile binary
make run     # build + run
make test    # run tests
make lint    # vet + staticcheck
```

## Project Structure

```
cmd/server/          Entry point
internal/
  config/            Environment config
  database/          SQLite open + migrate
    migrations/      SQL migration files
  models/            Data models + DB stores
  monitor/           Monitor checker implementations
  scheduler/         Periodic check scheduler
  notifier/          Notification providers (coming soon)
  web/
    handlers/        HTTP request handlers
    templates/       HTML templates (SSR)
Dockerfile
compose.yaml
Makefile
```

## Roadmap

- [ ] DNS monitor type
- [ ] Push (heartbeat) monitor type
- [ ] Notification providers (Slack, Telegram, Email, Webhook)
- [ ] Public status pages
- [ ] Certificate expiry monitoring
- [ ] Latency charts
- [ ] API endpoints (JSON)
