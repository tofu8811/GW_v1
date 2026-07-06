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

The simplest frontend integration is polling:

- Poll `/admin/metrics/summary?from=now-60s&to=now`
- Poll `/admin/metrics/rps?from=now-60s&to=now&interval=1s`
- Poll `/admin/metrics/error-rate?from=now-60s&to=now&interval=1s`
- Poll `/admin/metrics/status-codes?from=now-60s&to=now`

Polling every 1-2 seconds is enough for a graduation-project dashboard and avoids adding WebSocket
state management before it is necessary.
