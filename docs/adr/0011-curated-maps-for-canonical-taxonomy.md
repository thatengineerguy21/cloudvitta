# 11. Curated Maps for Canonical Taxonomy

Date: 2026-08-06

## Status

Accepted

## Context

Cloud providers use diverse, proprietary naming conventions for regions (e.g., AWS uses `us-east-1`, Azure uses `eastus`) and product categories. To perform apples-to-apples comparisons, CloudVitta must normalize these into a canonical taxonomy (e.g., `region_group = us-east`).

We needed to decide whether this taxonomy mapping should be dynamically inferred by parsing the strings, or explicitly hardcoded in curated maps. Furthermore, we needed to define the failure mode when a previously unknown region or category is encountered in the upstream API.

## Decision

We will use **Static Curated Maps** (`internal/matching/catalogmap` and `internal/matching/regionmap`) as the single source of truth, strictly rejecting dynamic inference.

When an unknown region or category is encountered:
1. The ingestion job **does not crash** and **does not drop the row**.
2. The raw data is ingested with the canonical field left unmapped.
3. A `region_not_mapped` (or `category_not_mapped`) warning is attached, and the item is excluded from cross-provider comparisons.
4. This gap triggers an observability metric, signaling that a human review and map update is required.

## Consequences

### Positive
- Prevents silent data corruption. A wrong dynamically-inferred mapping would silently poison comparisons and aggregates, which is significantly worse than an honest omission.
- Failures are highly visible via warnings and metrics, transforming customer-reported bugs into proactive system alerts.
- New regions are infrequent enough that periodic, deliberate map updates are a reasonable maintenance cadence.

### Negative
- Requires a developer to manually update the Go maps and deploy a new version of the ingestion worker whenever a provider introduces a new region or product category.

## Alternatives Considered

### Alternative 1: Dynamic String Parsing and Regular Expression Heuristics
Rejected. Provider naming schemes are inconsistent (e.g., `us-east-1`, `eastus`, `us-central1`). Dynamic string inference causes silent misclassification that pollutes comparison results and corrupts pricing aggregates.

### Alternative 2: LLM Semantic Classification at Ingestion
Rejected. Using an LLM to categorize regions or products during bulk ingestion introduces non-deterministic outputs, adds unnecessary network latency, and incurs ongoing token costs without mathematical guarantees of taxonomic correctness.
