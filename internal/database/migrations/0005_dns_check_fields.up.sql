-- 0005_dns_check_fields.up.sql
-- Adds DNS-monitor-specific fields: record type to query and expected answer value.
ALTER TABLE monitors ADD COLUMN dns_record_type TEXT NOT NULL DEFAULT 'A';
ALTER TABLE monitors ADD COLUMN dns_expected    TEXT NOT NULL DEFAULT '';
