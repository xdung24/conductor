-- 0001_init.down.sql (users DB)
-- Drops all tables and indexes created by the consolidated 0001_init.up.sql.

DROP TABLE IF EXISTS summary_tokens;
DROP TABLE IF EXISTS app_settings;
DROP TABLE IF EXISTS registration_tokens;
DROP INDEX IF EXISTS idx_api_keys_username;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS push_tokens;
DROP TABLE IF EXISTS users;
