-- 0003_status_page_slugs.up.sql (users DB)
-- Maps every status-page slug to its owning user so the unauthenticated
-- /status/:slug endpoint can locate the correct per-user DB without a UUID.
-- Slugs are unique across all users (enforced by the PRIMARY KEY).
CREATE TABLE IF NOT EXISTS status_page_slugs (
    slug     TEXT NOT NULL PRIMARY KEY,
    username TEXT NOT NULL
);
