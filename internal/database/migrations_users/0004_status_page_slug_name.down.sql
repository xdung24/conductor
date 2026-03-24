-- SQLite does not support DROP COLUMN on older versions; recreate the table without the name column.
CREATE TABLE status_page_slugs_old (slug TEXT PRIMARY KEY, username TEXT NOT NULL);
INSERT INTO status_page_slugs_old (slug, username) SELECT slug, username FROM status_page_slugs;
DROP TABLE status_page_slugs;
ALTER TABLE status_page_slugs_old RENAME TO status_page_slugs;
