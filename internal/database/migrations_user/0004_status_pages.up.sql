-- Public Status Pages
CREATE TABLE IF NOT EXISTS status_pages (
    id          INTEGER  PRIMARY KEY AUTOINCREMENT,
    name        TEXT     NOT NULL,
    slug        TEXT     NOT NULL UNIQUE,
    description TEXT     NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL
);

-- Join table: which monitors are shown on which status page, and in what order.
CREATE TABLE IF NOT EXISTS status_page_monitors (
    page_id    INTEGER NOT NULL REFERENCES status_pages(id) ON DELETE CASCADE,
    monitor_id INTEGER NOT NULL REFERENCES monitors(id)     ON DELETE CASCADE,
    position   INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (page_id, monitor_id)
);
