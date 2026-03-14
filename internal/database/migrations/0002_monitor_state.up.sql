-- 0002_monitor_state.up.sql
-- Track the last known status per monitor to detect state changes
-- (avoid sending duplicate DOWN/UP notifications)
ALTER TABLE monitors ADD COLUMN last_status INTEGER;
ALTER TABLE monitors ADD COLUMN last_notified_status INTEGER;
