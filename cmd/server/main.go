package main

import (
	"context"
	"database/sql"
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

	// Open and migrate the shared users database.
	usersDB, err := database.Open(filepath.Join(cfg.DataDir, "users.db"))
	if err != nil {
		slog.Error("failed to open users database", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := usersDB.Close(); err != nil {
			slog.Error("failed to close users database", "error", err)
		}
	}()

	if err := database.MigrateUsersDB(usersDB); err != nil {
		slog.Error("failed to run users migrations", "error", err)
		os.Exit(1)
	}

	// Create per-user DB registry.
	registry := database.NewRegistry(cfg.DataDir)
	defer registry.Close()

	// Create multi-scheduler.
	msched := scheduler.NewMulti()
	defer msched.Stop()

	// Initialize databases and schedulers for all existing users.
	userStore := models.NewUserStore(usersDB)
	existingUsers, err := userStore.ListAll()
	if err != nil {
		slog.Error("failed to list users", "error", err)
		os.Exit(1)
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

	// Set up DockerHostLookup so the DockerChecker can resolve docker_host_id
	// values to their connection details at check time using the per-user DB.
	monitor.DockerHostLookup = func(db *sql.DB, id int64) (string, string) {
		h, err := models.NewDockerHostStore(db).Get(id)
		if err != nil || h == nil {
			return "", ""
		}
		return h.SocketPath, h.HTTPURL
	}

	// Set up ProxyLookup so the HTTP checker can resolve proxy_id values to
	// their proxy URL at schedule time using the per-user DB.
	monitor.ProxyLookup = func(db *sql.DB, id int64) string {
		p, err := models.NewProxyStore(db).Get(id)
		if err != nil || p == nil {
			return ""
		}
		return p.URL
	}

	// On first startup (no users) generate a short-lived registration token and
	// print the setup URL to the console so the operator can create the admin account.
	if len(existingUsers) == 0 {
		regTokenStore := models.NewRegistrationTokenStore(usersDB)
		token, err := regTokenStore.Generate("system", 30*time.Minute)
		if err != nil {
			slog.Warn("failed to generate setup token", "error", err)
		} else {
			addr := cfg.ListenAddr
			if len(addr) > 0 && addr[0] == ':' {
				addr = "localhost" + addr
			}
			setupURL := "http://" + addr + "/register?token=" + token
			slog.Info("first run: register admin account", "url", setupURL, "expires_in", "30m")
		}
	}

	// Start the web server. The handlers will use the usersDB and registry to access user and host data,
	// and the multi-scheduler to start/stop checks when users are created/deleted.
	systemMailer := mailer.New(cfg)
	if systemMailer.Enabled() {
		slog.Info("system SMTP enabled", "host", cfg.SystemSMTPHost)
	}
	gin.SetMode(gin.ReleaseMode)
	router := web.NewRouter(usersDB, registry, msched, cfg, systemMailer)

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("conductor listening", "addr", cfg.ListenAddr)
		gin.SetMode(gin.DebugMode)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server with a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down gracefully")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
