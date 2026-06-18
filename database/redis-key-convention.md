# Quy ước Redis Key - API Gateway

Redis dùng cho dữ liệu tạm thời, dữ liệu realtime và các dữ liệu nằm trong luồng xử lý nóng. PostgreSQL vẫn là nguồn sự thật cho dữ liệu cấu hình.

Nguyên tắc chính:

```text
PostgreSQL = luật và cấu hình
Redis      = bộ đếm, cache, trạng thái realtime, token tạm thời
```

Dùng dấu `:` để phân cấp namespace:

```text
<nhóm>:<loại>:<định_danh>
```

Mọi key tạm thời phải có TTL.

## 1. Cache cấu hình

Gateway nên đọc cấu hình route/service từ Redis hoặc in-memory cache trong lúc xử lý request. Không nên truy vấn PostgreSQL trong mỗi request.

### Cấu hình route

```text
cfg:route:{method}:{path}
```

Ví dụ:

```text
cfg:route:GET:/api/products
cfg:route:POST:/api/orders
```

Kiểu dữ liệu:

```text
String JSON
```

Ví dụ value:

```json
{
  "route_id": "80000000-0000-0000-0000-000000000001",
  "path": "/api/products",
  "method": "GET",
  "strip_prefix": true,
  "rewrite_target": "/products",
  "auth_required": false,
  "rate_limit_id": "50000000-0000-0000-0000-000000000001",
  "service": {
    "id": "60000000-0000-0000-0000-000000000001",
    "name": "product-service",
    "protocol": "http",
    "lb_strategy": "round_robin",
    "timeout_ms": 5000,
    "retry_count": 1
  }
}
```

Lệnh test gợi ý:

```redis
SET cfg:route:GET:/api/products '{"path":"/api/products","method":"GET"}'
GET cfg:route:GET:/api/products
```

### Phiên bản cấu hình

```text
cfg:version
```

Kiểu dữ liệu:

```text
String number
```

Cách dùng:

```redis
INCR cfg:version
GET cfg:version
```

Tăng giá trị này mỗi khi admin thay đổi routes, services, instances, CORS, API keys, blacklist hoặc rate limit policies.

### Kênh reload cấu hình

```text
cfg:reload
```

Đây là Redis Pub/Sub channel.

Cách dùng:

```redis
PUBLISH cfg:reload routes
PUBLISH cfg:reload services
PUBLISH cfg:reload rate_limits
PUBLISH cfg:reload security
```

Các node Gateway subscribe channel này và reload local cache khi cấu hình thay đổi.

### Cache plugin pipeline theo route

Metadata plugin và cấu hình gốc được lưu trong PostgreSQL. Redis chỉ lưu bản đã tổng hợp để Gateway đọc nhanh trong luồng request nóng.

#### Danh sách plugin của route

```text
cfg:plugins:{route_id}
```

Ví dụ:

```text
cfg:plugins:80000000-0000-0000-0000-000000000001
```

Kiểu dữ liệu:

```text
String JSON
```

Ví dụ value:

```json
[
  {
    "code": "cors",
    "phase": "before_request",
    "priority": 10,
    "is_required": true,
    "config": {
      "allowed_origins": ["http://localhost:3000"],
      "allowed_methods": ["GET", "POST"]
    }
  },
  {
    "code": "auth",
    "phase": "before_request",
    "priority": 30,
    "is_required": true,
    "config": {
      "required": true,
      "roles": ["admin"]
    }
  }
]
```

#### Pipeline plugin đã sắp xếp theo phase và priority

```text
cfg:pipeline:{route_id}
```

Ví dụ:

```text
cfg:pipeline:80000000-0000-0000-0000-000000000001
```

Kiểu dữ liệu:

```text
String JSON
```

Gateway nên đọc key này sau khi match route. Nếu cache miss, Gateway load từ PostgreSQL bằng `route_plugins` join `gateway_plugins`, sắp xếp theo `phase` và `priority`, sau đó ghi lại vào Redis.

#### Metadata plugin theo code

```text
cfg:plugin:{plugin_code}
```

Ví dụ:

```text
cfg:plugin:auth
cfg:plugin:rate_limit
cfg:plugin:cors
```

Kiểu dữ liệu:

```text
String JSON
```

Ví dụ value:

```json
{
  "code": "rate_limit",
  "name": "Rate Limit",
  "phase": "before_request",
  "default_priority": 40,
  "is_active": true
}
```

Khi admin thay đổi `gateway_plugins`, `route_plugins`, `cors_configs`, `rate_limit_policies` hoặc `routes`, cần tăng `cfg:version` và publish `cfg:reload` để các Gateway node clear/reload pipeline liên quan.

Lệnh test gợi ý:

```redis
SET cfg:pipeline:80000000-0000-0000-0000-000000000001 '[{"code":"auth","phase":"before_request","priority":30}]'
GET cfg:pipeline:80000000-0000-0000-0000-000000000001
```
## 2. Rate limiting

Luật rate limit được lưu trong PostgreSQL. Redis chỉ lưu bộ đếm.

### Fixed window theo IP

```text
rl:ip:{ip}:{window_start}
```

Ví dụ:

```text
rl:ip:127.0.0.1:202606161530
```

Kiểu dữ liệu:

```text
String number
```

Cách dùng:

```redis
INCR rl:ip:127.0.0.1:202606161530
EXPIRE rl:ip:127.0.0.1:202606161530 60 NX
TTL rl:ip:127.0.0.1:202606161530
```

### Fixed window theo user

```text
rl:user:{user_id}:{window_start}
```

Ví dụ:

```text
rl:user:40000000-0000-0000-0000-000000000001:202606161530
```

### Fixed window theo API key

```text
rl:api_key:{api_key_id}:{window_start}
```

Ví dụ:

```text
rl:api_key:a0000000-0000-0000-0000-000000000001:202606161530
```

### Token bucket

Chỉ nên làm phần này sau khi fixed window đã chạy ổn.

```text
rl:bucket:{limit_type}:{identifier}
```

Ví dụ:

```text
rl:bucket:ip:127.0.0.1
rl:bucket:user:40000000-0000-0000-0000-000000000001
rl:bucket:api_key:a0000000-0000-0000-0000-000000000001
```

Kiểu dữ liệu:

```text
Hash
```

Fields:

```text
tokens
ts
```

Logic token bucket nên chạy bằng Lua script để toàn bộ thao tác đọc, tính toán và ghi được thực hiện nguyên tử.

## 3. Trạng thái health check

PostgreSQL lưu cấu hình tĩnh của service instances. Redis lưu trạng thái alive/down realtime.

### Health của instance

```text
health:instance:{instance_id}
```

Ví dụ:

```text
health:instance:70000000-0000-0000-0000-000000000001
```

Kiểu dữ liệu:

```text
Hash
```

Fields:

```text
status
last_check
latency_ms
fail_count
```

Cách dùng:

```redis
HSET health:instance:70000000-0000-0000-0000-000000000001 status alive last_check 1717920050 latency_ms 12 fail_count 0
EXPIRE health:instance:70000000-0000-0000-0000-000000000001 30
HGETALL health:instance:70000000-0000-0000-0000-000000000001
```

### Danh sách instance alive theo service

```text
health:service:{service_id}:alive
```

Ví dụ:

```text
health:service:60000000-0000-0000-0000-000000000001:alive
```

Kiểu dữ liệu:

```text
Set
```

Cách dùng:

```redis
SADD health:service:60000000-0000-0000-0000-000000000001:alive 70000000-0000-0000-0000-000000000001
SREM health:service:60000000-0000-0000-0000-000000000001:alive 70000000-0000-0000-0000-000000000002
SMEMBERS health:service:60000000-0000-0000-0000-000000000001:alive
```

Load balancer chỉ nên chọn instance nằm trong set này.

## 4. Load balancing

### Bộ đếm round-robin

```text
lb:rr:{service_id}
```

Ví dụ:

```text
lb:rr:60000000-0000-0000-0000-000000000001
```

Kiểu dữ liệu:

```text
String number
```

Cách dùng:

```redis
INCR lb:rr:60000000-0000-0000-0000-000000000001
GET lb:rr:60000000-0000-0000-0000-000000000001
```

Logic chọn instance:

```text
selected_index = counter % len(alive_instances)
```

## 5. JWT blacklist

Dùng khi user logout hoặc token bị thu hồi trước thời điểm hết hạn.

```text
jwt:blacklist:{jti}
```

Ví dụ:

```text
jwt:blacklist:demo-jti-123
```

Kiểu dữ liệu:

```text
String
```

Cách dùng:

```redis
SET jwt:blacklist:demo-jti-123 "1" EX 900
EXISTS jwt:blacklist:demo-jti-123
TTL jwt:blacklist:demo-jti-123
```

TTL phải bằng thời gian sống còn lại của JWT.

Luồng kiểm tra JWT:

```text
1. Decode JWT
2. Lấy jti
3. Kiểm tra EXISTS jwt:blacklist:{jti}
4. Từ chối request nếu key tồn tại
```

## 6. Refresh token

Không lưu raw refresh token. Chỉ lưu hash của token.

```text
refresh:{token_hash}
```

Ví dụ:

```text
refresh:sha256_token_hash_here
```

Kiểu dữ liệu:

```text
String user_id
```

Cách dùng:

```redis
SET refresh:sha256_token_hash_here "40000000-0000-0000-0000-000000000001" EX 604800
GET refresh:sha256_token_hash_here
TTL refresh:sha256_token_hash_here
DEL refresh:sha256_token_hash_here
```

TTL gợi ý:

```text
7 ngày = 604800 giây
```

## 7. Cache IP blacklist

PostgreSQL lưu blacklist làm nguồn sự thật. Redis có thể cache các blacklist đang active để kiểm tra request nhanh hơn.

### Blacklist IP chính xác

```text
blacklist:ip
```

Kiểu dữ liệu:

```text
Set
```

Cách dùng:

```redis
SADD blacklist:ip 127.0.0.1
SISMEMBER blacklist:ip 127.0.0.1
SREM blacklist:ip 127.0.0.1
```

### Blacklist CIDR

CIDR matching nên xử lý trong backend sau khi load danh sách CIDR active từ PostgreSQL hoặc Redis.

Key cache gợi ý:

```text
blacklist:cidr
```

Kiểu dữ liệu:

```text
Set
```

Ví dụ:

```redis
SADD blacklist:cidr 10.0.0.0/24
SMEMBERS blacklist:cidr
```

## 8. Lệnh kiểm tra hữu ích

Mở Redis CLI:

```powershell
docker exec -it gateway-redis redis-cli
```

Kiểm tra kết nối:

```redis
PING
```

Liệt kê key theo nhóm:

```redis
KEYS cfg:*
KEYS rl:*
KEYS health:*
KEYS lb:*
KEYS jwt:blacklist:*
KEYS refresh:*
KEYS blacklist:*
```

Kiểm tra kiểu dữ liệu của key:

```redis
TYPE key_name
```

Kiểm tra TTL:

```redis
TTL key_name
```

Đọc các kiểu dữ liệu thường dùng:

```redis
GET key_name
HGETALL key_name
SMEMBERS key_name
```

Xóa key test:

```redis
DEL key_name
```

## 9. Thứ tự triển khai trong backend

Thứ tự triển khai backend khuyến nghị:

```text
1. Kết nối Redis
2. Thêm Redis ping vào /ready
3. Cache route config với cfg:route:...
4. Thêm fixed-window rate limit với rl:...
5. Thêm bộ đếm round-robin với lb:rr:...
6. Thêm trạng thái health check với health:...
7. Thêm JWT blacklist với jwt:blacklist:...
8. Thêm lưu refresh token với refresh:...
9. Thêm cache IP blacklist với blacklist:...
```

Redis có thể trống sau khi chạy migrations và seeds. Đây là điều bình thường vì migrations và SQL seeds chỉ ghi vào PostgreSQL. Redis keys chỉ xuất hiện khi backend Gateway bắt đầu xử lý request hoặc background worker ghi trạng thái realtime.


DANH SÁCH CÁC KEY CHÍNH
cfg:version
cfg:route:{method}:{path}

rl:ip:{ip}:{window_start}
rl:user:{user_id}:{window_start}
rl:api_key:{api_key_id}:{window_start}

health:instance:{instance_id}
health:service:{service_id}:alive

jwt:blacklist:{jti}
refresh:{token_hash}

lb:rr:{service_id}

