# CloudVitta Observability Architecture

This document describes the observability specification for CloudVitta. It lists the OpenTelemetry spans, metrics, and structured log attributes for all system layers.

---

## 1. Overview

CloudVitta exports telemetry to OpenTelemetry collectors (such as Grafana Cloud). Telemetry includes:
- **Distributed Traces**: Context propagation using W3C TraceContext headers, automatic database instrumentation via `otelpgx`, Redis client hooks via `redisotel`, storage spans, DLQ spans, and adapter spans.
- **Metrics**: Dual export via local Prometheus endpoint (`/metrics`) and remote OTLP metric exporter.
- **Structured Logs**: JSON logs emitted by Go `log/slog` through `otelslog`, carrying active `trace_id` and `span_id`.

---

## 2. Distributed Tracing Spans

The table below defines all custom and auto-instrumented spans:

| Layer / Package | Span Name | Kind | Attributes | Description |
|---|---|---|---|---|
| `transport/rest` | `HTTP {METHOD} {route}` | Server | `http.method`, `http.route`, `http.status_code` | Auto-instrumented by `otelhttp` |
| `service` | `ComparePrices` | Internal | `category`, `provider`, `region` | Pricing comparison lifecycle |
| `service` | `CalculateWorkload` | Internal | `categories.count`, `providers.count` | Composite multi-category calculation |
| `store` | `pgx.Connect`, `pgx.Query`, `pgx.Exec`, `pgx.Batch` | Client | `db.system=postgresql`, `db.statement` | Auto-instrumented by `otelpgx` |
| `cache` | `redis.{command}` | Client | `db.system=redis`, `db.operation` | Auto-instrumented by `redisotel` |
| `storage` | `gcs.write` | Client | `gcs.bucket`, `gcs.path` | GCS object stream upload |
| `storage` | `gcs.read` | Client | `gcs.bucket`, `gcs.path` | GCS object stream read |
| `storage` | `gcs.list` | Client | `gcs.bucket`, `gcs.prefix` | GCS bucket object listing |
| `storage` | `memory.write` | Internal | `storage.path` | In-memory raw storage write |
| `storage` | `memory.read` | Internal | `storage.path` | In-memory raw storage read |
| `storage` | `memory.list` | Internal | `storage.prefix` | In-memory raw storage list |
| `dlq` | `dlq.record` | Internal | `dlq.provider`, `dlq.category`, `dlq.status` | DLQ failure entry write |
| `dlq` | `dlq.clear` | Internal | `dlq.provider`, `dlq.category` | DLQ record removal |
| `dlq` | `dlq.get` | Internal | `dlq.provider`, `dlq.category` | DLQ entry lookup |
| `adapter` | `{provider}.fetch` | Internal | `provider`, `category` | Ingestion fetch and normalization |

---

## 3. Metrics Catalog

The table below lists all custom application metrics:

| Metric Name | Type | Unit | Attributes / Labels | Description |
|---|---|---|---|---|
| `cache_requests_total` | Int64Counter | `1` | `result` (`hit` \| `miss`) | Cache lookup operations count |
| `db.pool.active_connections` | Int64ObservableGauge | `{connection}` | None | Current active acquired PostgreSQL connections |
| `db.pool.idle_connections` | Int64ObservableGauge | `{connection}` | None | Current idle PostgreSQL connections in pool |
| `db.pool.total_connections` | Int64ObservableGauge | `{connection}` | None | Current total allocated PostgreSQL pool connections |
| `db.pool.max_connections` | Int64ObservableGauge | `{connection}` | None | Maximum configured PostgreSQL connections |
| `storage_operation_duration_seconds` | Float64Histogram | `s` | `operation` (`write` \| `read` \| `list`), `backend` (`gcs` \| `memory`), `status` (`ok` \| `error`) | Duration of storage read, write, and list calls |
| `dlq_operations_total` | Int64Counter | `1` | `operation` (`record` \| `clear` \| `get`), `status` (`ok` \| `error`) | Total count of DLQ client operations |
| `adapter.fetch.duration_seconds` | Float64Histogram | `s` | `provider`, `category`, `status` (`ok` \| `error`) | Wall-clock execution time of adapter Fetch |
| `adapter.fetch.bytes_total` | Int64Counter | `By` | `provider`, `category` | Cumulative raw JSON response bytes streamed to storage |
| `{provider}.fetch.count` | Int64Counter | `1` | `category`, `status` (`success` \| `error`) | Total provider fetch attempts |
| `{provider}.normalize.unmapped_count` | Int64Counter | `1` | `provider`, `category` | Count of unmapped SKUs routed to quarantine |

---

## 4. Structured Logging Invariants

All log lines use structured `slog` key-value pairs and automatically receive `trace_id` and `span_id` when called with `slog.*Context`:

- **Database Pool**: Logged at `slog.Info` on startup with `max_conns` and `min_conns`.
- **Redis Connection**: Logged at `slog.Info` on startup with `addr`.
- **Storage**:
  - `slog.DebugContext` on write success with `bucket` and `path`.
  - `slog.ErrorContext` on write, read, or list errors with `bucket`, `path`, and `error`.
- **DLQ**:
  - `slog.InfoContext` on record with `provider`, `category`, `status`, and `consecutive_failures`.
  - `slog.InfoContext` on clear with `provider` and `category`.
  - `slog.WarnContext` on unexpected get errors with `provider`, `category`, and `error`.
- **Ingestion**: Logged with `provider`, `category`, `observations`, `unmapped`, and `ignored`.
