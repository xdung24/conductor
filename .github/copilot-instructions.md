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
- **Templates**: Go `html/template` embedded via `embed.FS` — parsed at startup, baked into the binary

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
cmd/server/              Entry point (main.go) — opens DBs, starts schedulers, HTTP server + graceful shutdown
internal/
  config/                Env-based config (LISTEN_ADDR, DATA_DIR, SECRET_KEY)
  database/              SQLite open/close + migration runners + per-user Registry
    migrations_users/    Embedded SQL for shared users.db  (users + push_tokens tables)
    migrations_user/     Embedded SQL for per-user data.db (monitors, heartbeats, notifications, …)
    registry.go          Registry — lazily opens & caches per-user *sql.DB connections
  models/                Data types (Monitor, Heartbeat, User) + DB stores
  monitor/               Monitor checker implementations (HTTP, TCP, Ping, DNS, SMTP, Push)
  scheduler/             Goroutine-per-monitor periodic check scheduler + MultiScheduler
  notifier/              Notification providers (Slack, Discord, ntfy, Telegram, Email, Webhook)
  web/
    router.go            Gin router setup + embedded template loading
    embed.go             //go:embed directive for templates/*.html
    router_test.go       Single TestTemplatesParse test (parses all templates)
    handlers/
      dashboard.go       Auth middleware, setup/login/logout, dashboard, per-request context helpers
      monitors.go        Monitor CRUD handlers
      notifications.go   Notification provider CRUD + test + log handlers
      users.go           User management handlers (list, create, change-password, delete)
      auth_token.go      HMAC token sign/verify
    templates/           HTML templates (SSR, dark theme)
      partials.html      Shared CSS (styles) and navbar defines
      dashboard.html     Monitor list page
      monitor_form.html  Create/edit monitor form
      monitor_detail.html Monitor heartbeat history
      notification_list.html  Notification providers list
      notification_form.html  Create/edit notification form
      notification_log.html   Notification send history
      users.html         User management list page
      user_form.html     Create user / change password form
      login.html         Login page
      setup.html         First-run setup wizard
      error.html         Error page
Dockerfile               Multi-stage, non-root, alpine-based
compose.yaml             Docker Compose
Makefile                 build, run, dev, test, lint, clean, docker-build
```

## Database Architecture

The app uses **two tiers of SQLite databases** to eliminate write-lock contention between users.

### Shared users database (`DATA_DIR/users.db`)
- Migration source: `internal/database/migrations_users/`
- Tables: `users`, `push_tokens`
- `push_tokens` maps `token → username` so the unauthenticated `GET /push/:token` endpoint can resolve the owning user without touching any per-user DB.
- Opened once at startup by `database.Open` + `database.MigrateUsersDB`.

### Per-user data database (`DATA_DIR/users/<username>/data.db`)
- Migration source: `internal/database/migrations_user/`
- Tables: `monitors`, `heartbeats`, `notifications`, `monitor_notifications`, `notification_logs`
- Opened lazily on first request via `database.Registry.Get(username)`.
- Each user gets exactly one writer connection (`SetMaxOpenConns(1)`).

### Registry (`internal/database/registry.go`)
- `Registry.Get(username)` opens + migrates + caches the per-user DB.
- `Registry.Remove(username)` closes and evicts a cached connection (used on user deletion).
- `Registry.Close()` closes all connections on shutdown.

### MultiScheduler (`internal/scheduler/scheduler.go`)
- Wraps `map[username]*Scheduler`.
- `StartForUser(username, db)` creates and starts a Scheduler for one user (no-op if already running).
- `ForUser(username)` returns the per-user Scheduler.
- `StopUser(username)` stops and removes a single user's Scheduler.

### Auth middleware (`AuthRequired()` in `dashboard.go`)
After validating the session cookie it calls `registry.Get(username)` and injects two keys into the Gin context:
- `"sm_user_db"` → `*sql.DB` (per-user DB)
- `"sm_username"` → `string`

All protected handlers access stores via context helpers defined on `Handler`:
- `h.monitorStore(c)`, `h.heartbeatStore(c)`, `h.notifStore(c)`, `h.notifLogStore(c)`, `h.schedFor(c)`

## Key Conventions

- **Module path**: always `github.com/xdung24/service-monitor`
- **Indentation**: tabs (Go standard)
- **Naming**: Go idiomatic — camelCase for Go, snake_case for SQL columns
- **Error handling**: always wrap errors with `fmt.Errorf("context: %w", err)`
- **Error checking**: always check every error return value — never discard with `_` unless the call genuinely cannot fail or the error is intentionally ignored (document why with a comment). This applies to all store methods (`Create`, `Update`, `Delete`, `SetActive`, etc.) and any other function that returns an `error`.
- **No CGO**: all dependencies must be pure Go (no CGO required)
- **Templates**: each page is a `{{ define "filename.html" }}` block in its own file, pulling in `{{ template "styles" }}` and `{{ template "navbar" }}` from `partials.html`
- **SQL migrations**: filename format `NNNN_description.up.sql` / `NNNN_description.down.sql`; embedded via `//go:embed` in `database.go`
- **Never edit existing migration files** — always add a new numbered migration
- **Database context (noctx)**: never call `(*sql.DB).Exec`, `(*sql.DB).Query`, or `(*sql.DB).QueryRow` directly — always use the `Context` variants (`ExecContext`, `QueryContext`, `QueryRowContext`) passing `context.Background()` at minimum

## Feature Roadmap

All planned, in-progress, and completed features are tracked in **`FEATURES.md`** at the repository root.

**Rules:**
- Whenever a feature is implemented, update its status in `FEATURES.md` from `⬜ Planned` / `🚧 In Progress` to `✅ Done`.
- Whenever a new feature is identified (gap analysis, user request, etc.), add a row to the appropriate section of `FEATURES.md`.
- Before starting any new feature work, mark it `🚧 In Progress` in `FEATURES.md`.

## Adding a New Monitor Type

1. Add a constant to `internal/models/models.go` (`MonitorTypeFoo MonitorType = "foo"`)
2. If the type needs extra DB columns, create migration `NNNN_foo_fields.up.sql` / `.down.sql`
3. Update `Monitor` struct and all SQL queries in `internal/models/store.go`
4. Implement `Checker` interface in `internal/monitor/checker.go` (or a new file)
5. Register it in `checkerFor()` switch in `checker.go`
6. Add the `<option>` to `monitor_form.html`; add any type-specific form fields with JS show/hide
7. Update `monitorFromForm()` in `internal/web/handlers/monitors.go` to parse the new fields
8. Mark the feature `✅ Done` in `FEATURES.md`

## Adding a New Notification Provider

1. Create `internal/notifier/foo.go` implementing the `Provider` interface
2. Register it in `internal/notifier/notifier.go` (`Registry["foo"] = FooProvider{}`)
3. Add a fieldset to `notification_form.html` and toggle it in the existing `showFields()` JS
4. Mark the feature `✅ Done` in `FEATURES.md`

## User Management Routes

All routes are under the `AuthRequired` middleware group:

| Method | Path | Handler |
|--------|------|---------|
| GET | `/admin/users` | `UserList` |
| GET | `/admin/users/new` | `UserNew` |
| POST | `/admin/users` | `UserCreate` |
| GET | `/admin/users/:username/password` | `UserPasswordPage` |
| POST | `/admin/users/:username/password` | `UserChangePassword` |
| POST | `/admin/users/:username/delete` | `UserDelete` |

Deleting: cannot delete own account or last user. Deletion also calls `UnregisterAllPushTokens`, `StopUser`, `Registry.Remove`.

## Testing Philosophy

- **Prefer minimal tests** — do not add unit tests for every function or handler.
- **Write tests only for logic that has real failure modes**: parsers, calculators, token sign/verify, data transformations.
- **Do not write tests for**: HTTP handlers, template rendering, DB queries, or glue code — these are covered by integration/manual testing.
- **Templates**: a single `TestTemplatesParse` in `internal/web/router_test.go` covers all templates at once. No per-template tests needed. Templates are also embedded via `embed.FS` and parsed by `mustParseTemplates()` at startup — a bad template panics immediately before the server accepts requests.
- **Do not suggest adding tests** unless explicitly asked or the code is pure logic (no I/O, no framework deps).

## Database

- SQLite with WAL mode enabled, `_foreign_keys=ON`
- Single writer per DB file (`SetMaxOpenConns(1)`)
- Migrations are embedded in the binary — never edit existing migration files, always add new ones
- Two embed FSes in `database.go`: `usersMigrationsFS` (`migrations_users/*.sql`) and `userMigrationsFS` (`migrations_user/*.sql`)

## Communication Style

- Do not add emojis or icons to responses unless explicitly asked.

## Security Notes

- Passwords hashed with `bcrypt.DefaultCost`
- Session tokens: HMAC-SHA256 signed, stored in `HttpOnly` cookies
- `SECRET_KEY` env var must be set to a strong random value in production
- Templates use `html/template` (auto-escaping) — never use `text/template` for HTML
- Push tokens are registered in `users.db` so they can be resolved without exposing per-user DB paths
