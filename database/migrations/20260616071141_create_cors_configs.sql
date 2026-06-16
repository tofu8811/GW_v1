-- +goose Up
CREATE TABLE cors_configs (
    id                UUID PRIMARY KEY,
    route_id          UUID NOT NULL UNIQUE REFERENCES routes(id) ON DELETE CASCADE,
    allowed_origins   TEXT[] NOT NULL DEFAULT '{}',
    allowed_methods   TEXT[] NOT NULL DEFAULT '{}',
    allowed_headers   TEXT[] NOT NULL DEFAULT '{}',
    allow_credentials BOOLEAN NOT NULL DEFAULT FALSE,
    max_age           INTEGER NOT NULL DEFAULT 3600
);

-- +goose Down
DROP TABLE IF EXISTS cors_configs;
