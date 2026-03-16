-- Docker host registry and container monitor support.
CREATE TABLE IF NOT EXISTS docker_hosts (
    id          INTEGER  PRIMARY KEY AUTOINCREMENT,
    name        TEXT     NOT NULL,
    socket_path TEXT     NOT NULL DEFAULT '',
    http_url    TEXT     NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE monitors ADD COLUMN docker_host_id      INTEGER NOT NULL DEFAULT 0;
ALTER TABLE monitors ADD COLUMN docker_container_id TEXT    NOT NULL DEFAULT '';
