package models

import (
	"database/sql"
)

// AppSettingsStore provides runtime-configurable key-value settings backed
// by the shared users database.  Keys are stable string constants; values
// are always stored as plain text.
type AppSettingsStore struct {
	db *sql.DB
}

// NewAppSettingsStore creates an AppSettingsStore backed by db.
func NewAppSettingsStore(db *sql.DB) *AppSettingsStore {
	return &AppSettingsStore{db: db}
}

// get returns the raw string value for key, or "" when the key is absent.
func (s *AppSettingsStore) get(key string) string {
	var v string
	_ = s.db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, key).Scan(&v)
	return v
}

// set upserts a key-value pair.
func (s *AppSettingsStore) set(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO app_settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}

// RegistrationEnabled reports whether the public /register endpoint is open
// to anyone (i.e. no invite token required).
func (s *AppSettingsStore) RegistrationEnabled() bool {
	return s.get("registration_enabled") == "true"
}

// SetRegistrationEnabled persists the open-registration toggle.
func (s *AppSettingsStore) SetRegistrationEnabled(enabled bool) error {
	v := "false"
	if enabled {
		v = "true"
	}
	return s.set("registration_enabled", v)
}
