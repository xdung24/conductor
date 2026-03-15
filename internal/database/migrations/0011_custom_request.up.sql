ALTER TABLE monitors ADD COLUMN http_request_headers TEXT NOT NULL DEFAULT '';
ALTER TABLE monitors ADD COLUMN http_request_body TEXT NOT NULL DEFAULT '';
