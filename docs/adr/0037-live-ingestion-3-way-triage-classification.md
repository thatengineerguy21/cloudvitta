# 37. Live Ingestion 3-Way Triage Classification and Component Composition

Date: 2026-08-25

## Status

Accepted

## Context

During live provider pricing feed audits across cloud providers (AWS, Azure, GCP, Oracle OCI, IBM Cloud, Alibaba Cloud, DigitalOcean), normalizers encountered large volumes of unmodeled catalog records. Examples include:
- Cloud SQL billing feeds containing over 6,700 network egress SKUs under database service envelopes.
- Oracle OCI publishing more than 500 enterprise software and APEX products in a monolithic pricing catalog.
- GCP publishing Compute Engine virtual machine rates as separate Core and RAM component SKUs rather than monolithic instance SKUs.
- Azure publishing multiple distinct meters (such as data storage versus write operations) under identical `skuId` identifiers.

Earlier ingestion logic treated all unmapped records as quarantined taxonomy items. This caused false-positive threshold alarms where the unmapped ratio exceeded the maximum 5% threshold, blocking valid ingestion jobs.

## Decision

1. **Foundational 3-Way Classification Model**:
   - **In-Scope Valid (`ItemClassificationNormalized`)**: Successfully parsed into domain `PriceObservation` records.
   - **In-Scope Unknown (`ItemClassificationQuarantined`)**: In-scope resources with unmapped taxonomy attributes (unknown shapes, tiers, regions, or database engines). Records route to `quarantine.Sink` and increment `UnmappedCount`.
   - **Out-of-Scope Discarded (`ItemClassificationIgnored`)**: Unmodeled provider catalog lines (such as network egress under database feeds, non-IaaS enterprise services, and unsupported commitments). Records increment `IgnoredCount` and are discarded without quarantine.

2. **In-Scope Threshold Formula**:
   The unmapped ratio is computed strictly against in-scope items:
   $$\text{ratio} = \frac{\text{UnmappedCount}}{\text{UnmappedCount} + \text{ValidCount}}$$
   Ignored records are excluded from the denominator.

3. **GCP Compute Component SKU Assembly (Question #24)**:
   - GCP normalizers collect standalone Core and RAM component rates.
   - Machine type observations are synthesized using curated specifications (`knownGCPVMSpecs`), aligning with Oracle OCI flexible shape synthesis (ADR 0026).
   - Monolithic predefined instances containing both Core and RAM continue to be normalized directly.

4. **Multi-Meter SKU Disambiguation**:
   - Azure composite SKU IDs are formatted as `{skuId}:{meterId}` to ensure independent anomaly tracking across distinct billing meters.
   - IBM Cloud storage queries execute separate API requests for `cloud-object-storage` and `is.volume` and merge responses client-side.

## Consequences

### Positive

- **Eliminates False-Positive Job Blocks**: Ingestion jobs do not fail on unmodeled provider catalog lines.
- **Accurate Quarantine Signals**: `quarantine.Sink` captures genuine taxonomy drift (new regions, instances, tiers) that requires engineer attention.
- **GCP Compute Parity**: CloudVitta surfaces standard GCP machine types (e.g. N2, E2, C2, N1) accurately from component pricing.

### Negative

- Provider adapters must implement filtering guards to separate in-scope unknown items from out-of-scope discarded items.

## Alternatives Considered

### Alternative 1: Quarantining All Unmapped Items
Rejected. Quarantining thousands of out-of-scope lines triggers false-positive DLQ job blocks and produces excessive noise in quarantine digests.

### Alternative 2: Ignoring All Unmapped Items Without Quarantine
Rejected. Ignoring unmapped items silently conceals genuine provider taxonomy changes (such as new cloud regions or instance families).
