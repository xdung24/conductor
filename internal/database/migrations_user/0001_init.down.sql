-- 0001_init.down.sql (per-user data DB)
-- Drops all tables and indexes created by the consolidated 0001_init.up.sql.

DROP INDEX IF EXISTS idx_heartbeats_monitor_created;
DROP INDEX IF EXISTS idx_downtime_monitor_started;
DROP TABLE IF EXISTS downtime_events;
DROP TABLE IF EXISTS proxies;
DROP TABLE IF EXISTS monitor_maintenance;
DROP TABLE IF EXISTS maintenance_windows;
DROP TABLE IF EXISTS monitor_tags;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS status_page_monitors;
DROP TABLE IF EXISTS status_pages;
DROP TABLE IF EXISTS docker_hosts;
DROP INDEX IF EXISTS idx_notification_logs_notification_id;
DROP INDEX IF EXISTS idx_notification_logs_monitor_id;
DROP INDEX IF EXISTS idx_notification_logs_created_at;
DROP TABLE IF EXISTS notification_logs;
DROP TABLE IF EXISTS monitor_notifications;
DROP TABLE IF EXISTS notifications;
DROP INDEX IF EXISTS idx_heartbeats_created_at;
DROP INDEX IF EXISTS idx_heartbeats_monitor_id;
DROP TABLE IF EXISTS heartbeats;
DROP INDEX IF EXISTS idx_monitors_push_token;
DROP TABLE IF EXISTS monitors;
