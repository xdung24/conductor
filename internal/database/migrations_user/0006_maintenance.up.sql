-- Scheduled maintenance windows — suppress alerts during planned downtime.
CREATE TABLE IF NOT EXISTS maintenance_windows (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    name       TEXT     NOT NULL,
    start_time DATETIME NOT NULL,
    end_time   DATETIME NOT NULL,
    active     INTEGER  NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- Many-to-many: which monitors are covered by a maintenance window.
CREATE TABLE IF NOT EXISTS monitor_maintenance (
    window_id  INTEGER NOT NULL REFERENCES maintenance_windows(id) ON DELETE CASCADE,
    monitor_id INTEGER NOT NULL REFERENCES monitors(id)            ON DELETE CASCADE,
    PRIMARY KEY (window_id, monitor_id)
);
