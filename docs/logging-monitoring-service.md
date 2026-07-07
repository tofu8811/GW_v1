# Logging & Monitoring Service

## Architecture

The gateway records request/response metadata in middleware and uses Fiber `requestid` as `trace_id`.
When `RABBITMQ_URL` is configured, the gateway publishes each log event to RabbitMQ and also keeps
the existing JSONL file sink as a fallback/debug trail. If RabbitMQ is unavailable, request handling
continues and the error is written to the application logger.

Flow:

```text
client -> gateway-api -> RabbitMQ -> log-service -> Elasticsearch gateway-logs-*
                                      |
                                      +-> /admin/logs and /admin/metrics/*
```

## RabbitMQ topology

- Exchange: `gateway.logs.exchange`
- Type: `topic`
- Queue: `gateway.logs.queue`
- Routing keys:
  - `gateway.request.completed`
  - `gateway.request.error`
- Dead-letter exchange: `gateway.logs.dlx`
- Dead-letter queue: `gateway.logs.dlq`

Messages are published as persistent JSON documents. The log-service acknowledges messages only
after Elasticsearch bulk indexing succeeds.

## Admin routes

All routes below use JWT and RBAC:

- `GET /admin/logging/health`: requires `health:read`
- `GET /admin/logs`: requires `logs:read`
- `GET /admin/metrics/summary`: requires `metrics:read`
- `GET /admin/metrics/rps`: requires `metrics:read`
- `GET /admin/metrics/error-rate`: requires `metrics:read`
- `GET /admin/metrics/latency`: requires `metrics:read`
- `GET /admin/metrics/status-codes`: requires `metrics:read`
- `GET /admin/metrics/top-routes`: requires `metrics:read`
- `GET /admin/metrics/realtime/stream`: requires `metrics:read`

Common metric query parameters:

```text
from
to
service_name
route_id
method
status_class
status_code
client_ip
interval
```

`GET /admin/logs` also supports:

```text
page
limit
q
sort
```

`GET /admin/metrics/top-routes` also supports:

```text
sort_by=requests|errors|latency
limit
```

## Realtime dashboard

The dashboard uses Server-Sent Events for realtime updates:

```text
GET /admin/metrics/realtime/stream?window=60s&interval=1s&top_limit=10
```

Supported query parameters:

```text
window
interval
service_name
route_id
method
status_class
status_code
client_ip
top_limit
```

The service clamps expensive realtime queries:

- `window`: default `60s`, min `10s`, max `15m`
- `interval`: default `1s`, min `1s`, max `1m`
- `top_limit`: default `10`, max `50`

SSE events:

```text
event: metrics
id: <generated_at>
data: <DashboardSnapshot JSON>

event: ping
data: {}

event: error
data: {"code":"elasticsearch_unavailable","message":"metrics temporarily unavailable"}
```

The SSE snapshot is built from a single Elasticsearch aggregation query per tick. It includes:

- summary: total requests, error count, error rate, avg/p50/p95/p99 latency
- rps time series
- error-rate time series
- status class/code distribution
- top routes by `normalized_path`

Native browser `EventSource` cannot send an `Authorization` header. If the admin frontend keeps using
Bearer tokens, use an EventSource polyfill that supports custom headers. Other options are HttpOnly
auth cookies or a short-lived stream token.

Kibana can still be used for debugging and internal observation, but the admin dashboard reads
Elasticsearch through these backend routes.
