# Gateway API

Hạ tầng backend cho đồ án xây dựng API Gateway.

## Thiết lập hiện tại

Repository hiện có phần hạ tầng database local để phát triển:

- PostgreSQL 16: lưu cấu hình/control-plane
- Redis 7: cache, bộ đếm rate limit, trạng thái health check, token blacklist
- pgAdmin: giao diện quản lý PostgreSQL

## Yêu cầu

- Docker Desktop
- Docker Compose
- Go
- goose migration CLI

## Thiết lập môi trường

Tạo file `.env` local từ file mẫu:

```powershell
cd "D:\PROJECT (GRADUATION)\gateway-api"
Copy-Item .env.example .env
```

Nếu port, user hoặc mật khẩu local khác file mẫu, hãy chỉnh lại trong `.env`.

## Chạy database services

Từ thư mục `database`, chạy Docker Compose với file `.env` ở root project:

```powershell
cd "D:\PROJECT (GRADUATION)\gateway-api\database"
docker compose --env-file ..\.env -f docker-compose.yml up -d
```

Kiểm tra container đang chạy:

```powershell
docker compose --env-file ..\.env -f docker-compose.yml ps
```

## Cài goose

Cài goose CLI một lần:

```powershell
go install github.com/pressly/goose/v3/cmd/goose@latest
```

Kiểm tra goose đã dùng được chưa:

```powershell
goose -version
```

Nếu PowerShell báo không tìm thấy `goose`, hãy thêm thư mục Go binary vào `PATH`. Thường là:

```text
C:\Users\<your-user>\go\bin
```

## Chạy migrations

Schema được quản lý bằng các file goose migration trong:

```text
database/migrations
```

Từ root project, chạy:

```powershell
cd "D:\PROJECT (GRADUATION)\gateway-api"

goose -dir database/migrations postgres "postgres://gateway_user:gateway_password@localhost:5433/gateway_db?sslmode=disable" up
```

Kiểm tra trạng thái migration:

```powershell
goose -dir database/migrations postgres "postgres://gateway_user:gateway_password@localhost:5433/gateway_db?sslmode=disable" status
```

Kiểm tra version migration hiện tại:

```powershell
goose -dir database/migrations postgres "postgres://gateway_user:gateway_password@localhost:5433/gateway_db?sslmode=disable" version
```

Rollback migration mới nhất:

```powershell
goose -dir database/migrations postgres "postgres://gateway_user:gateway_password@localhost:5433/gateway_db?sslmode=disable" down
```

Nếu bạn đổi `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` hoặc port PostgreSQL trong `.env`, hãy sửa lại connection string trước khi chạy goose.

## Thứ tự migrations

Các migration hiện được tách theo trách nhiệm và sắp xếp theo thứ tự phụ thuộc khóa ngoại:

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

Lưu ý trong giai đoạn phát triển: nếu bạn đã từng chạy file migration cũ gộp một file, sau đó mới tách thành nhiều file nhỏ, hãy reset volume database local trước khi chạy bộ migration mới.

```powershell
cd "D:\PROJECT (GRADUATION)\gateway-api\database"
docker compose --env-file ..\.env -f docker-compose.yml down -v
docker compose --env-file ..\.env -f docker-compose.yml up -d
```

Sau đó quay lại root project và chạy lại `goose up`.

## Chạy seed data

Seed files nằm trong:

```text
database/seeds
```

Seed đã được tách thành nhiều file nhỏ và nên chạy theo thứ tự tên file:

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

Chạy toàn bộ seed từ root project:

```powershell
cd "D:\PROJECT (GRADUATION)\gateway-api"

Get-ChildItem .\database\seeds\*.sql | Sort-Object Name | ForEach-Object {
    docker cp $_.FullName "gateway-postgres:/tmp/$($_.Name)"
    docker exec gateway-postgres psql -U gateway_user -d gateway_db -f "/tmp/$($_.Name)"
}
```

Chạy thủ công một file seed:

```powershell
docker cp ".\database\seeds\001_seed_roles_permissions.sql" gateway-postgres:/tmp/001_seed_roles_permissions.sql
docker exec -it gateway-postgres psql -U gateway_user -d gateway_db -f /tmp/001_seed_roles_permissions.sql
```

Các file seed dùng `ON CONFLICT DO NOTHING`, nên có thể chạy lại nhiều lần mà không bị lỗi trùng dữ liệu.

`password_hash` của user demo và `key_hash` của API key demo hiện là placeholder. Khi làm backend auth thật, hãy thay bằng bcrypt/argon2 password hash và SHA-256 API key hash thật.

## Kiểm tra dữ liệu database

Mở `psql` bên trong PostgreSQL container:

```powershell
docker exec -it gateway-postgres psql -U gateway_user -d gateway_db
```

Một số câu query kiểm tra nhanh:

```sql
SELECT * FROM goose_db_version ORDER BY id;
SELECT name, description FROM roles;
SELECT username, email, is_active FROM users;
SELECT name, protocol, lb_strategy FROM services;
SELECT path, method, auth_required FROM routes;
SELECT name, limit_type, max_requests, window_seconds FROM rate_limit_policies;
```

Thoát `psql`:

```sql
\q
```

## Tạo Elasticsearch index template bằng Postman

Elasticsearch dùng index template để định nghĩa kiểu dữ liệu cho request logs. File template được lưu tại:

```text
database/elasticsearch/index-templates/gateway-logs-template.json
```

Trước khi tạo template, kiểm tra Elasticsearch đã chạy:

```text
GET http://localhost:9200
```

Trong Postman, tạo request:

```text
Method: PUT
URL: http://localhost:9200/_index_template/gateway-logs-template
```

Tab `Headers`:

```text
Content-Type: application/json
```

Tab `Body`:

```text
raw -> JSON
```

Copy toàn bộ nội dung file `database/elasticsearch/index-templates/gateway-logs-template.json` vào body, rồi bấm `Send`.

Nếu thành công, Elasticsearch sẽ trả:

```json
{
  "acknowledged": true
}
```

Kiểm tra template đã được tạo:

```text
GET http://localhost:9200/_index_template/gateway-logs-template?pretty
```

## Tạo log mẫu trong Elasticsearch bằng Postman

Sau khi đã tạo index template, tạo request mới trong Postman:

```text
Method: POST
URL: http://localhost:9200/gateway-logs-2026.06.16/_doc
```

Tab `Headers`:

```text
Content-Type: application/json
```

Tab `Body` chọn `raw -> JSON`, rồi dùng dữ liệu mẫu:

```json
{
  "@timestamp": "2026-06-16T08:15:42.317Z",
  "trace_id": "demo-trace-001",
  "route_id": "80000000-0000-0000-0000-000000000001",
  "service_name": "product-service",
  "method": "GET",
  "path": "/api/products",
  "client_ip": "127.0.0.1",
  "status_code": 200,
  "response_time_ms": 38.5,
  "upstream_latency_ms": 31.2,
  "request_size": 512,
  "response_size": 1843
}
```

Nếu thành công, Elasticsearch sẽ trả `result` là `created`.

Search log vừa tạo:

```text
GET http://localhost:9200/gateway-logs-*/_search?pretty
```

Search theo service:

```text
Method: GET
URL: http://localhost:9200/gateway-logs-*/_search?pretty
```

Body `raw -> JSON`:

```json
{
  "query": {
    "term": {
      "service_name": "product-service"
    }
  }
}
```

## Xem log bằng Kibana

Mở Kibana:

```text
http://localhost:5601
```

Tạo Data View:

```text
Stack Management -> Data Views -> Create data view
```

Nhập:

```text
Name: Gateway Logs
Index pattern: gateway-logs-*
Timestamp field: @timestamp
```

Sau đó vào `Discover`, chọn data view `Gateway Logs` để xem log.

Nếu không thấy log, kiểm tra time range ở góc trên bên phải. Với log mẫu trong README, timestamp là ngày `2026-06-16`, nên cần chọn khoảng thời gian có chứa ngày này.

## Truy cập pgAdmin

Mở pgAdmin trong trình duyệt:

```text
http://localhost:5050
```

Thông tin đăng nhập mặc định lấy từ `.env`:

```text
Email: PGADMIN_DEFAULT_EMAIL
Password: PGADMIN_DEFAULT_PASSWORD
```

Đăng ký PostgreSQL server trong pgAdmin:

```text
Host: postgres
Port: 5432
Database: POSTGRES_DB
Username: POSTGRES_USER
Password: POSTGRES_PASSWORD
```

Với giá trị mặc định trong `.env.example`:

```text
Database: gateway_db
Username: gateway_user
Password: gateway_password
```

## Dừng services

```powershell
cd "D:\PROJECT (GRADUATION)\gateway-api\database"
docker compose --env-file ..\.env -f docker-compose.yml down
```

Dừng services và xóa luôn volume/data local:

```powershell
docker compose --env-file ..\.env -f docker-compose.yml down -v
```
