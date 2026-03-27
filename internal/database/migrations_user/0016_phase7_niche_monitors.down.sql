-- 0016_phase7_niche_monitors.down.sql

DROP TABLE IF EXISTS remote_browsers;
ALTER TABLE monitors DROP COLUMN remote_browser_id;
ALTER TABLE monitors DROP COLUMN gamedig_given_port_only;
ALTER TABLE monitors DROP COLUMN gamedig_game;
