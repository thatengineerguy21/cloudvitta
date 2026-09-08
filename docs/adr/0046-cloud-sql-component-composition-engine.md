# 46. Cloud SQL Component Composition Engine and Instance Shape Synthesis

Date: 2026-09-08

## Status

Accepted

## Context

Google Cloud Platform does not publish pre-bundled virtual machine instance SKUs in its Cloud SQL Cloud Billing Catalog (`9662-B51E-5089`). Instead, Google bills Cloud SQL database compute as discrete, independent hourly meters for vCPU and RAM:
1. `Cloud SQL for [Engine]: [Zonal|Regional] - Enterprise [Plus] vCPU in [Region]`
2. `Cloud SQL for [Engine]: [Zonal|Regional] - Enterprise [Plus] RAM in [Region]`

In contrast, AWS RDS and Azure Database publish pre-bundled virtual instance shapes (e.g. `db.m6i.xlarge`, `Standard_D4ds_v5`).

A previous normalizer implementation expected instance identifiers matching `db-custom-(\d+)-(\d+)`. Because real catalog feeds do not contain this pattern, all vCPU and RAM meters silently defaulted to fabricated values of 2 vCPU and 8 GB RAM at single-unit rates (~$0.05/hr). This defect corrupted database records and caused dimensional distance calculations to exceed the maximum allowable threshold ($0.50$), rejecting queries for standard specifications ($4\text{ vCPU}, 16\text{ GB RAM}$) with `no_match`.

## Decision

We implement a dedicated component composition engine for Google Cloud SQL in `internal/adapter/provider/gcp/normalize.go`, following the precedent established for GCP Compute Engine and Oracle flexible shapes (ADR 0026, ADR 0037):

1. **Curated Instance Shapes (`knownGCPCloudSQLSpecs`)**:
   We define standard Cloud SQL instance shapes covering the canonical 1:3.75 ratio (1/3.75, 2/7.5, 4/15, 8/30, 16/60, 32/120, 64/240), 1:4 ratio (1/4, 2/8, 4/16, 8/32, 16/64), and 1:8 ratio (2/16, 4/32, 8/64, 16/128).

2. **Database Component Rate Collector**:
   During stream normalization, incoming Cloud SQL vCPU and RAM meters are identified and collected by composite key:
   $$\text{Key} = (\text{engine}, \text{tier}, \text{multiAZ}, \text{region})$$
   Extended support surcharge meters and non-database SKUs are excluded from component collection.

3. **Instance Synthesis Engine (`composeCloudSQLInstancePricing`)**:
   At the end of the SKU stream, the adapter iterates over collected component pairs and curated shapes, synthesizing instance observations:
   $$\text{Price}_{\text{hourly}} = (\text{corePrice} \times \text{vCPU}) + (\text{ramPrice} \times \text{RAMGB})$$
   $$\text{SkuID} = \text{SKU-GCP-CLOUDSQL-}\{\text{ENGINE}\}\text{-}\{\text{TIER}\}\text{-}\{\text{VCPU}\}\text{VCPU-}\{\text{RAM}\}\text{GB}[-\text{HA}]$$
   Synthesized observations carry `component_type = "instance"`.

4. **Preservation of ADR 0030 Storage Join**:
   Storage SKUs (`Storage PD SSD`, `Storage PD HDD`) continue to be normalized with `component_type = "storage"` and are joined at query time by `DatabaseRDBMSScorer`.

5. **Fail-Loud Quarantine for Unparseable SKUs**:
   The fabricated `(2, 8, tier)` fallback is completely removed. Unrecognized database SKUs route to `quarantine.Sink` with `kind = "database_attributes"`.

## Consequences

### Positive
- Completely eliminates fabricated attribute corruption from the GCP relational database ingestion path.
- Corrects the mathematical cutoff breach in `DatabaseRDBMSScorer`, enabling exact and close matches for standard customer database queries (e.g. 4 vCPU / 16 GB).
- Retains compatibility with the existing query-time storage join (ADR 0030) and multi-provider comparison endpoints.

### Negative
- CloudVitta must maintain a curated list of standard Cloud SQL instance shapes in code.

## Alternatives Considered

### Alternative 1: Retaining Fallback Placeholder Attributes
Rejected. Violates the core honesty invariant of CloudVitta ("never let the API lie") and violates consistency rule 9 (`08-CONSISTENCY-RULES.md`).

### Alternative 2: Query-Time 3-Way Join (vCPU Meter + RAM Meter + Storage)
Rejected. A 3-way query-time join would require restructuring the transport-agnostic `DatabaseRDBMSScorer` specifically for GCP, introducing unnecessary complexity into the shared matching engine.
