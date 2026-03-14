-- 0003_dns_server.down.sql
-- SQLite does not support DROP COLUMN before v3.35; use a table rebuild.
CREATE TABLE monitors_new AS SELECT
    id, name, type, url, interval_seconds, timeout_seconds, active, retries,
    created_at, updated_at
FROM monitors;
DROP TABLE monitors;
ALTER TABLE monitors_new RENAME TO monitors;
