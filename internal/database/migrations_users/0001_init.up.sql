-- 0001_init.up.sql (users DB)
-- Consolidated schema for all migrations (0001–0007).
-- Shared database: stores user accounts, push tokens, API keys, registration
-- tokens, app settings, and status-page summary tokens.

CREATE TABLE IF NOT EXISTS users (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    username     TEXT    NOT NULL UNIQUE,
    password     TEXT    NOT NULL,
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    -- 0003: TOTP two-factor authentication
    totp_secret  TEXT,
    totp_enabled INTEGER NOT NULL DEFAULT 0,
    -- 0006: admin role
    is_admin     INTEGER NOT NULL DEFAULT 0
);

-- Promote the earliest-registered user to admin.
UPDATE users SET is_admin = 1 WHERE id = (SELECT MIN(id) FROM users);

-- Maps every push-monitor token to its owning user so the unauthenticated
-- /push/:token endpoint can locate the correct per-user database.
CREATE TABLE IF NOT EXISTS push_tokens (
    token    TEXT NOT NULL PRIMARY KEY,
    username TEXT NOT NULL
);

-- 0002: Per-user API keys for token-based access.
-- token_hash stores SHA-256(plain_token) so lookup by hash is O(1).
CREATE TABLE IF NOT EXISTS api_keys (
    id           INTEGER  PRIMARY KEY AUTOINCREMENT,
    username     TEXT     NOT NULL,
    name         TEXT     NOT NULL,
    token_hash   TEXT     NOT NULL UNIQUE,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_api_keys_username ON api_keys (username);

-- 0004/0005: Invite / self-registration tokens with optional expiry.
CREATE TABLE IF NOT EXISTS registration_tokens (
    token      TEXT     NOT NULL PRIMARY KEY,
    created_by TEXT     NOT NULL,
    used_at    DATETIME,
    -- 0005: optional expiry (e.g. 30-minute startup token)
    expires_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 0005: Runtime admin settings (key-value).
-- registration_enabled defaults to true; admin can disable via the UI.
CREATE TABLE IF NOT EXISTS app_settings (
    key   TEXT NOT NULL PRIMARY KEY,
    value TEXT NOT NULL
);
INSERT OR IGNORE INTO app_settings (key, value) VALUES ('registration_enabled', 'true');

-- 0006: Maps every status-page summary UUID to its owning user so the
-- unauthenticated /summary/:uuid endpoint can locate the correct per-user DB.
CREATE TABLE IF NOT EXISTS summary_tokens (
    uuid     TEXT NOT NULL PRIMARY KEY,
    username TEXT NOT NULL
);

-- 0007: Adds account-level disable flag and password-reset tokens.
-- Allow admins to temporarily lock an account.
ALTER TABLE users ADD COLUMN disabled INTEGER NOT NULL DEFAULT 0;

-- Short-lived tokens used by the admin-generated password-reset flow.
-- Each token is single-use and expires 30 minutes after creation.
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    token      TEXT     NOT NULL PRIMARY KEY,
    username   TEXT     NOT NULL,
    expires_at DATETIME NOT NULL,
    used_at    DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
