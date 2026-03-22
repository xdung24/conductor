-- 0002_user_features.up.sql (users DB)
-- Adds account-level disable flag and password-reset tokens.

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
