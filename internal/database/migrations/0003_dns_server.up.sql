-- 0003_dns_server.up.sql
-- Adds an optional custom DNS server address (host:port) for each monitor.
ALTER TABLE monitors ADD COLUMN dns_server TEXT NOT NULL DEFAULT '';
