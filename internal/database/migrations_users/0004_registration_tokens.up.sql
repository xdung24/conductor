CREATE TABLE IF NOT EXISTS registration_tokens (
    token      TEXT     NOT NULL PRIMARY KEY,
    created_by TEXT     NOT NULL,
    used_at    DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
