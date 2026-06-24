-- +goose Up
CREATE TABLE roles (
    id          UUID CONSTRAINT roles_pkey PRIMARY KEY,
    name        VARCHAR(50) NOT NULL CONSTRAINT roles_name_unique UNIQUE,
    description TEXT
);

CREATE TABLE permissions (
    id          UUID CONSTRAINT permissions_pkey PRIMARY KEY,
    resource    VARCHAR(100) NOT NULL,
    action      VARCHAR(50)  NOT NULL,
    CONSTRAINT permissions_resource_action_unique UNIQUE (resource, action)
);

CREATE TABLE role_permissions (
    role_id       UUID NOT NULL CONSTRAINT role_permissions_role_id_fkey REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL CONSTRAINT role_permissions_permission_id_fkey REFERENCES permissions(id) ON DELETE CASCADE,
    CONSTRAINT role_permissions_pkey PRIMARY KEY (role_id, permission_id)
);

-- +goose Down
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
