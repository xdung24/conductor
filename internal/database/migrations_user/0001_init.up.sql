-- 0001_init.up.sql (per-user data DB)
-- Consolidated schema for all migrations (0001–0015).

CREATE TABLE IF NOT EXISTS monitors (
    id                       INTEGER  PRIMARY KEY AUTOINCREMENT,
    name                     TEXT     NOT NULL,
    type                     TEXT     NOT NULL DEFAULT 'http',
    url                      TEXT     NOT NULL DEFAULT '',
    interval_seconds         INTEGER  NOT NULL DEFAULT 60,
    timeout_seconds          INTEGER  NOT NULL DEFAULT 30,
    active                   INTEGER  NOT NULL DEFAULT 1,
    retries                  INTEGER  NOT NULL DEFAULT 1,
    dns_server               TEXT     NOT NULL DEFAULT '',
    dns_record_type          TEXT     NOT NULL DEFAULT 'A',
    dns_expected             TEXT     NOT NULL DEFAULT '',
    http_accepted_statuses   TEXT     NOT NULL DEFAULT '',
    http_ignore_tls          INTEGER  NOT NULL DEFAULT 0,
    http_method              TEXT     NOT NULL DEFAULT 'GET',
    http_keyword             TEXT     NOT NULL DEFAULT '',
    http_keyword_invert      INTEGER  NOT NULL DEFAULT 0,
    http_username            TEXT     NOT NULL DEFAULT '',
    http_password            TEXT     NOT NULL DEFAULT '',
    http_bearer_token        TEXT     NOT NULL DEFAULT '',
    http_max_redirects       INTEGER  NOT NULL DEFAULT 10,
    push_token               TEXT     NOT NULL DEFAULT '',
    http_header_name         TEXT     NOT NULL DEFAULT '',
    http_header_value        TEXT     NOT NULL DEFAULT '',
    http_body_type           TEXT     NOT NULL DEFAULT '',
    http_json_path           TEXT     NOT NULL DEFAULT '',
    http_json_expected       TEXT     NOT NULL DEFAULT '',
    http_xpath               TEXT     NOT NULL DEFAULT '',
    http_xpath_expected      TEXT     NOT NULL DEFAULT '',
    smtp_use_tls             INTEGER  NOT NULL DEFAULT 0,
    smtp_ignore_tls          INTEGER  NOT NULL DEFAULT 0,
    smtp_username            TEXT     NOT NULL DEFAULT '',
    smtp_password            TEXT     NOT NULL DEFAULT '',
    notify_on_failure        INTEGER  NOT NULL DEFAULT 1,
    notify_on_success        INTEGER  NOT NULL DEFAULT 1,
    notify_body_chars        INTEGER  NOT NULL DEFAULT 0,
    http_request_headers     TEXT     NOT NULL DEFAULT '',
    http_request_body        TEXT     NOT NULL DEFAULT '',
    last_status              INTEGER,
    last_notified_status     INTEGER,
    created_at               DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at               DATETIME DEFAULT CURRENT_TIMESTAMP,
    -- 0002: database monitor
    db_query                 TEXT     NOT NULL DEFAULT '',
    -- 0003: TLS certificate expiry alert
    cert_expiry_alert_days   INTEGER  NOT NULL DEFAULT 0,
    -- 0007: docker container monitor
    docker_host_id           INTEGER  NOT NULL DEFAULT 0,
    docker_container_id      TEXT     NOT NULL DEFAULT '',
    -- 0008: MQTT monitor
    mqtt_topic               TEXT     NOT NULL DEFAULT '',
    mqtt_username            TEXT     NOT NULL DEFAULT '',
    mqtt_password            TEXT     NOT NULL DEFAULT '',
    -- 0009: gRPC monitor
    grpc_protobuf            TEXT     NOT NULL DEFAULT '',
    grpc_service_name        TEXT     NOT NULL DEFAULT '',
    grpc_method              TEXT     NOT NULL DEFAULT '',
    grpc_body                TEXT     NOT NULL DEFAULT '',
    grpc_enable_tls          INTEGER  NOT NULL DEFAULT 0,
    -- 0010: SNMP, system service, manual, group monitors
    snmp_community           TEXT     NOT NULL DEFAULT 'public',
    snmp_oid                 TEXT     NOT NULL DEFAULT '',
    snmp_version             TEXT     NOT NULL DEFAULT '2c',
    snmp_expected            TEXT     NOT NULL DEFAULT '',
    service_name             TEXT     NOT NULL DEFAULT '',
    manual_status            INTEGER  NOT NULL DEFAULT 1,
    parent_id                INTEGER  NOT NULL DEFAULT 0,
    -- 0011: Kafka monitor
    kafka_topic              TEXT     NOT NULL DEFAULT '',
    -- 0013: RADIUS monitor
    radius_secret            TEXT     NOT NULL DEFAULT '',
    radius_called_station_id TEXT     NOT NULL DEFAULT '',
    -- 0014: proxy support
    proxy_id                 INTEGER  NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_monitors_push_token ON monitors(push_token) WHERE push_token != '';

CREATE TABLE IF NOT EXISTS heartbeats (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    monitor_id INTEGER  NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    status     INTEGER  NOT NULL DEFAULT 0,
    latency_ms INTEGER  NOT NULL DEFAULT 0,
    message    TEXT     NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_heartbeats_monitor_id ON heartbeats(monitor_id);
CREATE INDEX IF NOT EXISTS idx_heartbeats_created_at ON heartbeats(created_at);

CREATE TABLE IF NOT EXISTS notifications (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    name       TEXT     NOT NULL,
    type       TEXT     NOT NULL,
    config     TEXT     NOT NULL DEFAULT '{}',
    active     INTEGER  NOT NULL DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS monitor_notifications (
    monitor_id      INTEGER NOT NULL REFERENCES monitors(id)      ON DELETE CASCADE,
    notification_id INTEGER NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    PRIMARY KEY (monitor_id, notification_id)
);

CREATE TABLE IF NOT EXISTS notification_logs (
    id                INTEGER  PRIMARY KEY AUTOINCREMENT,
    monitor_id        INTEGER  REFERENCES monitors(id)      ON DELETE SET NULL,
    notification_id   INTEGER  REFERENCES notifications(id) ON DELETE SET NULL,
    monitor_name      TEXT     NOT NULL DEFAULT '',
    notification_name TEXT     NOT NULL DEFAULT '',
    event_status      INTEGER  NOT NULL DEFAULT 0,
    success           INTEGER  NOT NULL DEFAULT 1,
    error             TEXT     NOT NULL DEFAULT '',
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_notification_logs_created_at     ON notification_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_notification_logs_monitor_id     ON notification_logs(monitor_id);
CREATE INDEX IF NOT EXISTS idx_notification_logs_notification_id ON notification_logs(notification_id);

-- 0004: public status pages
CREATE TABLE IF NOT EXISTS status_pages (
    id           INTEGER  PRIMARY KEY AUTOINCREMENT,
    name         TEXT     NOT NULL,
    slug         TEXT     NOT NULL UNIQUE,
    description  TEXT     NOT NULL DEFAULT '',
    -- 0015: optional summary UUID for the public JSON API endpoint
    summary_uuid TEXT     NOT NULL DEFAULT '',
    created_at   DATETIME NOT NULL,
    updated_at   DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS status_page_monitors (
    page_id    INTEGER NOT NULL REFERENCES status_pages(id) ON DELETE CASCADE,
    monitor_id INTEGER NOT NULL REFERENCES monitors(id)     ON DELETE CASCADE,
    position   INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (page_id, monitor_id)
);

-- 0005: color-coded monitor tags
CREATE TABLE IF NOT EXISTS tags (
    id    INTEGER PRIMARY KEY AUTOINCREMENT,
    name  TEXT    NOT NULL UNIQUE,
    color TEXT    NOT NULL DEFAULT '#6366f1'
);

CREATE TABLE IF NOT EXISTS monitor_tags (
    monitor_id INTEGER NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    tag_id     INTEGER NOT NULL REFERENCES tags(id)     ON DELETE CASCADE,
    PRIMARY KEY (monitor_id, tag_id)
);

-- 0006: scheduled maintenance windows
CREATE TABLE IF NOT EXISTS maintenance_windows (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    name       TEXT     NOT NULL,
    start_time DATETIME NOT NULL,
    end_time   DATETIME NOT NULL,
    active     INTEGER  NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS monitor_maintenance (
    window_id  INTEGER NOT NULL REFERENCES maintenance_windows(id) ON DELETE CASCADE,
    monitor_id INTEGER NOT NULL REFERENCES monitors(id)            ON DELETE CASCADE,
    PRIMARY KEY (window_id, monitor_id)
);

-- 0007: docker host registry
CREATE TABLE IF NOT EXISTS docker_hosts (
    id          INTEGER  PRIMARY KEY AUTOINCREMENT,
    name        TEXT     NOT NULL,
    socket_path TEXT     NOT NULL DEFAULT '',
    http_url    TEXT     NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 0012: downtime tracking events
CREATE TABLE IF NOT EXISTS downtime_events (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    monitor_id INTEGER  NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    started_at DATETIME NOT NULL,
    ended_at   DATETIME,         -- NULL = incident still open
    duration_s INTEGER,          -- seconds, computed when closed
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_downtime_monitor_started
    ON downtime_events(monitor_id, started_at);

CREATE INDEX IF NOT EXISTS idx_heartbeats_monitor_created
    ON heartbeats(monitor_id, created_at DESC);

-- 0014: HTTP/SOCKS proxy registry
CREATE TABLE IF NOT EXISTS proxies (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    name       TEXT     NOT NULL,
    url        TEXT     NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- 0015: domain expiry monitor fields
ALTER TABLE monitors ADD COLUMN domain_expiry_alert_days INTEGER NOT NULL DEFAULT 30;
ALTER TABLE monitors ADD COLUMN doh_url TEXT NOT NULL DEFAULT '';
