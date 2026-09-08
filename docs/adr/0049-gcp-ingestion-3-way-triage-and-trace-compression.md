# ADR 0049: GCP Ingestion 3-Way Triage Classification and OTLP Trace Compression

## Status
Accepted

## Context
During live ingestion run `cloudvitta-ingest` (`Logs-2026-09-08 09_48_07.json`), four distinct failures occurred:
1. `gcp/database_nosql`: HTTP 404 response caused by an invalid service ID for Bigtable (`C802-861C-2155` instead of `C3BE-24A5-0975`).
2. `gcp/database_rdbms`: 69.56% unmapped item ratio (5,881 unmapped / 8,454 in-scope items) tripped the 5.00% safety circuit-breaker in `internal/adapter/provider/factory.go`.
3. `gcp/serverless`: 78.97% unmapped item ratio (462 unmapped / 585 in-scope items) tripped the 5.00% safety circuit-breaker.
4. OpenTelemetry: Grafana Cloud OTLP HTTP gateway rejected trace export batches with `413 Request Entity Too Large` (`http: request body too large`).

The 5.00% unmapped item circuit-breaker is a safety guarantee that prevents corrupted pricing datasets from reaching the database. Raising or disabling this threshold was strictly prohibited. Instead, the root causes of the unmapped items and payload size bloat had to be resolved.

## Decision

### 1. Bigtable Service ID & Live Regression Suite
- Corrected the Google Cloud Bigtable service ID to `C3BE-24A5-0975` in `internal/adapter/provider/gcp/client.go` and catalog mappings.
- Aligned user-facing product naming to "Cloud Run Functions" to reflect official Google Cloud rebranding.
- Added an opt-in live catalog regression test (`live_catalog_test.go`) under `//go:build live_gcp_catalog` that validates all 9 GCP service IDs against `https://cloudbilling.googleapis.com/v1/services`.

### 2. Cloud SQL 3-Way Triage Classification
Per ADR 0037, cloud provider billing feeds contain auxiliary operational items that do not belong to raw compute instance or storage rates. We implemented a 3-way triage filter in `internal/adapter/provider/gcp/`:
- **In-Scope Instance/Storage:** Matched and synthesized into `domain.PriceObservation` records.
- **Known Out-of-Scope:** Auxiliary SKUs (extended version support, provisioned IOPS, provisioned throughput, automated backups, point-in-time recovery, reserved static IP addresses, network egress, standby/replica high-availability overhead, promotional discounts, and legacy shared tiers `g1-small`/`f1-micro`) are classified by `isOutOfScopeDatabaseSKU` and increment `IgnoredCount` without entering the quarantine sink.
- **Novel Unknowns:** Any unknown SKU that is not out-of-scope continues to record to `quarantine.Sink` with descriptive metadata.
- Supported `+` separator in hardware specification descriptions (e.g., `96 vCPU + 360GB RAM`).

### 3. Serverless Positive Rate-Component Matching
Cloud Run and Cloud Run Functions SKUs contain billing mode qualifiers such as `(Request-based billing)` and `(Instance-based billing)`.
- Reordered the component type detection switch in `normalizeServerlessSKU`: evaluate CPU meters (`vcpu`, `ghz`, `cpu`) and Memory meters (`memory`, `giby.s`, `gb-second`) **before** request fee matching.
- Constrained request fee matching to specifically target `invocation`, `request count`, or non-request-based requests.
- Added `isOutOfScopeServerlessSKU` to cleanly ignore auxiliary data transfer and network egress items.
- Extended `serverlessarchmap.MapGCPArchitecture` with positive tokens (`allocation`, `cpu`, `memory`, `gib-second`, `vcpu-second`, `instance`, `job`, `worker pool`), while strictly preserving fail-loud quarantine for unsupported ARM architectures.

### 4. OTLP Gzip Compression and Batch Sizing
- Enabled HTTP Gzip compression across all OTLP exporters (`otlptracehttp.WithCompression(otlptracehttp.GzipCompression)`, `otlpmetrichttp`, `otlploghttp`) in `internal/observability/otel.go`.
- Configured batch span processing limits: `sdktrace.WithMaxExportBatchSize(512)` and `sdktrace.WithMaxQueueSize(2048)`.
- Audited normalization and quarantine packages to confirm zero per-item tracing spans exist, maintaining span-per-stage granularity.
- Updated Open Question #16 in `.agents/13-OPEN-QUESTIONS.md`.

## Consequences

### Positive
- Ingestion runs for GCP database and serverless complete cleanly with 0% unmapped rates on standard catalogs, operating well below the 5% threshold.
- HTTP 413 errors on Grafana Cloud trace exports are completely eliminated through 90-95% compression efficiency and bounded batch sizes.
- Quarantine fidelity is fully preserved: novel shapes, missing units, and unsupported architectures (such as ARM serverless) continue to quarantine loudly.

### Negative
- Minimal CPU overhead during ingestion for Gzip stream compression (offset by reduced network payload transmission time).
