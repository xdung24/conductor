-- SQLite does not support DROP COLUMN before 3.35.0; recreate the table.
CREATE TABLE users_backup AS SELECT id, username, password, created_at FROM users;
DROP TABLE users;
ALTER TABLE users_backup RENAME TO users;
