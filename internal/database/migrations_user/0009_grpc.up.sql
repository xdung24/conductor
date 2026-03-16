-- gRPC keyword monitor support: protobuf definition and call parameters.
ALTER TABLE monitors ADD COLUMN grpc_protobuf     TEXT    NOT NULL DEFAULT '';
ALTER TABLE monitors ADD COLUMN grpc_service_name TEXT    NOT NULL DEFAULT '';
ALTER TABLE monitors ADD COLUMN grpc_method       TEXT    NOT NULL DEFAULT '';
ALTER TABLE monitors ADD COLUMN grpc_body         TEXT    NOT NULL DEFAULT '';
ALTER TABLE monitors ADD COLUMN grpc_enable_tls   INTEGER NOT NULL DEFAULT 0;
