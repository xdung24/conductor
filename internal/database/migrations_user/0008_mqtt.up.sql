-- MQTT monitor support: broker credentials and topic subscription.
ALTER TABLE monitors ADD COLUMN mqtt_topic    TEXT NOT NULL DEFAULT '';
ALTER TABLE monitors ADD COLUMN mqtt_username TEXT NOT NULL DEFAULT '';
ALTER TABLE monitors ADD COLUMN mqtt_password TEXT NOT NULL DEFAULT '';
