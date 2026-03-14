-- 0004_notification_logs.up.sql
-- Records every notification delivery attempt for auditing / history view.
CREATE TABLE IF NOT EXISTS notification_logs (
    id                INTEGER  PRIMARY KEY AUTOINCREMENT,
    monitor_id        INTEGER  REFERENCES monitors(id)      ON DELETE SET NULL,
    notification_id   INTEGER  REFERENCES notifications(id) ON DELETE SET NULL,
    monitor_name      TEXT     NOT NULL DEFAULT '',
    notification_name TEXT     NOT NULL DEFAULT '',
    event_status      INTEGER  NOT NULL DEFAULT 0,  -- 0=down, 1=up at time of send
    success           INTEGER  NOT NULL DEFAULT 1,  -- 1=delivered, 0=failed
    error             TEXT     NOT NULL DEFAULT '',
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_notification_logs_created_at    ON notification_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_notification_logs_monitor_id    ON notification_logs(monitor_id);
CREATE INDEX IF NOT EXISTS idx_notification_logs_notification_id ON notification_logs(notification_id);
