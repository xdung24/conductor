package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/xdung24/service-monitor/internal/config"
	"github.com/xdung24/service-monitor/internal/database"
	"github.com/xdung24/service-monitor/internal/models"
	"github.com/xdung24/service-monitor/internal/scheduler"
	"github.com/xdung24/service-monitor/internal/web"
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

	router := web.NewRouter(usersDB, registry, msched, cfg)

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("service-monitor listening on %s", cfg.ListenAddr)
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
