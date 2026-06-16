-- +goose Up
CREATE TABLE audit_logs (
    id          BIGSERIAL PRIMARY KEY,
    user_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    action      VARCHAR(20) NOT NULL CHECK (action IN ('create', 'update', 'delete')),
    entity_type VARCHAR(50) NOT NULL,
    entity_id   UUID,
    old_value   JSONB,
    new_value   JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_entity
ON audit_logs(entity_type, entity_id);

CREATE INDEX idx_audit_time
ON audit_logs(created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_audit_time;
DROP INDEX IF EXISTS idx_audit_entity;
DROP TABLE IF EXISTS audit_logs;
