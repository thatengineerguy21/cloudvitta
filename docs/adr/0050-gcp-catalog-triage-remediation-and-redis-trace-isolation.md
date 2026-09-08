# ADR 0050: GCP Catalog Triage Remediation, Bangkok Region Support, and Redis Trace Isolation

## Status
Accepted

## Context
During live pricing ingestion (`Logs-2026-09-08 11_27_11.json`), three errors occurred:
1. `gcp/serverless`: 5.91% unmapped item ratio (14 unmapped / 237 in-scope items, 518 ignored) tripped the 5.00% safety circuit-breaker in `internal/adapter/provider/factory.go`.
2. `gcp/database_rdbms`: 17.68% unmapped item ratio (531 unmapped / 3,004 in-scope items, 16,716 ignored) tripped the 5.00% safety circuit-breaker.
3. OpenTelemetry: Grafana Cloud OTLP HTTP gateway rejected trace export batches with `413 Request Entity Too Large` (`http: request body too large`).

Live catalog inspection of all 923 Serverless SKUs and 20,991 Database SKUs showed:
- All 14 unmapped serverless SKUs failed region mapping because Google Cloud Bangkok (`asia-southeast4`) was absent from `internal/matching/regionmap/gcp.go`.
- The 531 unmapped database items consisted of:
  - 172 auxiliary `ServerlessExport` SKUs.
  - 172 legacy Gen2 shared-core `Micro instance` SKUs (where GCP descriptions use "Micro instance" instead of literal "f1-micro").
  - 172 legacy Gen2 shared-core `Small instance` SKUs (where GCP descriptions use "Small instance" instead of literal "g1-small").
  - 14 obsolete first-generation Cloud SQL `SQLGen1Instances` D-tiers (`D0` through `D32`).
  - 1 item in region `asia-southeast4`.
- The OpenTelemetry HTTP 413 error occurred because Redis auto-instrumentation (`redisotel.InstrumentTracing`) enabled `db.statement` logging by default. During post-ingestion cache warming, multi-megabyte JSON payloads were stored on trace span attributes and batched in groups of 512 spans.

## Decision

### 1. Google Cloud Bangkok Region Mapping
- Add `asia-southeast4` mapped to canonical region group `ap-southeast` in `internal/matching/regionmap/gcp.go`.
- Add unit tests for `asia-southeast4` in `internal/matching/regionmap/gcp_test.go`.

### 2. Cloud SQL 3-Way Triage Expansion
- Extend `isOutOfScopeDatabaseSKU` in `internal/adapter/provider/gcp/normalize.go` to classify:
  - `ServerlessExport` resource group and descriptions containing "serverless export".
  - `SQLGen2InstancesF1Micro` resource group and descriptions containing "micro instance".
  - `SQLGen2InstancesG1Small` resource group and descriptions containing "small instance".
  - `SQLGen1Instances` resource group and descriptions matching obsolete D-tiers ("usage - hour" or "D0"-"D32").
- Add unit tests for these out-of-scope line items in `internal/adapter/provider/gcp/database_test.go`.

### 3. Redis Ingestion Trace Isolation & Batch Size Tuning
- Introduce `cache.WithoutTracer()` functional option in `internal/cache/client.go` to disable Redis tracing in bulk ingestion workloads (`cmd/ingest/main.go`), matching `store.WithoutTracer()`.
- Disable statement payload logging via `redisotel.WithDBStatement(false)` when Redis tracing is enabled.
- Lower `sdktrace.WithMaxExportBatchSize` from 512 to 128 in `internal/observability/otel.go`.

## Consequences

### Positive
- Live catalog verification against all 923 Serverless SKUs and 20,991 Database SKUs proves a 0.00% unmapped ratio (0 quarantined items) across both categories.
- Multi-megabyte cache payloads are removed from trace spans, keeping OTLP HTTP trace exports well below the Grafana Cloud 4MB request entity limit.
- Circuit breakers continue to enforce strict 5.00% safety guarantees.

### Negative
- None.
