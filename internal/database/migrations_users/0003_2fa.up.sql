-- 0003_2fa.up.sql
-- Add TOTP two-factor authentication fields to the users table.

ALTER TABLE users ADD COLUMN totp_secret  TEXT;
ALTER TABLE users ADD COLUMN totp_enabled INTEGER NOT NULL DEFAULT 0;
