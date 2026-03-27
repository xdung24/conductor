-- 0016_phase7_niche_monitors.up.sql
-- Phase 7: GameDig/Steam/Browser monitor support and remote browser configs.

ALTER TABLE monitors ADD COLUMN gamedig_game TEXT NOT NULL DEFAULT '';
ALTER TABLE monitors ADD COLUMN gamedig_given_port_only INTEGER NOT NULL DEFAULT 0;
ALTER TABLE monitors ADD COLUMN remote_browser_id INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS remote_browsers (
    id           INTEGER  PRIMARY KEY AUTOINCREMENT,
    name         TEXT     NOT NULL,
    endpoint_url TEXT     NOT NULL,
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);
