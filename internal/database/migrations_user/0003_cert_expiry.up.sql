-- Add TLS certificate expiry alert threshold (in days) to monitors.
-- When > 0 and the HTTP monitor uses TLS, the check will return DOWN
-- if the leaf certificate expires within this many days.
ALTER TABLE monitors ADD COLUMN cert_expiry_alert_days INTEGER NOT NULL DEFAULT 0;
