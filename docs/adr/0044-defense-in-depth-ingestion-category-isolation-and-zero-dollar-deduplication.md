# 44. Defense-in-Depth Ingestion Category Isolation and Zero-Dollar Deduplication

Date: 2026-09-07

## Status

Accepted

## Context

Investigation into record accumulation in the `price_observations` table revealed two critical defects:
1. **Category Contamination**: Provider price payloads contain multiple service families. In AWS, the bulk `AmazonEC2` payload contains Data Transfer Out items. In GCP, the Compute Engine service ID (`6F81-5844-456A`) returns virtual machine instances, network egress, and disk storage. The normalizers classified items into network or storage categories, but the provider adapters and orchestrator lacked isolation filters. As a result, network records were stored under `service_category = 'compute'` with empty compute attributes (`vcpu: 0, ram_gb: 0`), and GCP compute and network jobs emitted duplicate cross-category observations.
2. **Zero-Dollar ($0.00) Deduplication Bypass**: Unchanged observation deduplication contained an explicit `!oldPrice.IsZero()` guard. For zero-dollar observations ($0.00), such as free-tier data transfer or free ingress, this check evaluated to `false`. The system executed an insert on every ingestion run instead of updating `last_seen_at`. Because `fetched_at` was part of the unique conflict key, duplicate records accumulated on every run.
3. **Anomaly Detection Blindspot**: Pricing transitions between $0.00 (free) and billable rates (> $0.00) were skipped because of non-zero checks.

We need a comprehensive architectural solution that prevents category contamination across all providers, ensures idempotent deduplication for all price amounts including $0.00, flags directional free-to-billable price transitions, and cleans up historical database corruption.

## Decision

We establish defense-in-depth category isolation, zero-dollar deduplication idempotency, and directional anomaly detection:

1. **Normalizer Explicit Category Assignment**:
   - In [`internal/adapter/provider/aws/normalize.go`](file:///D:/02-code/cloudvitta/internal/adapter/provider/aws/normalize.go), when a product is identified as `isNetworkProduct`, set `category = "network"` directly instead of mapping the parent payload service code (`AmazonEC2`).
   - When a product is identified as `isStorageProduct`, set `category = "storage"` directly.

2. **Adapter-Level Category Filtering**:
   - In `Fetch()` across all 7 cloud provider adapters (AWS, Azure, GCP, Oracle, IBM, Alibaba, DigitalOcean), filter returned observations to match the configured adapter category `a.category`.
   - Adapters never emit observations outside their declared responsibility.

3. **Orchestrator Invariant Guard**:
   - In [`internal/service/ingest_orchestrator.go`](file:///D:/02-code/cloudvitta/internal/service/ingest_orchestrator.go), enforce an invariant guard before database operations: drop and log any observation where `obs.ServiceCategory != job.Category` or `obs.Provider != job.Provider`.

4. **Zero-Dollar Deduplication Idempotency**:
   - Remove `!oldPrice.IsZero()` guards from deduplication checks in `IngestionOrchestrator` and `IngestionService`.
   - When `hasPrior && oldPrice.Equal(obs.PriceAmount)`, the observation is recognized as unchanged. The system updates `last_seen_at` on the existing row without inserting a new row.

5. **Directional Anomaly Detection**:
   - Flag transitions between $0.00 and > $0.00 as `anomaly_status = "pending_review:free_to_billable"` with metric attribute `anomaly_type = "free_to_billable"`.
   - Flag transitions between > $0.00 and $0.00 as `anomaly_status = "pending_review:billable_to_free"` with metric attribute `anomaly_type = "billable_to_free"`.
   - Retain ratio threshold anomaly evaluation for non-zero price changes.

6. **Forward-Only Database Remediation**:
   - Forward-only migration `migrations/0006_cleanup_category_contamination_and_duplicates.sql` purges misclassified Data Transfer records from `compute` and deduplicates historical $0.00 records, retaining the earliest record and bumping `last_seen_at` to the latest observed timestamp.

## Consequences

### Positive
- **Guaranteed Category Purity**: Virtual machine compute comparison views never contain networking or storage records.
- **Deduplication Idempotency**: Zero-dollar cloud pricing records update `last_seen_at` cleanly without record inflation.
- **Enhanced Operational Observability**: Human reviewers receive enriched anomaly metadata distinguishing free-to-billable and billable-to-free transitions.
- **Defense-in-Depth Safety**: Normalizer corrections, adapter filters, and orchestrator invariant guards ensure cross-category contamination cannot recur.

### Negative
- Adapters require explicit `WithCategory(...)` options when filtering to a single category, which tests must respect when asserting category-specific behavior.

## Alternatives Considered

1. **Orchestrator-Only Filtering**: Rejected because provider adapters would still stream and buffer out-of-scope records across the network and heap, creating unnecessary memory overhead in Cloud Run.
2. **Adapter-Only Filtering**: Rejected because a bug in an adapter normalizer could still contaminate database tables if the orchestrator lacked an invariant boundary.
