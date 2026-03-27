package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xdung24/conductor/internal/config"
	"github.com/xdung24/conductor/internal/database"
	"github.com/xdung24/conductor/internal/mailer"
	"github.com/xdung24/conductor/internal/models"
	"github.com/xdung24/conductor/internal/monitor"
	"github.com/xdung24/conductor/internal/scheduler"
	"github.com/xdung24/conductor/internal/web"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := config.Load()

	// migrate opens the DB and wires global lookups; no defers registered yet
	// so os.Exit is safe here.
	usersDB, registry, err := migrate(cfg)
	if err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := usersDB.Close(); err != nil {
			slog.Error("failed to close users database", "error", err)
		}
	}()
	defer registry.Close()

	systemMailer := mailer.New(cfg)
	if systemMailer.Enabled() {
		slog.Info("system SMTP enabled", "host", cfg.SystemSMTPHost)
	}

	msched, err := runScheduler(usersDB, registry)
	if err != nil {
		// return so defers above run cleanly.
		slog.Error("scheduler failed", "error", err)
		return
	}
	defer msched.Stop()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := runHTTPServer(ctx, cfg, usersDB, registry, msched, systemMailer); err != nil {
		slog.Error("server error", "error", err)
	}
}

// migrate opens and migrates the shared users database, creates the per-user
// DB registry, and wires up the global monitor lookup functions.
// Returns the open usersDB and registry; the caller is responsible for closing both.
func migrate(cfg *config.Config) (*sql.DB, *database.Registry, error) {
	usersDB, err := database.Open(filepath.Join(cfg.DataDir, "users.db"))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open users database: %w", err)
	}

	if err := database.MigrateUsersDB(usersDB); err != nil {
		_ = usersDB.Close()
		return nil, nil, fmt.Errorf("failed to run users migrations: %w", err)
	}

	registry := database.NewRegistry(cfg.DataDir)

	// Wire global lookup functions used by the monitor checkers.
	monitor.DockerHostLookup = func(db *sql.DB, id int64) (string, string) {
		h, err := models.NewDockerHostStore(db).Get(id)
		if err != nil || h == nil {
			return "", ""
		}
		return h.SocketPath, h.HTTPURL
	}
	monitor.ProxyLookup = func(db *sql.DB, id int64) string {
		p, err := models.NewProxyStore(db).Get(id)
		if err != nil || p == nil {
			return ""
		}
		return p.URL
	}
	monitor.RemoteBrowserLookup = func(db *sql.DB, id int64) string {
		rb, err := models.NewRemoteBrowserStore(db).Get(id)
		if err != nil || rb == nil {
			return ""
		}
		return rb.EndpointURL
	}

	return usersDB, registry, nil
}

// runScheduler opens per-user databases, starts a scheduler for every existing
// user, and prints the first-run setup URL when no users exist yet.
// Returns the running MultiScheduler; the caller is responsible for stopping it.
func runScheduler(usersDB *sql.DB, registry *database.Registry) (*scheduler.MultiScheduler, error) {
	msched := scheduler.NewMulti()

	userStore := models.NewUserStore(usersDB)
	existingUsers, err := userStore.ListAll()
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	for _, u := range existingUsers {
		db, err := registry.Get(u.Username)
		if err != nil {
			slog.Warn("failed to open db for user", "username", u.Username, "error", err)
			continue
		}
		msched.StartForUser(u.Username, db)
	}
	slog.Info("initialized user databases", "count", len(existingUsers))

	// On first startup generate a short-lived registration token so the
	// operator can create the admin account.
	if len(existingUsers) == 0 {
		regTokenStore := models.NewRegistrationTokenStore(usersDB)
		token, err := regTokenStore.Generate("system", 30*time.Minute)
		if err != nil {
			slog.Warn("failed to generate setup token", "error", err)
		} else {
			// usersDB is shared so we need the listen addr from the caller;
			// use a placeholder host — the operator will see it in the log.
			slog.Info("first run: no users found — create admin account via setup URL",
				"path", "/register?token="+token, "expires_in", "30m")
		}
	}

	return msched, nil
}

// runHTTPServer builds the router, starts the HTTP server, and blocks until ctx
// is cancelled or the server exits with a fatal error. It performs a graceful
// shutdown before returning.
func runHTTPServer(
	ctx context.Context,
	cfg *config.Config,
	usersDB *sql.DB,
	registry *database.Registry,
	msched *scheduler.MultiScheduler,
	systemMailer *mailer.Mailer,
) error {
	gin.SetMode(gin.ReleaseMode)
	router := web.NewRouter(usersDB, registry, msched, cfg, systemMailer)

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	srvErr := make(chan error, 1)
	go func() {
		slog.Info("conductor listening", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			srvErr <- err
		}
	}()

	select {
	case err := <-srvErr:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
	}

	slog.Info("shutting down gracefully")
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		return fmt.Errorf("forced shutdown: %w", err)
	}
	slog.Info("server stopped")
	return nil
}
