-- SQLite does not support DROP COLUMN without table recreation.
-- Rolling back this migration leaves the column in place (value = 0 = disabled).
SELECT 1;
