# Gateway API

Backend infrastructure for the API Gateway graduation project.

## Current setup

This repository currently includes the local database stack for development:

- PostgreSQL 16: configuration/control-plane database
- Redis 7: cache, rate limit counters, health state, token blacklist
- pgAdmin: PostgreSQL management UI

## Requirements

- Docker Desktop
- Docker Compose
- Go
- goose migration CLI

## Environment setup

Create a local `.env` file from the example file:

```powershell
cd "D:\PROJECT (GRADUATION)\gateway-api"
Copy-Item .env.example .env
```

Update values in `.env` if your local ports or passwords are different.

## Start database services

From the `database` directory, run Docker Compose with the root `.env` file:

```powershell
cd "D:\PROJECT (GRADUATION)\gateway-api\database"
docker compose --env-file ..\.env -f docker-compose.yml up -d
```

Check running containers:

```powershell
docker compose --env-file ..\.env -f docker-compose.yml ps
```

## Install goose

Install the goose CLI once:

```powershell
go install github.com/pressly/goose/v3/cmd/goose@latest
```

Check that goose is available:

```powershell
goose -version
```

If PowerShell cannot find `goose`, add your Go binary directory to `PATH`. It is usually:

```text
C:\Users\<your-user>\go\bin
```

## Run migrations

The schema is managed by goose migration files in:

```text
database/migrations
```

From the project root, run:

```powershell
cd "D:\PROJECT (GRADUATION)\gateway-api"

goose -dir database/migrations postgres "postgres://gateway_user:gateway_password@localhost:5433/gateway_db?sslmode=disable" up
```

Check migration status:

```powershell
goose -dir database/migrations postgres "postgres://gateway_user:gateway_password@localhost:5433/gateway_db?sslmode=disable" status
```

Check current migration version:

```powershell
goose -dir database/migrations postgres "postgres://gateway_user:gateway_password@localhost:5433/gateway_db?sslmode=disable" version
```

Rollback the latest migration:

```powershell
goose -dir database/migrations postgres "postgres://gateway_user:gateway_password@localhost:54323/gateway_db?sslmode=disable" down
```

If you changed `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`, or the PostgreSQL port in `.env`, update the connection string before running goose.

## Migration order

Current migration files are split by responsibility and ordered by dependency:

```text
20260616071136_init_extensions.sql
20260616071137_create_services.sql
20260616071138_create_service_instances.sql
20260616071139_create_rate_limit_policies.sql
20260616071140_create_routes.sql
20260616071141_create_cors_configs.sql
20260616071142_create_roles_permissions.sql
20260616071143_create_users.sql
20260616071144_create_api_keys.sql
20260616071145_create_ip_blacklist.sql
20260616071146_create_aggregation.sql
20260616071147_create_audit_logs.sql
```

Important note during development: if you already applied the old single migration file and then split it into multiple files, reset the local database volume before running the new migration set.

```powershell
cd "D:\PROJECT (GRADUATION)\gateway-api\database"
docker compose --env-file ..\.env -f docker-compose.yml down -v
docker compose --env-file ..\.env -f docker-compose.yml up -d
```

Then run `goose up` again from the project root.

## Run seed data

Seed files are stored in:

```text
database/seeds
```

They are split into small files and should be run in filename order:

```text
001_seed_roles_permissions.sql
002_seed_users.sql
003_seed_rate_limit_policies.sql
004_seed_services.sql
005_seed_service_instances.sql
006_seed_routes_cors.sql
007_seed_api_keys.sql
008_seed_aggregation.sql
```

Run all seed files from the project root:

```powershell
cd "D:\PROJECT (GRADUATION)\gateway-api"

Get-ChildItem .\database\seeds\*.sql | Sort-Object Name | ForEach-Object {
    docker cp $_.FullName "gateway-postgres:/tmp/$($_.Name)"
    docker exec gateway-postgres psql -U gateway_user -d gateway_db -f "/tmp/$($_.Name)"
}
```

Run one seed file manually:

```powershell
docker cp ".\database\seeds\001_seed_roles_permissions.sql" gateway-postgres:/tmp/001_seed_roles_permissions.sql
docker exec -it gateway-postgres psql -U gateway_user -d gateway_db -f /tmp/001_seed_roles_permissions.sql
```

Seed files use `ON CONFLICT DO NOTHING`, so they are safe to run multiple times.

The demo user password hashes and demo API key hash are placeholders. Replace them later with real bcrypt/argon2 password hashes and SHA-256 API key hashes from the backend.

## Verify database data

Open `psql` inside the PostgreSQL container:

```powershell
docker exec -it gateway-postgres psql -U gateway_user -d gateway_db
```

Useful checks:

```sql
SELECT * FROM goose_db_version ORDER BY id;
SELECT name, description FROM roles;
SELECT username, email, is_active FROM users;
SELECT name, protocol, lb_strategy FROM services;
SELECT path, method, auth_required FROM routes;
SELECT name, limit_type, max_requests, window_seconds FROM rate_limit_policies;
```

Exit `psql`:

```sql
\q
```

## Access pgAdmin

Open pgAdmin in the browser:

```text
http://localhost:5050
```

Default login comes from `.env`:

```text
Email: PGADMIN_DEFAULT_EMAIL
Password: PGADMIN_DEFAULT_PASSWORD
```

Register the PostgreSQL server in pgAdmin:

```text
Host: postgres
Port: 5432
Database: POSTGRES_DB
Username: POSTGRES_USER
Password: POSTGRES_PASSWORD
```

With the default `.env.example` values:

```text
Database: gateway_db
Username: gateway_user
Password: gateway_password
```

## Stop services

```powershell
cd "D:\PROJECT (GRADUATION)\gateway-api\database"
docker compose --env-file ..\.env -f docker-compose.yml down
```

To stop and delete local volumes/data:

```powershell
docker compose --env-file ..\.env -f docker-compose.yml down -v
```
