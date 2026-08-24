# 36. DigitalOcean Declared Catalog Scope and Exclusion of Unsupported Categories

Date: 2026-08-24

## Status

Accepted

## Context

DigitalOcean offers a focused product catalog centered on developer infrastructure (Droplets, Spaces object storage, Volumes block storage, Managed Databases for PostgreSQL/MySQL, Managed Kubernetes DOKS, and DO Functions serverless compute). DigitalOcean does not offer managed NoSQL database engines (such as DynamoDB, Cosmos DB, or Firestore equivalents).

Open Question #7 asks how CloudVitta handles cloud providers with focused product catalogs without fabricating synthetic data or returning silent empty matrices.

## Decision

We resolve Open Question #7 by applying the Explicit Declaration of Supported Categories rule (ADR 0026) to DigitalOcean:

1. DigitalOcean adapter declares supported categories in `internal/adapter/provider/digitalocean/supported_categories.go`:
   - `compute`
   - `storage`
   - `network`
   - `database_rdbms`
   - `kubernetes`
   - `serverless`
2. DigitalOcean intentionally omits `database_nosql` from its supported category declaration.
3. Standalone comparison queries for unsupported categories return a standardized honesty warning (`category_not_supported`) without system errors.
4. Composite workload calculations (`POST /api/v1/calculate` and `calculate_workload` MCP tool) containing unsupported categories mark the result with `partial: true`, omit the full total `total_normalized_hourly_usd`, and calculate `partial_total_normalized_hourly_usd` from supported categories only (ADR 0022).

## Consequences

### Positive

- **Data Credibility**: Eliminates synthetic placeholders and artificial zero-dollar SKUs.
- **Contract Honesty**: Conforms strictly to ADR 0022 and ADR 0026 honesty vocabulary across REST and MCP transports.
- **Extensible Registry**: Uses the category registry architecture without invasive branching in core pricing engines.

### Negative

- Frontend and API clients must inspect `warnings` arrays to identify provider catalog exclusions.

## Alternatives Considered

### Alternative 1: Fabricating Synthetic NoSQL Pricing for DigitalOcean
Rejected. Returning estimated or synthetic NoSQL prices creates misleading cost projections for users.

### Alternative 2: Returning Zero Cost for Unsupported Categories
Rejected. A zero-cost observation implies that the service is free of charge rather than unavailable.
