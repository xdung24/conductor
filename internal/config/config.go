package config

import (
	"os"
)

// Config holds all application configuration sourced from environment variables.
// Runtime settings (e.g. registration_enabled) are stored in the database and
// editable by the admin via the web UI.
type Config struct {
	ListenAddr string
	DBPath     string
	DataDir    string
	SecretKey  string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		ListenAddr: getEnv("LISTEN_ADDR", ":3001"),
		DBPath:     getEnv("DB_PATH", "./data/service-monitor.db"),
		DataDir:    getEnv("DATA_DIR", "./data"),
		SecretKey:  getEnv("SECRET_KEY", "change-me-in-production"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
