-- +goose Up
CREATE TABLE cors_configs (
    id                UUID CONSTRAINT cors_configs_pkey PRIMARY KEY,
    route_id          UUID NOT NULL,
    allowed_origins   TEXT[] NOT NULL DEFAULT '{}',
    allowed_methods   TEXT[] NOT NULL DEFAULT '{}',
    allowed_headers   TEXT[] NOT NULL DEFAULT '{}',
    allow_credentials BOOLEAN NOT NULL DEFAULT FALSE,
    max_age           INTEGER NOT NULL DEFAULT 3600,
    CONSTRAINT cors_configs_route_id_unique UNIQUE (route_id),
    CONSTRAINT cors_configs_route_id_fkey FOREIGN KEY (route_id) REFERENCES routes(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE IF EXISTS cors_configs;
