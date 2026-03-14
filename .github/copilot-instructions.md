# Copilot Instructions for service-monitor

## Repository Overview

**service-monitor** is a self-hosted uptime monitoring tool written entirely in Go.
It uses server-side rendered HTML templates (no frontend build step) and SQLite for storage.

- **Language**: Go 1.25+
- **Module**: `github.com/xdung24/service-monitor`
- **HTTP Framework**: Gin (`github.com/gin-gonic/gin`)
- **Database**: SQLite via `modernc.org/sqlite` (pure Go, no CGO)
- **Migrations**: `github.com/golang-migrate/migrate/v4` with embedded SQL files
- **Auth**: bcrypt passwords + HMAC-signed session cookies
- **Templates**: Go `html/template` via Gin's `LoadHTMLGlob`

## Build & Run Commands

```bash
go build -o service-monitor ./cmd/server   # compile
go run ./cmd/server                         # run without building
go build ./...                              # check all packages compile
go vet ./...                                # lint
go test ./...                               # run tests
```

## Project Structure

```
cmd/server/              Entry point (main.go) — HTTP server + graceful shutdown
internal/
  config/                Env-based config (LISTEN_ADDR, DB_PATH, SECRET_KEY)
  database/              SQLite open/close + migration runner
    migrations/          Embedded SQL files (0001_init.up.sql, etc.)
  models/                Data types (Monitor, Heartbeat, User) + DB stores
  monitor/               Monitor checker implementations (HTTP, TCP, Ping)
  scheduler/             Goroutine-per-monitor periodic check scheduler
  notifier/              Notification providers (to be implemented)
  web/
    router.go            Gin router setup + template FuncMap
    handlers/
      dashboard.go       Auth middleware, setup/login/logout, dashboard
      monitors.go        Monitor CRUD handlers
      auth_token.go      HMAC token sign/verify
    templates/           HTML templates (SSR, dark theme)
      partials.html      Shared CSS (styles) and navbar defines
      dashboard.html     Monitor list page
      monitor_form.html  Create/edit monitor form
      monitor_detail.html Monitor heartbeat history
      login.html         Login page
      setup.html         First-run setup wizard
      error.html         Error page
Dockerfile               Multi-stage, non-root, alpine-based
compose.yaml             Docker Compose
Makefile                 build, run, dev, test, lint, clean, docker-build
```

## Key Conventions

- **Module path**: always `github.com/xdung24/service-monitor`
- **Indentation**: tabs (Go standard)
- **Naming**: Go idiomatic — camelCase for Go, snake_case for SQL columns
- **Error handling**: always wrap errors with `fmt.Errorf("context: %w", err)`
- **No CGO**: all dependencies must be pure Go (no CGO required)
- **Templates**: each page is a `{{ define "filename.html" }}` block in its own file, pulling in `{{ template "styles" }}` and `{{ template "navbar" }}` from `partials.html`
- **SQL migrations**: filename format `NNNN_description.up.sql` / `NNNN_description.down.sql`; embedded via `//go:embed` in `database.go`

## Adding a New Monitor Type

1. Add a constant to `internal/models/models.go` (`MonitorTypeFoo MonitorType = "foo"`)
2. Implement `Checker` interface in `internal/monitor/checker.go` (or a new file)
3. Register it in `checkerFor()` switch in `checker.go`
4. Add the option to `monitor_form.html` template

## Adding a New Notification Provider

1. Create `internal/notifier/foo.go` implementing a `Notifier` interface (to be defined)
2. Register it in `internal/notifier/notifier.go`
3. Add config fields to the `notifications` table if needed (JSON config blob)

## Database

- SQLite with WAL mode enabled, `_foreign_keys=ON`
- Single writer (`SetMaxOpenConns(1)`)
- Migrations are embedded in the binary — never edit existing migration files, always add new ones

## Security Notes

- Passwords hashed with `bcrypt.DefaultCost`
- Session tokens: HMAC-SHA256 signed, stored in `HttpOnly` cookies
- `SECRET_KEY` env var must be set to a strong random value in production
- Templates use `html/template` (auto-escaping) — never use `text/template` for HTML
