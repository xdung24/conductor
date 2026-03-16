-- SNMP monitor fields
ALTER TABLE monitors ADD COLUMN snmp_community TEXT    NOT NULL DEFAULT 'public';
ALTER TABLE monitors ADD COLUMN snmp_oid       TEXT    NOT NULL DEFAULT '';
ALTER TABLE monitors ADD COLUMN snmp_version   TEXT    NOT NULL DEFAULT '2c';
ALTER TABLE monitors ADD COLUMN snmp_expected  TEXT    NOT NULL DEFAULT '';

-- System service monitor field (service name for systemd / Windows SCM)
ALTER TABLE monitors ADD COLUMN service_name   TEXT    NOT NULL DEFAULT '';

-- Manual monitor: stored UP/DOWN state (1=UP, 0=DOWN)
ALTER TABLE monitors ADD COLUMN manual_status  INTEGER NOT NULL DEFAULT 1;

-- Group monitor: parent_id links child monitors to their group (0 = top-level)
ALTER TABLE monitors ADD COLUMN parent_id      INTEGER NOT NULL DEFAULT 0;
