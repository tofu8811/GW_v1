-- +goose Up
CREATE TABLE rate_limit_policies (
    id              UUID PRIMARY KEY,
    name            VARCHAR(100) NOT NULL UNIQUE,
    limit_type      VARCHAR(10) NOT NULL
                        CHECK (limit_type IN ('ip', 'user', 'api_key')),
    max_requests    INTEGER NOT NULL CHECK (max_requests > 0),
    window_seconds  INTEGER NOT NULL CHECK (window_seconds > 0),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS rate_limit_policies;
