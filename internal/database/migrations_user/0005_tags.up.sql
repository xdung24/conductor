-- Color-coded monitor labels / tags.
CREATE TABLE IF NOT EXISTS tags (
    id    INTEGER PRIMARY KEY AUTOINCREMENT,
    name  TEXT    NOT NULL UNIQUE,
    color TEXT    NOT NULL DEFAULT '#6366f1'
);

-- Many-to-many: monitors ↔ tags.
CREATE TABLE IF NOT EXISTS monitor_tags (
    monitor_id INTEGER NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    tag_id     INTEGER NOT NULL REFERENCES tags(id)     ON DELETE CASCADE,
    PRIMARY KEY (monitor_id, tag_id)
);
