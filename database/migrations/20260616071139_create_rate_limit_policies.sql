-- +goose Up
CREATE TABLE rate_limit_policies (
    id              UUID CONSTRAINT rate_limit_policies_pkey PRIMARY KEY,
    name            VARCHAR(100) NOT NULL CONSTRAINT rate_limit_policies_name_unique UNIQUE,
    limit_type      VARCHAR(10) NOT NULL
                        CONSTRAINT rate_limit_policies_limit_type_check CHECK (limit_type IN ('ip', 'user', 'api_key')),
    max_requests    INTEGER NOT NULL CONSTRAINT rate_limit_policies_max_requests_check CHECK (max_requests > 0),
    window_seconds  INTEGER NOT NULL CONSTRAINT rate_limit_policies_window_seconds_check CHECK (window_seconds > 0),
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS rate_limit_policies;
