-- 0002_api_keys.up.sql
-- Per-user API keys for token-based access.
-- token_hash stores SHA-256(plain_token) so looking up by hash is O(1).

CREATE TABLE IF NOT EXISTS api_keys (
    id           INTEGER  PRIMARY KEY AUTOINCREMENT,
    username     TEXT     NOT NULL,
    name         TEXT     NOT NULL,
    token_hash   TEXT     NOT NULL UNIQUE,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_api_keys_username ON api_keys (username);
