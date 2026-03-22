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

	// System SMTP — used for transactional emails (invite, password reset, etc.).
	// All fields are empty by default; setting SystemSMTPHost enables sending.
	SystemSMTPHost     string
	SystemSMTPPort     string
	SystemSMTPUsername string
	SystemSMTPPassword string
	SystemSMTPFrom     string
	SystemSMTPTLS      string // "true" (STARTTLS, default) or "false"
	SystemSMTPBCC      string // optional BCC added to every outgoing message
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		ListenAddr: getEnv("LISTEN_ADDR", ":3001"),
		DBPath:     getEnv("DB_PATH", "./data/conductor.db"),
		DataDir:    getEnv("DATA_DIR", "./data"),
		SecretKey:  getEnv("SECRET_KEY", "change-me-in-production"),

		SystemSMTPHost:     os.Getenv("SYSTEM_SMTP_HOST"),
		SystemSMTPPort:     getEnv("SYSTEM_SMTP_PORT", "587"),
		SystemSMTPUsername: os.Getenv("SYSTEM_SMTP_USERNAME"),
		SystemSMTPPassword: os.Getenv("SYSTEM_SMTP_PASSWORD"),
		SystemSMTPFrom:     os.Getenv("SYSTEM_SMTP_FROM"),
		SystemSMTPTLS:      getEnv("SYSTEM_SMTP_TLS", "true"),
		SystemSMTPBCC:      os.Getenv("SYSTEM_SMTP_BCC"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
