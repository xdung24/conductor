-- Add optional expiry to registration tokens so the startup token can
-- automatically expire after 30 minutes.
ALTER TABLE registration_tokens ADD COLUMN expires_at DATETIME;

-- Runtime admin settings (key-value).
-- registration_enabled defaults to false; admin enables it via the UI.
CREATE TABLE IF NOT EXISTS app_settings (
    key   TEXT NOT NULL PRIMARY KEY,
    value TEXT NOT NULL
);
INSERT OR IGNORE INTO app_settings (key, value) VALUES ('registration_enabled', 'true');
