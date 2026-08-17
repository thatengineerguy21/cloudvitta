# 6. Upsert-with-History and Rollups Over Append-Only Postgres

Date: 2026-08-06

## Status

Accepted

## Context

CloudVitta ingests pricing data for millions of SKUs across cloud providers. An append-only architecture for the `price_observations` table in Postgres (Neon) inserts a new row on every ingestion run, even when prices do not change.

Most On-Demand and Reserved prices change infrequently. An append-only model generates redundant rows, causes rapid table and index bloat, and increases serverless storage costs. Users consume aggregated trends and current prices, not high-frequency unchanged ticks.

## Decision

We use an **Upsert-with-History and Rollups** pattern for the `price_observations` table:
1. **Upsert-with-History**: The ingestion worker only inserts a new row when it detects an actual price change for a SKU.
2. **Timestamp Bumping**: When an ingestion run fetches an unchanged price, the database query updates the `last_seen_at` timestamp on the existing active row without inserting new rows or bloating indexes.
3. **Capped Retention and Rollups**: Raw `price_observations` rows are retained for approximately two weeks. Older rows are summarized into daily aggregates (Open-High-Low-Close), and expired raw rows are purged.

## Consequences

### Positive
- Substantially reduces redundant database rows and keeps serverless PostgreSQL (Neon) storage costs low.
- Keeps database indexes small and maintains fast query times on cache misses.
- Provides a stable, steady-state database size instead of unbounded growth.
- Directly supports user requirements for current prices and historical trend charts.

### Negative
- Requires ingestion queries to use `INSERT ... ON CONFLICT DO UPDATE` rather than simple append inserts.
- Requires automated background jobs to compute daily rollups and purge raw records older than two weeks.

## Alternatives Considered

### Alternative 1: Strict Append-Only Event Log Table
Rejected. Inserting a new row on every ingestion run creates millions of identical rows for stable On-Demand SKUs, rapidly exhausts database storage limits, and slows down read queries.

### Alternative 2: In-Place Overwrite Without Price History
Rejected. Overwriting existing rows without tracking changes destroys price history, prevents anomaly detection for unexpected price jumps, and disables historical trend visualization.
