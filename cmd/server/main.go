package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/xdung24/conductor/internal/config"
	"github.com/xdung24/conductor/internal/database"
	"github.com/xdung24/conductor/internal/models"
	"github.com/xdung24/conductor/internal/monitor"
	"github.com/xdung24/conductor/internal/scheduler"
	"github.com/xdung24/conductor/internal/web"
)

func main() {
	cfg := config.Load()

	// Open and migrate the shared users database.
	usersDB, err := database.Open(filepath.Join(cfg.DataDir, "users.db"))
	if err != nil {
		log.Fatalf("failed to open users database: %v", err)
	}
	defer func() {
		if err := usersDB.Close(); err != nil {
			log.Printf("failed to close users database: %v", err)
		}
	}()

	if err := database.MigrateUsersDB(usersDB); err != nil {
		log.Fatalf("failed to run users migrations: %v", err)
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
		log.Fatalf("failed to list users: %v", err)
	}
	for _, u := range existingUsers {
		db, err := registry.Get(u.Username)
		if err != nil {
			log.Printf("warning: failed to open db for user %q: %v", u.Username, err)
			continue
		}
		msched.StartForUser(u.Username, db)
	}
	log.Printf("initialized %d user database(s)", len(existingUsers))

	// Set up DockerHostLookup so the DockerChecker can resolve docker_host_id
	// values to their connection details at check time using the per-user DB.
	monitor.DockerHostLookup = func(db *sql.DB, id int64) (string, string) {
		h, err := models.NewDockerHostStore(db).Get(id)
		if err != nil || h == nil {
			return "", ""
		}
		return h.SocketPath, h.HTTPURL
	}

	// On first startup (no users) generate a short-lived registration token and
	// print the setup URL to the console so the operator can create the admin account.
	if len(existingUsers) == 0 {
		regTokenStore := models.NewRegistrationTokenStore(usersDB)
		token, err := regTokenStore.Generate("system", 30*time.Minute)
		if err != nil {
			log.Printf("warning: failed to generate setup token: %v", err)
		} else {
			addr := cfg.ListenAddr
			if len(addr) > 0 && addr[0] == ':' {
				addr = "localhost" + addr
			}
			log.Printf("=================================================================")
			log.Printf("  No users found — register your admin account (expires in 30 min)")
			log.Printf("  http://%s/register?token=%s", addr, token)
			log.Printf("=================================================================")
		}
	}

	router := web.NewRouter(usersDB, registry, msched, cfg)

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("conductor listening on %s", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}
	log.Println("server stopped")
}
