# 41. Dedicated Compute Instance Catalog Table and Hardware Specification Normalization

Date: 2026-09-04

## Status

Accepted

## Context

CloudVitta previously stored compute hardware specifications only in the polymorphic JSONB `attributes` column of the `price_observations` table.
The `price_observations` table stores time-series price observations across many geographical regions and currencies.

Querying this table for unique virtual machine specifications (vCPU count, memory GiB, instance family, CPU architecture, and workload category) required expensive table scans and JSONB filter operations over hundreds of thousands of regional price records.
Furthermore, web clients and MCP AI agents required fast access to compute instance inventory, hardware categories, and specification auto-completion without calculating prices.

## Decision

We introduce a dedicated `compute_instance_catalog` table via forward-only migration `0004_create_compute_instance_catalog.sql`:

1. **Dedicated Relational Catalog Schema**:
   - The table stores slowly changing virtual machine hardware specifications (`provider`, `instance_type_id`, `display_name`, `instance_family`, `category`, `vcpu`, `memory_gib`, `cpu_architecture`, `gpu_count`, `gpu_type`, `is_burstable`, `is_current_gen`, `first_seen_at`, `last_seen_at`, `attributes`).
   - A unique constraint on `(provider, instance_type_id)` supports atomic upserts.
   - B-tree indexes optimize lookups by provider, category, instance family, vCPU, and memory.

2. **Ingestion-Time Catalog Synchronization**:
   - During compute price observation ingestion, `SyncComputeCatalog` extracts hardware specifications from normalized price records.
   - It determines hardware category classification (`general_purpose`, `compute_optimized`, `memory_optimized`, `gpu_accelerated`, `storage_optimized`), CPU architecture (`arm64`, `x86_64`), and burstable flags.
   - It upserts records into `compute_instance_catalog` using SQLC queries.

3. **Two-Tier Caching for Catalog Telemetry**:
   - Global catalog summary data is cached in Redis with key format `{schema_version}:catalog:summary` and a 24-hour time-to-live.
   - Ingestion jobs invalidate or warm the cache on completion.

4. **Multi-Transport Access**:
   - REST endpoints `/api/v1/catalog/compute/summary` and `/api/v1/catalog/compute/instances` provide filtered queries and hardware summary rollups.
   - Model Context Protocol (MCP) tool `get_compute_catalog` exposes instance filtering directly to external AI agents.

5. **Frontend User Interface Integration**:
   - `/status` displays the `ComputeCatalogSummaryCard` with provider inventory metrics and category distribution matrix.
   - `/compare/compute` provides the `InstanceTypeAutocomplete` component to populate hardware search filters instantly from catalog presets.

## Consequences

### Positive

- Hardware specification queries no longer scan large regional pricing tables.
- Catalog summary requests respond in sub-millisecond time from Redis cache.
- The system provides a clear domain separation between immutable machine specifications and temporal price records.
- User interfaces and AI agents gain fast specification auto-completion.

### Negative

- Compute price ingestion requires an additional synchronization step to update catalog records.
- Added database table and index storage requirements.

## Alternatives Considered

1. **Querying `price_observations` with `DISTINCT ON`**:
   - High database CPU and memory usage when aggregating over large observation tables.
   - Slower query response times for client autocomplete components.

2. **Static JSON/YAML Hardware Files in Repository**:
   - Cannot reflect new instance types discovered during provider price ingestion without code deployments.
   - Does not allow database joins or SQL-based filtering.
