-- 0005_dns_check_fields.down.sql
-- Rebuild monitors table without dns_record_type and dns_expected columns.
CREATE TABLE monitors_new AS SELECT
    id, name, type, url, interval_seconds, timeout_seconds, active, retries,
    dns_server, last_status, last_notified_status, created_at, updated_at
FROM monitors;
DROP TABLE monitors;
ALTER TABLE monitors_new RENAME TO monitors;
