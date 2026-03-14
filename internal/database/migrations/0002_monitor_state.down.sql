-- 0002_monitor_state.down.sql
-- SQLite does not support DROP COLUMN before 3.35.0; recreate table instead.
CREATE TABLE monitors_backup AS SELECT
    id, name, type, url, interval_seconds, timeout_seconds, active, retries, created_at, updated_at
FROM monitors;
DROP TABLE monitors;
ALTER TABLE monitors_backup RENAME TO monitors;
