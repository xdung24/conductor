ALTER TABLE users ADD COLUMN is_admin INTEGER NOT NULL DEFAULT 0;

-- Promote the earliest-registered user to admin.
UPDATE users SET is_admin = 1 WHERE id = (SELECT MIN(id) FROM users);
